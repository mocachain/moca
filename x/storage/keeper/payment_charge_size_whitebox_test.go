package keeper

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddChargeSize(t *testing.T) {
	sum, err := addChargeSize(100, 28)
	require.NoError(t, err)
	require.Equal(t, uint64(128), sum)

	sum, err = addChargeSize(math.MaxUint64-1, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), sum)

	// two objects charged at ~2^63 each must not wrap the total back to zero
	_, err = addChargeSize(1<<63, 1<<63)
	require.ErrorContains(t, err, "total charge size out of range")
}
