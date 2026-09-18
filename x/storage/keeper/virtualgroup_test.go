package keeper_test

import (
	"errors"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/votepool"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestVerifyGVGSecondarySPsBlsSignature_Valid pins the success path: a signature
// produced by the sole secondary SP's real BLS key over the given hash verifies.
func (s *TestSuite) TestVerifyGVGSecondarySPsBlsSignature_Valid() {
	priv, err := bls.GenerateBlsKey()
	s.Require().NoError(err)
	sp := &sptypes.StorageProvider{Id: 5, BlsKey: priv.PublicKey().Marshal()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(5)).Return(sp, true)

	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, SecondarySpIds: []uint32{5}}
	hash := ethcrypto.Keccak256Hash([]byte("verify-gvg-bls-signature-valid"))
	sig, err := priv.Sign(hash[:], votepool.DST)
	s.Require().NoError(err)
	sigBz, err := sig.Marshal()
	s.Require().NoError(err)

	err = s.storageKeeper.VerifyGVGSecondarySPsBlsSignature(s.ctx, gvg, hash, sigBz)
	s.Require().NoError(err)
}

// TestVerifyGVGSecondarySPsBlsSignature_WrongKey signs with a key that is not the
// registered secondary SP's: the aggregated-signature check must reject it.
func (s *TestSuite) TestVerifyGVGSecondarySPsBlsSignature_WrongKey() {
	registeredPriv, err := bls.GenerateBlsKey()
	s.Require().NoError(err)
	wrongPriv, err := bls.GenerateBlsKey()
	s.Require().NoError(err)
	sp := &sptypes.StorageProvider{Id: 6, BlsKey: registeredPriv.PublicKey().Marshal()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(6)).Return(sp, true)

	gvg := &vgtypes.GlobalVirtualGroup{Id: 2, SecondarySpIds: []uint32{6}}
	hash := ethcrypto.Keccak256Hash([]byte("verify-gvg-bls-signature-wrong-key"))
	sig, err := wrongPriv.Sign(hash[:], votepool.DST)
	s.Require().NoError(err)
	sigBz, err := sig.Marshal()
	s.Require().NoError(err)

	err = s.storageKeeper.VerifyGVGSecondarySPsBlsSignature(s.ctx, gvg, hash, sigBz)
	s.Require().Error(err)
}

// TestVerifyGVGSecondarySPsBlsSignature_InvalidPubKeyBytes covers the
// bls.UnmarshalPublicKey error branch: a secondary SP with a malformed BLS key
// on record must fail closed rather than skip verification.
func (s *TestSuite) TestVerifyGVGSecondarySPsBlsSignature_InvalidPubKeyBytes() {
	sp := &sptypes.StorageProvider{Id: 7, BlsKey: []byte("too-short-to-be-a-real-bls-pubkey")}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(7)).Return(sp, true)

	gvg := &vgtypes.GlobalVirtualGroup{Id: 3, SecondarySpIds: []uint32{7}}
	hash := ethcrypto.Keccak256Hash([]byte("verify-gvg-bls-signature-invalid-pubkey"))

	err := s.storageKeeper.VerifyGVGSecondarySPsBlsSignature(s.ctx, gvg, hash, make([]byte, bls.SignatureSize))
	s.Require().ErrorIs(err, types.ErrInvalidBlsPubKey)
}

// TestGetObjectGVG_Found resolves an object's GVG through its bound LVG.
func (s *TestSuite) TestGetObjectGVG_Found() {
	bucketID := sdkmath.NewUint(42)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 3, GlobalVirtualGroupId: 9}},
	})
	gvg := &vgtypes.GlobalVirtualGroup{Id: 9}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(9)).Return(gvg, true)

	got, found := s.storageKeeper.GetObjectGVG(s.ctx, bucketID, 3)
	s.Require().True(found)
	s.Require().Same(gvg, got)
}

