package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/challenge/keeper"
)

// fixedRandaoMix returns a deterministic 64-byte mix, matching the length CometBFT
// actually puts in the block header (keeper.RandaoMixLength).
func fixedRandaoMix() []byte {
	mix := make([]byte, keeper.RandaoMixLength)
	for i := range mix {
		mix[i] = byte(i)
	}
	return mix
}

func TestSeedFromRandaoMix(t *testing.T) {
	mix := fixedRandaoMix()

	seed5a := keeper.SeedFromRandaoMix(mix, 5)
	require.Len(t, seed5a, 64)

	seed5b := keeper.SeedFromRandaoMix(mix, 5)
	require.Equal(t, seed5a, seed5b, "the same mix and iteration must produce the same seed")

	seed6 := keeper.SeedFromRandaoMix(mix, 6)
	require.Len(t, seed6, 64)
	require.NotEqual(t, seed5a, seed6, "different iterations must produce different seeds")
}

func TestRandomObjectID(t *testing.T) {
	mix := fixedRandaoMix()
	objectCount := math.NewUint(37)

	for iteration := uint64(0); iteration < 20; iteration++ {
		seed := keeper.SeedFromRandaoMix(mix, iteration)
		id := keeper.RandomObjectID(seed, objectCount)
		require.True(t, id.GT(math.ZeroUint()), "object id must start from 1, got %s", id)
		require.True(t, id.LTE(objectCount), "object id must not exceed the object count, got %s", id)
	}
}

func TestRandomRedundancyIndex(t *testing.T) {
	mix := fixedRandaoMix()

	// With only the primary sp available, the redundancy index must always resolve to it.
	for iteration := uint64(0); iteration < 5; iteration++ {
		seed := keeper.SeedFromRandaoMix(mix, iteration)
		require.Equal(t, int32(-1), keeper.RandomRedundancyIndex(seed, 1))
	}

	// With 4 sps (redundancy indices -1, 0, 1, 2), every result must land in that range.
	for iteration := uint64(0); iteration < 20; iteration++ {
		seed := keeper.SeedFromRandaoMix(mix, iteration)
		index := keeper.RandomRedundancyIndex(seed, 4)
		require.GreaterOrEqual(t, index, int32(-1))
		require.LessOrEqual(t, index, int32(2))
	}
}

func TestCalculateSegments(t *testing.T) {
	// Exact multiple: no remainder, so no extra partial segment is appended.
	require.Equal(t, uint64(10), keeper.CalculateSegments(1000, 100))

	// Non-exact: the remainder still needs a segment of its own.
	require.Equal(t, uint64(11), keeper.CalculateSegments(1001, 100))
}
