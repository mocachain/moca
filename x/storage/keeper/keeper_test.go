package keeper_test

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/gogoproto/proto"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/mocachain/moca/v2/internal/sequence"
	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
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

// ---------------------------------------------------------------------------
// MigrateBucket
// ---------------------------------------------------------------------------

// migrationFixture bundles a CREATED bucket plus the src/dst SP pair backing a
// MigrateBucket call. Individual tests mutate the returned fields *before*
// calling wire(), since wire() bakes them into gomock AnyTimes() expectations
// that cannot be overridden afterwards.
type migrationFixture struct {
	s            *TestSuite
	bucketName   string
	operator     sdk.AccAddress
	bucketID     sdkmath.Uint
	srcFamilyID  uint32
	srcSP        *sptypes.StorageProvider
	dstSP        *sptypes.StorageProvider
	dstFound     bool
	dstPriv      *ecdsa.PrivateKey
	streamStatus paymenttypes.StreamAccountStatus
}

// newMigrationFixture returns a fixture wired for MigrateBucket's happy path;
// callers mutate fields on the result before calling wire() to steer a
// specific error branch instead.
func newMigrationFixture(s *TestSuite) *migrationFixture {
	s.ctx = s.ctx.WithBlockHeight(100)
	dstPriv, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	return &migrationFixture{
		s:           s,
		bucketName:  "migrate-bucket-src",
		operator:    sample.RandAccAddress(),
		bucketID:    sdkmath.NewUint(1),
		srcFamilyID: uint32(1),
		srcSP:       &sptypes.StorageProvider{Id: 5, Status: sptypes.STATUS_IN_SERVICE, OperatorAddress: sample.RandAccAddress().String()},
		dstSP: &sptypes.StorageProvider{
			Id: 7, Status: sptypes.STATUS_IN_SERVICE,
			ApprovalAddress: gethcrypto.PubkeyToAddress(dstPriv.PublicKey).Hex(),
		},
		dstFound:     true,
		dstPriv:      dstPriv,
		streamStatus: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE,
	}
}

// wire persists the bucket and registers every mock MigrateBucket needs to
// reach its approval/rate-limit checks, using whatever field values the
// caller has already set.
func (f *migrationFixture) wire() {
	f.s.storageKeeper.StoreBucketInfo(f.s.ctx, &types.BucketInfo{
		Owner:                      f.operator.String(),
		BucketName:                 f.bucketName,
		Id:                         f.bucketID,
		PaymentAddress:             f.operator.String(),
		GlobalVirtualGroupFamilyId: f.srcFamilyID,
		BucketStatus:               types.BUCKET_STATUS_CREATED,
	})
	f.s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), f.srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: f.srcFamilyID, PrimarySpId: f.srcSP.Id}, true).AnyTimes()
	f.s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), f.srcSP.Id).Return(f.srcSP, true).AnyTimes()
	if f.dstFound {
		f.s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), f.dstSP.Id).Return(f.dstSP, true).AnyTimes()
	} else {
		f.s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), f.dstSP.Id).Return(nil, false).AnyTimes()
	}
	f.s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: f.streamStatus}, true).AnyTimes()
}

// approval signs approvalBytes with signer, returning a not-yet-expired
// Approval (relative to the fixture's block height 100) plus the signed bytes.
func (f *migrationFixture) approval(signer *ecdsa.PrivateKey) (*common.Approval, []byte) {
	approvalBytes := []byte("migrate-bucket-approval")
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), signer)
	f.s.Require().NoError(err)
	return &common.Approval{ExpiredHeight: uint64(f.s.ctx.BlockHeight() + 1000), Sig: sig}, approvalBytes
}

func (s *TestSuite) TestMigrateBucket_BucketNotFound() {
	err := s.storageKeeper.MigrateBucket(s.ctx, sample.RandAccAddress(), "migrate-missing-bucket", 1, &common.Approval{}, nil)
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestMigrateBucket_NotOwnerRejected() {
	f := newMigrationFixture(s)
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, sample.RandAccAddress(), f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestMigrateBucket_AlreadyMigratingRejected() {
	f := newMigrationFixture(s)
	f.s.storageKeeper.StoreBucketInfo(f.s.ctx, &types.BucketInfo{
		Owner: f.operator.String(), BucketName: f.bucketName, Id: f.bucketID,
		BucketStatus: types.BUCKET_STATUS_MIGRATING,
	})

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, &common.Approval{}, nil)
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestMigrateBucket_DiscontinuedRejected() {
	f := newMigrationFixture(s)
	f.s.storageKeeper.StoreBucketInfo(f.s.ctx, &types.BucketInfo{
		Owner: f.operator.String(), BucketName: f.bucketName, Id: f.bucketID,
		BucketStatus: types.BUCKET_STATUS_DISCONTINUED,
	})

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, &common.Approval{}, nil)
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestMigrateBucket_DstStorageProviderNotFound() {
	f := newMigrationFixture(s)
	f.dstFound = false
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestMigrateBucket_DstEqualsSrcRejected() {
	f := newMigrationFixture(s)
	f.dstSP = f.srcSP // same SP on both sides of the migration
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)
}

func (s *TestSuite) TestMigrateBucket_DstNotInServiceRejected() {
	f := newMigrationFixture(s)
	f.dstSP.Status = sptypes.STATUS_GRACEFUL_EXITING
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, sptypes.ErrStorageProviderNotInService)
}

func (s *TestSuite) TestMigrateBucket_FrozenStreamAccountRejected() {
	f := newMigrationFixture(s)
	f.streamStatus = paymenttypes.STREAM_ACCOUNT_STATUS_FROZEN
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, paymenttypes.ErrInvalidStreamAccountStatus)
}

