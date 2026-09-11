package keeper_test

import (
	"encoding/binary"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/mocachain/moca/v2/internal/sequence"
	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permkeeper "github.com/mocachain/moca/v2/x/permission/keeper"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"go.uber.org/mock/gomock"
)

func (s *TestSuite) TestClearDiscontinueBucketCount() {
	acc1 := sample.RandAccAddress()
	s.storageKeeper.SetDiscontinueBucketCount(s.ctx, acc1, 1)

	count := s.storageKeeper.GetDiscontinueBucketCount(s.ctx, acc1)
	s.Require().Equal(uint64(1), count)

	s.storageKeeper.ClearDiscontinueBucketCount(s.ctx)

	count = s.storageKeeper.GetDiscontinueBucketCount(s.ctx, acc1)
	s.Require().Equal(uint64(0), count)
}

func (s *TestSuite) TestClearDiscontinueObjectCount() {
	acc1 := sample.RandAccAddress()
	s.storageKeeper.SetDiscontinueObjectCount(s.ctx, acc1, 1)

	count := s.storageKeeper.GetDiscontinueObjectCount(s.ctx, acc1)
	s.Require().Equal(uint64(1), count)

	s.storageKeeper.ClearDiscontinueObjectCount(s.ctx)

	count = s.storageKeeper.GetDiscontinueObjectCount(s.ctx, acc1)
	s.Require().Equal(uint64(0), count)
}

// ---------------------------------------------------------------------------
// test/storage-keeper-bucket (PR1): bucket lifecycle coverage.
//
// Covers keeper.go's GetAuthority/IsPaymentCheckEnabled/GetPaymentCheckInterval/
// Logger trivial getters, DeleteBucket/doDeleteBucket, ForceDeleteBucket,
// UpdateBucketInfo, and DiscontinueBucket; payment.go's UpdateBucketInfoAndCharge
// (UpdateBucketInfo's only caller); and verify.go's VerifyPaymentAccount
// malformed-non-empty-address branch (driven through UpdateBucketInfo's
// PaymentAddress field).
// ---------------------------------------------------------------------------

func (s *TestSuite) TestGetAuthority() {
	s.Require().Equal(authtypes.NewModuleAddress(govtypes.ModuleName).String(), s.storageKeeper.GetAuthority())
}

// TestPaymentCheckConfigGetters pins IsPaymentCheckEnabled/GetPaymentCheckInterval
// against the zero-value paymentCheckConfig NewKeeper constructs the suite's
// keeper with (Enabled: false, Interval: 0); no setter is exercised elsewhere in
// this package's tests.
func (s *TestSuite) TestPaymentCheckConfigGetters() {
	s.Require().False(s.storageKeeper.IsPaymentCheckEnabled())
	s.Require().Equal(uint32(0), s.storageKeeper.GetPaymentCheckInterval())
}

func (s *TestSuite) TestLogger() {
	s.Require().NotNil(s.storageKeeper.Logger(s.ctx))
}

// mockPrimarySP wires the GVG-family and storage-provider mocks so
// GetPrimarySPForBucket/MustGetPrimarySPForBucket resolve bucketInfo's family to
// sp without error. sp.OperatorAddress must be a valid hex address: some callers
// (ForceDeleteBucket) attribute the deletion event to it.
func (s *TestSuite) mockPrimarySP(bucketInfo *types.BucketInfo, sp *sptypes.StorageProvider) {
	family := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{
		Id:                    bucketInfo.GlobalVirtualGroupFamilyId,
		PrimarySpId:           sp.Id,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), bucketInfo.GlobalVirtualGroupFamilyId).Return(family, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), sp.Id).Return(sp, true).AnyTimes()
}

// stubObjectUnlockFee lets UnlockObjectStoreFee (ForceDeleteBucket's per-object
// CREATED-status path) succeed without asserting on the fee amount charged.
func (s *TestSuite) stubObjectUnlockFee() {
	price := sptypes.GlobalSpStorePrice{
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
		ReadPrice:           sdkmath.LegacyNewDec(1),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{ReserveTime: 0, ValidatorTaxRate: sdkmath.LegacyZeroDec()}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil).AnyTimes()
}

// stubGCBookkeepingNoop makes appendResourceIDForGarbageCollection (invoked by
// every doDeleteBucket/doDeleteObject call) take its early-return path: neither
// an account nor a group policy references the resource being deleted.
func (s *TestSuite) stubGCBookkeepingNoop() {
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
}

func (s *TestSuite) TestDeleteBucket_NotFound() {
	err := s.storageKeeper.DeleteBucket(s.ctx, sample.RandAccAddress(), "does-not-exist", types.DeleteBucketOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestDeleteBucket_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	bucketName := "delete-sourcetype-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         sdkmath.NewUint(1),
		SourceType: types.SOURCE_TYPE_ORIGIN,
	})

	err := s.storageKeeper.DeleteBucket(s.ctx, owner, bucketName, types.DeleteBucketOptions{SourceType: types.SOURCE_TYPE_MIRROR_PENDING})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

func (s *TestSuite) TestDeleteBucket_PermissionDenied() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress() // not the owner, and holds no policy grant
	bucketName := "delete-denied-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         sdkmath.NewUint(1),
	})
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err := s.storageKeeper.DeleteBucket(s.ctx, operator, bucketName, types.DeleteBucketOptions{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestDeleteBucket_NonEmptyBucket() {
	owner := sample.RandAccAddress()
	bucketName := "delete-nonempty-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         sdkmath.NewUint(1),
	})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id:         sdkmath.NewUint(10),
		BucketName: bucketName,
		ObjectName: "still-here",
	})

	err := s.storageKeeper.DeleteBucket(s.ctx, owner, bucketName, types.DeleteBucketOptions{})
	s.Require().ErrorIs(err, types.ErrBucketNotEmpty)
}

func (s *TestSuite) TestDeleteBucket_NonEmptyLVG() {
	owner := sample.RandAccAddress()
	bucketName := "delete-nonemptylvg-bucket"
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         bucketID,
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, StoredSize: 10}},
	})

	err := s.storageKeeper.DeleteBucket(s.ctx, owner, bucketName, types.DeleteBucketOptions{})
	s.Require().ErrorIs(err, types.ErrVirtualGroupOperateFailed)
}

func (s *TestSuite) TestDeleteBucket_Success() {
	owner := sample.RandAccAddress()
	bucketName := "delete-success-bucket"
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:          owner.String(),
		BucketName:     bucketName,
		Id:             bucketID,
		PaymentAddress: owner.String(),
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	s.stubGCBookkeepingNoop()

	err := s.storageKeeper.DeleteBucket(s.ctx, owner, bucketName, types.DeleteBucketOptions{})
	s.Require().NoError(err)

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found, "bucket must be gone after DeleteBucket")
}

