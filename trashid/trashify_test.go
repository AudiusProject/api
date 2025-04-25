package trashid

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrashify(t *testing.T) {
	j1 := []byte(`
	{
		"data": {
			"id": 1,
			"user_id": 2,
			"special_id": 999,
			"tracks": [
				{
					"id": 3,
					"title": "fun",
					"value": "id",
					"other": "user_id",
					"aid": 333,
					"ida": 111
				}
			]
		}
	}
	`)

	var m map[string]any
	err := json.Unmarshal(j1, &m)
	assert.NoError(t, err)
	fmt.Println(m)

	j2 := Trashify(j1)
	fmt.Println(string(j2))
	err = json.Unmarshal(j2, &m)
	assert.NoError(t, err)
	fmt.Println(m)

}
