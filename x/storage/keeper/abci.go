package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/utils"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

func BeginBlocker(ctx sdk.Context, keeper Keeper) error {
	blockHeight := uint64(ctx.BlockHeight())
	countingWindow := keeper.DiscontinueCountingWindow(ctx)
	if blockHeight > 0 && countingWindow > 0 && blockHeight%countingWindow == 0 {
		keeper.ClearDiscontinueObjectCount(ctx)
		keeper.ClearDiscontinueBucketCount(ctx)
	}
	return nil
}

func EndBlocker(ctx sdk.Context, keeper Keeper) error {
	// the current block's delete bookkeeping lives in the regular KV store, so it has to be dropped
	// on every path out of this function -- including the early returns below, which previously
	// relied on the transient store being discarded at the end of the block.
	defer keeper.ClearCurrentBlockDeleteInfo(ctx)

	deletionMax := keeper.DiscontinueDeletionMax(ctx)
	if deletionMax == 0 {
		return nil
	}

	blockTime := ctx.BlockTime().Unix()

	// set ForceUpdateStreamRecordKey to true in context to force update frozen stream record
	ctx = ctx.WithValue(paymenttypes.ForceUpdateStreamRecordKey, true)

	// delete objects
	deleted, err := keeper.DeleteDiscontinueObjectsUntil(ctx, blockTime, deletionMax)
	if err != nil {
		ctx.Logger().Error("should not happen, fail to delete objects, err " + err.Error())
		panic("should not happen")
	}

	if deleted >= deletionMax {
		return nil
	}

	// delete buckets
	doDeleteBucket := true
	// on testnet, we had a hot fix to disable deleting buckets after discontinue since 5946512 height
	if ctx.BlockHeight() > 5946511 && ctx.ChainID() == utils.TestnetChainID+"-1" {
		doDeleteBucket = false
	}

	if doDeleteBucket {
		_, err = keeper.DeleteDiscontinueBucketsUntil(ctx, blockTime, deletionMax-deleted)
		if err != nil {
			ctx.Logger().Error("should not happen, fail to delete buckets, err " + err.Error())
			panic("should not happen")
		}
	}

	keeper.PersistDeleteInfo(ctx)

	// Permission GC
	keeper.GarbageCollectResourcesStalePolicy(ctx)

	// Payment Data Check: an opt-in, node-local (app.toml [payment-check]) read-only diagnostic.
	interval := int64(keeper.GetPaymentCheckInterval())
	if keeper.IsPaymentCheckEnabled() && interval > 0 && ctx.BlockHeight()%interval == 0 {
		runPaymentCheck(ctx, keeper)
	}
	return nil
}

// runPaymentCheck runs the opt-in payment diagnostic and reports what it finds
// instead of propagating it. Nothing here is recovered further up, so both an
// error return and one of the check's own Must*/parse panics would stop the
// node that opted in -- and again on every replay of the block -- while a node
// without the diagnostic carries on. The check only reads, so both nodes end
// the block in the same state; the findings themselves are already logged
// inside RunPaymentCheck.
func runPaymentCheck(ctx sdk.Context, keeper Keeper) {
	defer func() {
		if r := recover(); r != nil {
			ctx.Logger().Error("payment check panicked", "height", ctx.BlockHeight(), "panic", r)
		}
	}()

	if err := keeper.RunPaymentCheck(ctx); err != nil {
		ctx.Logger().Error("payment check failed", "height", ctx.BlockHeight(), "err", err.Error())
	}
}