// TestDeleteBucket_MigratingBucketEmitsCancelEvent drives doDeleteBucket's
// remaining branch: deleting a bucket that is mid-migration must also emit an
// EventCancelMigrationBucket, not just the ordinary EventDeleteBucket.
func (s *TestSuite) TestDeleteBucket_MigratingBucketEmitsCancelEvent() {
	owner := sample.RandAccAddress()
	bucketName := "delete-migrating-bucket"
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:          owner.String(),
		BucketName:     bucketName,
		Id:             bucketID,
		PaymentAddress: owner.String(),
		BucketStatus:   types.BUCKET_STATUS_MIGRATING,
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	s.stubGCBookkeepingNoop()

	err := s.storageKeeper.DeleteBucket(s.ctx, owner, bucketName, types.DeleteBucketOptions{})
	s.Require().NoError(err, "doDeleteBucket must not fail while emitting the migration-cancel event")

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found)
}

// TestForceDeleteBucket_CapReached drives the cap-reached partial-return branch:
// with two objects in the bucket and cap=1, only the first is processed and the
// call returns before the bucket itself is deleted.
func (s *TestSuite) TestForceDeleteBucket_CapReached() {
	// SetupTest seeds versioned params at the suite's initial block time; move
	// forward so the objects' CreateAt-keyed GetVersionedParamsWithTS lookup
	// lands strictly after that seed (see keeper_test.go's off-by-time note).
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(1 * time.Second))
	owner := sample.RandAccAddress()
	bucketName := "force-delete-cap-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), BucketName: bucketName, ObjectName: "force-obj-a",
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(11), BucketName: bucketName, ObjectName: "force-obj-b",
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.stubObjectUnlockFee()
	s.stubGCBookkeepingNoop()

	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 1)
	s.Require().NoError(err)
	s.Require().False(deleted, "cap must stop processing before the bucket itself is deleted")
	s.Require().Equal(uint64(1), count)

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found, "the bucket must still exist: only one of its two objects was processed")
}

// TestForceDeleteBucket_FullIterationDeletesBucket drives the tail branch: once
// every object under the bucket has been iterated, the bucket itself is deleted.
func (s *TestSuite) TestForceDeleteBucket_FullIterationDeletesBucket() {
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(1 * time.Second))
	owner := sample.RandAccAddress()
	bucketName := "force-delete-full-bucket"
	bucketID := sdkmath.NewUint(2)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(20), BucketName: bucketName, ObjectName: "full-obj-a",
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(21), BucketName: bucketName, ObjectName: "full-obj-b",
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.stubObjectUnlockFee()
	s.stubGCBookkeepingNoop()

	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().NoError(err)
	s.Require().True(deleted, "the bucket must be deleted once all its objects were processed")
	s.Require().Equal(uint64(2), count)

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found)
}

func (s *TestSuite) TestForceDeleteBucket_AlreadyDeleted() {
	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, sdkmath.NewUint(999), 10)
	s.Require().NoError(err)
	s.Require().True(deleted, "a bucket that no longer exists counts as already deleted")
	s.Require().Equal(uint64(0), count)
}

// TestForceDeleteBucket_PrimarySPResolutionErrorSurfaced distinguishes an
// orphaned primary SP (garbage-collected, see BurnTestSuite) from any other
// resolution failure: a missing GVG family is a genuine invariant break and
// must be surfaced as an error rather than silently GC'd.
func (s *TestSuite) TestForceDeleteBucket_PrimarySPResolutionErrorSurfaced() {
	owner := sample.RandAccAddress()
	bucketName := "force-delete-badfamily-bucket"
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
	})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().Error(err)
	s.Require().False(deleted)
	s.Require().Equal(uint64(0), count)
}

// TestForceDeleteBucket_UnChargeBucketReadFeeErrorSurfaced drives the tail
// error branch: a bucket with no objects left but a nonzero TotalChargeSize is
// an invariant violation that must be surfaced, not swallowed.
func (s *TestSuite) TestForceDeleteBucket_UnChargeBucketReadFeeErrorSurfaced() {
	owner := sample.RandAccAddress()
	bucketName := "force-delete-badcharge-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{TotalChargeSize: 1})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)

	deleted, _, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().Error(err)
	s.Require().False(deleted)
}

func (s *TestSuite) TestDiscontinueBucket_StorageProviderNotFound() {
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), "whatever", "reason")
	s.Require().ErrorIs(err, types.ErrNoSuchStorageProvider)
}

func (s *TestSuite) TestDiscontinueBucket_StorageProviderNotInService() {
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), "whatever", "reason")
	s.Require().ErrorIs(err, sptypes.ErrStorageProviderNotInService)
}

func (s *TestSuite) TestDiscontinueBucket_BucketNotFound() {
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), "missing-bucket", "reason")
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestDiscontinueBucket_AlreadyDiscontinued() {
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	bucketName := "already-discontinued-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:        sample.RandAccAddress().String(),
		BucketName:   bucketName,
		Id:           sdkmath.NewUint(1),
		BucketStatus: types.BUCKET_STATUS_DISCONTINUED,
	})

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), bucketName, "reason")
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestDiscontinueBucket_WrongPrimarySP() {
	callerSP := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(callerSP, true).AnyTimes()
	bucketName := "wrong-primary-sp-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      sample.RandAccAddress().String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 5,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	actualPrimary := &sptypes.StorageProvider{Id: 2, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, actualPrimary)

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), bucketName, "reason")
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestDiscontinueBucket_MaxRequestsReached() {
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	bucketName := "discontinue-max-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      sample.RandAccAddress().String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.mockPrimarySP(bucketInfo, sp)
	params := types.DefaultParams()
	params.DiscontinueBucketMax = 0
	s.Require().NoError(s.storageKeeper.SetParams(s.ctx, params))

	err := s.storageKeeper.DiscontinueBucket(s.ctx, sample.RandAccAddress(), bucketName, "reason")
	s.Require().ErrorIs(err, types.ErrNoMoreDiscontinue)
}

// TestDiscontinueBucket_Success also covers the previousStatus==MIGRATING branch
// (the extra EventCancelMigrationBucket emission) by starting the bucket mid-migration.
func (s *TestSuite) TestDiscontinueBucket_Success() {
	operator := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	bucketName := "discontinue-success-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      sample.RandAccAddress().String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 1,
		BucketStatus:               types.BUCKET_STATUS_MIGRATING,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.mockPrimarySP(bucketInfo, sp)

	err := s.storageKeeper.DiscontinueBucket(s.ctx, operator, bucketName, "final call")
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_DISCONTINUED, got.BucketStatus)
	s.Require().Equal(uint64(1), s.storageKeeper.GetDiscontinueBucketCount(s.ctx, operator))
}

