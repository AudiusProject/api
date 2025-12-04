package common

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"api.audius.co/proto/fumarole"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// FumaroleSubscription represents a single subscription on a consumer group
type FumaroleSubscription struct {
	consumerGroupName   string
	controlStream       grpc.BidiStreamingClient[fumarole.ControlCommand, fumarole.ControlResponse]
	blockFilters        *fumarole.BlockFilters
	dataCallback        DataCallback
	errorCallback       ErrorCallback
	lastCommittedOffset int64
	lastPollOffset      *int64
	blockchainID        []byte
	hasInternalSlotSub  bool
	cancel              context.CancelFunc
	running             bool
}

// FumaroleAdapter implements GrpcClient using the Fumarole architecture:
// - Shared gRPC connection
// - Multiple subscriptions on different consumer groups
// - Data plane (DownloadBlock RPC) for fetching block data
type FumaroleAdapter struct {
	config            GrpcConfig
	conn              *grpc.ClientConn
	client            fumarole.FumaroleClient
	mu                sync.Mutex
	subscriptions     map[string]*FumaroleSubscription
	downloadSemaphore chan struct{}
	lastSlot          uint64
	logger            *zap.Logger
	debugLogging      bool
}

var fumaroleKacp = keepalive.ClientParameters{
	Time:                30 * time.Second,
	Timeout:             10 * time.Second,
	PermitWithoutStream: true,
}

type fumaroleTokenAuth struct {
	token string
}

func (t fumaroleTokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"x-token": t.token,
	}, nil
}

func (t fumaroleTokenAuth) RequireTransportSecurity() bool {
	return true
}

// NewFumaroleAdapter creates a new shared Fumarole adapter
func NewFumaroleAdapter(config GrpcConfig, logger *zap.Logger) (*FumaroleAdapter, error) {
	adapter := &FumaroleAdapter{
		config:            config,
		subscriptions:     make(map[string]*FumaroleSubscription),
		downloadSemaphore: make(chan struct{}, 10), // Limit to 10 concurrent downloads
		logger:            logger.Named("FumaroleAdapter"),
		debugLogging:      config.DebugLogging,
	}

	if err := adapter.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return adapter, nil
}

func (c *FumaroleAdapter) connect() error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	server := c.config.Server
	if len(server) > 0 && server[0] != 'h' {
		server = "https://" + server
	}

	u, err := url.Parse(server)
	if err != nil {
		return fmt.Errorf("error parsing endpoint: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert pool: %w", err)
	}

	creds := credentials.NewClientTLSFromCert(pool, u.Hostname())

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithKeepaliveParams(fumaroleKacp))
	opts = append(opts, grpc.WithTransportCredentials(creds))
	opts = append(opts, grpc.WithPerRPCCredentials(fumaroleTokenAuth{token: c.config.ApiToken}))
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100 MB
			grpc.MaxCallSendMsgSize(100*1024*1024),
		),
	)
	opts = append(opts, grpc.WithDefaultServiceConfig(`{
		"methodConfig": [{
			"name": [{"service": "fumarole.Fumarole"}],
			"waitForReady": true,
			"retryPolicy": {
				"MaxAttempts": 4,
				"InitialBackoff": ".1s",
				"MaxBackoff": "1s",
				"BackoffMultiplier": 2.0,
				"RetryableStatusCodes": [ "UNAVAILABLE" ]
			}
		}]
	}`))

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	serverAddr := u.Hostname() + ":" + port

	conn, err := grpc.NewClient(serverAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	c.conn = conn
	c.client = fumarole.NewFumaroleClient(conn)
	return nil
}

// ensureConsumerGroup creates the consumer group if it doesn't exist
func (c *FumaroleAdapter) ensureConsumerGroup(ctx context.Context, consumerGroupName string) error {
	_, err := c.client.GetConsumerGroupInfo(ctx, &fumarole.GetConsumerGroupInfoRequest{
		ConsumerGroupName: consumerGroupName,
	})

	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		return fmt.Errorf("failed to check consumer group: %w", err)
	}

	_, err = c.client.CreateConsumerGroup(ctx, &fumarole.CreateConsumerGroupRequest{
		ConsumerGroupName:   consumerGroupName,
		InitialOffsetPolicy: fumarole.InitialOffsetPolicy_LATEST,
	})

	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	return nil
}