func (s *TestSuite) TestMigrateBucket_ApprovalExpiredRejected() {
	f := newMigrationFixture(s)
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)
	approval.ExpiredHeight = 1 // fixture block height is 100: already expired

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, types.ErrInvalidApproval)
}

func (s *TestSuite) TestMigrateBucket_InvalidSignatureRejected() {
	f := newMigrationFixture(s)
	f.wire()
	wrongKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	approval, approvalBytes := f.approval(wrongKey) // not dstSP's approval key

	err = s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorIs(err, types.ErrInvalidApproval)
}

func (s *TestSuite) TestMigrateBucket_RateLimitedRejected() {
	f := newMigrationFixture(s)
	f.wire()
	s.ctx.KVStore(s.storeKey).Set(types.GetBucketFlowRateLimitStatusKey(f.bucketName),
		s.cdc.MustMarshal(&types.BucketFlowRateLimitStatus{IsBucketLimited: true}))
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().ErrorContains(err, "rate limited")
}

func (s *TestSuite) TestMigrateBucket_Success() {
	f := newMigrationFixture(s)
	f.wire()
	approval, approvalBytes := f.approval(f.dstPriv)

	err := s.storageKeeper.MigrateBucket(s.ctx, f.operator, f.bucketName, f.dstSP.Id, approval, approvalBytes)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, f.bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_MIGRATING, got.BucketStatus)

	mig, found := s.storageKeeper.GetMigrationBucketInfo(s.ctx, f.bucketID)
	s.Require().True(found)
	s.Require().Equal(f.srcSP.Id, mig.SrcSpId)
	s.Require().Equal(f.dstSP.Id, mig.DstSpId)
	s.Require().Equal(f.srcFamilyID, mig.SrcGlobalVirtualGroupFamilyId)
}

// ---------------------------------------------------------------------------
// CompleteMigrateBucket: the non-empty-LVG rebinding path
// ---------------------------------------------------------------------------

// completeMigrateBucketRebindFixture seeds a bucket mid-migration to a dst SP
// with ONE bound local virtual group -- unlike setupMigratingBucket's other
// callers, which deliberately keep the bucket empty (see its doc comment) to
// avoid exercising verifyGVGSignatures / RebindingVirtualGroup's per-LVG loop.
// It returns everything needed to drive CompleteMigrateBucket down that loop,
// plus a valid BLS signature over the production migration doc.
func (s *TestSuite) completeMigrateBucketRebindFixture() (bucketName string, dstOperator sdk.AccAddress, dstFamID, srcGVGID, dstGVGID uint32, goodSig []byte) {
	const (
		dstSpID       = uint32(2)
		srcFamilyID   = uint32(1)
		secondarySPID = uint32(9)
	)
	dstFamID, srcGVGID, dstGVGID = uint32(7), uint32(10), uint32(20)

	dstOperator = sample.RandAccAddress()
	bucketName = s.setupMigratingBucket(dstSpID, srcFamilyID)
	bucketInfo, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)

	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{
		TotalChargeSize:    0,
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: srcGVGID}},
	})

	dstSP := &sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}
	srcSP := &sptypes.StorageProvider{Id: 5}

	blsPriv, err := bls.GenerateBlsKey()
	s.Require().NoError(err)
	secondarySP := &sptypes.StorageProvider{Id: secondarySPID, BlsKey: blsPriv.PublicKey().Marshal()}

	doc := types.NewSecondarySpMigrationBucketSignDoc(s.ctx.ChainID(), bucketInfo.Id, dstSP.Id, srcGVGID, dstGVGID)
	hash := doc.GetBlsSignHash()
	sig, err := blsPriv.Sign(hash[:], votepool.DST)
	s.Require().NoError(err)
	goodSig, err = sig.Marshal()
	s.Require().NoError(err)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(dstSP, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), dstFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: dstFamID, PrimarySpId: dstSP.Id}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSP.Id}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(srcSP, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	dstGVG := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: dstGVGID, PrimarySpId: dstSP.Id, SecondarySpIds: []uint32{secondarySPID}}
	srcGVG := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: srcGVGID, PrimarySpId: srcSP.Id}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), dstGVGID).Return(dstGVG, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), srcGVGID).Return(srcGVG, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), secondarySPID).Return(secondarySP, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVG(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return bucketName, dstOperator, dstFamID, srcGVGID, dstGVGID, goodSig
}

func (s *TestSuite) TestCompleteMigrateBucket_RebindsNonEmptyLocalVirtualGroups() {
	bucketName, dstOperator, dstFamID, srcGVGID, dstGVGID, goodSig := s.completeMigrateBucketRebindFixture()

	mappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: srcGVGID, DstGlobalVirtualGroupId: dstGVGID, SecondarySpBlsSignature: goodSig}}
	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, mappings)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
	s.Require().Equal(dstFamID, got.GlobalVirtualGroupFamilyId)

	internalBucketInfo, found := s.storageKeeper.GetInternalBucketInfo(s.ctx, got.Id)
	s.Require().True(found)
	s.Require().Equal(dstGVGID, internalBucketInfo.LocalVirtualGroups[0].GlobalVirtualGroupId, "the LVG must now point at the dst GVG")
}