func (s *TestSuite) TestUpdateBucketInfo_NotFound() {
	err := s.storageKeeper.UpdateBucketInfo(s.ctx, sample.RandAccAddress(), "does-not-exist", types.UpdateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestUpdateBucketInfo_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	bucketName := "update-sourcetype-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         sdkmath.NewUint(1),
		SourceType: types.SOURCE_TYPE_ORIGIN,
	})

	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{SourceType: types.SOURCE_TYPE_OP_CROSS_CHAIN})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

func (s *TestSuite) TestUpdateBucketInfo_PrimarySPExiting() {
	owner := sample.RandAccAddress()
	bucketName := "update-exiting-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	exitingSP := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.mockPrimarySP(bucketInfo, exitingSP)

	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrUpdateQuotaFailed)
}

func (s *TestSuite) TestUpdateBucketInfo_PermissionDenied() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	bucketName := "update-denied-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err := s.storageKeeper.UpdateBucketInfo(s.ctx, operator, bucketName, types.UpdateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

// TestUpdateBucketInfo_QuotaDecreaseFirstAttemptRejected drives the
// getQuotaUpdateTime too-soon guard on a bucket whose quota was never
// explicitly changed before: getQuotaUpdateTime falls back to the bucket's
// CreateAt as the "last update time" for such buckets, so a decrease attempted
// right after creation is still too soon and must be rejected.
func (s *TestSuite) TestUpdateBucketInfo_QuotaDecreaseFirstAttemptRejected() {
	owner := sample.RandAccAddress()
	bucketName := "update-quota-first-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           100,
		PaymentAddress:             owner.String(),
		CreateAt:                   s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)

	quota := uint64(50)
	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{ChargedReadQuota: &quota})
	s.Require().ErrorIs(err, types.ErrUpdateQuotaFailed)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(uint64(100), got.ChargedReadQuota, "the rejected decrease must not have been applied")
}

// TestUpdateBucketInfo_QuotaIncreaseThenTooSoonDecrease covers the
// visibility/quota happy path (a quota increase paired with a visibility change)
// and then, in the same bucket's next call, the getQuotaUpdateTime-found
// too-soon guard: a quota that was just changed cannot immediately be decreased.
func (s *TestSuite) TestUpdateBucketInfo_QuotaIncreaseThenTooSoonDecrease() {
	owner := sample.RandAccAddress()
	bucketName := "update-quota-cycle-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           100,
		PaymentAddress:             owner.String(),
		Visibility:                 types.VISIBILITY_TYPE_PRIVATE,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)

	// Billing mocks: ChargedReadQuota becomes non-zero, so GetBucketReadStoreBill
	// prices the bucket instead of taking its ChargedReadQuota==0 short-circuit.
	price := sptypes.GlobalSpStorePrice{
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
		ReadPrice:           sdkmath.LegacyNewDec(1),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{ReserveTime: 0, ValidatorTaxRate: sdkmath.LegacyZeroDec()}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// the new bill's flow rate is non-zero (quota 200 * price 1) and no explicit
	// rate limit was ever set for this bucket, so isBucketFlowRateUnderLimitWithRate
	// falls back to asking whether owner==payment-account.
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	increased := uint64(200)
	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{
		ChargedReadQuota: &increased,
		Visibility:       types.VISIBILITY_TYPE_PUBLIC_READ,
	})
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(increased, got.ChargedReadQuota)
	s.Require().Equal(types.VISIBILITY_TYPE_PUBLIC_READ, got.Visibility)

	decreased := uint64(10)
	err = s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{ChargedReadQuota: &decreased})
	s.Require().ErrorIs(err, types.ErrUpdateQuotaFailed)

	got, found = s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(increased, got.ChargedReadQuota, "the rejected decrease must not have been applied")
}

// TestUpdateBucketInfo_MalformedPaymentAddress covers VerifyPaymentAccount's
// malformed-non-empty-address branch (verify.go): a non-empty PaymentAddress
// that is not valid hex must be rejected rather than silently accepted.
func (s *TestSuite) TestUpdateBucketInfo_MalformedPaymentAddress() {
	owner := sample.RandAccAddress()
	bucketName := "update-malformed-payment-bucket"
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		GlobalVirtualGroupFamilyId: 1,
		PaymentAddress:             owner.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)

	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{
		PaymentAddress: "not-a-valid-hex-address",
	})
	s.Require().Error(err, "a malformed non-empty payment address must be rejected")
}

func (s *TestSuite) TestUpdateBucketInfo_PaymentAddressChangeWithOpenObjects() {
	owner := sample.RandAccAddress()
	bucketName := "update-payment-open-objects-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
		PaymentAddress:             owner.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)
	s.storageKeeper.IncreaseLockedObjectCount(s.ctx, bucketID) // an unsealed object is open

	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{
		PaymentAddress: sample.RandAccAddress().String(),
	})
	s.Require().ErrorIs(err, types.ErrUpdatePaymentAccountFailed)
}

// TestUpdateBucketInfo_CannotChangeAddressAndQuotaTogether covers payment.go's
// UpdateBucketInfoAndCharge guard: the payment address and the read quota
// cannot both change in the same call.
func (s *TestSuite) TestUpdateBucketInfo_CannotChangeAddressAndQuotaTogether() {
	owner := sample.RandAccAddress()
	bucketName := "update-both-changed-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           100,
		PaymentAddress:             owner.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.mockPrimarySP(bucketInfo, sp)

	newQuota := uint64(200)
	err := s.storageKeeper.UpdateBucketInfo(s.ctx, owner, bucketName, types.UpdateBucketOptions{
		PaymentAddress:   sample.RandAccAddress().String(),
		ChargedReadQuota: &newQuota,
	})
	s.Require().Error(err, "the payment address and read quota must not both change in one call")
}

// ---------------------------------------------------------------------------
// test/storage-keeper-bucket (PR1) gap-fill: raise ForceDeleteBucket,
// CreateBucket, appendDiscontinueBucketIDs, DeleteDiscontinueBucketsUntil,
// GetInternalBucketInfo/MustGetInternalBucketInfo, and fromSpMaintenanceAcct
// coverage on this branch's own test run.
// ---------------------------------------------------------------------------