// Subscribe creates a new subscription on the given consumer group
func (c *FumaroleAdapter) Subscribe(
	ctx context.Context,
	subRequest *pb.SubscribeRequest,
	dataCallback DataCallback,
	errorCallback ErrorCallback,
) error {
	return c.SubscribeWithConsumerGroup(ctx, "", subRequest, dataCallback, errorCallback)
}

// SubscribeWithConsumerGroup creates a new subscription on a specific consumer group
func (c *FumaroleAdapter) SubscribeWithConsumerGroup(
	ctx context.Context,
	consumerGroupName string,
	subRequest *pb.SubscribeRequest,
	dataCallback DataCallback,
	errorCallback ErrorCallback,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if consumerGroupName == "" {
		consumerGroupName = "audius-indexer-" + time.Now().Format("20060102-150405")
	}

	// Check if this consumer group already has a subscription
	if _, exists := c.subscriptions[consumerGroupName]; exists {
		return fmt.Errorf("consumer group %s already has an active subscription", consumerGroupName)
	}

	if c.debugLogging {
		c.logger.Debug("received subscription request",
			zap.String("consumer_group", consumerGroupName),
			zap.Int("account_filters_in_request", len(subRequest.Accounts)),
			zap.Int("tx_filters_in_request", len(subRequest.Transactions)))
	}

	initialReq := proto.Clone(subRequest).(*pb.SubscribeRequest)

	// Add internal slot subscription if not present
	hasInternalSlotSub := false
	if len(initialReq.Slots) == 0 {
		hasInternalSlotSub = true
		if initialReq.Slots == nil {
			initialReq.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
		}
		initialReq.Slots["__internal_slot__"] = &pb.SubscribeRequestFilterSlots{}
	}

	if err := c.ensureConsumerGroup(ctx, consumerGroupName); err != nil {
		return fmt.Errorf("failed to ensure consumer group: %w", err)
	}

	// Create control plane stream
	controlStream, err := c.client.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to create control stream: %w", err)
	}

	// Send initial join command
	joinCmd := &fumarole.ControlCommand{
		Command: &fumarole.ControlCommand_InitialJoin{
			InitialJoin: &fumarole.JoinControlPlane{
				ConsumerGroupName: &consumerGroupName,
			},
		},
	}

	if err := controlStream.Send(joinCmd); err != nil {
		return fmt.Errorf("failed to send join command: %w", err)
	}

	// Wait for initial state
	initResp, err := controlStream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive initial state: %w", err)
	}

	blockchainID := []byte{0} // Solana mainnet
	var lastCommittedOffset int64
	var lastPollOffset *int64

	if init := initResp.GetInit(); init != nil {
		blockchainID = init.BlockchainId
		if offset, ok := init.LastCommittedOffsets[0]; ok {
			lastCommittedOffset = offset
			lastPollOffset = &offset
		}
	} else {
		return fmt.Errorf("no init response received")
	}

	// Convert filters for DownloadBlock - deep copy to avoid sharing maps between subscriptions
	blockFilters := &fumarole.BlockFilters{
		Accounts:     make(map[string]*pb.SubscribeRequestFilterAccounts),
		Transactions: make(map[string]*pb.SubscribeRequestFilterTransactions),
		Entries:      make(map[string]*pb.SubscribeRequestFilterEntry),
		BlocksMeta:   make(map[string]*pb.SubscribeRequestFilterBlocksMeta),
	}

	// Deep copy account filters
	for k, v := range initialReq.Accounts {
		blockFilters.Accounts[k] = v
	}

	// Deep copy transaction filters
	for k, v := range initialReq.Transactions {
		blockFilters.Transactions[k] = v
	}

	// Deep copy entry filters
	for k, v := range initialReq.Entry {
		blockFilters.Entries[k] = v
	}

	// Deep copy blocks meta filters
	for k, v := range initialReq.BlocksMeta {
		blockFilters.BlocksMeta[k] = v
	}

	if c.debugLogging {
		accountKeys := make([]string, 0, len(blockFilters.GetAccounts()))
		for key := range blockFilters.GetAccounts() {
			accountKeys = append(accountKeys, key)
		}
		c.logger.Debug("created subscription filters",
			zap.String("consumer_group", consumerGroupName),
			zap.Int("account_filter_count", len(blockFilters.GetAccounts())),
			zap.Strings("account_filter_keys", accountKeys),
			zap.Int("tx_filters", len(blockFilters.GetTransactions())))
	}

	subCtx, cancel := context.WithCancel(ctx)

	// Create subscription
	sub := &FumaroleSubscription{
		consumerGroupName:   consumerGroupName,
		controlStream:       controlStream,
		blockFilters:        blockFilters,
		dataCallback:        dataCallback,
		errorCallback:       errorCallback,
		lastCommittedOffset: lastCommittedOffset,
		lastPollOffset:      lastPollOffset,
		blockchainID:        blockchainID,
		hasInternalSlotSub:  hasInternalSlotSub,
		cancel:              cancel,
		running:             true,
	}

	c.subscriptions[consumerGroupName] = sub

	// Start runtime loop for this subscription
	go c.subscriptionRuntime(subCtx, sub)

	return nil
}