func (s *TestSuite) TestCompleteMigrateBucket_InvalidGVGSignatureRejected() {
	bucketName, dstOperator, dstFamID, srcGVGID, dstGVGID, goodSig := s.completeMigrateBucketRebindFixture()

	badSig := append([]byte{}, goodSig...)
	badSig[0] ^= 0xFF
	mappings := []*types.GVGMapping{{SrcGlobalVirtualGroupId: srcGVGID, DstGlobalVirtualGroupId: dstGVGID, SecondarySpBlsSignature: badSig}}
	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, mappings)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)

	// nothing was mutated: the bucket is still migrating.
	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_MIGRATING, got.BucketStatus)
}

// ---------------------------------------------------------------------------
// CancelBucketMigration / RejectBucketMigration
// ---------------------------------------------------------------------------

func (s *TestSuite) TestCancelBucketMigration_Success() {
	bucketName := s.setupMigratingBucket(2, 1)
	bucketInfo, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	owner := sdk.MustAccAddressFromHex(bucketInfo.Owner)

	err := s.storageKeeper.CancelBucketMigration(s.ctx, owner, bucketName)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
	_, found = s.storageKeeper.GetMigrationBucketInfo(s.ctx, bucketInfo.Id)
	s.Require().False(found, "the migration record must be removed on cancel")
}

func (s *TestSuite) TestCancelBucketMigration_NotOwnerRejected() {
	bucketName := s.setupMigratingBucket(2, 1)

	err := s.storageKeeper.CancelBucketMigration(s.ctx, sample.RandAccAddress(), bucketName)
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestCancelBucketMigration_NotMigratingRejected() {
	owner := sample.RandAccAddress()
	bucketName := "cancel-not-migrating-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		BucketStatus: types.BUCKET_STATUS_CREATED,
	})

	err := s.storageKeeper.CancelBucketMigration(s.ctx, owner, bucketName)
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestCancelBucketMigration_BucketNotFound() {
	err := s.storageKeeper.CancelBucketMigration(s.ctx, sample.RandAccAddress(), "cancel-missing-bucket")
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestRejectBucketMigration_Success() {
	const dstSpID = uint32(3)
	bucketName := s.setupMigratingBucket(dstSpID, 1)
	bucketInfo, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)

	dstOperator := sample.RandAccAddress()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), dstSpID).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}).AnyTimes()

	err := s.storageKeeper.RejectBucketMigration(s.ctx, dstOperator, bucketName)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_CREATED, got.BucketStatus)
	_, found = s.storageKeeper.GetMigrationBucketInfo(s.ctx, bucketInfo.Id)
	s.Require().False(found)
}

func (s *TestSuite) TestRejectBucketMigration_NotDstSPRejected() {
	const dstSpID = uint32(3)
	bucketName := s.setupMigratingBucket(dstSpID, 1)
	dstOperator := sample.RandAccAddress()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), dstSpID).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}).AnyTimes()

	err := s.storageKeeper.RejectBucketMigration(s.ctx, sample.RandAccAddress(), bucketName)
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestRejectBucketMigration_NotMigratingRejected() {
	owner := sample.RandAccAddress()
	bucketName := "reject-not-migrating-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		BucketStatus: types.BUCKET_STATUS_CREATED,
	})

	err := s.storageKeeper.RejectBucketMigration(s.ctx, owner, bucketName)
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestRejectBucketMigration_BucketNotFound() {
	err := s.storageKeeper.RejectBucketMigration(s.ctx, sample.RandAccAddress(), "reject-missing-bucket")
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

// ---------------------------------------------------------------------------
// Discontinue-object / discontinue-bucket queue bookkeeping
// ---------------------------------------------------------------------------

func (s *TestSuite) TestAppendDiscontinueObjectIds_MergesWithExistingQueueEntry() {
	ts := int64(12345)
	s.storageKeeper.AppendDiscontinueObjectIds(s.ctx, ts, []sdkmath.Uint{sdkmath.NewUint(1), sdkmath.NewUint(2)})
	s.storageKeeper.AppendDiscontinueObjectIds(s.ctx, ts, []sdkmath.Uint{sdkmath.NewUint(3)})

	bz := s.ctx.KVStore(s.storeKey).Get(types.GetDiscontinueObjectIdsKey(ts))
	s.Require().NotNil(bz)
	var ids types.Ids
	s.cdc.MustUnmarshal(bz, &ids)
	s.Require().Equal([]sdkmath.Uint{sdkmath.NewUint(1), sdkmath.NewUint(2), sdkmath.NewUint(3)}, ids.Id)
}

func (s *TestSuite) TestDeleteDiscontinueObjectsUntil_PartialCapRequeuesRemainder() {
	ts := s.ctx.BlockTime().Unix()
	ids := []sdkmath.Uint{sdkmath.NewUint(101), sdkmath.NewUint(102), sdkmath.NewUint(103)}
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.GetDiscontinueObjectIdsKey(ts), s.cdc.MustMarshal(&types.Ids{Id: ids}))

	// None of these ids have a stored ObjectInfo, so ForceDeleteObject treats
	// each as "already deleted" and trivially succeeds with no mocks needed.
	deleted, err := s.storageKeeper.DeleteDiscontinueObjectsUntil(s.ctx, ts, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), deleted)

	bz := store.Get(types.GetDiscontinueObjectIdsKey(ts))
	s.Require().NotNil(bz, "the un-processed remainder must be requeued under the same key")
	var remaining types.Ids
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{ids[2]}, remaining.Id)
}

