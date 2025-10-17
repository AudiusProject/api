package meteora_dbc

var (
	// Discriminator for the Meteora DBC Pool account
	// See: https://github.com/MeteoraAg/dbc-go/blob/377824fc6056f10ee51074080c92362de52b09da/instructions/state.go#L75C27-L75C68
	POOL_DISCRIMINATOR = []byte{213, 224, 5, 209, 98, 69, 119, 92}
	// Discriminator for the Meteora DBC Pool Config account
	// See: https://github.com/MeteoraAg/dbc-go/blob/377824fc6056f10ee51074080c92362de52b09da/instructions/state.go#L30
	POOL_CONFIG_DISCRIMINATOR = []byte{26, 108, 14, 123, 116, 230, 129, 43}
)
