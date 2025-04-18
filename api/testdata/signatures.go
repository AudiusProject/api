package testdata

// SignatureData represents a message and its corresponding signature for a wallet
type SignatureData struct {
	Message   string
	Signature string
}

// TestSignatures contains a mapping of wallet addresses to their corresponding message and signature pairs
var TestSignatures = map[string]SignatureData{
	"0x7d273271690538cf855e5b3002a0dd8c154bb060": {
		Message:   "signature:1744763856446",
		Signature: "0xbb202be3a7f3a0aa22c1458ef6a3f2f8360fb86791c7b137e8562df0707825c11fa1db01096efd2abc5e6613c4d1e8d4ae1e2b993abdd555fe270c1b17bff0d21c",
	},
}

// GetSignatureData returns the message and signature for a given test wallet address
func GetSignatureData(walletAddress string) SignatureData {
	data, exists := TestSignatures[walletAddress]
	if !exists {
		panic("no signature data found for wallet address: " + walletAddress)
	}
	return data
}