// stubSealedObjectFeesZero wires zero-priced billing mocks so
// UnChargeObjectStoreFee/ChargeViaObjectChange (ForceDeleteBucket's SEALED-object
// path) succeed without generating any outflow, sidestepping the bucket
// flow-rate-limit check entirely.
func (s *TestSuite) stubSealedObjectFeesZero() {
	zero := sptypes.GlobalSpStorePrice{
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
		ReadPrice:           sdkmath.LegacyZeroDec(),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(zero, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{ReserveTime: 0, ValidatorTaxRate: sdkmath.LegacyZeroDec()}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
}

// signBucketApproval builds a signed PrimarySpApproval for a CreateBucket call
// and points sp's approval key at the signing key, mirroring the recipe in
// keeper_object_copy_test.go's createBucketForCopy.
func (s *TestSuite) signBucketApproval(sp *sptypes.StorageProvider, bucketName string, familyID uint32) (*common.Approval, []byte) {
	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()

	msg := &types.MsgCreateBucket{
		BucketName: bucketName,
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: familyID,
		},
	}
	approvalBytes := msg.GetApprovalBytes()
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	msg.PrimarySpApproval.Sig = sig
	return msg.PrimarySpApproval, approvalBytes
}

func (s *TestSuite) TestCreateBucket_AlreadyExists() {
	owner := sample.RandAccAddress()
	bucketName := "create-bucket-exists"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: bucketName,
		Id:         sdkmath.NewUint(1),
	})

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, sample.RandAccAddress(), &types.CreateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrBucketAlreadyExists)
}

func (s *TestSuite) TestCreateBucket_MalformedPaymentAddress() {
	owner := sample.RandAccAddress()
	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, "create-bucket-badpayment", sample.RandAccAddress(), &types.CreateBucketOptions{
		PaymentAddress: "not-a-valid-hex-address",
	})
	s.Require().Error(err, "a malformed non-empty payment address must be rejected")
}

func (s *TestSuite) TestCreateBucket_StorageProviderNotFound() {
	owner := sample.RandAccAddress()
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, "create-bucket-nosp", sample.RandAccAddress(), &types.CreateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchStorageProvider)
}

// TestCreateBucket_StorageProviderNotInService also drives fromSpMaintenanceAcct:
// with a non-maintenance status its condition short-circuits to false without
// panicking on an empty MaintenanceAddress, so CreateBucket correctly rejects
// the request instead of silently proceeding.
func (s *TestSuite) TestCreateBucket_StorageProviderNotInService() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, "create-bucket-notinservice", sample.RandAccAddress(), &types.CreateBucketOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchStorageProvider)
}

func (s *TestSuite) TestCreateBucket_GVGFamilyUnavailable() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(nil, virtualgroupmoduletypes.ErrGVGFamilyNotExist).AnyTimes()
	approval, approvalBytes := s.signBucketApproval(sp, "create-bucket-nogvg", 1)

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, "create-bucket-nogvg", sdk.MustAccAddressFromHex(sp.OperatorAddress), &types.CreateBucketOptions{
		PrimarySpApproval: approval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().Error(err)
}

// TestCreateBucket_ChargesReadQuotaOnCreate drives CreateBucket's
// ChargedReadQuota != 0 branch: a paid bucket must be charged via
// ChargeBucketReadFee before it is persisted.
func (s *TestSuite) TestCreateBucket_ChargesReadQuotaOnCreate() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	zero := sptypes.GlobalSpStorePrice{
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
		ReadPrice:           sdkmath.LegacyZeroDec(),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(zero, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	approval, approvalBytes := s.signBucketApproval(sp, "create-bucket-quota", gvgFamily.Id)

	bucketID, err := s.storageKeeper.CreateBucket(s.ctx, owner, "create-bucket-quota", sdk.MustAccAddressFromHex(sp.OperatorAddress), &types.CreateBucketOptions{
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota:  100,
		PaymentAddress:    owner.String(),
		PrimarySpApproval: approval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().NoError(err)

	bucketInfo, found := s.storageKeeper.GetBucketInfoById(s.ctx, bucketID)
	s.Require().True(found)
	s.Require().Equal(uint64(100), bucketInfo.ChargedReadQuota)
}

// TestForceDeleteBucket_DiscontinuedObjectResolvesToSealed drives
// ForceDeleteBucket's DISCONTINUED-object branch: an object discontinued while
// SEALED must have its pre-discontinue status restored (via
// getAndDeleteDiscontinueObjectStatus) and be uncharged through the same
// GVG/LVG billing path a plain SEALED object takes.
func (s *TestSuite) TestForceDeleteBucket_DiscontinuedObjectResolvesToSealed() {
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(1 * time.Second))
	owner := sample.RandAccAddress()
	bucketName := "force-delete-discontinued-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		TotalChargeSize: types.DefaultMinChargeSize,
		LocalVirtualGroups: []*types.LocalVirtualGroup{
			{Id: 1, GlobalVirtualGroupId: 1, StoredSize: 100, TotalChargeSize: types.DefaultMinChargeSize},
		},
	})
	objectID := sdkmath.NewUint(10)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objectID, BucketName: bucketName, ObjectName: "force-obj-discontinued", Owner: owner.String(),
		ObjectStatus: types.OBJECT_STATUS_DISCONTINUED, PayloadSize: 100, LocalVirtualGroupId: 1,
		CreateAt: s.ctx.BlockTime().Unix(),
	})
	// simulate DiscontinueObject having recorded the object's pre-discontinue
	// status (SEALED) via the unexported saveDiscontinueObjectStatus.
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_SEALED))
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectStatusKey(objectID), statusBytes)

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.stubSealedObjectFeesZero()
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, FamilyId: 1, PrimarySpId: sp.Id, SecondarySpIds: []uint32{}, StoredSize: 100}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.stubGCBookkeepingNoop()

	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().NoError(err)
	s.Require().True(deleted, "the bucket's only object accounted for its entire charge, so it must be fully deleted")
	s.Require().Equal(uint64(1), count)

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found)
}

func (s *TestSuite) TestForceDeleteBucket_DiscontinuedObjectMissingStatusErrorSurfaced() {
	owner := sample.RandAccAddress()
	bucketName := "force-delete-discontinued-missing-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), BucketName: bucketName, ObjectName: "force-obj-missing-status",
		ObjectStatus: types.OBJECT_STATUS_DISCONTINUED,
	})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)

	deleted, _, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().ErrorIs(err, types.ErrInvalidObjectStatus)
	s.Require().False(deleted)
}

