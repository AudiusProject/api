package comms

import (
	"encoding/json"
	"sync"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/contrib/websocket"
	"go.uber.org/zap"
)

type CommsWebsocketManager struct {
	mu             sync.Mutex
	websockets     map[int32][]*websocket.Conn
	recentMessages []*recentMessage
	logger         *zap.Logger
}

func NewCommsWebsocketManager(logger *zap.Logger) *CommsWebsocketManager {

	return &CommsWebsocketManager{
		websockets:     make(map[int32][]*websocket.Conn),
		recentMessages: []*recentMessage{},
		logger:         logger,
	}
}

type recentMessage struct {
	userId  int32
	sentAt  time.Time
	payload []byte
}

func (m *CommsWebsocketManager) RegisterWebsocket(userId int32, conn *websocket.Conn) {
	var pushErr error
	for _, r := range m.recentMessages {
		if time.Since(r.sentAt) < time.Second*10 && r.userId == userId {
			pushErr = conn.WriteMessage(websocket.TextMessage, r.payload)
			if pushErr != nil {
				m.logger.Info("websocket push failed: " + pushErr.Error())
				break
			}
		}
	}

	if pushErr == nil {
		m.mu.Lock()
		m.websockets[userId] = append(m.websockets[userId], conn)
		m.mu.Unlock()
	}
}

func (m *CommsWebsocketManager) removeWebsocket(userId int32, toRemove *websocket.Conn) {
	keep := make([]*websocket.Conn, 0, len(m.websockets[userId]))
	for _, s := range m.websockets[userId] {
		if s == toRemove {
			s.Close()

		} else {
			keep = append(keep, s)
		}
	}
	m.websockets[userId] = keep
}

/** Processes a message for a single sender/receiver pair and pushes to all applicable websockets */
func (m *CommsWebsocketManager) WebsocketPush(senderUserId int32, receiverUserId int32, rpcJson json.RawMessage, timestamp time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.websockets[receiverUserId] {
		encodedSenderUserId, _ := trashid.EncodeHashId(int(senderUserId))
		encodedReceiverUserId, _ := trashid.EncodeHashId(int(receiverUserId))

		// this struct should match ChatWebsocketEventData
		// but we create a matching anon struct here
		// so we can simply pass thru the RPC as a json.RawMessage
		// which is simpler than satisfying the quicktype generated schema.RPC struct
		data := struct {
			RPC      json.RawMessage `json:"rpc"`
			Metadata Metadata        `json:"metadata"`
		}{
			rpcJson,
			Metadata{Timestamp: timestamp.Format(time.RFC3339Nano), SenderUserID: encodedSenderUserId, ReceiverUserID: encodedReceiverUserId, UserID: encodedSenderUserId},
		}

		payload, err := json.Marshal(data)
		if err != nil {
			m.logger.Warn("invalid websocket json " + err.Error())
			return
		}
		err = s.WriteMessage(websocket.TextMessage, payload)
		if err != nil {
			m.logger.Info("websocket push failed: " + err.Error())
			m.removeWebsocket(receiverUserId, s)
		} else {
			m.logger.Debug("websocket push", zap.Int32("userId", receiverUserId), zap.String("payload", string(payload)))
		}

		// filter out expired messages and append new one
		recent2 := []*recentMessage{}
		for _, r := range m.recentMessages {
			if time.Since(r.sentAt) < time.Second*10 {
				recent2 = append(recent2, r)
			}
		}
		recent2 = append(recent2, &recentMessage{
			userId:  receiverUserId,
			sentAt:  time.Now(),
			payload: payload,
		})
		m.recentMessages = recent2
	}
}

/** Processes a message against all connected websockets. Used for chat blasts */
func (m *CommsWebsocketManager) WebsocketPushAll(senderUserId int32, rpcJson json.RawMessage, timestamp time.Time) {
	for receiverUserId := range m.websockets {
		m.WebsocketPush(senderUserId, receiverUserId, rpcJson, timestamp)
	}
}
