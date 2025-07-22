package indexer

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"sync"
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var kacp = keepalive.ClientParameters{
	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
	PermitWithoutStream: true,             // send pings even without active streams
}

// DataCallback defines the function signature for handling received data.
type DataCallback func(ctx context.Context, data *pb.SubscribeUpdate)

// ErrorCallback defines the function signature for handling errors.
type ErrorCallback func(err error)

type GrpcClient struct {
	config             GrpcConfig
	conn               *grpc.ClientConn
	mu                 sync.Mutex
	stream             pb.Geyser_SubscribeClient
	running            bool
	subRequest         *pb.SubscribeRequest
	lastSlot           uint64
	dataCallback       DataCallback
	errorCallback      ErrorCallback
	cancel             context.CancelFunc
	hasInternalSlotSub bool
}

type GrpcConfig struct {
	Server               string
	ApiToken             string
	MaxReconnectAttempts int
}

// Creates a new gRPC client.
func NewGrpcClient(config GrpcConfig) *GrpcClient {
	return &GrpcClient{
		config: config,
	}
}

// Connect to a gRPC server
// Assumes the caller holds the client mutex (c.mu).
func (c *GrpcClient) connect() error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	u, err := url.Parse(c.config.Server)
	if err != nil {
		return fmt.Errorf("error parsing endpoint: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	creds := credentials.NewClientTLSFromCert(pool, "")

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithKeepaliveParams(kacp))
	opts = append(opts, grpc.WithTransportCredentials(creds))

	server := u.Hostname() + ":443" // Hardcoding this for simplicity, assuming HTTPS

	conn, err := grpc.NewClient(server, opts...)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Subscribes to listen for Geyser gRPC events, calling the dataCallback
// when a message comes through, and errorCallback if errors occur.
func (c *GrpcClient) Subscribe(
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

	// Add internal slot subscription if user didn't provide one
	// Keeps track of the last slot received so that on reconnects we can replay from the last known slot.
	if len(initialReq.Slots) == 0 {
		c.hasInternalSlotSub = true
		internalSlotSubID := fmt.Sprintf("internal_slot_%d", rand.Intn(1000000))
		if initialReq.Slots == nil {
			initialReq.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
		}
		initialReq.Slots[internalSlotSubID] = &pb.SubscribeRequestFilterSlots{}
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

	geyserClient := pb.NewGeyserClient(c.conn)

	md := metadata.New(map[string]string{"x-token": c.config.ApiToken})
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := geyserClient.Subscribe(ctx)
	if err != nil {
		cancel()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to create subscribe stream: %w", err)
	}
	c.stream = stream

	if err := c.stream.Send(c.subRequest); err != nil {
		cancel()
		c.stream.CloseSend()
		c.stream = nil
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		return fmt.Errorf("failed to send subscription request: %w", err)
	}

	c.running = true
	go c.receiveLoop(ctx)
	return nil
}

// Close terminates the subscription and closes the connection.
func (c *GrpcClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.stream != nil {
		c.stream.CloseSend()
		c.stream = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.running = false
}

// receiveLoop runs in a goroutine, receiving messages and handling reconnects.
func (c *GrpcClient) receiveLoop(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.running = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		if c.stream != nil {
			c.stream.CloseSend()
			c.stream = nil
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
		stream := c.stream
		c.mu.Unlock()

		if stream == nil {
			if !c.attemptReconnect(ctx) {
				return
			}
			continue
		}

		resp, err := stream.Recv()

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
			c.stream = nil
			c.mu.Unlock()

			st, ok := status.FromError(err)
			if (ok && (st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded)) || err == io.EOF {
				if !c.attemptReconnect(ctx) {
					return
				}
			} else {
				return // Non-recoverable error
			}
		} else {
			suppressCallback := false

			if slotUpdate, ok := resp.UpdateOneof.(*pb.SubscribeUpdate_Slot); ok {
				if slotUpdate.Slot != nil {
					newSlot := slotUpdate.Slot.Slot
					if newSlot > 0 {
						c.mu.Lock()
						c.lastSlot = newSlot
						c.mu.Unlock()
					}
				}
				if c.hasInternalSlotSub {
					suppressCallback = true
				}
			}

			c.mu.Lock()
			callback := c.dataCallback
			c.mu.Unlock()
			if callback != nil && !suppressCallback {
				callback(ctx, resp)
			}
		}
	}
}

// attemptReconnect handles the logic for reconnecting.
func (c *GrpcClient) attemptReconnect(ctx context.Context) bool {
	const reconnectInterval = 5 * time.Second
	const maxReconnectWindow = 20 * time.Minute
	maxPossibleAttempts := int(maxReconnectWindow / reconnectInterval)
	if maxPossibleAttempts < 1 {
		maxPossibleAttempts = 1
	}

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
		default:
		}

		select {
		case <-time.After(reconnectInterval):
			c.mu.Lock()
			if ctx.Err() != nil {
				c.mu.Unlock()
				return false
			}
			apiKey := c.config.ApiToken
			subReq := c.subRequest // Use the initial request (potentially with internal slot sub)
			lastKnownSlot := c.lastSlot
			errorCb := c.errorCallback
			c.mu.Unlock()

			if err := c.connect(); err != nil {
				errMsg := fmt.Sprintf("Reconnect connection failed on attempt %d/%d: %v", currentAttempt, attemptsToMake, err)
				if errorCb != nil {
					errorCb(fmt.Errorf("%s", errMsg))
				}
				continue
			}

			c.mu.Lock()
			geyserClient := pb.NewGeyserClient(c.conn)
			streamCtx := ctx
			if apiKey != "" { // Check if API key exists before adding metadata
				md := metadata.New(map[string]string{"x-token": apiKey})
				streamCtx = metadata.NewOutgoingContext(streamCtx, md)
			}
			errorCb = c.errorCallback

			resubReq := proto.Clone(subReq).(*pb.SubscribeRequest)

			// Set FromSlot for replay if we have a last known slot
			if lastKnownSlot > 0 {
				// Using the FromSlot field (field 11) provided in the proto definition
				resubReq.FromSlot = &lastKnownSlot
			} else {
				// If no slot tracked yet, ensure FromSlot is nil (which proto.Clone should handle)
				resubReq.FromSlot = nil
			}

			// Note: The internal slot subscription (if added) remains in the resubReq.
			// This ensures slot tracking continues even if the user only subscribed
			// to non-slot types initially. It becomes slightly redundant for replay
			// once FromSlot is active, but guarantees tracking.

			stream, err := geyserClient.Subscribe(streamCtx)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to re-create stream on attempt %d/%d: %v", currentAttempt, attemptsToMake, err)
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("%s", errMsg))
				}
				continue
			}

			if err := stream.Send(resubReq); err != nil {
				errMsg := fmt.Sprintf("Failed to re-send subscription request on attempt %d/%d: %v", currentAttempt, attemptsToMake, err)
				stream.CloseSend()
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				if errorCb != nil {
					errorCb(fmt.Errorf("%s", errMsg))
				}
				continue
			}

			c.stream = stream
			c.mu.Unlock()
			return true // Reconnect successful

		case <-ctx.Done():
			return false
		}
	}

	maxAttemptsErr := fmt.Errorf("failed to reconnect after %d attempts", attemptsToMake)
	c.mu.Lock()
	errorCb := c.errorCallback
	c.mu.Unlock()
	if errorCb != nil {
		errorCb(maxAttemptsErr)
	}
	return false
}
