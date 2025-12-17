package config

type Node struct {
	// The unique identifier for the node
	Id string
	// Full origin URL (including protoocol)
	Endpoint string
	// The individual wallet address for the node
	DelegateWallet string
	// The wallet address for the node owner/operator
	Owner string
	// Service type (discovery-node, content-node, validator)
	ServiceType string
	// Date of registration
	RegisteredAt string
	// Block number of registration
	BlockNumber int64
}

var (
	ProdUploadNodes = []string{
		"https://creatornode.audius.co",
		"https://creatornode2.audius.co",
		"https://creatornode3.audius.co",
	}
	StageUploadNodes = []string{
		"https://creatornode6.staging.audius.co",
	}
	DevUploadNodes = []string{
		"http://audius-creator-node-1",
	}
)
