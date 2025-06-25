package esindexer

import (
	"testing"

	"github.com/test-go/testify/require"
)

func TestPending(t *testing.T) {
	pending := &pendingChanges{
		idMap: map[string][]int64{},
	}

	pending.enqueue("users", 1)
	pending.enqueue("users", 1)
	pending.enqueue("users", 2)
	pending.enqueue("tracks", 3)

	idMap := pending.take()
	require.Equal(t, []int64{1, 2}, idMap["users"])
	require.Empty(t, pending.idMap["users"])

}