// TestGetObjectGVG_LVGNotFound covers the early-return branch: no LVG with the
// requested ID means GetObjectGVG must not even reach the virtual group keeper.
func (s *TestSuite) TestGetObjectGVG_LVGNotFound() {
	bucketID := sdkmath.NewUint(43)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})

	got, found := s.storageKeeper.GetObjectGVG(s.ctx, bucketID, 3)
	s.Require().False(found)
	s.Require().Nil(got)
}

// TestSealObjectOnVirtualGroup_ExceedsMaxLVGLimit covers the cap-reached branch of
// the "no LVG bound to this GVG yet" path, ahead of the store-fee charge.
func (s *TestSuite) TestSealObjectOnVirtualGroup_ExceedsMaxLVGLimit() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), BucketName: "svg-limit-bucket", Id: sdkmath.NewUint(1),
		PaymentAddress: sample.RandAccAddress().String(), GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	params := types.DefaultParams()
	params.MaxLocalVirtualGroupNumPerBucket = 1
	s.Require().NoError(s.storageKeeper.SetParams(s.ctx, params))

	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{
			{Id: 1, GlobalVirtualGroupId: 100},
			{Id: 2, GlobalVirtualGroupId: 101},
		},
	})

	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), BucketName: bucketInfo.BucketName, ObjectName: "obj"}
	gvg := &vgtypes.GlobalVirtualGroup{Id: 7}
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(7), gomock.Any()).Return(gvg, nil)

	_, err := s.storageKeeper.SealObjectOnVirtualGroup(s.ctx, bucketInfo, 7, objectInfo)
	s.Require().ErrorContains(err, "exceed limitation")
}

// TestSealEmptyObjectOnVirtualGroup_FamilyNotFound covers the bucket's GVG family
// resolving to nothing at all (distinct from the family existing but empty).
func (s *TestSuite) TestSealEmptyObjectOnVirtualGroup_FamilyNotFound() {
	bucketInfo := &types.BucketInfo{GlobalVirtualGroupFamilyId: 55}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(55)).Return(nil, false)

	_, err := s.storageKeeper.SealEmptyObjectOnVirtualGroup(s.ctx, bucketInfo, &types.ObjectInfo{})
	s.Require().ErrorIs(err, vgtypes.ErrGVGFamilyNotExist)
}

// TestSealEmptyObjectOnVirtualGroup_NoGVGInFamily covers the family-exists-but-empty
// branch that CopyObject/UpdateObjectContent's zero-payload happy paths never hit.
func (s *TestSuite) TestSealEmptyObjectOnVirtualGroup_NoGVGInFamily() {
	bucketInfo := &types.BucketInfo{GlobalVirtualGroupFamilyId: 56}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(56)).
		Return(&vgtypes.GlobalVirtualGroupFamily{Id: 56}, true)

	_, err := s.storageKeeper.SealEmptyObjectOnVirtualGroup(s.ctx, bucketInfo, &types.ObjectInfo{})
	s.Require().ErrorIs(err, vgtypes.ErrGVGNotExist)
}

// TestSealObjectOnVirtualGroup_GetGVGIfAvailableError pins that
// SealObjectOnVirtualGroup passes through a GetGlobalVirtualGroupIfAvailable
// failure (e.g. the GVG is full) verbatim, before touching any bucket state.
func (s *TestSuite) TestSealObjectOnVirtualGroup_GetGVGIfAvailableError() {
	bucketInfo := &types.BucketInfo{BucketName: "svg-gvgerr-bucket", Id: sdkmath.NewUint(1)}
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), BucketName: bucketInfo.BucketName, ObjectName: "obj"}
	wantErr := errors.New("gvg unavailable")
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(9), gomock.Any()).Return(nil, wantErr)

	_, err := s.storageKeeper.SealObjectOnVirtualGroup(s.ctx, bucketInfo, 9, objectInfo)
	s.Require().ErrorIs(err, wantErr)
}

