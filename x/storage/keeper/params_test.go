package keeper_test

import (
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}

func GetVersionedParamsWithTimestamp(k *keeper.Keeper, ctx sdk.Context, ts int64) (val types.VersionedParams) {
	params, err := k.GetVersionedParamsWithTS(ctx, ts)
	if err != nil {
		fmt.Printf("GetParamsWithTimestamp err %s\n", err)
	}
	return params
}

func TestMultiVersionParams(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()

	blockTimeT1 := ctx.BlockTime().Unix()
	params.VersionedParams.MaxSegmentSize = 1
	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(1 * time.Hour))
	blockTimeT2 := ctx.BlockTime().Unix()
	params.VersionedParams.MaxSegmentSize = 2
	err = k.SetParams(ctx, params)
	require.NoError(t, err)

	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(1 * time.Hour))
	blockTimeT3 := ctx.BlockTime().Unix()
	params.VersionedParams.MaxSegmentSize = 3
	err = k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
	// default params
	require.EqualValues(t, GetVersionedParamsWithTimestamp(k, ctx, blockTimeT1+1).MaxSegmentSize, 1)
	require.EqualValues(t, GetVersionedParamsWithTimestamp(k, ctx, blockTimeT2+1).MaxSegmentSize, 2)
	require.EqualValues(t, GetVersionedParamsWithTimestamp(k, ctx, blockTimeT3+1).MaxSegmentSize, 3)
}

// TestParamsGetters covers the one-line getters that TestGetParams/TestMultiVersionParams
// never call directly: DiscontinueCountingWindow, DiscontinueObjectMax,
// DiscontinueBucketMax, DiscontinueConfirmPeriod, MaxSegmentSize,
// RedundantDataChunkNum, RedundantParityChunkNum, MinChargeSize, and
// StalePolicyCleanupMax. One SetParams call with distinct non-default values,
// then one assertion per getter.
func TestParamsGetters(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()
	params.DiscontinueCountingWindow = 111
	params.DiscontinueObjectMax = 222
	params.DiscontinueBucketMax = 333
	params.DiscontinueConfirmPeriod = 444
	params.StalePolicyCleanupMax = 555
	params.VersionedParams.MaxSegmentSize = 666
	params.VersionedParams.RedundantDataChunkNum = 7
	params.VersionedParams.RedundantParityChunkNum = 8
	params.VersionedParams.MinChargeSize = 999

	err := k.SetParams(ctx, params)
	require.NoError(t, err)
	// MaxSegmentSize resolves through the timestamped-history lookup (unlike the
	// others, which read the current, non-versioned Params), so it must be
	// queried strictly after the block time SetParams recorded it at.
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(1 * time.Second))

	require.Equal(t, uint64(111), k.DiscontinueCountingWindow(ctx))
	require.Equal(t, uint64(222), k.DiscontinueObjectMax(ctx))
	require.Equal(t, uint64(333), k.DiscontinueBucketMax(ctx))
	require.Equal(t, int64(444), k.DiscontinueConfirmPeriod(ctx))
	require.Equal(t, uint64(555), k.StalePolicyCleanupMax(ctx))
	require.Equal(t, uint32(7), k.RedundantDataChunkNum(ctx))
	require.Equal(t, uint32(8), k.RedundantParityChunkNum(ctx))
	require.Equal(t, uint64(999), k.MinChargeSize(ctx))

	segSize, err := k.MaxSegmentSize(ctx, ctx.BlockTime().Unix())
	require.NoError(t, err)
	require.Equal(t, uint64(666), segSize)
}
