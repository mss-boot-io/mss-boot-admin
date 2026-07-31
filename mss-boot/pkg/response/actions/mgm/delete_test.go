package mgm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseObjectIDs(t *testing.T) {
	ids, err := parseObjectIDs([]string{
		"507f1f77bcf86cd799439011",
		"507f1f77bcf86cd799439012",
	})

	require.NoError(t, err)
	require.Len(t, ids, 2)
}

func TestParseObjectIDsRejectsInvalidID(t *testing.T) {
	ids, err := parseObjectIDs([]string{"507f1f77bcf86cd799439011", "not-an-object-id"})

	require.Error(t, err)
	require.Nil(t, ids)
}