func (s *TestSuite) TestDeleteDiscontinueObjectsUntil_FullyDrainsDeletesKey() {
	ts := s.ctx.BlockTime().Unix()
	ids := []sdkmath.Uint{sdkmath.NewUint(201), sdkmath.NewUint(202)}
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.GetDiscontinueObjectIdsKey(ts), s.cdc.MustMarshal(&types.Ids{Id: ids}))

	deleted, err := s.storageKeeper.DeleteDiscontinueObjectsUntil(s.ctx, ts, 10)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), deleted)
	s.Require().False(store.Has(types.GetDiscontinueObjectIdsKey(ts)), "a fully-drained entry must be deleted, not left empty")
}

func (s *TestSuite) TestDeleteDiscontinueObjectsUntil_PropagatesForceDeleteError() {
	ts := s.ctx.BlockTime().Unix()
	// An object that exists but whose bucket does not: ForceDeleteObject's
	// bucket lookup fails, and the error must propagate out immediately.
	objID := sdkmath.NewUint(301)
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objID, BucketName: "discontinue-missing-bucket", ObjectName: "orphan-object",
	})
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectIdsKey(ts), s.cdc.MustMarshal(&types.Ids{Id: []sdkmath.Uint{objID}}))

	deleted, err := s.storageKeeper.DeleteDiscontinueObjectsUntil(s.ctx, ts, 10)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
	s.Require().Equal(uint64(0), deleted)
}

func (s *TestSuite) TestDeleteDiscontinueBucketsUntil_PartialCapRequeuesRemainder() {
	ts := s.ctx.BlockTime().Unix()
	ids := []sdkmath.Uint{sdkmath.NewUint(401), sdkmath.NewUint(402), sdkmath.NewUint(403)}
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.GetDiscontinueBucketIDsKey(ts), s.cdc.MustMarshal(&types.Ids{Id: ids}))

	// None of these ids have a stored BucketInfo, so ForceDeleteBucket treats
	// each as "already deleted" and trivially succeeds with no mocks needed.
	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, ts, 2)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), deleted)

	bz := store.Get(types.GetDiscontinueBucketIDsKey(ts))
	s.Require().NotNil(bz, "the un-processed remainder must be requeued under the same key")
	var remaining types.Ids
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{ids[2]}, remaining.Id)
}

func (s *TestSuite) TestDeleteDiscontinueBucketsUntil_FullyDrainsDeletesKey() {
	ts := s.ctx.BlockTime().Unix()
	ids := []sdkmath.Uint{sdkmath.NewUint(501), sdkmath.NewUint(502)}
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.GetDiscontinueBucketIDsKey(ts), s.cdc.MustMarshal(&types.Ids{Id: ids}))

	deleted, err := s.storageKeeper.DeleteDiscontinueBucketsUntil(s.ctx, ts, 10)
	s.Require().NoError(err)
	s.Require().Equal(uint64(2), deleted)
	s.Require().False(store.Has(types.GetDiscontinueBucketIDsKey(ts)), "a fully-drained entry must be deleted, not left empty")
}

// ---------------------------------------------------------------------------
// PersistDeleteInfo
// ---------------------------------------------------------------------------

func (s *TestSuite) TestPersistDeleteInfo_NoOpWhenKeyAbsent() {
	s.storageKeeper.PersistDeleteInfo(s.ctx)
	s.Require().False(s.ctx.KVStore(s.storeKey).Has(types.GetDeleteStalePoliciesKey(s.ctx.BlockHeight())))
}

func (s *TestSuite) TestPersistDeleteInfo_NoOpWhenEmpty() {
	store := s.ctx.KVStore(s.storeKey)
	store.Set(types.CurrentBlockDeleteStalePoliciesKey, s.cdc.MustMarshal(&types.DeleteInfo{
		BucketIds: &types.Ids{}, ObjectIds: &types.Ids{}, GroupIds: &types.Ids{},
	}))

	s.storageKeeper.PersistDeleteInfo(s.ctx)

	s.Require().False(store.Has(types.GetDeleteStalePoliciesKey(s.ctx.BlockHeight())),
		"an empty delete-info must not be persisted")
}

func (s *TestSuite) TestPersistDeleteInfo_PersistsAndEmitsEvent() {
	store := s.ctx.KVStore(s.storeKey)
	objID := sdkmath.NewUint(9)
	store.Set(types.CurrentBlockDeleteStalePoliciesKey, s.cdc.MustMarshal(&types.DeleteInfo{
		ObjectIds: &types.Ids{Id: []sdkmath.Uint{objID}},
	}))

	s.storageKeeper.PersistDeleteInfo(s.ctx)

	bz := store.Get(types.GetDeleteStalePoliciesKey(s.ctx.BlockHeight()))
	s.Require().NotNil(bz, "a non-empty delete-info must be persisted under the block height")
	var persisted types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &persisted)
	s.Require().Equal([]sdkmath.Uint{objID}, persisted.ObjectIds.Id)

	emitted := false
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type == proto.MessageName(&types.EventStalePolicyCleanup{}) {
			emitted = true
		}
	}
	s.Require().True(emitted, "EventStalePolicyCleanup must be emitted")
}

// ---------------------------------------------------------------------------
// GarbageCollectResourcesStalePolicy / garbageCollectionForResource
// ---------------------------------------------------------------------------