// subscriptionRuntime is the event loop for a single subscription
func (c *FumaroleAdapter) subscriptionRuntime(ctx context.Context, sub *FumaroleSubscription) {
	defer func() {
		sub.running = false
		if sub.controlStream != nil {
			sub.controlStream.CloseSend()
		}
		c.mu.Lock()
		delete(c.subscriptions, sub.consumerGroupName)
		c.mu.Unlock()
	}()

	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	controlChan := make(chan *fumarole.ControlResponse, 10)

	// Start goroutine to receive control messages
	go func() {
		for {
			stream := sub.controlStream
			if stream == nil {
				return
			}

			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF || status.Code(err) == codes.Canceled {
					return
				}
				if sub.errorCallback != nil {
					sub.errorCallback(fmt.Errorf("control plane error: %w", err))
				}
				return
			}

			select {
			case controlChan <- resp:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Initial history poll
	c.pollHistory(sub)

	pingID := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return

		case <-pingTicker.C:
			// Send ping
			stream := sub.controlStream
			if stream == nil {
				return
			}

			pingID++
			if err := stream.Send(&fumarole.ControlCommand{
				Command: &fumarole.ControlCommand_Ping{
					Ping: &fumarole.Ping{PingId: pingID},
				},
			}); err != nil {
				if sub.errorCallback != nil {
					sub.errorCallback(fmt.Errorf("ping failed: %w", err))
				}
			}

		case resp := <-controlChan:
			c.handleControlResponse(ctx, sub, resp)
		}
	}
}

// handleControlResponse processes control plane responses for a subscription
func (c *FumaroleAdapter) handleControlResponse(ctx context.Context, sub *FumaroleSubscription, resp *fumarole.ControlResponse) {
	if history := resp.GetPollHist(); history != nil {
		// Process blockchain events and download blocks asynchronously
		var lastOffset int64
		var lastShardId int32
		for _, event := range history.Events {
			if event.DeadError != nil {
				continue
			}

			// Track the last offset
			lastOffset = event.Offset
			lastShardId = event.BlockchainShardId

			// Update offset
			sub.lastPollOffset = &event.Offset

			// Download block asynchronously
			go c.downloadBlock(ctx, sub, event)
		}

		// Commit the last processed offset
		if len(history.Events) > 0 {
			c.commitOffset(sub, lastShardId, lastOffset)
		}

		// Poll for next batch
		c.pollHistory(sub)
	} else if commit := resp.GetCommitOffset(); commit != nil {
		sub.lastCommittedOffset = commit.Offset
	}
}

