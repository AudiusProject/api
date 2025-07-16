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
	"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0": {
		Message:   "signature:1746224298871",
		Signature: "0x00cc53200e1ee98248cd5556293e4a7ec70bfcde2a1e8e7aedbff471ac0ca8a0354d50e9bc62fbe32ad1d48dfe414ea99711030e88ba788e5bc607fef6c295311b",
	},
	"0x4954d18926ba0ed9378938444731be4e622537b2": {
		Message:   "signature:1746226663660",
		Signature: "0x8035c0154bc68de4c0e57bfe8b2adc880f04c0754c32677f895f63a15d2b5cb5720f89b5cc9acf079e3058485b10d50ae83a6b2f19080aeafb46890b43e297c51c",
	},
	"0x855d28d495ec1b06364bb7a521212753e2190b95": {
		Message:   "signature:1746226936204",
		Signature: "0xc2f6fd9c5837b481ac1ee3339a8a83267b36af5a53262d78b759fc810fa814ed0dee8e62a9b911e32b4586ca38f890f31fe83be24fe44f32c9b07d13d1906b2f1b",
	},
	"0x8A2c4dcb2Eb9c2C5bc6E28310E4B07011D230C0A": {
		Message:   "test",
		Signature: "0xae31f899e3847c61403075ba09b735c6087d6b69e7fda323dea3e3aa98c27d652c78601792f03868b621f02975581eb7b54f4272ad1de938ab1b3479120621271b",
	},
	"0x782aC09C59da685011d31e84de821B6b99b2b682": {
		Message:   "test",
		Signature: "0x809767951bd6df9434dd16564854332f5c8422ee6d64dad1574345551f5595ca0b5ec5bc9d38f379621a26b3f6a420d8a0353ddebf8eaeb258b9dab08d9486121c",
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
