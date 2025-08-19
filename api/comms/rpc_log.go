package comms

import (
	"encoding/json"
	"time"
)

// RpcLog was previously used to track messages sent between comms peer servers.
// We are now using it as a record of RPC requests received from clients.
// RelayedAt will be the timestamp when the server received the request.
// RelayedBy will be hard-coded to "bridge" to differentiate it from legacy rpclog messages.
type RpcLog struct {
	ID         string          `db:"id" json:"id"`
	RelayedAt  time.Time       `db:"relayed_at" json:"relayed_at"`
	AppliedAt  time.Time       `db:"applied_at" json:"applied_at"`
	RelayedBy  string          `db:"relayed_by" json:"relayed_by"`
	FromWallet string          `db:"from_wallet" json:"from_wallet"`
	Rpc        json.RawMessage `db:"rpc" json:"rpc"`
	Sig        string          `db:"sig" json:"sig"`
}
