package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/utils"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// BeginBlocker
// ---------------------------------------------------------------------------

func (s *TestSuite) TestBeginBlocker_WindowRolloverClearsCounts() {
	acc := sample.RandAccAddress()
	window := s.storageKeeper.DiscontinueCountingWindow(s.ctx) // default 10000
	s.storageKeeper.SetDiscontinueObjectCount(s.ctx, acc, 3)
	s.storageKeeper.SetDiscontinueBucketCount(s.ctx, acc, 4)

	ctx := s.ctx.WithBlockHeight(int64(window))
	err := keeper.BeginBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)

	s.Require().Equal(uint64(0), s.storageKeeper.GetDiscontinueObjectCount(s.ctx, acc))
	s.Require().Equal(uint64(0), s.storageKeeper.GetDiscontinueBucketCount(s.ctx, acc))
}

func (s *TestSuite) TestBeginBlocker_NonRolloverLeavesCountsIntact() {
	acc := sample.RandAccAddress()
	s.storageKeeper.SetDiscontinueObjectCount(s.ctx, acc, 3)

	ctx := s.ctx.WithBlockHeight(1) // not a multiple of the default 10000-block window
	err := keeper.BeginBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)

	s.Require().Equal(uint64(3), s.storageKeeper.GetDiscontinueObjectCount(s.ctx, acc))
}

func (s *TestSuite) TestBeginBlocker_ZeroBlockHeightNoOp() {
	acc := sample.RandAccAddress()
	s.storageKeeper.SetDiscontinueObjectCount(s.ctx, acc, 3)

	ctx := s.ctx.WithBlockHeight(0)
	err := keeper.BeginBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)

	s.Require().Equal(uint64(3), s.storageKeeper.GetDiscontinueObjectCount(s.ctx, acc),
		"genesis height must not trigger the rollover guard")
}

func (s *TestSuite) TestBeginBlocker_ZeroCountingWindowNoPanic() {
	// Params.Validate rejects a zero window through SetParams, so the only way
	// DiscontinueCountingWindow legitimately reads 0 is params never having
	// been initialized (GetParams falls back to a zero-value Params{} when the
	// key is absent) -- simulate that directly rather than fighting validation.
	s.ctx.KVStore(s.storeKey).Delete(types.ParamsKey)

	ctx := s.ctx.WithBlockHeight(100)
	s.Require().NotPanics(func() {
		err := keeper.BeginBlocker(ctx, *s.storageKeeper)
		s.Require().NoError(err)
	}, "a zero counting window must not divide by zero")
}

// ---------------------------------------------------------------------------
// EndBlocker
// ---------------------------------------------------------------------------

func (s *TestSuite) TestEndBlocker_DeletionMaxZeroEarlyReturn() {
	// As above: SetParams rejects a zero deletion max, so simulate
	// uninitialized params directly to reach the deletionMax==0 branch.
	s.ctx.KVStore(s.storeKey).Delete(types.ParamsKey)

	// No mocks are wired for anything past the deletionMax==0 check.
	err := keeper.EndBlocker(s.ctx, *s.storageKeeper)
	s.Require().NoError(err)
}

func (s *TestSuite) TestEndBlocker_DeletedReachesCapSkipsBucketsAndPaymentCheck() {
	params := types.DefaultParams()
	params.DiscontinueDeletionMax = 1
	s.Require().NoError(s.storageKeeper.SetParams(s.ctx, params))
	keeper.InitPaymentCheck(*s.storageKeeper, true, 1) // would run every block if reached

	blockTime := s.ctx.BlockTime().Unix()
	store := s.ctx.KVStore(s.storeKey)
	// A non-existent object id: ForceDeleteObject treats it as already-deleted,
	// so this trivially satisfies the deletionMax==1 cap with no other mocks.
	store.Set(types.GetDiscontinueObjectIdsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{sdkmath.NewUint(9001)}}))

	bucketID := sdkmath.NewUint(9002)
	store.Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{bucketID}}))

	// No paymentKeeper.GetAllStreamRecord expectation is registered: if the
	// deleted>=deletionMax short-circuit were removed, RunPaymentCheck would
	// call it and gomock would fail this test on an unexpected call.
	err := keeper.EndBlocker(s.ctx, *s.storageKeeper)
	s.Require().NoError(err)

	bz := store.Get(types.GetDiscontinueBucketIDsKey(blockTime))
	s.Require().NotNil(bz, "bucket deletion must be skipped once the object cap is reached")
	var remaining types.Ids
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{bucketID}, remaining.Id)
}

func (s *TestSuite) TestEndBlocker_TestnetHotfixSkipsBucketDeletion() {
	blockTime := s.ctx.BlockTime().Unix()
	bucketID := sdkmath.NewUint(9101)
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{bucketID}}))

	ctx := s.ctx.WithChainID(utils.TestnetChainID + "-1").WithBlockHeight(5946512)
	err := keeper.EndBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)

	bz := s.ctx.KVStore(s.storeKey).Get(types.GetDiscontinueBucketIDsKey(blockTime))
	s.Require().NotNil(bz, "the testnet post-height hotfix must skip bucket deletion")
}

