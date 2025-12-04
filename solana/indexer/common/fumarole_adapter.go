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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// FumaroleAdapter implements GrpcClient using the Fumarole architecture:
// - Control plane (Subscribe RPC) for blockchain history polling
// - Data plane (DownloadBlock RPC) for fetching block data
type FumaroleAdapter struct {
	config              GrpcConfig
	conn                *grpc.ClientConn
	client              fumarole.FumaroleClient
	mu                  sync.Mutex
	controlStream       grpc.BidiStreamingClient[fumarole.ControlCommand, fumarole.ControlResponse]
	running             bool
	subRequest          *pb.SubscribeRequest
	lastSlot            uint64
	dataCallback        DataCallback
	errorCallback       ErrorCallback
	cancel              context.CancelFunc
	consumerGroupName   string
	blockchainID        []byte
	hasInternalSlotSub  bool
	blockFilters        *fumarole.BlockFilters
	lastCommittedOffset int64
	lastPollOffset      *int64
	downloadSemaphore   chan struct{}
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

// NewFumaroleAdapter creates a new Fumarole adapter
func NewFumaroleAdapter(config GrpcConfig, consumerGroupName string) GrpcClient {
	if consumerGroupName == "" {
		consumerGroupName = "audius-indexer-" + time.Now().Format("20060102-150405")
	}
	return &FumaroleAdapter{
		config:            config,
		consumerGroupName: consumerGroupName,
		blockchainID:      []byte{0},               // Solana mainnet
		downloadSemaphore: make(chan struct{}, 10), // Limit to 10 concurrent downloads
	}
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
func (c *FumaroleAdapter) ensureConsumerGroup(ctx context.Context) error {
	_, err := c.client.GetConsumerGroupInfo(ctx, &fumarole.GetConsumerGroupInfoRequest{
		ConsumerGroupName: c.consumerGroupName,
	})

	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		return fmt.Errorf("failed to check consumer group: %w", err)
	}

	_, err = c.client.CreateConsumerGroup(ctx, &fumarole.CreateConsumerGroupRequest{
		ConsumerGroupName:   c.consumerGroupName,
		InitialOffsetPolicy: fumarole.InitialOffsetPolicy_LATEST,
	})

	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	return nil
}

// Subscribe implements dragonsMouth-like subscription using Fumarole
func (c *FumaroleAdapter) Subscribe(
	ctx context.Context,
	subRequest *pb.SubscribeRequest,
	dataCallback DataCallback,
	errorCallback ErrorCallback,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("client is already subscribed")
	}

	initialReq := proto.Clone(subRequest).(*pb.SubscribeRequest)

	// Add internal slot subscription if not present
	if len(initialReq.Slots) == 0 {
		c.hasInternalSlotSub = true
		if initialReq.Slots == nil {
			initialReq.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
		}
		initialReq.Slots["__internal_slot__"] = &pb.SubscribeRequestFilterSlots{}
	} else {
		c.hasInternalSlotSub = false
	}

	c.subRequest = initialReq
	c.dataCallback = dataCallback
	c.errorCallback = errorCallback

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	if c.conn == nil {
		if err := c.connect(); err != nil {
			cancel()
			return fmt.Errorf("failed to connect: %w", err)
		}
	}

	if err := c.ensureConsumerGroup(ctx); err != nil {
		cancel()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to ensure consumer group: %w", err)
	}

	// Create control plane stream
	controlStream, err := c.client.Subscribe(ctx)
	if err != nil {
		cancel()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to create control stream: %w", err)
	}
	c.controlStream = controlStream

	// Send initial join command
	consumerGroupName := c.consumerGroupName
	joinCmd := &fumarole.ControlCommand{
		Command: &fumarole.ControlCommand_InitialJoin{
			InitialJoin: &fumarole.JoinControlPlane{
				ConsumerGroupName: &consumerGroupName,
			},
		},
	}

	if err := controlStream.Send(joinCmd); err != nil {
		cancel()
		c.controlStream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to send join command: %w", err)
	}

	// Wait for initial state
	initResp, err := controlStream.Recv()
	if err != nil {
		cancel()
		c.controlStream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to receive initial state: %w", err)
	}

	if init := initResp.GetInit(); init != nil {
		c.blockchainID = init.BlockchainId
		if offset, ok := init.LastCommittedOffsets[0]; ok {
			c.lastCommittedOffset = offset
			c.lastPollOffset = &offset
		}
	} else {
		return fmt.Errorf("no init response received")
	}

	// Convert filters for DownloadBlock
	c.blockFilters = &fumarole.BlockFilters{
		Accounts:     initialReq.Accounts,
		Transactions: initialReq.Transactions,
		Entries:      initialReq.Entry,
		BlocksMeta:   initialReq.BlocksMeta,
	}

	c.running = true

	// Start runtime loop
	go c.fumaroleRuntime(ctx)

	return nil
}