// TestSealObjectOnVirtualGroup_ChargeObjectStoreFeeError covers the PayloadSize==0
// charge-error branch: GetObjectChargeSize succeeds (real versioned params are
// seeded), but the downstream price lookup IsPriceChanged needs fails.
func (s *TestSuite) TestSealObjectOnVirtualGroup_ChargeObjectStoreFeeError() {
	bucketInfo := &types.BucketInfo{BucketName: "svg-chargeerr-bucket", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id: sdkmath.NewUint(1), BucketName: bucketInfo.BucketName, ObjectName: "obj",
		PayloadSize: 0, CreateAt: s.ctx.BlockTime().Unix(),
	}
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.VersionedParams{}))

	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, PrimarySpId: 1}
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), gomock.Any()).Return(gvg, nil)
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, errors.New("price unavailable"))

	_, err := s.storageKeeper.SealObjectOnVirtualGroup(s.ctx, bucketInfo, 1, objectInfo)
	s.Require().Error(err)
}

// TestSealObjectOnVirtualGroup_UnlockAndChargeObjectStoreFeeError covers the
// PayloadSize>0 charge-error branch: GetObjectLockFee's price lookup fails
// before any real versioned-params state is even needed.
func (s *TestSuite) TestSealObjectOnVirtualGroup_UnlockAndChargeObjectStoreFeeError() {
	bucketInfo := &types.BucketInfo{BucketName: "svg-unlockchargeerr-bucket", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), BucketName: bucketInfo.BucketName, ObjectName: "obj", PayloadSize: 500}
	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, PrimarySpId: 1}
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), gomock.Any()).Return(gvg, nil)
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, errors.New("price unavailable"))

	_, err := s.storageKeeper.SealObjectOnVirtualGroup(s.ctx, bucketInfo, 1, objectInfo)
	s.Require().Error(err)
}

// TestSealObjectOnVirtualGroup_SetGVGError covers the SetGVGAndEmitUpdateEvent
// error branch, reached only after the store-fee charge itself succeeds.
func (s *TestSuite) TestSealObjectOnVirtualGroup_SetGVGError() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), BucketName: "svg-setgvgerr-bucket", Id: sdkmath.NewUint(1),
		PaymentAddress: sample.RandAccAddress().String(), GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id: sdkmath.NewUint(1), BucketName: bucketInfo.BucketName, ObjectName: "obj",
		PayloadSize: 0, CreateAt: s.ctx.BlockTime().Unix(),
	}
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.VersionedParams{}))

	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, PrimarySpId: 1}
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), gomock.Any()).Return(gvg, nil)
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroupFamily{Id: 1}, true)
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true)
	price := sptypes.GlobalSpStorePrice{PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyNewDec(1), ReadPrice: sdkmath.LegacyNewDec(1)}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{ReserveTime: 0, ValidatorTaxRate: sdkmath.LegacyZeroDec()}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(errors.New("set gvg failed"))

	_, err := s.storageKeeper.SealObjectOnVirtualGroup(s.ctx, bucketInfo, 1, objectInfo)
	s.Require().Error(err)
}

// TestDeleteObjectFromVirtualGroup_GVGNotFound covers the early GVG-lookup
// failure, before any LVG/GVG size bookkeeping is touched.
func (s *TestSuite) TestDeleteObjectFromVirtualGroup_GVGNotFound() {
	bucketInfo := &types.BucketInfo{BucketName: "delete-from-vg-notfound-bucket", Id: sdkmath.NewUint(1)}
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 9}},
	})
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), LocalVirtualGroupId: 1, PayloadSize: 100}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(9)).Return(nil, false)

	err := s.storageKeeper.DeleteObjectFromVirtualGroup(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorIs(err, vgtypes.ErrGVGNotExist)
}