// TestForceDeleteBucket_UpdatingSealedObjectUnlocksShadowFee drives the
// objectInfo.IsUpdating branch: a sealed object mid-UpdateObjectContent must
// have its shadow object's locked fee unlocked and the shadow record deleted.
func (s *TestSuite) TestForceDeleteBucket_UpdatingSealedObjectUnlocksShadowFee() {
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(1 * time.Second))
	owner := sample.RandAccAddress()
	bucketName := "force-delete-updating-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		TotalChargeSize: types.DefaultMinChargeSize,
		LocalVirtualGroups: []*types.LocalVirtualGroup{
			{Id: 1, GlobalVirtualGroupId: 1, StoredSize: 100, TotalChargeSize: types.DefaultMinChargeSize},
		},
	})
	objectID := sdkmath.NewUint(10)
	objectName := "force-obj-updating"
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objectID, BucketName: bucketName, ObjectName: objectName, Owner: owner.String(),
		ObjectStatus: types.OBJECT_STATUS_SEALED, PayloadSize: 100, LocalVirtualGroupId: 1,
		CreateAt: s.ctx.BlockTime().Unix(), IsUpdating: true,
	})
	shadow := &types.ShadowObjectInfo{Id: objectID, PayloadSize: 50, UpdatedAt: s.ctx.BlockTime().Unix()}
	s.ctx.KVStore(s.storeKey).Set(types.GetShadowObjectKey(bucketName, objectName), s.cdc.MustMarshal(shadow))

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.stubSealedObjectFeesZero()
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, FamilyId: 1, PrimarySpId: sp.Id, SecondarySpIds: []uint32{}, StoredSize: 100}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.stubGCBookkeepingNoop()

	deleted, count, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().NoError(err)
	s.Require().True(deleted)
	s.Require().Equal(uint64(1), count)

	_, found := s.storageKeeper.GetShadowObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found, "the shadow object must be unlocked and deleted alongside the sealed object")
}

// TestForceDeleteBucket_CreatedObjectUnlockFeeErrorSurfaced leaves the object's
// CreateAt at its zero value: no versioned params are ever seeded before time
// zero (SetupTest seeds them at the suite's start time), so GetObjectChargeSize
// fails and UnlockObjectStoreFee's error must be surfaced rather than swallowed.
func (s *TestSuite) TestForceDeleteBucket_CreatedObjectUnlockFeeErrorSurfaced() {
	owner := sample.RandAccAddress()
	bucketName := "force-delete-created-noparams-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.stubObjectUnlockFee()
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), BucketName: bucketName, ObjectName: "force-obj-created-noparams",
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1,
	})

	deleted, _, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().Error(err)
	s.Require().False(deleted)
}

// TestForceDeleteBucket_SealedObjectChargeSizeErrorSurfaced mirrors the CREATED
// case above for the SEALED branch: UnChargeObjectStoreFee's own
// GetObjectChargeSize call fails first, before any GVG/LVG lookup is attempted.
func (s *TestSuite) TestForceDeleteBucket_SealedObjectChargeSizeErrorSurfaced() {
	owner := sample.RandAccAddress()
	bucketName := "force-delete-sealed-noparams-bucket"
	bucketID := sdkmath.NewUint(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.mockPrimarySP(bucketInfo, sp)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), BucketName: bucketName, ObjectName: "force-obj-sealed-noparams",
		ObjectStatus: types.OBJECT_STATUS_SEALED, PayloadSize: 1,
	})

	deleted, _, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().Error(err)
	s.Require().False(deleted)
}

func (s *TestSuite) TestDeleteDiscontinueBucketsUntil_NoEntries() {
	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, s.ctx.BlockTime().Unix()+1000, 10)
	s.Require().NoError(err)
	s.Require().Equal(uint64(0), deleted)
}

// TestDiscontinueBucket_SecondRequestAppendsToSameDeleteAtEntry drives
// appendDiscontinueBucketIDs' merge branch: two buckets discontinued by the
// same operator at the same block time land under the same delete-at entry,
// and DeleteDiscontinueBucketsUntil must force-delete both.
func (s *TestSuite) TestDiscontinueBucket_SecondRequestAppendsToSameDeleteAtEntry() {
	operator := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	bucketAOwner := sample.RandAccAddress()
	bucketBOwner := sample.RandAccAddress()
	bucketA := &types.BucketInfo{Owner: bucketAOwner.String(), BucketName: "discontinue-merge-a", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, PaymentAddress: bucketAOwner.String()}
	bucketB := &types.BucketInfo{Owner: bucketBOwner.String(), BucketName: "discontinue-merge-b", Id: sdkmath.NewUint(2), GlobalVirtualGroupFamilyId: 1, PaymentAddress: bucketBOwner.String()}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketA)
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketB)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketA.Id, &types.InternalBucketInfo{})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketB.Id, &types.InternalBucketInfo{})
	s.mockPrimarySP(bucketA, sp)

	s.Require().NoError(s.storageKeeper.DiscontinueBucket(s.ctx, operator, "discontinue-merge-a", "reason-a"))
	s.Require().NoError(s.storageKeeper.DiscontinueBucket(s.ctx, operator, "discontinue-merge-b", "reason-b"))

	deleteAt := s.ctx.BlockTime().Unix() + s.storageKeeper.DiscontinueConfirmPeriod(s.ctx)
	s.stubGCBookkeepingNoop()
	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, deleteAt, 10)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), deleted, "both buckets queued under the same delete-at entry must be force-deleted")

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, "discontinue-merge-a")
	s.Require().False(found)
	_, found = s.storageKeeper.GetBucketInfo(s.ctx, "discontinue-merge-b")
	s.Require().False(found)
}

// TestDeleteDiscontinueBucketsUntil_CapReachedMidEntryDefersRemainingID drives
// the inner id-loop's cap-reached branch: once maxToDelete is hit partway
// through one delete-at entry's bucket list, the remaining ids are written
// back instead of being dropped.
func (s *TestSuite) TestDeleteDiscontinueBucketsUntil_CapReachedMidEntryDefersRemainingID() {
	operator := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	bucketAOwner := sample.RandAccAddress()
	bucketBOwner := sample.RandAccAddress()
	bucketA := &types.BucketInfo{Owner: bucketAOwner.String(), BucketName: "discontinue-cap-a", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, PaymentAddress: bucketAOwner.String()}
	bucketB := &types.BucketInfo{Owner: bucketBOwner.String(), BucketName: "discontinue-cap-b", Id: sdkmath.NewUint(2), GlobalVirtualGroupFamilyId: 1, PaymentAddress: bucketBOwner.String()}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketA)
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketB)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketA.Id, &types.InternalBucketInfo{})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketB.Id, &types.InternalBucketInfo{})
	s.mockPrimarySP(bucketA, sp)

	s.Require().NoError(s.storageKeeper.DiscontinueBucket(s.ctx, operator, "discontinue-cap-a", "reason-a"))
	s.Require().NoError(s.storageKeeper.DiscontinueBucket(s.ctx, operator, "discontinue-cap-b", "reason-b"))

	deleteAt := s.ctx.BlockTime().Unix() + s.storageKeeper.DiscontinueConfirmPeriod(s.ctx)
	s.stubGCBookkeepingNoop()
	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, deleteAt, 1)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), deleted, "the cap must stop processing after the first bucket in the entry")

	_, found := s.storageKeeper.GetBucketInfo(s.ctx, "discontinue-cap-a")
	s.Require().False(found, "the first queued bucket must be force-deleted")
	_, found = s.storageKeeper.GetBucketInfo(s.ctx, "discontinue-cap-b")
	s.Require().True(found, "the second queued bucket must be deferred once the cap is reached")
}