// fumaroleRuntime is the main event loop matching TokioFumeDragonsmouthRuntime::run()
func (c *FumaroleAdapter) fumaroleRuntime(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.running = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		if c.controlStream != nil {
			c.controlStream.CloseSend()
			c.controlStream = nil
		}
		c.mu.Unlock()
	}()

	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	// Channel for control responses
	controlChan := make(chan *fumarole.ControlResponse, 10)

	// Start goroutine to receive control messages
	go func() {
		for {
			c.mu.Lock()
			stream := c.controlStream
			c.mu.Unlock()

			if stream == nil {
				return
			}

			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF || status.Code(err) == codes.Canceled {
					return
				}
				c.mu.Lock()
				if c.errorCallback != nil {
					c.errorCallback(fmt.Errorf("control plane error: %w", err))
				}
				c.mu.Unlock()
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
	c.pollHistory()

	pingID := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return

		case <-pingTicker.C:
			// Send ping
			c.mu.Lock()
			stream := c.controlStream
			c.mu.Unlock()

			if stream == nil {
				return
			}

			pingID++
			if err := stream.Send(&fumarole.ControlCommand{
				Command: &fumarole.ControlCommand_Ping{
					Ping: &fumarole.Ping{PingId: pingID},
				},
			}); err != nil {
				c.mu.Lock()
				if c.errorCallback != nil {
					c.errorCallback(fmt.Errorf("ping failed: %w", err))
				}
				c.mu.Unlock()
			}

		case resp := <-controlChan:
			c.handleControlResponse(ctx, resp)
		}
	}
}

// handleControlResponse processes control plane responses
func (c *FumaroleAdapter) handleControlResponse(ctx context.Context, resp *fumarole.ControlResponse) {
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
			c.mu.Lock()
			c.lastPollOffset = &event.Offset
			c.mu.Unlock()

			// Download block asynchronously
			go c.downloadBlock(ctx, event)
		}

		// Commit the last processed offset
		if len(history.Events) > 0 {
			c.commitOffset(lastShardId, lastOffset)
		}

		// Poll for next batch
		c.pollHistory()
	} else if commit := resp.GetCommitOffset(); commit != nil {
		c.mu.Lock()
		c.lastCommittedOffset = commit.Offset
		c.mu.Unlock()
	}
}

// downloadBlock downloads a single block using DownloadBlock RPC
func (c *FumaroleAdapter) downloadBlock(ctx context.Context, event *fumarole.BlockchainEvent) {
	// Acquire semaphore slot
	select {
	case c.downloadSemaphore <- struct{}{}:
		defer func() { <-c.downloadSemaphore }()
	case <-ctx.Done():
		return
	}

	c.mu.Lock()
	client := c.client
	filters := c.blockFilters
	callback := c.dataCallback
	c.mu.Unlock()

	downloadStream, err := client.DownloadBlock(ctx, &fumarole.DownloadBlockShard{
		BlockchainId: event.BlockchainId,
		BlockUid:     event.BlockUid,
		ShardIdx:     event.BlockchainShardId,
		BlockFilters: filters,
	})

	if err != nil {
		c.mu.Lock()
		if c.errorCallback != nil {
			c.errorCallback(fmt.Errorf("download block failed slot %d: %w", event.Slot, err))
		}
		c.mu.Unlock()
		return
	}

	for {
		dataResp, err := downloadStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.mu.Lock()
			if c.errorCallback != nil {
				c.errorCallback(fmt.Errorf("download stream error slot %d: %w", event.Slot, err))
			}
			c.mu.Unlock()
			break
		}

		if dataResp.GetBlockShardDownloadFinish() != nil {
			break
		}

		if update := dataResp.GetUpdate(); update != nil {
			suppressCallback := false

			if slotUpdate, ok := update.UpdateOneof.(*pb.SubscribeUpdate_Slot); ok {
				if slotUpdate.Slot != nil && slotUpdate.Slot.Slot > 0 {
					c.mu.Lock()
					c.lastSlot = slotUpdate.Slot.Slot
					c.mu.Unlock()
				}
				if c.hasInternalSlotSub {
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
func (c *FumaroleAdapter) commitOffset(shardId int32, offset int64) {
	c.mu.Lock()
	stream := c.controlStream
	c.mu.Unlock()

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
		c.mu.Lock()
		if c.errorCallback != nil {
			c.errorCallback(fmt.Errorf("commit offset failed: %w", err))
		}
		c.mu.Unlock()
	}
}

// pollHistory sends a poll history command
func (c *FumaroleAdapter) pollHistory() {
	c.mu.Lock()
	stream := c.controlStream
	from := c.lastPollOffset
	c.mu.Unlock()

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
		c.mu.Lock()
		if c.errorCallback != nil {
			c.errorCallback(fmt.Errorf("poll history failed: %w", err))
		}
		c.mu.Unlock()
	}
}

func (c *FumaroleAdapter) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.controlStream != nil {
		c.controlStream.CloseSend()
		c.controlStream = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.running = false
}
