package trashid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashId(t *testing.T) {

	// when we serialize... it'll hash
	{
		h := HashId(44)
		j, err := json.Marshal(h)
		assert.NoError(t, err)
		assert.Equal(t, `"eYorL"`, string(j))
	}

	// when we parse... it accepts both numbers and hashid strings
	// this is necessary because:
	// - we want it to round trip without exploding
	// - we want to support numbers from jsonb columns in the db

	// works with hashids
	{
		var h HashId
		err := json.Unmarshal([]byte(`"eYorL"`), &h)
		assert.NoError(t, err)
		assert.Equal(t, 44, int(h))
	}

	// works with numbers
	{
		var h HashId
		err := json.Unmarshal([]byte("33"), &h)
		assert.NoError(t, err)
		assert.Equal(t, 33, int(h))
	}

	// errors on bad hashid
	{
		var h HashId
		err := json.Unmarshal([]byte(`"asdjkfalksdjfaklsdjf"`), &h)
		assert.Error(t, err)
		assert.Equal(t, 0, int(h))
	}
}