func (s *TestSuite) TestGarbageCollectResourcesStalePolicy_FullCleanupAcrossAllResourceTypes() {
	store := s.ctx.KVStore(s.storeKey)
	height := int64(10)
	objID, bucketID, groupID := sdkmath.NewUint(1), sdkmath.NewUint(2), sdkmath.NewUint(3)
	store.Set(types.GetDeleteStalePoliciesKey(height), s.cdc.MustMarshal(&types.DeleteInfo{
		ObjectIds: &types.Ids{Id: []sdkmath.Uint{objID}},
		BucketIds: &types.Ids{Id: []sdkmath.Uint{bucketID}},
		GroupIds:  &types.Ids{Id: []sdkmath.Uint{groupID}},
	}))

	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, objID).Return(uint64(1), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, objID).Return(uint64(2), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_BUCKET, bucketID).Return(uint64(3), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_BUCKET, bucketID).Return(uint64(4), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, groupID).Return(uint64(5), true).AnyTimes()
	// A GROUP id must only trigger ForceDeleteGroupMembers, never
	// ForceDeleteGroupPolicyForResource (no expectation is registered for
	// that combination, so gomock fails the test if it is ever called).
	s.permissionKeeper.EXPECT().ForceDeleteGroupMembers(gomock.Any(), gomock.Any(), gomock.Any(), groupID).Return(uint64(6), true).AnyTimes()

	s.storageKeeper.GarbageCollectResourcesStalePolicy(s.ctx)

	s.Require().False(store.Has(types.GetDeleteStalePoliciesKey(height)), "a fully-cleaned entry must be removed")
}

func (s *TestSuite) TestGarbageCollectResourcesStalePolicy_PartialCapHaltsIterationAndRequeues() {
	store := s.ctx.KVStore(s.storeKey)
	doneHeight, haltHeight := int64(10), int64(20)
	doneID := sdkmath.NewUint(11)
	haltFirst, haltSecond := sdkmath.NewUint(21), sdkmath.NewUint(22)

	store.Set(types.GetDeleteStalePoliciesKey(doneHeight), s.cdc.MustMarshal(&types.DeleteInfo{
		ObjectIds: &types.Ids{Id: []sdkmath.Uint{doneID}},
	}))
	store.Set(types.GetDeleteStalePoliciesKey(haltHeight), s.cdc.MustMarshal(&types.DeleteInfo{
		ObjectIds: &types.Ids{Id: []sdkmath.Uint{haltFirst, haltSecond}},
		BucketIds: &types.Ids{Id: []sdkmath.Uint{sdkmath.NewUint(99)}}, // must survive untouched
	}))

	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, doneID).Return(uint64(1), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, doneID).Return(uint64(2), true).AnyTimes()

	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, haltFirst).Return(uint64(3), true).AnyTimes()
	s.permissionKeeper.EXPECT().ForceDeleteGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, haltFirst).Return(uint64(4), true).AnyTimes()
	// The cap is reached on the second id: the whole GC pass must halt here,
	// leaving the bucket ids on the SAME entry completely untouched.
	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, haltSecond).Return(uint64(4), false).AnyTimes()

	s.storageKeeper.GarbageCollectResourcesStalePolicy(s.ctx)

	s.Require().False(store.Has(types.GetDeleteStalePoliciesKey(doneHeight)), "the fully-processed earlier entry must still be deleted")

	bz := store.Get(types.GetDeleteStalePoliciesKey(haltHeight))
	s.Require().NotNil(bz, "the halted entry must remain persisted, not deleted")
	var remaining types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{haltSecond}, remaining.ObjectIds.Id, "only the un-processed id must be requeued")
	s.Require().Equal([]sdkmath.Uint{sdkmath.NewUint(99)}, remaining.BucketIds.Id, "the halt must happen before the bucket pass ever runs")
}

func (s *TestSuite) TestGarbageCollectResourcesStalePolicy_GroupMembersCapRequeues() {
	store := s.ctx.KVStore(s.storeKey)
	height := int64(30)
	groupID := sdkmath.NewUint(41)
	store.Set(types.GetDeleteStalePoliciesKey(height), s.cdc.MustMarshal(&types.DeleteInfo{
		GroupIds: &types.Ids{Id: []sdkmath.Uint{groupID}},
	}))

	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, groupID).Return(uint64(1), true).AnyTimes()
	// group members alone hit the cap: the group's own policy call must not run.
	s.permissionKeeper.EXPECT().ForceDeleteGroupMembers(gomock.Any(), gomock.Any(), gomock.Any(), groupID).Return(uint64(1), false).AnyTimes()

	s.storageKeeper.GarbageCollectResourcesStalePolicy(s.ctx)

	bz := store.Get(types.GetDeleteStalePoliciesKey(height))
	s.Require().NotNil(bz, "the halted group entry must remain persisted")
	var remaining types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{groupID}, remaining.GroupIds.Id)
}

// ---------------------------------------------------------------------------
// SetTag
// ---------------------------------------------------------------------------

func (s *TestSuite) TestSetTag_Bucket_Success() {
	operator := sample.RandAccAddress()
	bucketName := "settag-bucket-owner"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: operator.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	grn := *types2.NewBucketGRN(bucketName)
	tags := &types.ResourceTags{Tags: []types.ResourceTags_Tag{{Key: "env", Value: "prod"}}}

	err := s.storageKeeper.SetTag(s.ctx, operator, grn, tags)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(tags.Tags, got.Tags.Tags)
}