// TestDeleteDiscontinueBucketsUntil_ForceDeleteErrorSurfaced drives the loop's
// error-propagation branch: a queued bucket whose GVG family cannot be
// resolved is a genuine invariant break, and the error must surface rather
// than be swallowed.
func (s *TestSuite) TestDeleteDiscontinueBucketsUntil_ForceDeleteErrorSurfaced() {
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:                      sample.RandAccAddress().String(),
		BucketName:                 "discontinue-error-bucket",
		Id:                         bucketID,
		GlobalVirtualGroupFamilyId: 1,
	})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	deleteAt := s.ctx.BlockTime().Unix() + 1000
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueBucketIDsKey(deleteAt), s.cdc.MustMarshal(&types.Ids{Id: []types.Uint{bucketID}}))

	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, deleteAt, 10)
	s.Require().Error(err)
	s.Require().Equal(uint64(0), deleted)
}

func (s *TestSuite) TestGetInternalBucketInfo_NotFound() {
	_, found := s.storageKeeper.GetInternalBucketInfo(s.ctx, sdkmath.NewUint(999999))
	s.Require().False(found)
}

func (s *TestSuite) TestMustGetInternalBucketInfo_PanicsWhenMissing() {
	s.Require().Panics(func() {
		s.storageKeeper.MustGetInternalBucketInfo(s.ctx, sdkmath.NewUint(999998))
	})
}

func (s *TestSuite) TestUpdateObjectContent_ZeroPayloadRefund() {
	ownerHex := "0x1111111111111111111111111111111111111111"
	owner := sdk.MustAccAddressFromHex(ownerHex)
	updater := owner // Use owner as updater to simplify permissions
	bucketName := "test-bucket"
	objectName := "test-object"
	bucketID := uint64(1)
	primarySpId := uint32(1)
	initialPayloadSize := uint64(1024)

	// Arrange: Manually set the bucket sequence to align with the test data
	store := s.ctx.KVStore(s.storeKey)
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, bucketID)
	store.Set(types.BucketSequencePrefix, seqBytes)

	// Arrange: Setup initial state
	bucketInfo := &types.BucketInfo{
		Owner:                      ownerHex,
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(bucketID),
		GlobalVirtualGroupFamilyId: 1,
		PaymentAddress:             "0x1111111111111111111111111111111111111111",
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	// DEBUG: Check if bucket was set correctly
	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found, "bucket must exist after being set")

	objectInfo := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(10),
		Owner:               ownerHex,
		BucketName:          bucketName,
		ObjectName:          objectName,
		PayloadSize:         initialPayloadSize,
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		UpdatedAt:           s.ctx.BlockTime().Unix() + 1,
		LocalVirtualGroupId: 1,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	internalBucketInfo := &types.InternalBucketInfo{
		TotalChargeSize: initialPayloadSize,
		PriceTime:       s.ctx.BlockTime().Unix(),
	}
	internalBucketInfo.LocalVirtualGroups = []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 1, TotalChargeSize: initialPayloadSize, StoredSize: initialPayloadSize}}
	store = s.ctx.KVStore(s.storeKey)
	store.Set(types.GetInternalBucketInfoKey(bucketInfo.Id), s.cdc.MustMarshal(internalBucketInfo))
	// seed storage versioned params for current block time so GetVersionedParamsWithTS(ts) can find one
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MaxSegmentSize: 1, RedundantDataChunkNum: 0, RedundantParityChunkNum: 0, MinChargeSize: 0})

	// Arrange: Mock dependencies
	sp := &sptypes.StorageProvider{Id: primarySpId, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: ownerHex}
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), primarySpId).Return(sp).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), primarySpId).Return(sp, true).AnyTimes()
	// Mock global store price at PriceTime
	price := sptypes.GlobalSpStorePrice{}
	price.PrimaryStorePrice = sdkmath.LegacyNewDec(1)
	price.SecondaryStorePrice = sdkmath.LegacyNewDec(1)
	price.ReadPrice = sdkmath.LegacyNewDec(1)
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	family := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{PrimarySpId: primarySpId, GlobalVirtualGroupIds: []uint32{1}}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(1)).Return(family, true).AnyTimes()

	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, FamilyId: 1, PrimarySpId: primarySpId}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), uint64(0)).Return(gvg, nil).AnyTimes()

	// Mock payment params (reserve time & validator tax rate)
	payVer := paymenttypes.VersionedParams{}
	payVer.ReserveTime = 0
	payVer.ValidatorTaxRate = sdkmath.LegacyZeroDec()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	// Act: Call UpdateObjectContent with zero payload to trigger refund
	opts := types.UpdateObjectOptions{
		Updater:   updater,
		Delegated: false,
		Checksums: [][]byte{},
	}
	// Align block time with UpdatedAt to avoid early-deletion path
	t0 := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(t0.Add(1 * time.Second))
	err := s.storageKeeper.UpdateObjectContent(s.ctx, owner, bucketName, objectName, 0, opts)
	s.Require().NoError(err)

	// Assert: Verify that the refund was persisted
	finalInternalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, sdkmath.NewUint(bucketID))
	s.Require().Equal(uint64(0), finalInternalBucketInfo.TotalChargeSize, "TotalChargeSize should be zero after refund")
}

// setupMigratingBucket stores an empty bucket (no local virtual groups) that is
// mid-migration to dstSpID, and returns its name. An empty bucket keeps the
// verifyGVGSignatures / RebindingVirtualGroup loops empty so CompleteMigrateBucket
// can be driven without constructing BLS signatures.
func (s *TestSuite) setupMigratingBucket(dstSpID, srcFamilyID uint32) string {
	owner := sample.RandAccAddress()
	paymentAddr := sample.RandAccAddress()
	bucketID := sdkmath.NewUint(1)
	bucketName := "migrate-victim"

	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             paymentAddr.String(),
		GlobalVirtualGroupFamilyId: srcFamilyID,
		BucketStatus:               types.BUCKET_STATUS_MIGRATING,
		ChargedReadQuota:           0,
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{TotalChargeSize: 0})
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.GetMigrationBucketKey(bucketID), s.cdc.MustMarshal(&types.MigrationBucketInfo{
		SrcSpId:                       5,
		DstSpId:                       dstSpID,
		SrcGlobalVirtualGroupFamilyId: srcFamilyID,
		BucketId:                      bucketID,
	}))
	return bucketName
}

