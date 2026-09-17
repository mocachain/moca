package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyPrefix(t *testing.T) {
	require.Equal(t, []byte("foo"), KeyPrefix("foo"))
}
