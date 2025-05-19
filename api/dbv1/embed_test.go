package dbv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbed(t *testing.T) {
	q := mustGetQuery("get_account_playlists.sql")
	assert.Contains(t, q, "FROM playlists")
}