func (s *TestSuite) TestEndBlocker_NonTestnetStillDeletesBuckets() {
	blockTime := s.ctx.BlockTime().Unix()
	bucketID := sdkmath.NewUint(9102)
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{bucketID}}))

	ctx := s.ctx.WithBlockHeight(5946512) // same height, but not the testnet chain ID
	err := keeper.EndBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)

	bz := s.ctx.KVStore(s.storeKey).Get(types.GetDiscontinueBucketIDsKey(blockTime))
	s.Require().Nil(bz, "off testnet, the same height must still run bucket deletion")
}

func (s *TestSuite) TestEndBlocker_PaymentCheckDisabledSkipsRunPaymentCheck() {
	keeper.InitPaymentCheck(*s.storageKeeper, false, 1)

	// No GetAllStreamRecord expectation: if the enabled gate were broken, this
	// test fails on gomock's unexpected-call panic instead of silently passing.
	err := keeper.EndBlocker(s.ctx, *s.storageKeeper)
	s.Require().NoError(err)
}

func (s *TestSuite) TestEndBlocker_IntervalMismatchSkipsRunPaymentCheck() {
	keeper.InitPaymentCheck(*s.storageKeeper, true, 7)

	ctx := s.ctx.WithBlockHeight(1) // 1 % 7 != 0
	err := keeper.EndBlocker(ctx, *s.storageKeeper)
	s.Require().NoError(err)
}

func (s *TestSuite) TestEndBlocker_RunsPaymentCheckWhenEnabledAndIntervalMatches() {
	keeper.InitPaymentCheck(*s.storageKeeper, true, 1)
	// Times(1), not AnyTimes(): proves RunPaymentCheck actually ran.
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{}).Times(1)

	err := keeper.EndBlocker(s.ctx, *s.storageKeeper)
	s.Require().NoError(err)
}

func (s *TestSuite) TestEndBlocker_PanicsOnRunPaymentCheckError() {
	keeper.InitPaymentCheck(*s.storageKeeper, true, 1)
	// A stream record with a positive lock balance and no bucket to justify it
	// (there are no buckets in the store) trips RunPaymentCheck's reconciliation
	// and makes it return a non-nil error; EndBlocker must turn that into a panic.
	// NetflowRate/FrozenNetflowRate must be explicit zero sdkmath.Ints, not the
	// struct's zero value: RunPaymentCheck's reverse-check loop calls IsNegative/
	// IsPositive on both fields, and an uninitialized sdkmath.Int wraps a nil
	// big.Int that panics on any method call -- which would otherwise satisfy
	// Require().Panics() without ever reaching the panic(err) this test targets.
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{
		{
			Account: "not-a-real-account", LockBalance: sdkmath.NewInt(1),
			NetflowRate: sdkmath.ZeroInt(), FrozenNetflowRate: sdkmath.ZeroInt(),
		},
	}).Times(1)

	s.Require().Panics(func() {
		_ = keeper.EndBlocker(s.ctx, *s.storageKeeper)
	})
}

func (s *TestSuite) TestEndBlocker_PanicsOnObjectDeletionError() {
	blockTime := s.ctx.BlockTime().Unix()
	// An object that exists but whose bucket does not: ForceDeleteObject's bucket
	// lookup fails and the error must propagate out of DeleteDiscontinueObjectsUntil,
	// which EndBlocker turns into a panic (mirrors
	// TestDeleteDiscontinueObjectsUntil_PropagatesForceDeleteError in keeper_test.go).
	objID := sdkmath.NewUint(9201)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objID, BucketName: "enddblocker-missing-bucket", ObjectName: "orphan-object",
	})
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectIdsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{objID}}))

	s.Require().Panics(func() {
		_ = keeper.EndBlocker(s.ctx, *s.storageKeeper)
	})
}

func (s *TestSuite) TestEndBlocker_PanicsOnBucketDeletionError() {
	blockTime := s.ctx.BlockTime().Unix()
	// A bucket whose GVG family has vanished from virtual-group state: ForceDeleteBucket's
	// primary-SP resolution returns a genuine (non-orphan) error, which propagates out of
	// DeleteDiscontinueBucketsUntil and EndBlocker turns into a panic.
	const familyID = uint32(9202)
	bucketID := sdkmath.NewUint(9203)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: bucketID, BucketName: "enddblocker-broken-family-bucket", GlobalVirtualGroupFamilyId: familyID,
	})
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{bucketID}}))
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), familyID).Return(nil, false).AnyTimes()

	s.Require().Panics(func() {
		_ = keeper.EndBlocker(s.ctx, *s.storageKeeper)
	})
}
