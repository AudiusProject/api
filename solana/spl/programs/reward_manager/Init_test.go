package reward_manager_test

import (
	"testing"

	"api.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestDecodeInitInstruction(t *testing.T) {
	// Real tx: yiuyxK1b46frtiwyXxYjdQiYjWTCseAzdQ9v1QBYauSFVES3Rys2ra1oL5LpEtrNj7iXBEWnnzexNVaLvQtfb5J
	tx, _ := solana.TransactionFromBase64("BDDqW2Oo93T2yV2OXCoU7qVInmz1DbycJOTRKLqd+AVwWDY5NzEjmk25pHHL5bFN4ZOC/nTvKuZefmcmQdCGlQH6r25AgpJIv+1rpRYF5B/72+n8AQs2IsakPmdxa/dnf97ceFqbWf4ZxP0UipNz7EAPuWPR6sRUEx3hrFSBxTYOj3OxpGOo+ynaHfJes0ZKyW9y2k1l+cLIULluHRMbr2SsOmwoYJ+6hr9ryWHM3DsEVk7F0N4vfcMNl6VJHl9tD2p0BmVgFisnvvGEyqD3JpDP71hJvLTnkPRjwOTZ7hTjZlIx8MOs0r+C+oS7GaAnGNB6mdmSRtpcO62G7EKVNQkEAQYODEGlGTyYV6m3qgg3TUC5U41P87be2RdiY/CunBHsFoJVyHCOCzOaVvjwSnhzOePw2h0PdZtZoDtRSUjmk1/y8K9Vr6aFE//+YMERHbGK6X9VgImUXxI0vkqo3I4XBj3B63zyljTW4zdpLmkFyo2aNvOJxPWhLnmIzL+2VJylUNkPSfLDCuvSCgk1ulPCS9+nsU1h7IZtxi+o33BwjnNacR2HATo4YvD7S0HYRo5sRrfXBogTAPgo62BtVTvufgxJftUvMiHP762eFC1g7kv4CjmMhMqm9vEMINfPHqAHjcWq1a+wCrbqueokzWO8uOIIXgoQYWfXKetsgPV2YjF0twAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACPv8sDXigTQ9N1LvzB/tpvTcRVTacJc6U1GkArRuD5aoEIC9R/GrqTm/wLsLOG8Pyj0IGO4v4zb7fBLqNFMUGbWDpCdtNJgYHDgwTnmr3q+/teKcBczNa7qnV/lvbY5HBqfVFxksXFEhjMlMPUrxf1ja7gibof1E49vZigAAAAAG3fbh12Whk9nL4UbO63msHLSF7V9bN5E6jPWFfv8AqT14bhcFf4fnwl+QecKEMg9sh+RvFcFnA/WHtsNjMqYVCAgCAAE0AAAAAGCaFAAAAAAAQgAAAAAAAAC1g6QnbTSYGBw4ME55q96vv7XinAXMzWu6p1f5b22ORwgCAAI0AAAAAPAdHwAAAAAApQAAAAAAAAAG3fbh12Whk9nL4UbO63msHLSF7V9bN5E6jPWFfv8AqQsHAQIJAwoNDAIAAwsHAQMKAAUIDCkCyNDCm21UApXo/IrHJFby9NQQiMjlslbTAuovTgS487/YaVreFHq2jQsHAQMKAAQIDCkCFZIA+Ews8ACzoBTNTYJEUAzMNsrkiC2aOKKh/GUplnGa8PsVy5aNCgsHAQMKAAYIDCkCQiVBJzCHvsgzxX08Fbnhf5Gb+x9kcNrzvTL1AUUSvN8NAiMvVkClvQsHAQMKAAcIDCkCQo4y0fbZL8bN+Jq06bzmi8bKBWAAAAAAAAAAAAAAAAAAAAAAAAAAAAsDAQMIAQE=")

	compiled := tx.Message.Instructions[2]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)

	decoded, err := reward_manager.DecodeInstruction(accounts, compiled.Data)
	require.NoError(t, err)

	inst, ok := decoded.Impl.(*reward_manager.Init)
	require.True(t, ok)

	assert.Equal(t, uint8(3), inst.MinVotes)
	assert.Equal(t, "6mrvbuCnBir7VeD3Vr9zdL8McwfLCT4Xh8mTMGukfqUs", inst.RewardManagerState().PublicKey.String())
	assert.Equal(t, "CoS8SobE44wGq8CGi2BekLDih7GhSKgjszhTyitfGMPW", inst.TokenSourceAccount().PublicKey.String())
	assert.Equal(t, "c57yR7PxEWhHw72BstEZv5w28RkA7E1eUjSLd8vxMP3", inst.Mint().PublicKey.String())
	assert.Equal(t, "GrFKAqUtubUMG4xm7hHqPctBjDxA3MNCBQqNPYNr1Se4", inst.Manager().PublicKey.String())
	assert.Equal(t, "CK468ayRnucvCaB1s9vUN3ue7VhnPqSQRFmceHeqPCRE", inst.Authority().PublicKey.String())
}
