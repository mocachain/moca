package keeper_test

import (
	"encoding/binary"

	sdkmath "cosmossdk.io/math"
	"github.com/mocachain/moca/v2/testutil/sample"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/utils"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
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

// requireDiscontinueDeleteFailedEvent asserts exactly one discontinue_delete_failed event was
// emitted for resourceID.
func (s *TestSuite) requireDiscontinueDeleteFailedEvent(resourceType gnfdresource.ResourceType, resourceID sdkmath.Uint) {
	matched := 0
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type != types.EventTypeDiscontinueDeleteFailed {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range ev.Attributes {
			attrs[attr.Key] = attr.Value
		}
		if attrs[types.AttributeKeyResourceID] == resourceID.String() {
			s.Require().Equal(resourceType.String(), attrs[types.AttributeKeyResourceType])
			s.Require().NotEmpty(attrs[types.AttributeKeyError])
			matched++
		}
	}
	s.Require().Equal(1, matched, "exactly one %s event must name %s", types.EventTypeDiscontinueDeleteFailed, resourceID)
}

// seedHealthyDiscontinuedObject stores a bucket with a resolvable primary SP plus a
// discontinued object in it that ForceDeleteObject can collect without error.
func (s *TestSuite) seedHealthyDiscontinuedObject(bucketID, objectID sdkmath.Uint) {
	owner := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{
		Owner: owner.String(), BucketName: "endblocker-gc-bucket", Id: bucketID,
		GlobalVirtualGroupFamilyId: 1, PaymentAddress: owner.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.mockPrimarySP(bucketInfo, &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String(),
	})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objectID, BucketName: bucketInfo.BucketName, ObjectName: "healthy-object",
		Owner: owner.String(), ObjectStatus: types.OBJECT_STATUS_DISCONTINUED,
		// strictly after the versioned-params timestamp: GetVersionedParamsWithTS is exclusive of ts
		CreateAt: s.ctx.BlockTime().Unix() + 1,
	})
	// simulates DiscontinueObject's saveDiscontinueObjectStatus recording CREATED as the pre-discontinue status.
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_CREATED))
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectStatusKey(objectID), statusBytes)
	s.stubObjectUnlockFee()
	s.stubGCBookkeepingNoop()
}

func (s *TestSuite) TestEndBlocker_ObjectDeletionErrorIsContainedToTheItem() {
	blockTime := s.ctx.BlockTime().Unix()
	store := s.ctx.KVStore(s.storeKey)

	// An object that exists but whose bucket does not: ForceDeleteObject's bucket lookup fails.
	badID := sdkmath.NewUint(9201)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: badID, BucketName: "endblocker-missing-bucket", ObjectName: "orphan-object",
	})
	goodID := sdkmath.NewUint(9202)
	s.seedHealthyDiscontinuedObject(sdkmath.NewUint(9200), goodID)

	// The failing id is queued ahead of the healthy one on purpose.
	store.Set(types.GetDiscontinueObjectIdsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{badID, goodID}}))

	s.Require().NotPanics(func() {
		s.Require().NoError(keeper.EndBlocker(s.ctx, *s.storageKeeper))
	}, "a single undeletable object must not halt the block")

	s.Require().False(store.Has(types.GetDiscontinueObjectIdsKey(blockTime)),
		"the failing id must be dropped from the queue instead of being retried every block")
	_, found := s.storageKeeper.GetObjectInfoById(s.ctx, goodID)
	s.Require().False(found, "the object queued behind the failing one must still be deleted")
	_, found = s.storageKeeper.GetObjectInfoById(s.ctx, badID)
	s.Require().True(found, "the failing item must not leave partial state behind")
	s.requireDiscontinueDeleteFailedEvent(gnfdresource.RESOURCE_TYPE_OBJECT, badID)
}

func (s *TestSuite) TestEndBlocker_BucketDeletionErrorIsContainedToTheItem() {
	blockTime := s.ctx.BlockTime().Unix()
	store := s.ctx.KVStore(s.storeKey)

	// A bucket whose GVG family has vanished from virtual-group state: ForceDeleteBucket's
	// primary-SP resolution returns a genuine (non-orphan) error.
	const badFamilyID = uint32(9210)
	badID := sdkmath.NewUint(9211)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: badID, BucketName: "endblocker-broken-family-bucket", GlobalVirtualGroupFamilyId: badFamilyID,
	})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), badFamilyID).Return(nil, false).AnyTimes()

	owner := sample.RandAccAddress()
	goodID := sdkmath.NewUint(9212)
	goodBucket := &types.BucketInfo{
		Owner: owner.String(), BucketName: "endblocker-healthy-bucket", Id: goodID,
		GlobalVirtualGroupFamilyId: 1, PaymentAddress: owner.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, goodBucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, goodID, &types.InternalBucketInfo{})
	s.mockPrimarySP(goodBucket, &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String(),
	})
	s.stubGCBookkeepingNoop()

	store.Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{badID, goodID}}))

	s.Require().NotPanics(func() {
		s.Require().NoError(keeper.EndBlocker(s.ctx, *s.storageKeeper))
	}, "a single undeletable bucket must not halt the block")

	s.Require().False(store.Has(types.GetDiscontinueBucketIDsKey(blockTime)),
		"the failing id must be dropped from the queue instead of being retried every block")
	_, found := s.storageKeeper.GetBucketInfo(s.ctx, goodBucket.BucketName)
	s.Require().False(found, "the bucket queued behind the failing one must still be deleted")
	_, found = s.storageKeeper.GetBucketInfoById(s.ctx, badID)
	s.Require().True(found, "the failing item must not leave partial state behind")
	s.requireDiscontinueDeleteFailedEvent(gnfdresource.RESOURCE_TYPE_BUCKET, badID)
}

// TestEndBlocker_OrphanedPrimarySPStillGarbageCollects pins the ErrStorageProviderNotFound
// special case: a bucket whose primary SP is gone is still collected, not reported as failed.
func (s *TestSuite) TestEndBlocker_OrphanedPrimarySPStillGarbageCollects() {
	blockTime := s.ctx.BlockTime().Unix()
	store := s.ctx.KVStore(s.storeKey)

	const familyID = uint32(9220)
	bucketID := sdkmath.NewUint(9221)
	owner := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner: owner.String(), BucketName: "endblocker-orphan-sp-bucket", Id: bucketID,
		GlobalVirtualGroupFamilyId: familyID, PaymentAddress: owner.String(),
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), familyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: 77}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(77)).Return(nil, false).AnyTimes()
	s.stubGCBookkeepingNoop()

	store.Set(types.GetDiscontinueBucketIDsKey(blockTime), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{bucketID}}))

	s.Require().NoError(keeper.EndBlocker(s.ctx, *s.storageKeeper))

	_, found := s.storageKeeper.GetBucketInfoById(s.ctx, bucketID)
	s.Require().False(found, "an orphaned bucket must still be garbage-collected")
	for _, ev := range s.ctx.EventManager().Events() {
		s.Require().NotEqual(types.EventTypeDiscontinueDeleteFailed, ev.Type,
			"a missing primary SP is handled by the orphan path, not reported as a deletion failure")
	}
}
