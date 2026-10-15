package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// ---------------------------------------------------------------------------
// Shared fixtures for msgServer tests.
//
// Every handler in msg_server.go is a thin wrapper: unwrap ctx, parse hex
// addresses, call the matching Keeper method, map an error to `return nil,
// err`. Coverage per handler is one not-found-style error call (the wrapped
// Keeper method's own cheapest error branch, usually needing no mocks at all
// since it fails on a plain KV-store miss before touching any mocked keeper)
// plus one happy-path call that drives the same Keeper method to success.
// These helpers build the minimum fixtures the happy paths need; each test
// registers whatever mock expectations its own scenario touches on top.
// ---------------------------------------------------------------------------

// newInServiceSP builds an in-service storage provider with distinct random
// addresses and wires spKeeper.GetStorageProvider(id) to resolve it.
func (s *TestSuite) newInServiceSP(id uint32) *sptypes.StorageProvider {
	sp := &sptypes.StorageProvider{
		Id:              id,
		Status:          sptypes.STATUS_IN_SERVICE,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
		SealAddress:     sample.RandAccAddress().String(),
		GcAddress:       sample.RandAccAddress().String(),
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), id).Return(sp, true).AnyTimes()
	return sp
}

// seedGVGFamily wires virtualGroupKeeper.GetGVGFamily(family.Id) to resolve family.
func (s *TestSuite) seedGVGFamily(family *vgtypes.GlobalVirtualGroupFamily) {
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), family.Id).Return(family, true).AnyTimes()
}

// seedMsgServerBucket stores a CREATED bucket owned by owner, self-billed,
// on the given GVG family, with empty (zero-charge) internal info.
func (s *TestSuite) seedMsgServerBucket(owner sdk.AccAddress, name string, id uint64, familyID uint32) *types.BucketInfo {
	b := &types.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 name,
		Id:                         sdkmath.NewUint(id),
		PaymentAddress:             owner.String(),
		GlobalVirtualGroupFamilyId: familyID,
		BucketStatus:               types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, b)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, b.Id, &types.InternalBucketInfo{PriceTime: s.ctx.BlockTime().Unix()})
	return b
}

// seedVersionedParamsBefore stores vp one second before the current block
// time. Storage's own GetVersionedParamsWithTS resolves via a reverse
// iterator that excludes an entry exactly at the query timestamp, so a
// fixture must seed strictly before the time the code under test queries at.
func (s *TestSuite) seedVersionedParamsBefore(vp types.VersionedParams) {
	before := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(before, vp))
}

// stubDeleteGCMocks stubs the stale-policy garbage-collection lookups that
// doDeleteBucket/doDeleteObject/DeleteGroup run on every deletion, with "no
// policies reference this resource" -- the cheapest case, which skips the
// rest of the bookkeeping.
func (s *TestSuite) stubDeleteGCMocks() {
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
}

// stubObjectFeeMocks stubs the price/payment dependencies shared by the
// object and bucket fee paths (lock/unlock/charge/uncharge) at zero rates,
// so the resulting stream-record and bill math is trivially satisfied.
func (s *TestSuite) stubObjectFeeMocks() {
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyZeroDec(),
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{
		ValidatorTaxRate: sdkmath.LegacyZeroDec(),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{
		StaticBalance: sdkmath.NewInt(100),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
}

// createObjectBucketFixture seeds a bucket + primary SP + a one-GVG family so
// CreateObject-family handlers can seal a zero-payload object on it. It
// returns the primary SP (its OperatorAddress is what a delegated call must
// use as the msg operator).
func (s *TestSuite) createObjectBucketFixture(owner sdk.AccAddress, bucketName string, familyID uint32) *sptypes.StorageProvider {
	sp := s.newInServiceSP(1)
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id, GlobalVirtualGroupIds: []uint32{1}})
	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, FamilyId: familyID, PrimarySpId: sp.Id, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(1)).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), gomock.Any()).Return(gvg, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	s.seedVersionedParamsBefore(types.VersionedParams{})
	s.stubObjectFeeMocks()
	return sp
}

// zeroPayloadSealedObjectFixture stores a bucket with a sealed, non-empty
// object already bound to a local virtual group, ready for
// UpdateObjectContent/DelegateUpdateObjectContent to re-seal at payload 0.
// Returns the primary SP (its OperatorAddress is needed for delegated calls).
func (s *TestSuite) zeroPayloadSealedObjectFixture(bucketName, objectName string, owner sdk.AccAddress) *sptypes.StorageProvider {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id, GlobalVirtualGroupIds: []uint32{1}})
	gvg := &vgtypes.GlobalVirtualGroup{Id: 1, FamilyId: familyID, PrimarySpId: sp.Id, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(1)).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), uint32(1), gomock.Any()).Return(gvg, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	bucket := &types.BucketInfo{
		Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		PaymentAddress: owner.String(), GlobalVirtualGroupFamilyId: familyID, BucketStatus: types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 1, TotalChargeSize: 1024, StoredSize: 1024}},
	})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: types.OBJECT_STATUS_SEALED, PayloadSize: 1024, LocalVirtualGroupId: 1,
		CreateAt: s.ctx.BlockTime().Unix(),
	})

	s.seedVersionedParamsBefore(types.VersionedParams{})
	s.stubObjectFeeMocks()
	return sp
}

