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

// FumaroleAdapter adapts the Fumarole Subscribe API to work like dragonsMouth
// It translates Geyser SubscribeRequest to Fumarole's control plane + data plane pattern
type FumaroleAdapter struct {
	config             GrpcConfig
	conn               *grpc.ClientConn
	mu                 sync.Mutex
	controlStream      grpc.ClientStream
	dataStream         grpc.ClientStream
	running            bool
	subRequest         *pb.SubscribeRequest
	lastSlot           uint64
	dataCallback       DataCallback
	errorCallback      ErrorCallback
	cancel             context.CancelFunc
	consumerGroupName  string
	blockchainID       []byte
	shardOffsets       map[int32]int64
	hasInternalSlotSub bool
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

// NewFumaroleAdapter creates a new Fumarole adapter that mimics dragonsMouth behavior
func NewFumaroleAdapter(config GrpcConfig, consumerGroupName string) GrpcClient {
	if consumerGroupName == "" {
		consumerGroupName = "audius-indexer-" + time.Now().Format("20060102-150405")
	}
	return &FumaroleAdapter{
		config:            config,
		consumerGroupName: consumerGroupName,
		shardOffsets:      make(map[int32]int64),
		blockchainID:      []byte{0}, // Solana mainnet blockchain ID
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
	return nil
}

// Subscribe implements dragonsMouth-like subscription using Fumarole's Subscribe (control plane)
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

	// Add internal slot subscription if not present (for tracking)
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

	// Create control plane stream (Subscribe RPC)
	controlStream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "Subscribe",
		Handler:       nil,
		ServerStreams: true,
		ClientStreams: true,
	}, "/fumarole.Fumarole/Subscribe")

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

	if err := controlStream.SendMsg(joinCmd); err != nil {
		cancel()
		c.controlStream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to send join command: %w", err)
	}

	// Wait for initial state
	var initResp fumarole.ControlResponse
	if err := controlStream.RecvMsg(&initResp); err != nil {
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
		c.shardOffsets = init.LastCommittedOffsets
	}

	// Create data plane stream (SubscribeData RPC)
	dataStream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "SubscribeData",
		Handler:       nil,
		ServerStreams: true,
		ClientStreams: true,
	}, "/fumarole.Fumarole/SubscribeData")

	if err != nil {
		cancel()
		c.controlStream.CloseSend()
		c.controlStream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to create data stream: %w", err)
	}
	c.dataStream = dataStream

	// Convert Geyser filters to Fumarole BlockFilters and send
	blockFilters := c.convertToBlockFilters(initialReq)
	dataCmd := &fumarole.DataCommand{
		Command: &fumarole.DataCommand_FilterUpdate{
			FilterUpdate: blockFilters,
		},
	}

	if err := dataStream.SendMsg(dataCmd); err != nil {
		cancel()
		c.dataStream = nil
		c.controlStream.CloseSend()
		c.controlStream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to send filter update: %w", err)
	}

	c.running = true

	// Start loops for both streams
	go c.controlLoop(ctx)
	go c.dataLoop(ctx)

	return nil
}

// convertToBlockFilters converts Geyser SubscribeRequest to Fumarole BlockFilters
func (c *FumaroleAdapter) convertToBlockFilters(req *pb.SubscribeRequest) *fumarole.BlockFilters {
	return &fumarole.BlockFilters{
		Accounts:     req.Accounts,
		Transactions: req.Transactions,
		Entries:      req.Entry,
		BlocksMeta:   req.BlocksMeta,
	}
}

// controlLoop handles the control plane stream (for heartbeat, offset commits, etc.)
func (c *FumaroleAdapter) controlLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	pingID := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send periodic ping
			c.mu.Lock()
			stream := c.controlStream
			c.mu.Unlock()

			if stream == nil {
				return
			}

			pingID++
			pingCmd := &fumarole.ControlCommand{
				Command: &fumarole.ControlCommand_Ping{
					Ping: &fumarole.Ping{PingId: pingID},
				},
			}

			if err := stream.SendMsg(pingCmd); err != nil {
				c.mu.Lock()
				if c.errorCallback != nil {
					c.errorCallback(fmt.Errorf("control plane ping failed: %w", err))
				}
				c.mu.Unlock()
			}
		}
	}
}

