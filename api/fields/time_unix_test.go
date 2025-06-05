package fields

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeUnix_MarshalJSON(t *testing.T) {
	now := time.Unix(1717500000, 0).UTC()
	tu := TimeUnix(now)

	b, err := tu.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, []byte("1717500000"), b)

	// Also test via json.Marshal
	type testStruct struct {
		Time TimeUnix `json:"time"`
	}
	s := testStruct{Time: tu}
	b, err = json.Marshal(s)
	assert.NoError(t, err)
	// Should be: {"time":1717500000}
	assert.Contains(t, string(b), `"time":1717500000`)
}

func TestTimeUnix_Scan(t *testing.T) {
	now := time.Now().UTC()
	var tu TimeUnix

	err := tu.Scan(now)
	assert.NoError(t, err)
	assert.Equal(t, TimeUnix(now), tu)

	err = tu.Scan("not a time")
	assert.Error(t, err)
}