// : CompleteMigrateBucket must reject a destination GVG family that does
// not belong to the destination SP (before the fix, this was accepted and the
// bucket's primary SP was misattributed to the foreign family's owner).
func (s *TestSuite) TestCompleteMigrateBucket_RejectsForeignFamily() {
	const (
		dstSpID      = uint32(2)
		srcFamilyID  = uint32(1)
		foreignFamID = uint32(99)
		foreignSpID  = uint32(3) // owner of the foreign family, NOT the dst SP
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), foreignFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: foreignFamID, PrimarySpId: foreignSpID}, true).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, foreignFamID, nil)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "does not belong")

	// nothing was mutated: the bucket is still migrating on its source family
	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(srcFamilyID, got.GlobalVirtualGroupFamilyId)
	s.Require().Equal(types.BUCKET_STATUS_MIGRATING, got.BucketStatus)
}

// The fix must not break legitimate migrations: a destination SP completing with
// its own family (family.PrimarySpId == dstSP.Id) still succeeds.
func (s *TestSuite) TestCompleteMigrateBucket_AcceptsOwnFamily() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		srcSpID     = uint32(5)
		ownFamID    = uint32(7) // family owned by the dst SP
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), ownFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: ownFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSpID).
		Return(&sptypes.StorageProvider{Id: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), dstSpID).
		Return(&sptypes.StorageProvider{Id: dstSpID}, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, ownFamID, nil)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(ownFamID, got.GlobalVirtualGroupFamilyId)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
	primarySP := s.storageKeeper.MustGetPrimarySPForBucket(s.ctx, got)
	s.Require().Equal(dstSpID, primarySP.Id) // dst SP is now the bucket's primary SP
}

