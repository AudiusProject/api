package comms

import (
	"encoding/json"
	"sync"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/contrib/websocket"
	"go.uber.org/zap"
)

const (
	sendQueueSize      = 128 // per-connection limit
	pingInterval       = 30 * time.Second
	readIdleTimeout    = 60 * time.Second
	writeDeadline      = 10 * time.Second // Timeout for pushing a message to a client
	recentTTL          = 10 * time.Second
	maxIncomingMsgSize = 1 << 20 // 1MB limit to incoming messages
)

type CommsWebsocketManager struct {
	mu             sync.RWMutex
	clients        map[int32]map[*Client]struct{} // userId -> set of clients (could be connected from multiple devices)
	recentMessages []*recentMessage
	logger         *zap.Logger
}

type Client struct {
	userId int32
	conn   *websocket.Conn
	send   chan []byte
	quit   chan struct{}

	manager *CommsWebsocketManager
}

type recentMessage struct {
	userId  int32
	sentAt  time.Time
	payload []byte
}

func NewCommsWebsocketManager(logger *zap.Logger) *CommsWebsocketManager {
	return &CommsWebsocketManager{
		clients:        make(map[int32]map[*Client]struct{}),
		recentMessages: []*recentMessage{},
		logger:         logger,
	}
}

// RegisterWebsocket wires up a long-lived read/write loop.
// Do NOT write directly to conn here; only the write pump writes.
func (m *CommsWebsocketManager) RegisterWebsocket(userId int32, conn *websocket.Conn) {
	cl := &Client{
		userId:  userId,
		conn:    conn,
		send:    make(chan []byte, sendQueueSize),
		quit:    make(chan struct{}),
		manager: m,
	}

	// Add to manager
	m.mu.Lock()
	if m.clients[userId] == nil {
		m.clients[userId] = make(map[*Client]struct{})
	}
	m.clients[userId][cl] = struct{}{}
	m.mu.Unlock()

	// Replay very recent messages for this user by enqueuing them
	now := time.Now()
	m.mu.RLock()
	for _, r := range m.recentMessages {
		if r.userId == userId && now.Sub(r.sentAt) < recentTTL {
			select {
			case cl.send <- r.payload:
			default:
				// If they connect with a full buffer immediately, just drop replay.
				m.logger.Info("ws replay dropped due to full buffer", zap.Int32("userId", userId))
			}
		}
	}
	m.mu.RUnlock()

	// Start pumps and block so the connection is not closed
	done := make(chan struct{})
	go func() {
		cl.readPump()
		close(done)
	}()
	go cl.writePump()
	<-done
}

func (m *CommsWebsocketManager) removeClient(cl *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.clients[cl.userId]
	if set != nil {
		if _, ok := set[cl]; ok {
			delete(set, cl)
			if len(set) == 0 {
				delete(m.clients, cl.userId)
			}
		}
	}
	// Close underlying connection and channels
	_ = cl.conn.Close()
	select {
	case <-cl.quit:
		// already closed
	default:
		close(cl.quit)
	}
}

func (cl *Client) readPump() {
	// Keep the connection alive by consuming control/data frames and handling pongs.
	cl.conn.SetReadLimit(maxIncomingMsgSize)
	_ = cl.conn.SetReadDeadline(time.Now().Add(readIdleTimeout))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(readIdleTimeout))
	})

	// We don't expect app-level inbound messages, but we still need to read to
	// receive pings/closes and keep deadlines fresh.
	for {
		mt, r, err := cl.conn.NextReader()
		if err != nil {
			cl.manager.logger.Debug("ws read closed",
				zap.Int32("userId", cl.userId),
				zap.Error(err))
			cl.manager.removeClient(cl)
			return
		}
		// Discard payload if any; we only care about keeping the socket healthy.
		switch mt {
		case websocket.TextMessage, websocket.BinaryMessage:
			// Efficiently drain without allocating:
			buf := make([]byte, 1024)
			for {
				_, derr := r.Read(buf)
				if derr != nil {
					break
				}
			}
		}
	}
}

func (cl *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				// Manager closed channel; send a close frame if possible.
				_ = cl.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				_ = cl.conn.WriteMessage(websocket.CloseMessage, nil)
				cl.manager.removeClient(cl)
				return
			}
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				cl.manager.logger.Info("ws write error",
					zap.Int32("userId", cl.userId),
					zap.Error(err))
				cl.manager.removeClient(cl)
				return
			}

		case <-ticker.C:
			// Keep-alive ping
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				cl.manager.logger.Debug("ws ping failed",
					zap.Int32("userId", cl.userId),
					zap.Error(err))
				cl.manager.removeClient(cl)
				return
			}

		case <-cl.quit:
			// Manager requested shutdown
			return
		}
	}
}

// Push to a single receiver (all connected clients)
func (m *CommsWebsocketManager) WebsocketPush(senderUserId int32, receiverUserId int32, rpcJson json.RawMessage, timestamp time.Time) {
	encodedSenderUserId, _ := trashid.EncodeHashId(int(senderUserId))
	encodedReceiverUserId, _ := trashid.EncodeHashId(int(receiverUserId))

	data := struct {
		RPC      json.RawMessage `json:"rpc"`
		Metadata Metadata        `json:"metadata"`
	}{
		rpcJson,
		Metadata{
			Timestamp:      timestamp.Format(time.RFC3339Nano),
			SenderUserID:   encodedSenderUserId,
			ReceiverUserID: encodedReceiverUserId,
			UserID:         encodedSenderUserId,
		},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		m.logger.Warn("invalid websocket json " + err.Error())
		return
	}

	// Fanout
	m.mu.RLock()
	targets := m.clients[receiverUserId]
	m.mu.RUnlock()

	for cl := range targets {
		select {
		case cl.send <- payload:
			// ok
		default:
			// If we get here, the client buffer is full (too slow in processing)
			// and we will drop them for now. They can re-connect if needed.
			m.logger.Info("ws buffer full; dropping client",
				zap.Int32("userId", receiverUserId))
			m.removeClient(cl)
		}
	}

	// Clear expired messages out of the recentMessages buffer
	m.mu.Lock()
	now := time.Now()
	kept := m.recentMessages[:0]
	for _, r := range m.recentMessages {
		if now.Sub(r.sentAt) < recentTTL {
			kept = append(kept, r)
		}
	}
	m.recentMessages = append(kept, &recentMessage{
		userId:  receiverUserId,
		sentAt:  now,
		payload: payload,
	})
	m.mu.Unlock()

	m.logger.Debug("websocket push",
		zap.Int32("userId", receiverUserId),
		zap.Int("numClients", len(targets)))
}

// Push to all connected clients
func (m *CommsWebsocketManager) WebsocketPushAll(senderUserId int32, rpcJson json.RawMessage, timestamp time.Time) {
	m.mu.RLock()
	userIds := make([]int32, 0, len(m.clients))
	for uid := range m.clients {
		userIds = append(userIds, uid)
	}
	m.mu.RUnlock()

	for _, uid := range userIds {
		m.WebsocketPush(senderUserId, uid, rpcJson, timestamp)
	}
}
