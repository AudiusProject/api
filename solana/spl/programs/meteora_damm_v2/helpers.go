package meteora_damm_v2

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
)

// RateLimiterParams holds parsed values from ParseRateLimiterSecondFactor.
type RateLimiterParams struct {
	MaxLimiterDuration uint32
	MaxFeeBps          uint32
}

// ParseRateLimiterSecondFactor parses an 8-byte little-endian buffer into two uint32s.
// See: https://github.com/MeteoraAg/damm-v2-sdk/blob/7e43186971d7515ebaf682f4e44d15d0a4e0e8aa/src/helpers/common.ts#L114
func ParseRateLimiterSecondFactor(b []byte) (RateLimiterParams, error) {
	var p RateLimiterParams
	if err := bin.NewBorshDecoder(b).Decode(&p); err != nil {
		return RateLimiterParams{}, fmt.Errorf("unable to decode rate limiter: %w", err)
	}
	return p, nil
}
