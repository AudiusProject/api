package meteora_damm_v2

import (
	"database/sql/driver"
	"fmt"
	"math/big"

	bin "github.com/gagliardetto/binary"
)

// Struct wrapper for little-endian uint256 (as big.Int).
// Implements Borsh serialization and database Valuer interface
type Uint256LE struct {
	big.Int
}

func (i *Uint256LE) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	var b [32]byte
	err := decoder.Decode(&b)
	if err != nil {
		return err
	}
	i.SetBytes(reverseBytes(b[:]))
	return nil
}

func (i Uint256LE) MarshalWithEncoder(encoder *bin.Encoder) error {
	b := i.Bytes()
	if len(b) > 32 {
		return fmt.Errorf("Int256LE: integer too large to encode")
	}
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	_, err := encoder.Write(reverseBytes(padded))
	return err
}

func (i Uint256LE) Value() (driver.Value, error) {
	return i.String(), nil
}

// reverseBytes reverses a byte slice to match TypeScript Buffer.reverse() behavior
func reverseBytes(b []byte) []byte {
	reversed := make([]byte, len(b))
	for i, j := 0, len(b)-1; i < len(b); i, j = i+1, j-1 {
		reversed[i] = b[j]
	}
	return reversed
}
