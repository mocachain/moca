package keeper_test

import (
	"encoding/binary"
	"errors"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
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

// TestSetGroupInfo_PersistsAndOverwrites covers the raw ID-indexed setter used
// to persist mutations made through the higher-level group methods.
func (s *TestSuite) TestSetGroupInfo_PersistsAndOverwrites() {
	groupID := sdkmath.NewUint(9001)
	groupInfo := &types.GroupInfo{
		Id:        groupID,
		Owner:     sample.RandAccAddress().String(),
		GroupName: "set-group-info-name",
		Extra:     "v1",
	}
	s.storageKeeper.SetGroupInfo(s.ctx, groupInfo)

	got, found := s.storageKeeper.GetGroupInfoById(s.ctx, groupID)
	s.Require().True(found)
	s.Require().Equal("v1", got.Extra)

	groupInfo.Extra = "v2"
	s.storageKeeper.SetGroupInfo(s.ctx, groupInfo)

	got, found = s.storageKeeper.GetGroupInfoById(s.ctx, groupID)
	s.Require().True(found)
	s.Require().Equal("v2", got.Extra, "SetGroupInfo must overwrite the existing record")
}

// TestCreateGroup_DuplicateNameRejected covers CreateGroup's remaining branch:
// a second group with the same owner+name must be rejected rather than
// silently overwriting or double-minting the group NFT.
func (s *TestSuite) TestCreateGroup_DuplicateNameRejected() {
	owner := sample.RandAccAddress()
	groupName := "dup-group"

	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)

	_, err = s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().ErrorIs(err, types.ErrGroupAlreadyExists)
}

func (s *TestSuite) TestDeleteGroup_NotFound() {
	operator := sample.RandAccAddress()
	err := s.storageKeeper.DeleteGroup(s.ctx, operator, "missing-group", types.DeleteGroupOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

func (s *TestSuite) TestDeleteGroup_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	groupName := "deletegroup-sourcetype"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{SourceType: types.SOURCE_TYPE_ORIGIN})
	s.Require().NoError(err)

	err = s.storageKeeper.DeleteGroup(s.ctx, owner, groupName, types.DeleteGroupOptions{SourceType: types.SOURCE_TYPE_MIRROR_PENDING})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

// DeleteGroup's ErrAccessDenied branch is not covered here: it is dead code
// through the public API (see "Findings" in the PR body).
func (s *TestSuite) TestDeleteGroup_Success() {
	owner := sample.RandAccAddress()
	groupName := "deletegroup-success"
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)

	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, groupID).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, groupID).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), groupID).Return(false).AnyTimes()

	err = s.storageKeeper.DeleteGroup(s.ctx, owner, groupName, types.DeleteGroupOptions{})
	s.Require().NoError(err)

	_, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().False(found, "group must be removed from the name index")
	_, found = s.storageKeeper.GetGroupInfoById(s.ctx, groupID)
	s.Require().False(found, "group must be removed from the id index")
}

func (s *TestSuite) TestLeaveGroup_NotFound() {
	member := sample.RandAccAddress()
	owner := sample.RandAccAddress()
	err := s.storageKeeper.LeaveGroup(s.ctx, member, owner, "missing-group", types.LeaveGroupOptions{})
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

func (s *TestSuite) TestLeaveGroup_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	groupName := "leavegroup-sourcetype"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)

	err = s.storageKeeper.LeaveGroup(s.ctx, member, owner, groupName, types.LeaveGroupOptions{SourceType: types.SOURCE_TYPE_MIRROR_PENDING})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

func (s *TestSuite) TestLeaveGroup_RemoveGroupMemberError() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	groupName := "leavegroup-removeerr"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	s.permissionKeeper.EXPECT().RemoveGroupMember(gomock.Any(), groupInfo.Id, member).Return(errors.New("remove failed"))

	err = s.storageKeeper.LeaveGroup(s.ctx, member, owner, groupName, types.LeaveGroupOptions{})
	s.Require().Error(err)
}

func (s *TestSuite) TestLeaveGroup_Success() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	groupName := "leavegroup-success"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	s.permissionKeeper.EXPECT().RemoveGroupMember(gomock.Any(), groupInfo.Id, member).Return(nil)

	err = s.storageKeeper.LeaveGroup(s.ctx, member, owner, groupName, types.LeaveGroupOptions{})
	s.Require().NoError(err)
}

func (s *TestSuite) TestUpdateGroupMember_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-sourcetype"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{SourceType: types.SOURCE_TYPE_MIRROR_PENDING})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