func (s *TestSuite) TestSetTag_Bucket_PermissionDenied() {
	owner := sample.RandAccAddress()
	bucketName := "settag-bucket-denied"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	grn := *types2.NewBucketGRN(bucketName)
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestSetTag_Object_Success() {
	operator := sample.RandAccAddress()
	bucketName, objectName := "settag-object-bucket", "settag-object"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: operator.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{Owner: operator.String(), BucketName: bucketName, ObjectName: objectName, Id: sdkmath.NewUint(1)})
	grn := *types2.NewObjectGRN(bucketName, objectName)
	tags := &types.ResourceTags{Tags: []types.ResourceTags_Tag{{Key: "k", Value: "v"}}}

	err := s.storageKeeper.SetTag(s.ctx, operator, grn, tags)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().True(found)
	s.Require().Equal(tags.Tags, got.Tags.Tags)
}

func (s *TestSuite) TestSetTag_Object_PermissionDenied() {
	owner := sample.RandAccAddress()
	bucketName, objectName := "settag-object-denied-bucket", "settag-object-denied"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{Owner: owner.String(), BucketName: bucketName, ObjectName: objectName, Id: sdkmath.NewUint(1)})
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	grn := *types2.NewObjectGRN(bucketName, objectName)
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestSetTag_Group_Success() {
	owner := sample.RandAccAddress()
	groupName := "settag-group"
	groupID := sdkmath.NewUint(1)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Owner: owner.String(), GroupName: groupName, Id: groupID})
	s.ctx.KVStore(s.storeKey).Set(types.GetGroupKey(owner, groupName), sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(groupID))

	grn := *types2.NewGroupGRN(owner, groupName)
	tags := &types.ResourceTags{Tags: []types.ResourceTags_Tag{{Key: "k", Value: "v"}}}
	err := s.storageKeeper.SetTag(s.ctx, owner, grn, tags)
	s.Require().NoError(err)

	got, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	s.Require().Equal(tags.Tags, got.Tags.Tags)
}

func (s *TestSuite) TestSetTag_Group_PermissionDenied() {
	owner := sample.RandAccAddress()
	groupName := "settag-group-denied"
	groupID := sdkmath.NewUint(1)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Owner: owner.String(), GroupName: groupName, Id: groupID})
	s.ctx.KVStore(s.storeKey).Set(types.GetGroupKey(owner, groupName), sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(groupID))
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	grn := *types2.NewGroupGRN(owner, groupName)
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// Gap-fill: appendResourceIDForGarbageCollection / garbageCollectionForResource
// ---------------------------------------------------------------------------

func (s *TestSuite) TestAppendResourceIDForGarbageCollection_GroupWithMembersStillEnqueuesForCleanup() {
	owner := sample.RandAccAddress()
	groupName, groupID := "gc-group-with-members", sdkmath.NewUint(77)
	// a second group deleted in the SAME block: the first append creates the
	// current-block queue entry from scratch, the second must load and extend it.
	groupName2, groupID2 := "gc-group-with-members-2", sdkmath.NewUint(78)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Owner: owner.String(), GroupName: groupName, Id: groupID})
	s.ctx.KVStore(s.storeKey).Set(types.GetGroupKey(owner, groupName), sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(groupID))
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Owner: owner.String(), GroupName: groupName2, Id: groupID2})
	s.ctx.KVStore(s.storeKey).Set(types.GetGroupKey(owner, groupName2), sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(groupID2))

	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_GROUP, gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	s.Require().NoError(s.storageKeeper.DeleteGroup(s.ctx, owner, groupName, types.DeleteGroupOptions{}))
	s.Require().NoError(s.storageKeeper.DeleteGroup(s.ctx, owner, groupName2, types.DeleteGroupOptions{}))

	_, found := s.storageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().False(found)

	bz := s.ctx.KVStore(s.storeKey).Get(types.CurrentBlockDeleteStalePoliciesKey)
	s.Require().NotNil(bz, "a group with members must still be queued for GC even with no direct policy")
	var deleteInfo types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &deleteInfo)
	s.Require().Equal([]sdkmath.Uint{groupID, groupID2}, deleteInfo.GroupIds.Id, "both deletions in the same block must accumulate in one queue entry")
}

func (s *TestSuite) TestAppendResourceIDForGarbageCollection_BucketAppendsToCurrentBlockQueue() {
	const familyID = uint32(31)
	bucketID := sdkmath.NewUint(81)
	bucketName := "gc-bucket-with-policy"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: bucketID, BucketName: bucketName, GlobalVirtualGroupFamilyId: familyID,
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
	})
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{})
	// rate-limited so ForceDeleteBucket's UnChargeBucketReadFee no-ops instead of
	// needing a payment/price fixture -- irrelevant to the branch under test.
	s.ctx.KVStore(s.storeKey).Set(types.GetBucketFlowRateLimitStatusKey(bucketName),
		s.cdc.MustMarshal(&types.BucketFlowRateLimitStatus{IsBucketLimited: true}))

	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), familyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: uint32(5)}, true).AnyTimes()
	// the primary SP is gone: ForceDeleteBucket treats this as an orphaned bucket
	// and continues (attributing the deletion to the module) rather than erroring.
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(5)).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_BUCKET, bucketID).Return(true).AnyTimes()

	bucketDeleted, objectsDeleted, err := s.storageKeeper.ForceDeleteBucket(s.ctx, bucketID, 10)
	s.Require().NoError(err)
	s.Require().True(bucketDeleted)
	s.Require().Equal(uint64(0), objectsDeleted)

	bz := s.ctx.KVStore(s.storeKey).Get(types.CurrentBlockDeleteStalePoliciesKey)
	s.Require().NotNil(bz, "a bucket with an account policy must be queued for GC")
	var deleteInfo types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &deleteInfo)
	s.Require().Equal([]sdkmath.Uint{bucketID}, deleteInfo.BucketIds.Id)
}