// downloadBlock downloads a single block using DownloadBlock RPC
func (c *FumaroleAdapter) downloadBlock(ctx context.Context, sub *FumaroleSubscription, event *fumarole.BlockchainEvent) {
	// Acquire semaphore slot
	select {
	case c.downloadSemaphore <- struct{}{}:
		defer func() { <-c.downloadSemaphore }()
	case <-ctx.Done():
		return
	}

	client := c.client
	filters := sub.blockFilters
	callback := sub.dataCallback

	downloadStream, err := client.DownloadBlock(ctx, &fumarole.DownloadBlockShard{
		BlockchainId: event.BlockchainId,
		BlockUid:     event.BlockUid,
		ShardIdx:     event.BlockchainShardId,
		BlockFilters: filters,
	})

	if err != nil {
		if sub.errorCallback != nil {
			sub.errorCallback(fmt.Errorf("download block failed slot %d: %w", event.Slot, err))
		}
		c.logger.Error("DownloadBlock RPC failed",
			zap.Uint64("slot", event.Slot),
			zap.Error(err))
		return
	}

	updateCount := 0

	for {
		dataResp, err := downloadStream.Recv()
		if err == io.EOF {
			if c.debugLogging {
				c.logger.Debug("finished downloading slot",
					zap.Uint64("slot", event.Slot),
					zap.Int("updates", updateCount))
			}
			break
		}
		if err != nil {
			if sub.errorCallback != nil {
				sub.errorCallback(fmt.Errorf("download stream error slot %d: %w", event.Slot, err))
			}
			c.logger.Error("download stream error",
				zap.Uint64("slot", event.Slot),
				zap.Int("updates", updateCount),
				zap.Error(err))
			break
		}

		if dataResp.GetBlockShardDownloadFinish() != nil {

			break
		}

		if update := dataResp.GetUpdate(); update != nil {
			updateCount++
			suppressCallback := false

			if slotUpdate, ok := update.UpdateOneof.(*pb.SubscribeUpdate_Slot); ok {
				if slotUpdate.Slot != nil && slotUpdate.Slot.Slot > 0 {
					c.mu.Lock()
					c.lastSlot = slotUpdate.Slot.Slot
					c.mu.Unlock()
				}
				if sub.hasInternalSlotSub {
					suppressCallback = true
				}
			}

			if callback != nil && !suppressCallback {
				callback(ctx, update)
			}
		}
	}
}

// commitOffset sends a commit offset command to acknowledge processed blocks
func (c *FumaroleAdapter) commitOffset(sub *FumaroleSubscription, shardId int32, offset int64) {
	stream := sub.controlStream
	if stream == nil {
		return
	}

	if err := stream.Send(&fumarole.ControlCommand{
		Command: &fumarole.ControlCommand_CommitOffset{
			CommitOffset: &fumarole.CommitOffset{
				ShardId: shardId,
				Offset:  offset,
			},
		},
	}); err != nil {
		if sub.errorCallback != nil {
			sub.errorCallback(fmt.Errorf("commit offset failed: %w", err))
		}
	}
}

// pollHistory sends a poll history command
func (c *FumaroleAdapter) pollHistory(sub *FumaroleSubscription) {
	stream := sub.controlStream
	from := sub.lastPollOffset

	if stream == nil {
		return
	}

	if err := stream.Send(&fumarole.ControlCommand{
		Command: &fumarole.ControlCommand_PollHist{
			PollHist: &fumarole.PollBlockchainHistory{
				ShardId: 0,
				From:    from,
				Limit:   proto.Int64(100),
			},
		},
	}); err != nil {
		if sub.errorCallback != nil {
			sub.errorCallback(fmt.Errorf("poll history failed: %w", err))
		}
	}
}

func (c *FumaroleAdapter) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close all subscriptions
	for _, sub := range c.subscriptions {
		if sub.cancel != nil {
			sub.cancel()
		}
		if sub.controlStream != nil {
			sub.controlStream.CloseSend()
		}
	}
	c.subscriptions = make(map[string]*FumaroleSubscription)

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}