// dataLoop handles the data plane stream (receiving SubscribeUpdate messages)
func (c *FumaroleAdapter) dataLoop(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.running = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		if c.dataStream != nil {
			c.dataStream.CloseSend()
			c.dataStream = nil
		}
		if c.controlStream != nil {
			c.controlStream.CloseSend()
			c.controlStream = nil
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.mu.Lock()
		stream := c.dataStream
		c.mu.Unlock()

		if stream == nil {
			if !c.attemptReconnect(ctx) {
				return
			}
			continue
		}

		var resp fumarole.DataResponse
		err := stream.RecvMsg(&resp)

		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}

			c.mu.Lock()
			if c.errorCallback != nil {
				c.errorCallback(err)
			}
			c.dataStream = nil
			c.mu.Unlock()

			st, ok := status.FromError(err)

			// Handle rate limiting (429)
			if ok && st.Code() == codes.Unavailable {
				msg := st.Message()
				if containsSubstring(msg, "429") || containsSubstring(msg, "Too Many Requests") {
					select {
					case <-time.After(60 * time.Second):
						if !c.attemptReconnect(ctx) {
							return
						}
					case <-ctx.Done():
						return
					}
					continue
				}
			}

			if (ok && (st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded)) || err == io.EOF {
				if !c.attemptReconnect(ctx) {
					return
				}
			} else {
				return
			}
		} else {
			// Extract SubscribeUpdate from DataResponse
			update := resp.GetUpdate()
			if update == nil {
				continue
			}

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

			c.mu.Lock()
			callback := c.dataCallback
			c.mu.Unlock()

			if callback != nil && !suppressCallback {
				callback(ctx, update)
			}
		}
	}
}

func (c *FumaroleAdapter) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.dataStream != nil {
		c.dataStream.CloseSend()
		c.dataStream = nil
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

func (c *FumaroleAdapter) attemptReconnect(ctx context.Context) bool {
	const reconnectInterval = 30 * time.Second
	const maxReconnectWindow = 20 * time.Minute
	maxPossibleAttempts := int(maxReconnectWindow / reconnectInterval)

	c.mu.Lock()
	userRequestedAttempts := c.config.MaxReconnectAttempts
	c.mu.Unlock()

	attemptsToMake := userRequestedAttempts
	if attemptsToMake <= 0 {
		attemptsToMake = maxPossibleAttempts
	} else if attemptsToMake > maxPossibleAttempts {
		attemptsToMake = maxPossibleAttempts
	}

	for currentAttempt := 1; currentAttempt <= attemptsToMake; currentAttempt++ {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(reconnectInterval):
			c.mu.Lock()
			if ctx.Err() != nil {
				c.mu.Unlock()
				return false
			}
			subReq := c.subRequest
			errorCb := c.errorCallback
			c.mu.Unlock()

			if err := c.connect(); err != nil {
				if errorCb != nil {
					errorCb(fmt.Errorf("reconnect failed on attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}

			// Recreate control stream
			c.mu.Lock()
			controlStream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{
				StreamName:    "Subscribe",
				Handler:       nil,
				ServerStreams: true,
				ClientStreams: true,
			}, "/fumarole.Fumarole/Subscribe")

			if err != nil {
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("failed to re-create control stream on attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}
			c.controlStream = controlStream

			// Rejoin control plane
			consumerGroupName := c.consumerGroupName
			joinCmd := &fumarole.ControlCommand{
				Command: &fumarole.ControlCommand_InitialJoin{
					InitialJoin: &fumarole.JoinControlPlane{
						ConsumerGroupName: &consumerGroupName,
					},
				},
			}

			if err := controlStream.SendMsg(joinCmd); err != nil {
				controlStream.CloseSend()
				c.controlStream = nil
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("failed to rejoin control plane on attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}

			// Wait for initial state
			var initResp fumarole.ControlResponse
			if err := controlStream.RecvMsg(&initResp); err != nil {
				controlStream.CloseSend()
				c.controlStream = nil
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("failed to receive initial state on reconnect attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}

			// Recreate data stream
			dataStream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{
				StreamName:    "SubscribeData",
				Handler:       nil,
				ServerStreams: true,
				ClientStreams: true,
			}, "/fumarole.Fumarole/SubscribeData")

			if err != nil {
				controlStream.CloseSend()
				c.controlStream = nil
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("failed to re-create data stream on attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}
			c.dataStream = dataStream

			// Resend filters
			blockFilters := c.convertToBlockFilters(subReq)
			dataCmd := &fumarole.DataCommand{
				Command: &fumarole.DataCommand_FilterUpdate{
					FilterUpdate: blockFilters,
				},
			}

			if err := dataStream.SendMsg(dataCmd); err != nil {
				dataStream.CloseSend()
				c.dataStream = nil
				controlStream.CloseSend()
				c.controlStream = nil
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("failed to resend filters on attempt %d/%d: %w", currentAttempt, attemptsToMake, err))
				}
				continue
			}

			c.mu.Unlock()
			return true
		}
	}

	c.mu.Lock()
	errorCb := c.errorCallback
	c.mu.Unlock()

	if errorCb != nil {
		errorCb(fmt.Errorf("failed to reconnect after %d attempts", attemptsToMake))
	}
	return false
}

func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