func (s *TestSuite) TestAppendResourceIDForGarbageCollection_ObjectAppendsToCurrentBlockQueue() {
	const familyID = uint32(11)
	bucketID, objID := sdkmath.NewUint(91), sdkmath.NewUint(92)
	bucketName, objectName := "gc-object-bucket", "gc-object"

	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: bucketID, BucketName: bucketName, GlobalVirtualGroupFamilyId: familyID,
		PaymentAddress: sample.RandAccAddress().String(),
	})
	// seed a versioned-params entry at the current block time, then move the
	// clock forward: GetVersionedParamsWithTS resolves strictly before the
	// queried timestamp (see the package's own risk note on this footgun).
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MaxSegmentSize: 1}))
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Second))

	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Id: objID, BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: types.OBJECT_STATUS_CREATED, PayloadSize: 100,
		UpdatedAt: s.ctx.BlockTime().Unix(),
	})
	// mirrors saveDiscontinueObjectStatus's own encoding: a real DiscontinueObject
	// call would have queued this object's pre-discontinue status the same way.
	statusBz := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBz, uint32(types.OBJECT_STATUS_CREATED))
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectStatusKey(objID), statusBz)

	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyNewDec(1)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ReserveTime: 0, ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), familyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: uint32(6)}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(6)).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, objID).Return(true).AnyTimes()

	err := s.storageKeeper.ForceDeleteObject(s.ctx, objID)
	s.Require().NoError(err)

	_, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found)

	bz := s.ctx.KVStore(s.storeKey).Get(types.CurrentBlockDeleteStalePoliciesKey)
	s.Require().NotNil(bz, "an object with an account policy must be queued for GC")
	var deleteInfo types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &deleteInfo)
	s.Require().Equal([]sdkmath.Uint{objID}, deleteInfo.ObjectIds.Id)
}

func (s *TestSuite) TestGarbageCollectResourcesStalePolicy_GroupPolicyForResourceCapRequeues() {
	store := s.ctx.KVStore(s.storeKey)
	height := int64(40)
	objID := sdkmath.NewUint(51)
	store.Set(types.GetDeleteStalePoliciesKey(height), s.cdc.MustMarshal(&types.DeleteInfo{
		ObjectIds: &types.Ids{Id: []sdkmath.Uint{objID}},
	}))

	s.permissionKeeper.EXPECT().ForceDeleteAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, objID).Return(uint64(1), true).AnyTimes()
	// the account-policy pass succeeds, but the group-policy pass for the SAME
	// id hits the cap: the whole GC pass must halt here, not treat the id as done.
	s.permissionKeeper.EXPECT().ForceDeleteGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any(), gnfdresource.RESOURCE_TYPE_OBJECT, objID).Return(uint64(1), false).AnyTimes()

	s.storageKeeper.GarbageCollectResourcesStalePolicy(s.ctx)

	bz := store.Get(types.GetDeleteStalePoliciesKey(height))
	s.Require().NotNil(bz, "the halted entry must remain persisted")
	var remaining types.DeleteInfo
	s.cdc.MustUnmarshal(bz, &remaining)
	s.Require().Equal([]sdkmath.Uint{objID}, remaining.ObjectIds.Id, "the un-processed id must be requeued")
}

// ---------------------------------------------------------------------------
// Gap-fill: CompleteMigrateBucket
// ---------------------------------------------------------------------------

func (s *TestSuite) TestCompleteMigrateBucket_BucketNotFound() {
	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, sample.RandAccAddress(), "cmb-missing-bucket", 1, nil)
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestCompleteMigrateBucket_DstStorageProviderNotFound() {
	bucketName := "cmb-dst-sp-missing"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Id: sdkmath.NewUint(1), BucketName: bucketName})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, sample.RandAccAddress(), bucketName, 1, nil)
	s.Require().ErrorIs(err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteMigrateBucket_NotMigratingRejected() {
	operator := sample.RandAccAddress()
	bucketName := "cmb-not-migrating"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: sdkmath.NewUint(1), BucketName: bucketName, BucketStatus: types.BUCKET_STATUS_CREATED,
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), operator).
		Return(&sptypes.StorageProvider{Id: 1, OperatorAddress: operator.String()}, true).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, operator, bucketName, 1, nil)
	s.Require().ErrorIs(err, types.ErrInvalidBucketStatus)
}

func (s *TestSuite) TestCompleteMigrateBucket_MigrationInfoNotFound() {
	operator := sample.RandAccAddress()
	bucketName := "cmb-no-migration-record"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Id: sdkmath.NewUint(1), BucketName: bucketName, BucketStatus: types.BUCKET_STATUS_MIGRATING,
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), operator).
		Return(&sptypes.StorageProvider{Id: 1, OperatorAddress: operator.String()}, true).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, operator, bucketName, 1, nil)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)
}

