package dbv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueCollaboratorIDs(t *testing.T) {
	assert.Equal(
		t,
		[]int32{2, 3, 4},
		uniqueCollaboratorIDs([]int32{2, 2, 1, 3, 3, 4}, 1),
	)
	assert.Nil(t, uniqueCollaboratorIDs(nil, 1))
}
