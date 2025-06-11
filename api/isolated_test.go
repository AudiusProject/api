package api

import (
	"testing"

	"github.com/test-go/testify/require"
)

func TestIsolation(t *testing.T) {
	app := emptyTestApp(t)

	count := -1
	err := app.pool.QueryRow(t.Context(), "select count(*) from users").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
