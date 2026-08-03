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

func TestIntId(t *testing.T) {

	// when we serialize... it emits a plain number (not a hash string)
	{
		i := IntId(44)
		j, err := json.Marshal(i)
		assert.NoError(t, err)
		assert.Equal(t, `44`, string(j))
	}

	// when we parse a hashid string... it decodes to the numeric value
	{
		var i IntId
		err := json.Unmarshal([]byte(`"eYorL"`), &i)
		assert.NoError(t, err)
		assert.Equal(t, 44, int(i))
	}

	// when we parse a raw number... it works as-is
	{
		var i IntId
		err := json.Unmarshal([]byte("33"), &i)
		assert.NoError(t, err)
		assert.Equal(t, 33, int(i))
	}

	// errors on bad hashid string
	{
		var i IntId
		err := json.Unmarshal([]byte(`"asdjkfalksdjfaklsdjf"`), &i)
		assert.Error(t, err)
		assert.Equal(t, 0, int(i))
	}
}