// TestDeleteObjectFromVirtualGroup_PanicsOnInconsistentInvariant covers the
// defensive panic guarding against a corrupt LVG record (a fully-charged-off
// LVG that still claims stored bytes).
func (s *TestSuite) TestDeleteObjectFromVirtualGroup_PanicsOnInconsistentInvariant() {
	bucketInfo := &types.BucketInfo{BucketName: "delete-from-vg-panic-bucket", Id: sdkmath.NewUint(1)}
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 9, StoredSize: 50, TotalChargeSize: 0}},
	})
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), LocalVirtualGroupId: 1, PayloadSize: 10}
	gvg := &vgtypes.GlobalVirtualGroup{Id: 9, StoredSize: 1000}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(9)).Return(gvg, true)

	s.Require().Panics(func() {
		_ = s.storageKeeper.DeleteObjectFromVirtualGroup(s.ctx, bucketInfo, objectInfo)
	})
}

// TestDeleteObjectFromVirtualGroup_SetGVGError covers the SetGVGAndEmitUpdateEvent
// error branch on the "LVG stays alive" path (TotalChargeSize still nonzero).
func (s *TestSuite) TestDeleteObjectFromVirtualGroup_SetGVGError() {
	bucketInfo := &types.BucketInfo{BucketName: "delete-from-vg-setgvgerr-bucket", Id: sdkmath.NewUint(1)}
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 9, StoredSize: 100, TotalChargeSize: 50}},
	})
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(1), LocalVirtualGroupId: 1, PayloadSize: 10}
	gvg := &vgtypes.GlobalVirtualGroup{Id: 9, StoredSize: 1000}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(9)).Return(gvg, true)
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gvg).Return(errors.New("set gvg failed"))

	err := s.storageKeeper.DeleteObjectFromVirtualGroup(s.ctx, bucketInfo, objectInfo)
	s.Require().Error(err)
}

// TestRebindingVirtualGroup_HappyPath drives the full per-LVG rebind loop: the
// LVG moves from its src GVG to the dst GVG named in gvgMappings, and both GVGs'
// stored sizes move with it.
func (s *TestSuite) TestRebindingVirtualGroup_HappyPath() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), BucketName: "rebind-happy-bucket", Id: sdkmath.NewUint(1),
		PaymentAddress: sample.RandAccAddress().String(),
	}
	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: 5, StoredSize: 300}
	internalBucketInfo := &types.InternalBucketInfo{LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	dstGVG := &vgtypes.GlobalVirtualGroup{Id: 6, StoredSize: 1000}
	srcGVG := &vgtypes.GlobalVirtualGroup{Id: 5, StoredSize: 2000}
	gvgMappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: 5, DstGlobalVirtualGroupId: 6}}

	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(6)).Return(dstGVG, true)
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(5)).Return(srcGVG, true)
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVG(gomock.Any(), srcGVG).Return(nil)
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, gvgMappings)
	s.Require().NoError(err)

	s.Require().Equal(uint32(6), lvg.GlobalVirtualGroupId, "lvg rebinds to the dst GVG")
	s.Require().Equal(uint64(1700), srcGVG.StoredSize, "src GVG loses the lvg's stored size")
	s.Require().Equal(uint64(1300), dstGVG.StoredSize, "dst GVG gains the lvg's stored size")
}

// TestRebindingVirtualGroup_DstNotInMapping covers the case where an LVG's
// current GVG has no entry at all in the caller-supplied mapping.
func (s *TestSuite) TestRebindingVirtualGroup_DstNotInMapping() {
	bucketInfo := &types.BucketInfo{BucketName: "rebind-nomap-bucket", Id: sdkmath.NewUint(1)}
	internalBucketInfo := &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 5}},
	}

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, nil)
	s.Require().ErrorIs(err, types.ErrVirtualGroupOperateFailed)
}

