#!/bin/bash
set -e

# Generate Fumarole protobuf files
echo "Generating Fumarole protobuf files..."

# Find the yellowstone-grpc proto directory
YELLOWSTONE_PROTO=$(go list -m -f '{{.Dir}}' github.com/rpcpool/yellowstone-grpc/examples/golang 2>/dev/null)/proto

if [ ! -d "$YELLOWSTONE_PROTO" ]; then
    echo "Error: Could not find yellowstone-grpc proto directory"
    echo "Make sure github.com/rpcpool/yellowstone-grpc/examples/golang is in go.mod"
    exit 1
fi

echo "Using yellowstone-grpc proto from: $YELLOWSTONE_PROTO"

# Generate the protobuf code
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    -I proto/fumarole \
    -I "$YELLOWSTONE_PROTO" \
    proto/fumarole/fumarole.proto

echo "Protobuf generation complete!"
echo "Generated files should be in: proto/fumarole/"