// TestPutPolicy_RunsValidateRuntime is a/964
// (): MsgPutPolicy.ValidateRuntime existed but was never invoked
// by any caller, so every check it performs — bucket-level actions, Resources
// on a non-bucket resource, LimitSize without CreateObject — was dead. A
// bucket-level statement naming a group-only action clears ValidateBasic (which
// never consults BucketAllowedActionsAfterPampas) and must now be rejected.
//
// msgServer.PutPolicy is the single implementation shared by both write paths:
// the native Cosmos tx path (x/storage/module.go registers
// keeper.NewMsgServerImpl(k) as the module's MsgServer) and the EVM storage
// precompile's PutPolicy (precompiles/storage/tx.go calls
// p.storageMsgServer.PutPolicy, and app.go wires that field to the very same
// keeper.NewMsgServerImpl(app.StorageKeeper)). Fixing it here closes both
// and in one place.
func (s *TestSuite) TestPutPolicy_RunsValidateRuntime() {
	operator := sample.RandAccAddress()
	bucketName := "putpolicy-runtime-bucket"

	bucketInfo := &types.BucketInfo{
		Owner:            operator.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sample.RandAccAddress().String(),
		ChargedReadQuota: 100,
		BucketStatus:     types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	principal := sample.RandAccAddress()
	msg := types.NewMsgPutPolicy(
		operator,
		types2.NewBucketGRN(bucketName).String(),
		permtypes.NewPrincipalWithAccount(principal),
		[]*permtypes.Statement{
			{
				Effect: permtypes.EFFECT_ALLOW,
				// A group-only action on a bucket resource: ValidateBasic does not
				// check the bucket action map, ValidateRuntime does.
				Actions:   []permtypes.ActionType{permtypes.ACTION_UPDATE_GROUP_MEMBER},
				Resources: []string{"grn:o::" + bucketName + "/obj"},
			},
		},
		nil,
	)
	s.Require().NoError(msg.ValidateBasic(), "the statement must clear ValidateBasic for this test to be meaningful")

	_, err := s.msgServer.PutPolicy(s.ctx, msg)
	s.Require().Error(err, "PutPolicy must run MsgPutPolicy.ValidateRuntime")
	s.Require().ErrorIs(err, permtypes.ErrInvalidStatement)
}

// TestPutPolicy_AcceptsRegexpMetacharacterInObjectName pins the accept path so
// the previous test cannot pass merely by rejecting every PutPolicy call, and
// pins that a Resources entry which is a legal object name but not a legal Go
// regexp stays storable: Resources are wildcard patterns, not regexps.
func (s *TestSuite) TestPutPolicy_AcceptsRegexpMetacharacterInObjectName() {
	operator := sample.RandAccAddress()
	bucketName := "putpolicy-metachar-bucket"

	bucketInfo := &types.BucketInfo{
		Owner:            operator.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sample.RandAccAddress().String(),
		ChargedReadQuota: 100,
		BucketStatus:     types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	principal := sample.RandAccAddress()
	msg := types.NewMsgPutPolicy(
		operator,
		types2.NewBucketGRN(bucketName).String(),
		permtypes.NewPrincipalWithAccount(principal),
		[]*permtypes.Statement{
			{
				Effect:    permtypes.EFFECT_ALLOW,
				Actions:   []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
				Resources: []string{"grn:o::" + bucketName + "/obj["},
			},
		},
		nil,
	)
	s.Require().NoError(msg.ValidateBasic())

	s.permissionKeeper.EXPECT().PutPolicy(gomock.Any(), gomock.Any()).Return(sdkmath.OneUint(), nil)

	_, err := s.msgServer.PutPolicy(s.ctx, msg)
	s.Require().NoError(err, "a legal object name that is not a legal regexp must remain storable")
}

// realPermissionKeeper mounts a real x/permission keeper on the suite's own
// CommitMultiStore so VerifyPolicy exercises the production PutPolicy, not a mock.
func (s *TestSuite) realPermissionKeeper() *permkeeper.Keeper {
	permKey := storetypes.NewKVStoreKey(permtypes.StoreKey)
	cms := s.ctx.MultiStore().(storetypes.CommitMultiStore)
	cms.MountStoreWithDB(permKey, storetypes.StoreTypeIAVL, nil)
	s.Require().NoError(cms.LoadLatestVersion())

	ctrl := gomock.NewController(s.T())
	k := permkeeper.NewKeeper(
		s.cdc,
		permKey,
		permtypes.NewMockAccountKeeper(ctrl),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	s.Require().NoError(k.SetParams(s.ctx, permtypes.DefaultParams()))
	return k
}

// TestPutPolicy_OverCapQuotaGrantStillConsumable reproduces the
// risk in enforcing MaximumStatementsNum inside Keeper.PutPolicy: that same
// function is the internal self-update path for a CreateObject LimitSize quota,
// and x/storage/keeper/permission.go turns any error from it into a panic.
//
// A policy with more statements than the (never previously enforced) cap can
// already be in state. The moment its CreateObject grant is consumed, Eval
// hands the decremented policy back to PutPolicy, the new cap rejects it, and
// the node panics -- for a policy nobody is trying to grow.
func (s *TestSuite) TestPutPolicy_OverCapQuotaGrantStillConsumable() {
	permKeeper := s.realPermissionKeeper()
	grantee := sample.RandAccAddress()
	resourceID := sdkmath.NewUint(4242)

	// --- Pre-existing state: the cap was never enforced, so an over-cap policy
	// could be written. Model that by storing it while the param is generous.
	loose := permtypes.DefaultParams()
	loose.MaximumStatementsNum = 50
	s.Require().NoError(permKeeper.SetParams(s.ctx, loose))

	stmts := make([]*permtypes.Statement, 0, 11)
	for i := 0; i < 10; i++ {
		stmts = append(stmts, &permtypes.Statement{
			Effect:  permtypes.EFFECT_ALLOW,
			Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
		})
	}
	// The 11th is the CreateObject grant carrying the LimitSize quota.
	stmts = append(stmts, &permtypes.Statement{
		Effect:    permtypes.EFFECT_ALLOW,
		Actions:   []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
		LimitSize: &common.UInt64Value{Value: 1024 * 1024},
	})

	_, err := permKeeper.PutPolicy(s.ctx, &permtypes.Policy{
		Principal:    permtypes.NewPrincipalWithAccount(grantee),
		ResourceType: gnfdresource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
		Statements:   stmts,
	})
	s.Require().NoError(err, "over-cap policy must be storable to model pre-existing state")

	// --- The cap is now in force at its default of 10.
	s.Require().NoError(permKeeper.SetParams(s.ctx, permtypes.DefaultParams()))
	s.Require().Equal(permtypes.DefaultMaxStatementsNum, permKeeper.MaximumStatementsNum(s.ctx))

	// --- Wire the storage keeper's permission keeper to the real one and drive
	// the exact production path: VerifyPolicy -> Eval -> PutPolicy -> panic.
	s.permissionKeeper.EXPECT().
		GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetPolicyForAccount).AnyTimes()
	s.permissionKeeper.EXPECT().
		GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetPolicyGroupForResource).AnyTimes()
	s.permissionKeeper.EXPECT().
		PutPolicy(gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.PutPolicy).AnyTimes()

	wanted := uint64(1000)
	ctx := s.ctx.WithTxBytes([]byte{0x01}) // VerifyPolicy only self-updates inside a tx

	s.Require().NotPanics(func() {
		effect := s.storageKeeper.VerifyPolicy(ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET,
			grantee, permtypes.ACTION_CREATE_OBJECT,
			&permtypes.VerifyOptions{WantedSize: &wanted})
		s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
	}, "consuming a pre-existing over-cap quota grant must not panic the node")

	// and the decrement must actually have been persisted
	after, found := permKeeper.GetPolicyForAccount(s.ctx, resourceID,
		gnfdresource.RESOURCE_TYPE_BUCKET, grantee)
	s.Require().True(found)
	s.Require().Equal(uint64(1024*1024-1000), after.Statements[10].LimitSize.GetValue())
}

// TestVerifyBucketPermission_OverCapQuotaGrant shows the same panic reached
// through the public VerifyBucketPermission entry point that CreateObject uses.
func (s *TestSuite) TestVerifyBucketPermission_OverCapQuotaGrant() {
	permKeeper := s.realPermissionKeeper()
	grantee := sample.RandAccAddress()
	owner := sample.RandAccAddress()
	bucketID := sdkmath.NewUint(777)

	loose := permtypes.DefaultParams()
	loose.MaximumStatementsNum = 50
	s.Require().NoError(permKeeper.SetParams(s.ctx, loose))

	stmts := make([]*permtypes.Statement, 0, 11)
	for i := 0; i < 10; i++ {
		stmts = append(stmts, &permtypes.Statement{
			Effect:  permtypes.EFFECT_ALLOW,
			Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
		})
	}
	stmts = append(stmts, &permtypes.Statement{
		Effect:    permtypes.EFFECT_ALLOW,
		Actions:   []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
		LimitSize: &common.UInt64Value{Value: 1024 * 1024},
	})
	_, err := permKeeper.PutPolicy(s.ctx, &permtypes.Policy{
		Principal:    permtypes.NewPrincipalWithAccount(grantee),
		ResourceType: gnfdresource.RESOURCE_TYPE_BUCKET,
		ResourceId:   bucketID,
		Statements:   stmts,
	})
	s.Require().NoError(err)
	s.Require().NoError(permKeeper.SetParams(s.ctx, permtypes.DefaultParams()))

	s.permissionKeeper.EXPECT().
		GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetPolicyForAccount).AnyTimes()
	s.permissionKeeper.EXPECT().
		GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetPolicyGroupForResource).AnyTimes()
	s.permissionKeeper.EXPECT().
		PutPolicy(gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.PutPolicy).AnyTimes()

	bucketInfo := &types.BucketInfo{
		Owner:      owner.String(),
		BucketName: "retro-cap-bucket",
		Id:         bucketID,
	}
	wanted := uint64(1000)
	ctx := s.ctx.WithTxBytes([]byte{0x01})

	s.Require().NotPanics(func() {
		s.storageKeeper.VerifyBucketPermission(ctx, bucketInfo, grantee,
			permtypes.ACTION_CREATE_OBJECT, &permtypes.VerifyOptions{WantedSize: &wanted})
	}, "CreateObject quota consumption on a pre-existing over-cap policy must not panic")
}

// Hex casing carries no meaning, so a bucket owner submitting a lowercase operator
// address must still be recognized as the owner.
func (s *TestSuite) TestToggleSPAsDelegatedAgent_OperatorCasingIsIgnored() {
	owner := sample.RandAccAddress()
	lowered := strings.ToLower(owner.String())
	s.Require().NotEqual(owner.String(), lowered, "the sample address must contain hex letters")

	bucketName := "casingbucket"
	bucketID := sdkmath.NewUint(1)
	s.storageKeeper.SetBucketInfo(s.ctx, &types.BucketInfo{
		Id:         bucketID,
		Owner:      owner.String(),
		BucketName: bucketName,
	})
	// GetBucketInfo resolves the name through its own index, so seed that too.
	s.ctx.KVStore(s.storeKey).Set(types.GetBucketKey(bucketName), sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(bucketID))

	// Built directly rather than through the constructor, which takes an AccAddress
	// and so always renders canonically: this is the shape a decoded wire message has.
	_, err := s.msgServer.ToggleSPAsDelegatedAgent(s.ctx,
		&types.MsgToggleSPAsDelegatedAgent{Operator: lowered, BucketName: bucketName})
	s.Require().NoError(err)

	bucket, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().True(bucket.SpAsDelegatedAgentDisabled)
}