// sealObjectFixture stores a bucket and a CREATED, non-empty object ready to
// be sealed, with exactly one secondary SP so its BLS signature verifies
// without aggregation. It returns the primary SP's seal address (the
// message's Operator), the GVG id, and the secondary SP's signature over the
// seal doc.
func (s *TestSuite) sealObjectFixture(bucketName, objectName string) (sealAddr string, gvgID uint32, sig []byte) {
	const familyID, secondaryID, theGVGID uint32 = 1, 2, 1
	owner := sample.RandAccAddress()
	primary := s.newInServiceSP(1)
	s.spKeeper.EXPECT().GetStorageProviderBySealAddr(gomock.Any(), gomock.Any()).Return(primary, true).AnyTimes()
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: primary.Id})

	blsPriv, err := bls.GenerateBlsKey()
	s.Require().NoError(err)
	secondary := &sptypes.StorageProvider{Id: secondaryID, BlsKey: blsPriv.PublicKey().Marshal()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), secondaryID).Return(secondary, true).AnyTimes()

	gvg := &vgtypes.GlobalVirtualGroup{
		Id: theGVGID, FamilyId: familyID, PrimarySpId: primary.Id, SecondarySpIds: []uint32{secondaryID},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), theGVGID).Return(gvg, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), theGVGID, gomock.Any()).Return(gvg, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	s.seedVersionedParamsBefore(types.VersionedParams{RedundantDataChunkNum: 1, RedundantParityChunkNum: 0})
	s.stubObjectFeeMocks()

	checksums := [][]byte{[]byte("checksum-1")}
	objectID := sdkmath.NewUint(10)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objectID, Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
		PayloadSize: 1024, ObjectStatus: types.OBJECT_STATUS_CREATED, Checksums: checksums,
		CreateAt: s.ctx.BlockTime().Unix(),
	})

	doc := types.NewSecondarySpSealObjectSignDoc(s.ctx.ChainID(), theGVGID, objectID, types.GenerateHash(checksums))
	hash := doc.GetBlsSignHash()
	blsSig, err := blsPriv.Sign(hash[:], votepool.DST)
	s.Require().NoError(err)
	sigBz, err := blsSig.Marshal()
	s.Require().NoError(err)

	return primary.SealAddress, theGVGID, sigBz
}

// ---------------------------------------------------------------------------
// CreateBucket
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerCreateBucket_AlreadyExists() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-dup-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})

	_, err := s.msgServer.CreateBucket(s.ctx, &types.MsgCreateBucket{
		Creator: owner.String(), BucketName: bucketName, PrimarySpAddress: sample.RandAccAddress().String(),
		PrimarySpApproval: &common.Approval{},
	})
	s.Require().ErrorIs(err, types.ErrBucketAlreadyExists)
}

func (s *TestSuite) TestMsgServerCreateBucket_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), familyID).
		Return(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id}, nil).AnyTimes()

	owner := sample.RandAccAddress()
	bucketName := "msgserver-new-bucket"
	msg := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketName,
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpAddress: sp.OperatorAddress,
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: familyID,
		},
	}
	approvalBytes := msg.GetApprovalBytes()
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	msg.PrimarySpApproval.Sig = sig

	resp, err := s.msgServer.CreateBucket(s.ctx, msg)
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetBucketInfoById(s.ctx, resp.BucketId)
	s.Require().True(found)
	s.Require().Equal(owner.String(), got.Owner)
}

// ---------------------------------------------------------------------------
// DeleteBucket
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerDeleteBucket_NotFound() {
	_, err := s.msgServer.DeleteBucket(s.ctx, &types.MsgDeleteBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-delete-bucket-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerDeleteBucket_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-delete-bucket"
	bucket := &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1), PaymentAddress: owner.String()}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{})
	s.stubDeleteGCMocks()

	_, err := s.msgServer.DeleteBucket(s.ctx, &types.MsgDeleteBucket{Operator: owner.String(), BucketName: bucketName})
	s.Require().NoError(err)
	_, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found)
}

// ---------------------------------------------------------------------------
// UpdateBucketInfo
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerUpdateBucketInfo_NotFound() {
	_, err := s.msgServer.UpdateBucketInfo(s.ctx, &types.MsgUpdateBucketInfo{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-update-bucket-info-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerUpdateBucketInfo_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})
	owner := sample.RandAccAddress()
	bucketName := "msgserver-update-bucketinfo-bucket"
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	s.stubObjectFeeMocks()

	_, err := s.msgServer.UpdateBucketInfo(s.ctx, &types.MsgUpdateBucketInfo{
		Operator: owner.String(), BucketName: bucketName, Visibility: types.VISIBILITY_TYPE_PUBLIC_READ,
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().Equal(types.VISIBILITY_TYPE_PUBLIC_READ, got.Visibility)
}

// A non-nil ChargedReadQuota is unwrapped from its wire wrapper type and
// forwarded to the Keeper; increasing it never trips the decrease-cooldown
// guard, so this pins the plain pass-through path end to end.
func (s *TestSuite) TestMsgServerUpdateBucketInfo_ChargedReadQuotaIncrease() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})
	owner := sample.RandAccAddress()
	bucketName := "msgserver-update-bucketinfo-quota-bucket"
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	s.stubObjectFeeMocks()

	newQuota := uint64(100)
	_, err := s.msgServer.UpdateBucketInfo(s.ctx, &types.MsgUpdateBucketInfo{
		Operator: owner.String(), BucketName: bucketName,
		ChargedReadQuota: &common.UInt64Value{Value: newQuota},
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().Equal(newQuota, got.ChargedReadQuota)
}

// ---------------------------------------------------------------------------
// DiscontinueBucket
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerDiscontinueBucket_NotFoundSP() {
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	_, err := s.msgServer.DiscontinueBucket(s.ctx, &types.MsgDiscontinueBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "b-discontinue-bucket-not-found-s-p", Reason: "reason-discontinue-bucket-not-found-s-p",
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerDiscontinueBucket_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})
	owner := sample.RandAccAddress()
	bucketName := "msgserver-discontinue-bucket"
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)

	gcOperator := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	_, err := s.msgServer.DiscontinueBucket(s.ctx, &types.MsgDiscontinueBucket{
		Operator: gcOperator.String(), BucketName: bucketName, Reason: "test",
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_DISCONTINUED, got.BucketStatus)
}

