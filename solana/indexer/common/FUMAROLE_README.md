# Fumarole gRPC Client

This directory contains the Fumarole adapter for connecting to Yellowstone Fumarole endpoints.

## Current Status

The Fumarole adapter is **fully functional** using hand-crafted protobuf message types that match the Fumarole schema. The adapter works as a drop-in replacement for the standard Geyser client.

### Proto Files

The Fumarole protobuf definitions are stored in `proto/fumarole/`:

- `fumarole.proto` - Main Fumarole service definition
- `geyser.proto` - Geyser message types (imported from yellowstone-grpc)
- `solana-storage.proto` - Solana storage types

### Generated Code (Optional)

The adapter currently uses manually defined message types. To generate proper protobuf code (optional):

```bash
# Run the generation script
./scripts/generate_fumarole_proto.sh
```

This will generate `fumarole.pb.go` and `fumarole_grpc.pb.go` in `proto/fumarole/`.

## Usage

### Basic Usage

```go
import (
    "api.audius.co/solana/indexer/common"
    pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// Create Fumarole adapter
config := common.GrpcConfig{
    Server:               "https://your-fumarole-endpoint.com:443",
    ApiToken:             "your-api-token",
    MaxReconnectAttempts: 10,
}

// Optional: provide a consumer group name for resumable subscriptions
// If not provided, a unique name will be generated
consumerGroupName := "my-indexer-group"

client := common.NewFumaroleAdapter(config, consumerGroupName)

// Use it like any other GrpcClient
subscribeRequest := &pb.SubscribeRequest{
    Accounts: map[string]*pb.SubscribeRequestFilterAccounts{
        "account_filter": {
            Account: []string{"your-account-pubkey"},
        },
    },
}

err := client.Subscribe(ctx, subscribeRequest, dataCallback, errorCallback)
if err != nil {
    log.Fatal(err)
}

defer client.Close()
```

### Migration from Standard Geyser Client

The Fumarole adapter implements the same `GrpcClient` interface as the standard gRPC client, making migration straightforward:

```go
// Old (standard Geyser):
// client := common.NewGrpcClient(config)

// New (Fumarole):
client := common.NewFumaroleAdapter(config, "my-consumer-group")

// Rest of your code stays the same!
```

## How It Works

The Fumarole adapter translates between the standard Geyser Subscribe API and Fumarole's dual-stream architecture:

1. **Control Plane** (`Subscribe` RPC): Manages consumer group state, heartbeats, and offset commits
2. **Data Plane** (`SubscribeData` RPC): Delivers filtered blockchain updates

### Key Features

- **Consumer Groups**: Supports resumable subscriptions via consumer groups
- **Auto-reconnect**: Automatically reconnects both control and data planes with exponential backoff
- **Rate Limit Handling**: Special handling for 429 errors with longer backoff
- **Slot Tracking**: Maintains last seen slot for efficient replay on reconnect
- **Filter Translation**: Automatically converts Geyser filters to Fumarole BlockFilters

### Differences from Standard Geyser

1. **Two Streams**: Fumarole uses separate control and data streams (vs. single stream)
2. **Consumer Groups**: Enables offset management and resumable subscriptions
3. **No Ping RPC**: Heartbeat is handled via the control plane ping mechanism
4. **Block Filters**: Uses Fumarole's BlockFilters format instead of raw Geyser filters

## Architecture

```
┌─────────────────────┐
│  Your Indexer Code  │
└──────────┬──────────┘
           │ GrpcClient interface
           │
┌──────────▼──────────┐
│ FumaroleAdapter     │
├─────────────────────┤
│ - Control Stream    │ ◄─── JoinControlPlane, Ping, CommitOffset
│ - Data Stream       │ ◄─── FilterUpdate, SubscribeUpdate
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│ Fumarole Server     │
└─────────────────────┘
```

## Troubleshooting

### 429 Too Many Requests

If you see this error:

- The adapter will automatically wait 60 seconds before reconnecting
- Consider using a higher-tier API key
- Ensure you're not running multiple instances with the same consumer group

### Connection Refused

Check:

- Your API token is valid
- The server URL includes the correct port (usually 443 or 9090)
- TLS is enabled (Fumarole requires HTTPS)

### Missing Updates

- Fumarole uses consumer groups for offset management
- If you restart with the same consumer group name, it will resume from the last committed offset
- Use a new consumer group name to start from the latest slot