func (s *TestSuite) TestUpdateGroupMember_AccessDenied() {
	owner := sample.RandAccAddress()
	nonOwner := sample.RandAccAddress()
	groupName := "updatemember-denied"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err = s.storageKeeper.UpdateGroupMember(s.ctx, nonOwner, groupInfo, types.UpdateGroupMemberOptions{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestUpdateGroupMember_InvalidAddAddress() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-badadd"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{
		MembersToAdd:           []string{""},
		MembersExpirationToAdd: []*time.Time{nil},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestUpdateGroupMember_AddGroupMemberError() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-adderr"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	newMember := sample.RandAccAddress()

	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), groupInfo.Id, newMember, gomock.Any()).Return(errors.New("add failed"))

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{
		MembersToAdd:           []string{newMember.String()},
		MembersExpirationToAdd: []*time.Time{nil},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestUpdateGroupMember_InvalidDeleteAddress() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-baddelete"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{
		MembersToDelete: []string{""},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestUpdateGroupMember_RemoveGroupMemberError() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-removeerr"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	member := sample.RandAccAddress()

	s.permissionKeeper.EXPECT().RemoveGroupMember(gomock.Any(), groupInfo.Id, member).Return(errors.New("remove failed"))

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{
		MembersToDelete: []string{member.String()},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestUpdateGroupMember_Success() {
	owner := sample.RandAccAddress()
	groupName := "updatemember-success"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	added := sample.RandAccAddress()
	removed := sample.RandAccAddress()

	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), groupInfo.Id, added, gomock.Any()).Return(nil)
	s.permissionKeeper.EXPECT().RemoveGroupMember(gomock.Any(), groupInfo.Id, removed).Return(nil)

	err = s.storageKeeper.UpdateGroupMember(s.ctx, owner, groupInfo, types.UpdateGroupMemberOptions{
		MembersToAdd:           []string{added.String()},
		MembersExpirationToAdd: []*time.Time{nil},
		MembersToDelete:        []string{removed.String()},
	})
	s.Require().NoError(err)
}

func (s *TestSuite) TestRenewGroupMember_SourceTypeMismatch() {
	owner := sample.RandAccAddress()
	groupName := "renewmember-sourcetype"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.RenewGroupMember(s.ctx, owner, groupInfo, types.RenewGroupMemberOptions{SourceType: types.SOURCE_TYPE_MIRROR_PENDING})
	s.Require().ErrorIs(err, types.ErrSourceTypeMismatch)
}

func (s *TestSuite) TestRenewGroupMember_AccessDenied() {
	owner := sample.RandAccAddress()
	nonOwner := sample.RandAccAddress()
	groupName := "renewmember-denied"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err = s.storageKeeper.RenewGroupMember(s.ctx, nonOwner, groupInfo, types.RenewGroupMemberOptions{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestRenewGroupMember_InvalidAddress() {
	owner := sample.RandAccAddress()
	groupName := "renewmember-badaddr"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.RenewGroupMember(s.ctx, owner, groupInfo, types.RenewGroupMemberOptions{
		Members:           []string{""},
		MembersExpiration: []*time.Time{nil},
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestRenewGroupMember_NewMemberAddError() {
	owner := sample.RandAccAddress()
	groupName := "renewmember-adderr"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	member := sample.RandAccAddress()

	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupInfo.Id, member).Return(nil, false)
	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), groupInfo.Id, member, gomock.Any()).Return(errors.New("add failed"))

	err = s.storageKeeper.RenewGroupMember(s.ctx, owner, groupInfo, types.RenewGroupMemberOptions{
		Members:           []string{member.String()},
		MembersExpiration: []*time.Time{nil},
	})
	s.Require().Error(err)
}

// TestRenewGroupMember_AddsNewMember covers the branch where the renewed
// address is not yet a member: RenewGroupMember must add it rather than error.
func (s *TestSuite) TestRenewGroupMember_AddsNewMember() {
	owner := sample.RandAccAddress()
	groupName := "renewmember-new"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	member := sample.RandAccAddress()

	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupInfo.Id, member).Return(nil, false)
	s.permissionKeeper.EXPECT().AddGroupMember(gomock.Any(), groupInfo.Id, member, gomock.Any()).Return(nil)

	err = s.storageKeeper.RenewGroupMember(s.ctx, owner, groupInfo, types.RenewGroupMemberOptions{
		Members:           []string{member.String()},
		MembersExpiration: []*time.Time{nil},
	})
	s.Require().NoError(err)
}

// TestRenewGroupMember_UpdatesExistingMember covers the branch where the
// renewed address is already a member: RenewGroupMember must update its
// expiration in place rather than adding a duplicate.
func (s *TestSuite) TestRenewGroupMember_UpdatesExistingMember() {
	owner := sample.RandAccAddress()
	groupName := "renewmember-existing"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	member := sample.RandAccAddress()
	existing := &permtypes.GroupMember{Id: sdkmath.NewUint(77), GroupId: groupInfo.Id, Member: member.String()}

	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupInfo.Id, member).Return(existing, true)
	s.permissionKeeper.EXPECT().UpdateGroupMember(gomock.Any(), groupInfo.Id, member, existing.Id, gomock.Any())

	err = s.storageKeeper.RenewGroupMember(s.ctx, owner, groupInfo, types.RenewGroupMemberOptions{
		Members:           []string{member.String()},
		MembersExpiration: []*time.Time{nil},
	})
	s.Require().NoError(err)
}

func (s *TestSuite) TestUpdateGroupExtra_AccessDenied() {
	owner := sample.RandAccAddress()
	nonOwner := sample.RandAccAddress()
	groupName := "updateextra-denied"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err = s.storageKeeper.UpdateGroupExtra(s.ctx, nonOwner, groupInfo, "new-extra")
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

// TestUpdateGroupExtra_Changed covers the branch that persists a new value.
func (s *TestSuite) TestUpdateGroupExtra_Changed() {
	owner := sample.RandAccAddress()
	groupName := "updateextra-changed"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{Extra: "old"})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.UpdateGroupExtra(s.ctx, owner, groupInfo, "new")
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetGroupInfoById(s.ctx, groupInfo.Id)
	s.Require().True(found)
	s.Require().Equal("new", got.Extra)
}

// TestUpdateGroupExtra_Unchanged covers the branch that skips the KV write
// when the new value equals the current one.
func (s *TestSuite) TestUpdateGroupExtra_Unchanged() {
	owner := sample.RandAccAddress()
	groupName := "updateextra-unchanged"
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{Extra: "same"})
	s.Require().NoError(err)
	groupInfo, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)

	err = s.storageKeeper.UpdateGroupExtra(s.ctx, owner, groupInfo, "same")
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetGroupInfoById(s.ctx, groupInfo.Id)
	s.Require().True(found)
	s.Require().Equal("same", got.Extra)
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
