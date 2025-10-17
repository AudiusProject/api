package meteora_damm_v2

var (
	// Discriminator for Meteora DAMM V2 pool accounts
	// See: https://github.com/MeteoraAg/damm-v2-go/blob/3cc12838bce93a9cd546b22a1caaabaa81ce81f7/instructions/state.go#L31
	POOL_DISCRIMINATOR = []byte{241, 154, 109, 4, 17, 177, 109, 188}
	// Discriminator for Meteora DAMM V2 position accounts
	// See: https://github.com/MeteoraAg/damm-v2-go/blob/3cc12838bce93a9cd546b22a1caaabaa81ce81f7/instructions/state.go#L128
	POSITION_DISCRIMINATOR = []byte{170, 188, 143, 228, 122, 64, 247, 208}
)