// ---------------------------------------------------------------------------
// CreateObject / DelegateCreateObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerCreateObject_NotFoundBucket() {
	s.seedVersionedParamsBefore(types.VersionedParams{})
	_, err := s.msgServer.CreateObject(s.ctx, &types.MsgCreateObject{
		Creator: sample.RandAccAddress().String(), BucketName: "nsb-create-object-not-found-bucket", ObjectName: "obj-create-object-not-found-bucket",
		ExpectChecksums: [][]byte{[]byte("cs-create-object-not-found-bucket")}, PrimarySpApproval: &common.Approval{},
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCreateObject_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-create-object-bucket"
	const familyID uint32 = 1
	s.createObjectBucketFixture(owner, bucketName, familyID)

	resp, err := s.msgServer.CreateObject(s.ctx, &types.MsgCreateObject{
		Creator: owner.String(), BucketName: bucketName, ObjectName: "objtxt-create-object-success",
		Visibility: types.VISIBILITY_TYPE_PRIVATE, ExpectChecksums: [][]byte{[]byte("cs-create-object-success")},
		PrimarySpApproval: &common.Approval{},
	})
	s.Require().NoError(err)
	obj, found := s.storageKeeper.GetObjectInfoById(s.ctx, resp.ObjectId)
	s.Require().True(found)
	s.Require().Equal(types.OBJECT_STATUS_SEALED, obj.ObjectStatus)
}

// A checksum count that does not match the redundancy scheme is rejected by
// the wrapper itself, before the bucket is even looked up.
func (s *TestSuite) TestMsgServerCreateObject_InvalidChecksumLength() {
	s.seedVersionedParamsBefore(types.VersionedParams{})
	_, err := s.msgServer.CreateObject(s.ctx, &types.MsgCreateObject{
		Creator: sample.RandAccAddress().String(), BucketName: "b-create-object-invalid-checksum-length", ObjectName: "o-create-object-invalid-checksum-length",
		ExpectChecksums: [][]byte{}, PrimarySpApproval: &common.Approval{},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerDelegateCreateObject_NotFoundBucket() {
	_, err := s.msgServer.DelegateCreateObject(s.ctx, &types.MsgDelegateCreateObject{
		Operator: sample.RandAccAddress().String(), Creator: sample.RandAccAddress().String(),
		BucketName: "nsb-delegate-create-object-not-found-bucket", ObjectName: "obj-delegate-create-object-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerDelegateCreateObject_Success() {
	owner := sample.RandAccAddress()
	creator := sample.RandAccAddress()
	bucketName := "msgserver-delegate-create-bucket"
	const familyID uint32 = 1
	sp := s.createObjectBucketFixture(owner, bucketName, familyID)
	// the delegated creator (not the bucket owner) is the permission subject for a
	// delegated create, so it needs an explicit CreateObject grant.
	allowPolicy := &permtypes.Policy{
		Statements: []*permtypes.Statement{{Effect: permtypes.EFFECT_ALLOW, Actions: []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT}}},
	}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(allowPolicy, true).AnyTimes()

	resp, err := s.msgServer.DelegateCreateObject(s.ctx, &types.MsgDelegateCreateObject{
		Operator: sp.OperatorAddress, Creator: creator.String(), BucketName: bucketName, ObjectName: "delegated-obj",
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)
	obj, found := s.storageKeeper.GetObjectInfoById(s.ctx, resp.ObjectId)
	s.Require().True(found)
	s.Require().Equal(creator.String(), obj.Creator)
}

// ---------------------------------------------------------------------------
// CancelCreateObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerCancelCreateObject_NotFoundBucket() {
	_, err := s.msgServer.CancelCreateObject(s.ctx, &types.MsgCancelCreateObject{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-cancel-create-object-not-found-bucket", ObjectName: "obj-cancel-create-object-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCancelCreateObject_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})
	operator := sample.RandAccAddress()
	bucketName := "msgserver-cancel-create-bucket"
	objectName := "created-obj"
	s.seedMsgServerBucket(operator, bucketName, 1, familyID)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: operator.String(), BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.seedVersionedParamsBefore(types.VersionedParams{})
	s.stubObjectFeeMocks()

	_, err := s.msgServer.CancelCreateObject(s.ctx, &types.MsgCancelCreateObject{
		Operator: operator.String(), BucketName: bucketName, ObjectName: objectName,
	})
	s.Require().NoError(err)
	_, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found)
}

// ---------------------------------------------------------------------------
// SealObject / SealObjectV2
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerSealObject_NotFoundBucket() {
	_, err := s.msgServer.SealObject(s.ctx, &types.MsgSealObject{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-seal-object-not-found-bucket", ObjectName: "o-seal-object-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerSealObject_Success() {
	bucketName, objectName := "msgserver-seal-bucket", "seal-object"
	sealAddr, gvgID, sig := s.sealObjectFixture(bucketName, objectName)

	_, err := s.msgServer.SealObject(s.ctx, &types.MsgSealObject{
		Operator: sealAddr, BucketName: bucketName, ObjectName: objectName,
		GlobalVirtualGroupId: gvgID, SecondarySpBlsAggSignatures: sig,
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().Equal(types.OBJECT_STATUS_SEALED, got.ObjectStatus)
}

func (s *TestSuite) TestMsgServerSealObjectV2_NotFoundBucket() {
	_, err := s.msgServer.SealObjectV2(s.ctx, &types.MsgSealObjectV2{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-seal-object-v2-not-found-bucket", ObjectName: "o-seal-object-v2-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerSealObjectV2_Success() {
	bucketName, objectName := "msgserver-seal-v2-bucket", "seal-object-v2"
	sealAddr, gvgID, sig := s.sealObjectFixture(bucketName, objectName)

	_, err := s.msgServer.SealObjectV2(s.ctx, &types.MsgSealObjectV2{
		Operator: sealAddr, BucketName: bucketName, ObjectName: objectName,
		GlobalVirtualGroupId: gvgID, SecondarySpBlsAggSignatures: sig,
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().Equal(types.OBJECT_STATUS_SEALED, got.ObjectStatus)
}

// ---------------------------------------------------------------------------
// CopyObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerCopyObject_NotFoundSrcBucket() {
	_, err := s.msgServer.CopyObject(s.ctx, &types.MsgCopyObject{
		Operator: sample.RandAccAddress().String(), SrcBucketName: "no-such-src", SrcObjectName: "o-copy-object-not-found-src-bucket",
		DstBucketName: "no-such-dst", DstObjectName: "o2", DstPrimarySpApproval: &common.Approval{},
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCopyObject_Success() {
	owner := sample.RandAccAddress()
	sp, gvgFamily, privKey := s.newCopyObjectSP()
	srcBucket, dstBucket := "msgserver-copy-src", "msgserver-copy-dst"
	s.createBucketForCopy(owner, srcBucket, sp, privKey, gvgFamily)
	s.createBucketForCopy(owner, dstBucket, sp, privKey, gvgFamily)
	_, err := s.storageKeeper.CreateObject(s.ctx, owner, srcBucket, "objtxt-copy-object-success", 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN, Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	msg := &types.MsgCopyObject{
		Operator: owner.String(), SrcBucketName: srcBucket, SrcObjectName: "objtxt-copy-object-success",
		DstBucketName: dstBucket, DstObjectName: "copy.txt",
		DstPrimarySpApproval: &common.Approval{ExpiredHeight: uint64(s.ctx.BlockHeight() + 1000)},
	}
	approvalBytes := msg.GetApprovalBytes()
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	msg.DstPrimarySpApproval.Sig = sig

	resp, err := s.msgServer.CopyObject(s.ctx, msg)
	s.Require().NoError(err)
	copied, found := s.storageKeeper.GetObjectInfoById(s.ctx, resp.ObjectId)
	s.Require().True(found)
	s.Require().Equal(owner.String(), copied.Owner)
}

// ---------------------------------------------------------------------------
// DeleteObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerDeleteObject_NotFoundBucket() {
	_, err := s.msgServer.DeleteObject(s.ctx, &types.MsgDeleteObject{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-delete-object-not-found-bucket", ObjectName: "o-delete-object-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerDeleteObject_SealedSuccess() {
	operator := sample.RandAccAddress()
	bucketName := "msgserver-delete-object-bucket"
	objectName := "sealedobj-delete-object-sealed-success"

	bucket := &types.BucketInfo{
		Owner: operator.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		BucketStatus: types.BUCKET_STATUS_CREATED, PaymentAddress: operator.String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})
	object := &types.ObjectInfo{
		Id: sdkmath.NewUint(10), BucketName: bucketName, ObjectName: objectName, Owner: operator.String(),
		ObjectStatus: types.OBJECT_STATUS_SEALED, PayloadSize: 1,
		CreateAt: s.ctx.BlockTime().Unix(), UpdatedAt: s.ctx.BlockTime().Unix(), LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	s.seedVersionedParamsBefore(types.VersionedParams{MinChargeSize: 1})
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 0})
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id: 0, FamilyId: 0, PrimarySpId: 0, SecondarySpIds: []uint32{}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.newInServiceSP(0)
	s.stubObjectFeeMocks()
	s.stubDeleteGCMocks()

	_, err := s.msgServer.DeleteObject(s.ctx, &types.MsgDeleteObject{
		Operator: operator.String(), BucketName: bucketName, ObjectName: objectName,
	})
	s.Require().NoError(err)
	_, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found)
}

// ---------------------------------------------------------------------------
// RejectSealObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerRejectSealObject_NotFoundBucket() {
	_, err := s.msgServer.RejectSealObject(s.ctx, &types.MsgRejectSealObject{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-reject-seal-object-not-found-bucket", ObjectName: "o-reject-seal-object-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerRejectSealObject_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	sealOperator := sample.RandAccAddress()
	sp.SealAddress = sealOperator.String()
	s.spKeeper.EXPECT().GetStorageProviderBySealAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})

	owner := sample.RandAccAddress()
	bucketName := "msgserver-reject-seal-bucket"
	objectName := "created-obj"
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix(),
	})
	s.seedVersionedParamsBefore(types.VersionedParams{})
	s.stubObjectFeeMocks()

	_, err := s.msgServer.RejectSealObject(s.ctx, &types.MsgRejectSealObject{
		Operator: sealOperator.String(), BucketName: bucketName, ObjectName: objectName,
	})
	s.Require().NoError(err)
	_, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found)
}

// ---------------------------------------------------------------------------
// DiscontinueObject
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerDiscontinueObject_NotFoundSP() {
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	_, err := s.msgServer.DiscontinueObject(s.ctx, &types.MsgDiscontinueObject{
		Operator: sample.RandAccAddress().String(), BucketName: "b-discontinue-object-not-found-s-p", Reason: "reason-discontinue-object-not-found-s-p",
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerDiscontinueObject_Success() {
	const familyID uint32 = 1
	sp := s.newInServiceSP(1)
	s.spKeeper.EXPECT().GetStorageProviderByGcAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: sp.Id})
	owner := sample.RandAccAddress()
	bucketName := "msgserver-discontinue-object-bucket"
	s.seedMsgServerBucket(owner, bucketName, 1, familyID)
	object := &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: owner.String(), BucketName: bucketName, ObjectName: "obj-discontinue-object-success",
		ObjectStatus: types.OBJECT_STATUS_SEALED, CreateAt: s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	operator := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	_, err := s.msgServer.DiscontinueObject(s.ctx, &types.MsgDiscontinueObject{
		Operator: operator.String(), BucketName: bucketName, ObjectIds: []sdkmath.Uint{object.Id}, Reason: "reason-discontinue-object-success",
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfoById(s.ctx, object.Id)
	s.Require().True(found)
	s.Require().Equal(types.OBJECT_STATUS_DISCONTINUED, got.ObjectStatus)
}

// ---------------------------------------------------------------------------
// UpdateObjectInfo
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerUpdateObjectInfo_NotFoundBucket() {
	_, err := s.msgServer.UpdateObjectInfo(s.ctx, &types.MsgUpdateObjectInfo{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-update-object-info-not-found-bucket", ObjectName: "o-update-object-info-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerUpdateObjectInfo_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-update-objinfo-bucket"
	objectName := "update-objinfo-object"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
	})

	_, err := s.msgServer.UpdateObjectInfo(s.ctx, &types.MsgUpdateObjectInfo{
		Operator: owner.String(), BucketName: bucketName, ObjectName: objectName, Visibility: types.VISIBILITY_TYPE_PUBLIC_READ,
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().Equal(types.VISIBILITY_TYPE_PUBLIC_READ, got.Visibility)
}

// ---------------------------------------------------------------------------
// CreateGroup / DeleteGroup / LeaveGroup / UpdateGroupMember / RenewGroupMember / UpdateGroupExtra
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerCreateGroup_AlreadyExists() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "dup-group"})
	s.Require().NoError(err)

	_, err = s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "dup-group"})
	s.Require().ErrorIs(err, types.ErrGroupAlreadyExists)
}

func (s *TestSuite) TestMsgServerCreateGroup_Success() {
	owner := sample.RandAccAddress()
	resp, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "g1", Extra: "e"})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetGroupInfoById(s.ctx, resp.GroupId)
	s.Require().True(found)
	s.Require().Equal("e", got.Extra)
}

func (s *TestSuite) TestMsgServerDeleteGroup_NotFound() {
	_, err := s.msgServer.DeleteGroup(s.ctx, &types.MsgDeleteGroup{Operator: sample.RandAccAddress().String(), GroupName: "nogrp-delete-group-not-found"})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

func (s *TestSuite) TestMsgServerDeleteGroup_Success() {
	owner := sample.RandAccAddress()
	groupName := "del-group"
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: groupName})
	s.Require().NoError(err)
	s.stubDeleteGCMocks()

	_, err = s.msgServer.DeleteGroup(s.ctx, &types.MsgDeleteGroup{Operator: owner.String(), GroupName: groupName})
	s.Require().NoError(err)
	_, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().False(found)
}

func (s *TestSuite) TestMsgServerLeaveGroup_NotFound() {
	_, err := s.msgServer.LeaveGroup(s.ctx, &types.MsgLeaveGroup{
		Member: sample.RandAccAddress().String(), GroupOwner: sample.RandAccAddress().String(), GroupName: "nogrp-leave-group-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

func (s *TestSuite) TestMsgServerLeaveGroup_Success() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "leave-group"})
	s.Require().NoError(err)
	s.permissionKeeper.EXPECT().RemoveGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	_, err = s.msgServer.LeaveGroup(s.ctx, &types.MsgLeaveGroup{Member: member.String(), GroupOwner: owner.String(), GroupName: "leave-group"})
	s.Require().NoError(err)
}

func (s *TestSuite) TestMsgServerUpdateGroupMember_NotFound() {
	_, err := s.msgServer.UpdateGroupMember(s.ctx, &types.MsgUpdateGroupMember{
		Operator: sample.RandAccAddress().String(), GroupOwner: sample.RandAccAddress().String(), GroupName: "nogrp-update-group-member-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

// A member address the keeper cannot parse surfaces as an error from the
// wrapped Keeper.UpdateGroupMember call, not from the wrapper's own
// not-found check.
func (s *TestSuite) TestMsgServerUpdateGroupMember_KeeperError() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "upd-member-bad-addr-group"})
	s.Require().NoError(err)

	_, err = s.msgServer.UpdateGroupMember(s.ctx, &types.MsgUpdateGroupMember{
		Operator: owner.String(), GroupOwner: owner.String(), GroupName: "upd-member-bad-addr-group",
		MembersToAdd: []*types.MsgGroupMember{{Member: "not-a-valid-address"}},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerUpdateGroupMember_Success() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "upd-member-group"})
	s.Require().NoError(err)
	newMember := sample.RandAccAddress()
	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	_, err = s.msgServer.UpdateGroupMember(s.ctx, &types.MsgUpdateGroupMember{
		Operator: owner.String(), GroupOwner: owner.String(), GroupName: "upd-member-group",
		MembersToAdd: []*types.MsgGroupMember{{Member: newMember.String()}},
	})
	s.Require().NoError(err)
}

func (s *TestSuite) TestMsgServerRenewGroupMember_NotFound() {
	_, err := s.msgServer.RenewGroupMember(s.ctx, &types.MsgRenewGroupMember{
		Operator: sample.RandAccAddress().String(), GroupOwner: sample.RandAccAddress().String(), GroupName: "nogrp-renew-group-member-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

// Same as UpdateGroupMember: an unparsable member address surfaces as an
// error from the wrapped Keeper.RenewGroupMember call.
func (s *TestSuite) TestMsgServerRenewGroupMember_KeeperError() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "renew-member-bad-addr-group"})
	s.Require().NoError(err)

	_, err = s.msgServer.RenewGroupMember(s.ctx, &types.MsgRenewGroupMember{
		Operator: owner.String(), GroupOwner: owner.String(), GroupName: "renew-member-bad-addr-group",
		Members: []*types.MsgGroupMember{{Member: "not-a-valid-address"}},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerRenewGroupMember_Success() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "renew-member-group"})
	s.Require().NoError(err)
	member := sample.RandAccAddress()
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	_, err = s.msgServer.RenewGroupMember(s.ctx, &types.MsgRenewGroupMember{
		Operator: owner.String(), GroupOwner: owner.String(), GroupName: "renew-member-group",
		Members: []*types.MsgGroupMember{{Member: member.String()}},
	})
	s.Require().NoError(err)
}

func (s *TestSuite) TestMsgServerUpdateGroupExtra_NotFound() {
	_, err := s.msgServer.UpdateGroupExtra(s.ctx, &types.MsgUpdateGroupExtra{
		Operator: sample.RandAccAddress().String(), GroupOwner: sample.RandAccAddress().String(), GroupName: "nogrp-update-group-extra-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

// A non-owner operator with no grant is denied by the wrapped
// Keeper.UpdateGroupExtra call.
func (s *TestSuite) TestMsgServerUpdateGroupExtra_KeeperError() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: "extra-denied-group"})
	s.Require().NoError(err)
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	stranger := sample.RandAccAddress()
	_, err = s.msgServer.UpdateGroupExtra(s.ctx, &types.MsgUpdateGroupExtra{
		Operator: stranger.String(), GroupOwner: owner.String(), GroupName: "extra-denied-group", Extra: "x",
	})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestMsgServerUpdateGroupExtra_Success() {
	owner := sample.RandAccAddress()
	groupName := "extra-group"
	_, err := s.msgServer.CreateGroup(s.ctx, &types.MsgCreateGroup{Creator: owner.String(), GroupName: groupName})
	s.Require().NoError(err)

	_, err = s.msgServer.UpdateGroupExtra(s.ctx, &types.MsgUpdateGroupExtra{
		Operator: owner.String(), GroupOwner: owner.String(), GroupName: groupName, Extra: "new-extra",
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	s.Require().Equal("new-extra", got.Extra)
}

// ---------------------------------------------------------------------------
// DeletePolicy
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerDeletePolicy_NotFoundBucket() {
	_, err := s.msgServer.DeletePolicy(s.ctx, &types.MsgDeletePolicy{
		Operator:  sample.RandAccAddress().String(),
		Resource:  types2.NewBucketGRN("nsb-delete-policy-not-found-bucket").String(),
		Principal: permtypes.NewPrincipalWithAccount(sample.RandAccAddress()),
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

// A resource string that does not parse as a GRN is rejected before the
// wrapper ever calls the Keeper.
func (s *TestSuite) TestMsgServerDeletePolicy_InvalidResource() {
	_, err := s.msgServer.DeletePolicy(s.ctx, &types.MsgDeletePolicy{
		Operator: sample.RandAccAddress().String(), Resource: "not-a-grn",
		Principal: permtypes.NewPrincipalWithAccount(sample.RandAccAddress()),
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerDeletePolicy_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-delete-policy-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	principal := permtypes.NewPrincipalWithAccount(sample.RandAccAddress())
	s.permissionKeeper.EXPECT().DeletePolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sdkmath.OneUint(), nil).AnyTimes()

	_, err := s.msgServer.DeletePolicy(s.ctx, &types.MsgDeletePolicy{
		Operator: owner.String(), Resource: types2.NewBucketGRN(bucketName).String(), Principal: principal,
	})
	s.Require().NoError(err)
}

// ---------------------------------------------------------------------------
// UpdateParams
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerUpdateParams_WrongAuthority() {
	_, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: sample.RandAccAddress().String(), Params: types.DefaultParams(),
	})
	s.Require().ErrorIs(err, govtypes.ErrInvalidSigner)
}

func (s *TestSuite) TestMsgServerUpdateParams_Success() {
	_, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.storageKeeper.GetAuthority(), Params: types.DefaultParams(),
	})
	s.Require().NoError(err)
}

// An authorized caller submitting invalid params surfaces the error from the
// wrapped Keeper.SetParams call, not from the authority check.
func (s *TestSuite) TestMsgServerUpdateParams_InvalidParams() {
	_, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.storageKeeper.GetAuthority(), Params: types.Params{},
	})
	s.Require().Error(err)
}

// ---------------------------------------------------------------------------
// MigrateBucket / CompleteMigrateBucket / CancelMigrateBucket / RejectMigrateBucket
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerMigrateBucket_NotFound() {
	_, err := s.msgServer.MigrateBucket(s.ctx, &types.MsgMigrateBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-migrate-bucket-not-found", DstPrimarySpId: 2,
		DstPrimarySpApproval: &common.Approval{},
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerMigrateBucket_Success() {
	const srcFamilyID uint32 = 1
	const srcSpID uint32 = 1
	const dstSpID uint32 = 2
	owner := sample.RandAccAddress()
	bucketName := "msgserver-migrate-bucket"

	s.seedGVGFamily(&vgtypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID})
	srcSP := &sptypes.StorageProvider{Id: srcSpID, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSpID).Return(srcSP, true).AnyTimes()

	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	dstSP := &sptypes.StorageProvider{
		Id: dstSpID, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String(),
		ApprovalAddress: gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), dstSpID).Return(dstSP, true).AnyTimes()

	s.seedMsgServerBucket(owner, bucketName, 1, srcFamilyID)
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()

	msg := &types.MsgMigrateBucket{
		Operator: owner.String(), BucketName: bucketName, DstPrimarySpId: dstSpID,
		DstPrimarySpApproval: &common.Approval{ExpiredHeight: uint64(s.ctx.BlockHeight() + 1000)},
	}
	approvalBytes := msg.GetApprovalBytes()
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	msg.DstPrimarySpApproval.Sig = sig

	_, err = s.msgServer.MigrateBucket(s.ctx, msg)
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_MIGRATING, got.BucketStatus)
}

func (s *TestSuite) TestMsgServerCompleteMigrateBucket_NotFound() {
	_, err := s.msgServer.CompleteMigrateBucket(s.ctx, &types.MsgCompleteMigrateBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-complete-migrate-bucket-not-found", GlobalVirtualGroupFamilyId: 1,
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCompleteMigrateBucket_Success() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		srcSpID     = uint32(5)
		ownFamID    = uint32(7)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), ownFamID).
		Return(&vgtypes.GlobalVirtualGroupFamily{Id: ownFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&vgtypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSpID).
		Return(&sptypes.StorageProvider{Id: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), dstSpID).
		Return(&sptypes.StorageProvider{Id: dstSpID}, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	_, err := s.msgServer.CompleteMigrateBucket(s.ctx, &types.MsgCompleteMigrateBucket{
		Operator: dstOperator.String(), BucketName: bucketName, GlobalVirtualGroupFamilyId: ownFamID,
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
}

func (s *TestSuite) TestMsgServerCancelMigrateBucket_NotFound() {
	_, err := s.msgServer.CancelMigrateBucket(s.ctx, &types.MsgCancelMigrateBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-cancel-migrate-bucket-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCancelMigrateBucket_Success() {
	bucketName := s.setupMigratingBucket(2, 1)
	info, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	owner := sdk.MustAccAddressFromHex(info.Owner)

	_, err := s.msgServer.CancelMigrateBucket(s.ctx, &types.MsgCancelMigrateBucket{
		Operator: owner.String(), BucketName: bucketName,
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
}

func (s *TestSuite) TestMsgServerRejectMigrateBucket_NotFound() {
	_, err := s.msgServer.RejectMigrateBucket(s.ctx, &types.MsgRejectMigrateBucket{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-reject-migrate-bucket-not-found",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerRejectMigrateBucket_Success() {
	const dstSpID = uint32(2)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, 1)
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), dstSpID).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}).AnyTimes()

	_, err := s.msgServer.RejectMigrateBucket(s.ctx, &types.MsgRejectMigrateBucket{
		Operator: dstOperator.String(), BucketName: bucketName,
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
}

// ---------------------------------------------------------------------------
// SetTag
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerSetTag_NotFoundBucket() {
	_, err := s.msgServer.SetTag(s.ctx, &types.MsgSetTag{
		Operator: sample.RandAccAddress().String(), Resource: types2.NewBucketGRN("nsb-set-tag-not-found-bucket").String(), Tags: &types.ResourceTags{},
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

// A resource string that does not parse as a GRN is rejected before the
// wrapper ever calls the Keeper.
func (s *TestSuite) TestMsgServerSetTag_InvalidResource() {
	_, err := s.msgServer.SetTag(s.ctx, &types.MsgSetTag{
		Operator: sample.RandAccAddress().String(), Resource: "not-a-grn", Tags: &types.ResourceTags{},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerSetTag_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-settag-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})

	_, err := s.msgServer.SetTag(s.ctx, &types.MsgSetTag{
		Operator: owner.String(), Resource: types2.NewBucketGRN(bucketName).String(),
		Tags: &types.ResourceTags{Tags: []types.ResourceTags_Tag{{Key: "k", Value: "v"}}},
	})
	s.Require().NoError(err)
	got, _ := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().Equal("k", got.Tags.Tags[0].Key)
}

// ---------------------------------------------------------------------------
// UpdateObjectContent / DelegateUpdateObjectContent / CancelUpdateObjectContent
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerUpdateObjectContent_NotFoundBucket() {
	s.seedVersionedParamsBefore(types.VersionedParams{})
	_, err := s.msgServer.UpdateObjectContent(s.ctx, &types.MsgUpdateObjectContent{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-update-object-content-not-found-bucket", ObjectName: "o-update-object-content-not-found-bucket",
		ExpectChecksums: [][]byte{[]byte("cs-update-object-content-not-found-bucket")},
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

// A checksum count that does not match the redundancy scheme is rejected by
// the wrapper itself, before the bucket is even looked up.
func (s *TestSuite) TestMsgServerUpdateObjectContent_InvalidChecksumLength() {
	s.seedVersionedParamsBefore(types.VersionedParams{})
	_, err := s.msgServer.UpdateObjectContent(s.ctx, &types.MsgUpdateObjectContent{
		Operator: sample.RandAccAddress().String(), BucketName: "b-update-object-content-invalid-checksum-length", ObjectName: "o-update-object-content-invalid-checksum-length",
		ExpectChecksums: [][]byte{},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestMsgServerUpdateObjectContent_ZeroPayloadSuccess() {
	owner := sample.RandAccAddress()
	bucketName, objectName := "msgserver-update-content-bucket", "sealedobj-update-object-content-zero-payload-success"
	s.zeroPayloadSealedObjectFixture(bucketName, objectName, owner)

	_, err := s.msgServer.UpdateObjectContent(s.ctx, &types.MsgUpdateObjectContent{
		Operator: owner.String(), BucketName: bucketName, ObjectName: objectName,
		PayloadSize: 0, ExpectChecksums: [][]byte{[]byte("cs-update-object-content-zero-payload-success")},
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().Equal(uint64(0), got.PayloadSize)
}

func (s *TestSuite) TestMsgServerDelegateUpdateObjectContent_NotFoundBucket() {
	_, err := s.msgServer.DelegateUpdateObjectContent(s.ctx, &types.MsgDelegateUpdateObjectContent{
		Operator: sample.RandAccAddress().String(), Updater: sample.RandAccAddress().String(),
		BucketName: "nsb-delegate-update-object-content-not-found-bucket", ObjectName: "o-delegate-update-object-content-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerDelegateUpdateObjectContent_Success() {
	owner := sample.RandAccAddress()
	bucketName, objectName := "msgserver-delegate-update-bucket", "sealedobj-delegate-update-object-content-success"
	sp := s.zeroPayloadSealedObjectFixture(bucketName, objectName, owner)

	_, err := s.msgServer.DelegateUpdateObjectContent(s.ctx, &types.MsgDelegateUpdateObjectContent{
		Operator: sp.OperatorAddress, Updater: owner.String(), BucketName: bucketName, ObjectName: objectName,
		PayloadSize: 0,
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().Equal(uint64(0), got.PayloadSize)
}

func (s *TestSuite) TestMsgServerCancelUpdateObjectContent_NotFoundBucket() {
	_, err := s.msgServer.CancelUpdateObjectContent(s.ctx, &types.MsgCancelUpdateObjectContent{
		Operator: sample.RandAccAddress().String(), BucketName: "nsb-cancel-update-object-content-not-found-bucket", ObjectName: "o-cancel-update-object-content-not-found-bucket",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMsgServerCancelUpdateObjectContent_Success() {
	owner := sample.RandAccAddress()
	bucketName := "msgserver-cancel-update-bucket"
	objectName := "updating-obj"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1), PaymentAddress: owner.String()})
	object := &types.ObjectInfo{
		Id: sdkmath.NewUint(10), Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: types.OBJECT_STATUS_SEALED, IsUpdating: true, PayloadSize: 2048,
		CreateAt: s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)
	shadow := &types.ShadowObjectInfo{Id: object.Id, Operator: owner.String(), PayloadSize: 4096, UpdatedAt: s.ctx.BlockTime().Unix()}
	s.ctx.KVStore(s.storeKey).Set(types.GetShadowObjectKey(bucketName, objectName), s.cdc.MustMarshal(shadow))

	s.seedVersionedParamsBefore(types.VersionedParams{})
	s.stubObjectFeeMocks()

	_, err := s.msgServer.CancelUpdateObjectContent(s.ctx, &types.MsgCancelUpdateObjectContent{
		Operator: owner.String(), BucketName: bucketName, ObjectName: objectName,
	})
	s.Require().NoError(err)
	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().False(got.IsUpdating)
}

// ---------------------------------------------------------------------------
// SetBucketFlowRateLimit
// ---------------------------------------------------------------------------

func (s *TestSuite) TestMsgServerSetBucketFlowRateLimit_NotOwner() {
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	_, err := s.msgServer.SetBucketFlowRateLimit(s.ctx, &types.MsgSetBucketFlowRateLimit{
		Operator: sample.RandAccAddress().String(), BucketOwner: sample.RandAccAddress().String(),
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "b-set-bucket-flow-rate-limit-not-owner", FlowRateLimit: sdkmath.NewInt(10),
	})
	s.Require().ErrorIs(err, paymenttypes.ErrNotPaymentAccountOwner)
}

func (s *TestSuite) TestMsgServerSetBucketFlowRateLimit_Success() {
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	operator := sample.RandAccAddress()

	_, err := s.msgServer.SetBucketFlowRateLimit(s.ctx, &types.MsgSetBucketFlowRateLimit{
		Operator: operator.String(), BucketOwner: sample.RandAccAddress().String(),
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "nsb-set-bucket-flow-rate-limit-success", FlowRateLimit: sdkmath.NewInt(10),
	})
	s.Require().NoError(err)
}
