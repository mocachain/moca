package keeper_test

import (
	"encoding/binary"
	"time"

	sdkmath "cosmossdk.io/math"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/testutil/sample"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	types2 "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"go.uber.org/mock/gomock"
)

// orphanCommonMocks wires the dependencies a force-delete needs, with the primary SP
// deliberately UNRESOLVABLE (GetStorageProvider -> (nil, false)) so the delete path must
// fall back to the bucket owner and garbage-collect instead of panicking.
func (s *BurnTestSuite) orphanCommonMocks() {
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return(&types2.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 42}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).
		Return(&types2.GlobalVirtualGroup{Id: 0}, true).AnyTimes()
	// The residual family's primary SP no longer exists.
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	// Any NFT burn (bucket, or a sealed object) routes through the EVM keeper; let it succeed.
	s.evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()
}

func (s *BurnTestSuite) seedVersionedParams() {
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
}

// TestForceDeleteObjectOrphanedSPIsGarbageCollected asserts that when a discontinued
// object's primary SP can no longer be resolved, ForceDeleteObject does not panic; it
// garbage-collects the object using the owner as the deletion-event operator.
func (s *BurnTestSuite) TestForceDeleteObjectOrphanedSPIsGarbageCollected() {
	owner := sample.RandAccAddress()
	bucketName := "orphan-obj-bucket"

	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:          owner.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(50),
		BucketStatus:   types.BUCKET_STATUS_DISCONTINUED,
		PaymentAddress: sample.RandAccAddress().String(),
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, sdkmath.NewUint(50), &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	object := &types.ObjectInfo{
		Id:           sdkmath.NewUint(500),
		BucketName:   bucketName,
		ObjectName:   "orphan-obj",
		Owner:        owner.String(),
		ObjectStatus: types.OBJECT_STATUS_CREATED,
		PayloadSize:  1,
		CreateAt:     s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)
	s.seedVersionedParams()

	// simulate saveDiscontinueObjectStatus
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_CREATED))
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	s.orphanCommonMocks()

	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().NoError(err, "orphaned object (primary SP gone) must be garbage-collected, not panic")
	_, found := s.storageKeeper.GetObjectInfoById(s.ctx, object.Id)
	s.Require().False(found, "orphaned object should be deleted")
}

// TestForceDeleteBucketOrphanedSPIsGarbageCollected asserts the same for an (empty)
// discontinued bucket whose primary SP can no longer be resolved.
func (s *BurnTestSuite) TestForceDeleteBucketOrphanedSPIsGarbageCollected() {
	owner := sample.RandAccAddress()
	bucketName := "orphan-bucket"

	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:          owner.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(60),
		BucketStatus:   types.BUCKET_STATUS_DISCONTINUED,
		PaymentAddress: sample.RandAccAddress().String(),
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, sdkmath.NewUint(60), &types.InternalBucketInfo{
		PriceTime: s.ctx.BlockTime().Unix(),
	})
	s.seedVersionedParams()

	s.orphanCommonMocks()

	bucketDeleted, _, err := s.storageKeeper.ForceDeleteBucket(s.ctx, sdkmath.NewUint(60), 100)
	s.Require().NoError(err, "orphaned bucket (primary SP gone) must be garbage-collected, not panic")
	s.Require().True(bucketDeleted, "orphaned bucket should be deleted")
	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found, "orphaned bucket should be removed")
}