func (s *TestSuite) TestCompleteMigrateBucket_DstStorageProviderMismatch() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	// the operator resolves to a DIFFERENT SP than the one recorded as the
	// migration's destination -- reject before touching virtual-group/payment state.
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: 999, OperatorAddress: dstOperator.String()}, true).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, 1, nil)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)
}

func (s *TestSuite) TestCompleteMigrateBucket_DstGVGFamilyNotFound() {
	const (
		dstSpID      = uint32(2)
		srcFamilyID  = uint32(1)
		missingFamID = uint32(404)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), missingFamID).Return(nil, false).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, missingFamID, nil)
	s.Require().ErrorIs(err, virtualgroupmoduletypes.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestCompleteMigrateBucket_SrcGVGFamilyNotFound() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		dstFamID    = uint32(7)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), dstFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: dstFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	// the bucket's OWN (src) family has vanished from virtual-group state.
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).Return(nil, false).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, nil)
	s.Require().ErrorIs(err, virtualgroupmoduletypes.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestCompleteMigrateBucket_StreamAccountFrozenRejected() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		srcSpID     = uint32(5)
		dstFamID    = uint32(7)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), dstFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: dstFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID}, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_FROZEN}, true).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, nil)
	s.Require().ErrorIs(err, paymenttypes.ErrInvalidStreamAccountStatus)
}

func (s *TestSuite) TestCompleteMigrateBucket_SettleAndDistributeGVGFamilyErrorPropagates() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		srcSpID     = uint32(5)
		dstFamID    = uint32(7)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), dstFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: dstFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSpID).Return(&sptypes.StorageProvider{Id: srcSpID}, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("settle boom")).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, nil)
	s.Require().ErrorIs(err, virtualgroupmoduletypes.ErrSettleFailed)
}

func (s *TestSuite) TestCompleteMigrateBucket_UnChargeBucketReadStoreFeeErrorPropagates() {
	const (
		dstSpID     = uint32(2)
		srcFamilyID = uint32(1)
		srcSpID     = uint32(5)
		dstFamID    = uint32(7)
	)
	dstOperator := sample.RandAccAddress()
	bucketName := s.setupMigratingBucket(dstSpID, srcFamilyID)

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), dstOperator).
		Return(&sptypes.StorageProvider{Id: dstSpID, OperatorAddress: dstOperator.String()}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), dstFamID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: dstFamID, PrimarySpId: dstSpID}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), srcFamilyID).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: srcFamilyID, PrimarySpId: srcSpID}, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSpID).Return(&sptypes.StorageProvider{Id: srcSpID}, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetStreamRecord(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{Status: paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(errors.New("apply boom")).AnyTimes()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, nil)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)
}

func (s *TestSuite) TestCompleteMigrateBucket_RebindingVirtualGroupErrorPropagates() {
	// reuse the non-empty-LVG fixture but pass no GVG mappings: verifyGVGSignatures
	// trivially passes on an empty slice, so RebindingVirtualGroup's own "gvg not
	// found in mapping" check is what fails, propagating out as ErrMigrationBucketFailed.
	bucketName, dstOperator, dstFamID, _, _, _ := s.completeMigrateBucketRebindFixture()

	err := s.storageKeeper.CompleteMigrateBucket(s.ctx, dstOperator, bucketName, dstFamID, nil)
	s.Require().ErrorIs(err, types.ErrMigrationBucketFailed)

	got, found := s.storageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().True(found)
	s.Require().Equal(types.BUCKET_STATUS_MIGRATING, got.BucketStatus, "a failed rebind must not mutate the bucket")
}

// ---------------------------------------------------------------------------
// Gap-fill: SetTag
// ---------------------------------------------------------------------------

func (s *TestSuite) TestSetTag_Bucket_NotFound() {
	grn := *types2.NewBucketGRN("settag-missing-bucket")
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestSetTag_Object_NotFound() {
	bucketName := "settag-object-missing-bucket"
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Id: sdkmath.NewUint(1), BucketName: bucketName})
	grn := *types2.NewObjectGRN(bucketName, "settag-missing-object")
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrNoSuchObject)
}

func (s *TestSuite) TestSetTag_Object_BucketNotFound() {
	bucketName, objectName := "settag-orphan-object-bucket", "settag-orphan-object"
	// the object's own index exists, but its bucket does not.
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{Id: sdkmath.NewUint(1), BucketName: bucketName, ObjectName: objectName})
	grn := *types2.NewObjectGRN(bucketName, objectName)
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestSetTag_Object_MalformedGRNPropagatesError() {
	// an empty bucket name inside an object GRN fails GetBucketAndObjectName's own
	// validation before SetTag ever looks anything up in the store.
	grn := *types2.NewObjectGRN("", "settag-object")
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, gnfderrors.ErrInvalidGRN)
}

func (s *TestSuite) TestSetTag_Group_NotFound() {
	grn := *types2.NewGroupGRN(sample.RandAccAddress(), "settag-missing-group")
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), grn, &types.ResourceTags{})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestSetTag_UnknownResourceType_Rejected() {
	// the zero-value GRN carries RESOURCE_TYPE_UNSPECIFIED, which none of the
	// bucket/object/group constructors ever produce -- SetTag's default case.
	err := s.storageKeeper.SetTag(s.ctx, sample.RandAccAddress(), types2.GRN{}, &types.ResourceTags{})
	s.Require().ErrorIs(err, gnfderrors.ErrInvalidGRN)
}