// TestRebindingVirtualGroup_DstFamilyMismatch covers a dst GVG that resolves
// in chain state but belongs to a family other than the bucket's destination
// family: rebinding to it must be rejected rather than silently accepted, and
// no GVG size bookkeeping must be touched.
func (s *TestSuite) TestRebindingVirtualGroup_DstFamilyMismatch() {
	bucketInfo := &types.BucketInfo{BucketName: "rebind-familymismatch-bucket", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 7}
	internalBucketInfo := &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 5, StoredSize: 300}},
	}
	dstGVG := &vgtypes.GlobalVirtualGroup{Id: 6, FamilyId: 99, StoredSize: 1000}
	gvgMappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: 5, DstGlobalVirtualGroupId: 6}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(6)).Return(dstGVG, true)

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, gvgMappings)
	s.Require().ErrorIs(err, types.ErrInvalidGlobalVirtualGroup)

	s.Require().Equal(uint64(1000), dstGVG.StoredSize, "dst GVG must not gain the lvg's stored size")
	s.Require().Equal(uint32(5), internalBucketInfo.LocalVirtualGroups[0].GlobalVirtualGroupId, "lvg must not be rebound")
}

// TestRebindingVirtualGroup_DstGVGNotFound covers the dst GVG resolving to
// nothing in chain state even though the mapping names it.
func (s *TestSuite) TestRebindingVirtualGroup_DstGVGNotFound() {
	bucketInfo := &types.BucketInfo{BucketName: "rebind-dstnotfound-bucket", Id: sdkmath.NewUint(1)}
	internalBucketInfo := &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 5}},
	}
	gvgMappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: 5, DstGlobalVirtualGroupId: 6}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(6)).Return(nil, false)

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, gvgMappings)
	s.Require().ErrorIs(err, types.ErrVirtualGroupOperateFailed)
}

// TestRebindingVirtualGroup_SrcGVGNotFound covers the src GVG (the LVG's current
// GVG) resolving to nothing in chain state.
func (s *TestSuite) TestRebindingVirtualGroup_SrcGVGNotFound() {
	bucketInfo := &types.BucketInfo{BucketName: "rebind-srcnotfound-bucket", Id: sdkmath.NewUint(1)}
	internalBucketInfo := &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 5}},
	}
	gvgMappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: 5, DstGlobalVirtualGroupId: 6}}
	dstGVG := &vgtypes.GlobalVirtualGroup{Id: 6}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(6)).Return(dstGVG, true)
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(5)).Return(nil, false)

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, gvgMappings)
	s.Require().ErrorIs(err, types.ErrVirtualGroupOperateFailed)
}

// TestRebindingVirtualGroup_SettleError covers SettleAndDistributeGVG failing
// on the src GVG before any state is rebound.
func (s *TestSuite) TestRebindingVirtualGroup_SettleError() {
	bucketInfo := &types.BucketInfo{BucketName: "rebind-settleerr-bucket", Id: sdkmath.NewUint(1)}
	internalBucketInfo := &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 5, StoredSize: 100}},
	}
	gvgMappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: 5, DstGlobalVirtualGroupId: 6}}
	dstGVG := &vgtypes.GlobalVirtualGroup{Id: 6}
	srcGVG := &vgtypes.GlobalVirtualGroup{Id: 5, StoredSize: 100}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(6)).Return(dstGVG, true)
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(5)).Return(srcGVG, true)
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVG(gomock.Any(), srcGVG).Return(errors.New("settle failed"))

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, gvgMappings)
	s.Require().ErrorIs(err, types.ErrVirtualGroupOperateFailed)
}

// TestRebindingVirtualGroup_ChargeBucketReadStoreFeeError covers the trailing
// ChargeBucketReadStoreFee call failing after the (here, empty) rebind loop.
func (s *TestSuite) TestRebindingVirtualGroup_ChargeBucketReadStoreFeeError() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), BucketName: "rebind-chargeerr-bucket", Id: sdkmath.NewUint(1),
		PaymentAddress: sample.RandAccAddress().String(),
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(errors.New("apply flows failed"))

	err := s.storageKeeper.RebindingVirtualGroup(s.ctx, bucketInfo, internalBucketInfo, nil)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)
}
