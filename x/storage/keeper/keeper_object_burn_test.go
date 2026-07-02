package keeper_test

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	types2 "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// BurnTestSuite is an independent test suite dedicated to testing NFT burn functionality.
// It does not depend on TestSuite in payment_test.go to avoid gomock expectation conflicts.
type BurnTestSuite struct {
	suite.Suite

	cdc           codec.Codec
	storageKeeper *keeper.Keeper
	storeKey      storetypes.StoreKey

	accountKeeper      *types.MockAccountKeeper
	spKeeper           *types.MockSpKeeper
	permissionKeeper   *types.MockPermissionKeeper
	paymentKeeper      *types.MockPaymentKeeper
	virtualGroupKeeper *types.MockVirtualGroupKeeper
	evmKeeper          *types.MockEVMKeeper

	ctx sdk.Context
}

func (s *BurnTestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	header := testCtx.Ctx.BlockHeader()
	header.Time = time.Now()
	testCtx = testutil.TestContext{
		Ctx: sdk.NewContext(testCtx.CMS, header, false, testCtx.Ctx.Logger()),
		DB:  testCtx.DB,
		CMS: testCtx.CMS,
	}
	s.ctx = testCtx.Ctx

	ctrl := gomock.NewController(s.T())

	accountKeeper := types.NewMockAccountKeeper(ctrl)
	spKeeper := types.NewMockSpKeeper(ctrl)
	permissionKeeper := types.NewMockPermissionKeeper(ctrl)
	paymentKeeper := types.NewMockPaymentKeeper(ctrl)
	virtualGroupKeeper := types.NewMockVirtualGroupKeeper(ctrl)
	evmKeeper := types.NewMockEVMKeeper(ctrl)

	// cosmos/evm v0.6.0 migration: the production burn path now really routes the
	// ERC-721 burn through the EVM keeper. The storage keeper's CallEVM packs the
	// calldata and delegates to its CallEVMWithData, which calls the (mock) EVM
	// keeper's CallEVMWithData. Each test sets its own expectation on
	// CallEVMWithData (success or error) so the success/failure outcome is explicit
	// per test; tests whose object never minted an NFT never reach this path.

	s.storageKeeper = keeper.NewKeeper(
		encCfg.Codec,
		key,
		key,
		accountKeeper,
		spKeeper,
		paymentKeeper,
		permissionKeeper,
		virtualGroupKeeper,
		evmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	s.cdc = encCfg.Codec
	s.storeKey = key
	s.accountKeeper = accountKeeper
	s.spKeeper = spKeeper
	s.permissionKeeper = permissionKeeper
	s.paymentKeeper = paymentKeeper
	s.virtualGroupKeeper = virtualGroupKeeper
	s.evmKeeper = evmKeeper

	err := s.storageKeeper.SetParams(s.ctx, types.DefaultParams())
	s.Require().NoError(err)
}

func TestBurnTestSuite(t *testing.T) {
	suite.Run(t, new(BurnTestSuite))
}

func (s *BurnTestSuite) TestDeleteSealedObjectShouldBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-burn"
	objectName := "obj-sealed"

	bucket := &types.BucketInfo{
		Owner:          operator.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(1),
		BucketStatus:   types.BUCKET_STATUS_CREATED,
		PaymentAddress: sample.RandAccAddress().String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(10),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		SourceType:          0,
		PayloadSize:         1,
		CreateAt:            s.ctx.BlockTime().Unix(),
		UpdatedAt:           s.ctx.BlockTime().Unix(),
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// seed versioned params for both ts=0 and current block time
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// mock dependencies for fee uncharge and sp discovery
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 0}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroup{
		Id: 0, FamilyId: 0, PrimarySpId: 0, SecondarySpIds: []uint32{}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	}, true).AnyTimes()
	spAddress, _, _ := sample.RandSignBytes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ReserveTime: 10000, ValidatorTaxRate: sdkmath.LegacyNewDec(1)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	// cosmos/evm v0.6.0 migration: a sealed object with PayloadSize>0 takes the
	// burn path, which now really routes through keeper.CallEVM -> CallEVMWithData
	// -> the (mock) EVM keeper. With the burn succeeding, the delete completes and
	// the object is gone.
	s.evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()

	err := s.storageKeeper.DeleteObject(s.ctx, operator, bucketName, objectName, types.DeleteObjectOptions{SourceType: 0})
	s.Require().NoError(err, "sealed object delete should attempt burn (which succeeds) and remove the object")
	_, found := s.storageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found, "object should be deleted after a successful burn")
}

func (s *BurnTestSuite) TestDeleteCreatedObjectShouldNotBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-no-burn"
	objectName := "obj-created"

	bucket := &types.BucketInfo{
		Owner:          operator.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(2),
		BucketStatus:   types.BUCKET_STATUS_CREATED,
		PaymentAddress: sample.RandAccAddress().String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	object := &types.ObjectInfo{
		Id:           sdkmath.NewUint(20),
		BucketName:   bucketName,
		ObjectName:   objectName,
		Owner:        operator.String(),
		ObjectStatus: types.OBJECT_STATUS_CREATED,
		SourceType:   0,
		PayloadSize:  1,
		CreateAt:     s.ctx.BlockTime().Unix(),
		UpdatedAt:    s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// seed versioned params for both ts=0 and current block time
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// mock dependencies for fee uncharge and sp discovery
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 0}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroup{
		Id: 0, FamilyId: 0, PrimarySpId: 0, SecondarySpIds: []uint32{}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	}, true).AnyTimes()
	spAddress, _, _ := sample.RandSignBytes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ReserveTime: 10000, ValidatorTaxRate: sdkmath.LegacyNewDec(1)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	// A CREATED (never sealed) object never minted an NFT, so the burn path is
	// skipped entirely and keeper.CallEVM is never reached. Delete therefore
	// succeeds even though CallEVM is stubbed/disabled during the cosmos/evm
	// v0.6.0 migration.
	err := s.storageKeeper.DeleteObject(s.ctx, operator, bucketName, objectName, types.DeleteObjectOptions{SourceType: 0})
	s.Require().NoError(err, "created object should not trigger burn, so delete should succeed")
}

func (s *BurnTestSuite) TestDeleteSealedObjectBurnFailShouldFail() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-burn-fail"
	objectName := "obj-sealed-fail"

	bucket := &types.BucketInfo{
		Owner:          operator.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(3),
		BucketStatus:   types.BUCKET_STATUS_CREATED,
		PaymentAddress: sample.RandAccAddress().String(),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(30),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		SourceType:          0,
		PayloadSize:         1,
		CreateAt:            s.ctx.BlockTime().Unix(),
		UpdatedAt:           s.ctx.BlockTime().Unix(),
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// seed versioned params
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// mocks
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 0}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroup{
		Id: 0, FamilyId: 0, PrimarySpId: 0, SecondarySpIds: []uint32{}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	}, true).AnyTimes()
	spAddress, _, _ := sample.RandSignBytes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ReserveTime: 10000, ValidatorTaxRate: sdkmath.LegacyNewDec(1)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	// cosmos/evm v0.6.0 migration: a sealed object with PayloadSize>0 always takes
	// the burn path. The storage keeper's CallEVM packs the calldata and delegates
	// to its CallEVMWithData, which calls the (mock) EVM keeper's CallEVMWithData.
	// Make that EVM burn call fail; delete must surface the failure rather than
	// swallow it.
	s.evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("burn failed")).AnyTimes()

	err := s.storageKeeper.DeleteObject(s.ctx, operator, bucketName, objectName, types.DeleteObjectOptions{SourceType: 0})
	s.Require().Error(err, "delete should fail when burn fails")
	s.Require().Contains(err.Error(), "burn failed")
}

// TestForceDeleteSealedObjectShouldBurnNFT verifies that ForceDeleteObject also triggers burn for SEALED objects
func (s *BurnTestSuite) TestForceDeleteSealedObjectShouldBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-force-burn"
	objectName := "obj-force-sealed"

	bucket := &types.BucketInfo{
		Owner:                      operator.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		BucketStatus:               types.BUCKET_STATUS_CREATED,
		PaymentAddress:             sample.RandAccAddress().String(), // Required for UnChargeObjectStoreFee
		GlobalVirtualGroupFamilyId: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)

	// Set InternalBucketInfo with a fixed historical timestamp for PriceTime
	priceTime := s.ctx.BlockTime().Unix() - 1000
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          priceTime,
		TotalChargeSize:    1,
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0, TotalChargeSize: 1}},
	})

	// Use the same historical timestamp for object's CreateAt to ensure versioned params can match
	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(10),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		SourceType:          0,
		PayloadSize:         1,
		CreateAt:            priceTime, // Use the same timestamp
		UpdatedAt:           priceTime,
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// Seed versioned params covering key timestamps
	prev := s.ctx.BlockTime()
	// Set params for priceTime
	s.ctx = s.ctx.WithBlockTime(time.Unix(priceTime, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	// Set params for time=0
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	// Restore current time
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// mark as discontinue by manually setting the status key (simulating saveDiscontinueObjectStatus)
	store := s.ctx.KVStore(s.storeKey)
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_SEALED))
	store.Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	// setup GVG
	gvg := &types2.GlobalVirtualGroup{Id: 0}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              1,
		OperatorAddress: operator.String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()

	// Add missing GVGFamily mock
	gvgFamily := &types2.GlobalVirtualGroupFamily{
		Id:                    0,
		VirtualPaymentAddress: operator.String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	// Add permission mocks (doDeleteObject needs these)
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	// cosmos/evm v0.6.0 migration: a sealed object with PayloadSize>0 takes the
	// burn path, which now really routes through keeper.CallEVM -> CallEVMWithData
	// -> the (mock) EVM keeper. With the burn succeeding, ForceDelete completes and
	// the object is removed.
	s.evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()

	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().NoError(err, "ForceDelete sealed object should attempt burn (which succeeds) and remove the object")
	_, found := s.storageKeeper.GetObjectInfoById(s.ctx, object.Id)
	s.Require().False(found, "object should be deleted after a successful burn")
}

// TestForceDeleteCreatedObjectShouldNotBurnNFT verifies that ForceDeleteObject does not trigger burn for CREATED objects
func (s *BurnTestSuite) TestForceDeleteCreatedObjectShouldNotBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-force-no-burn"
	objectName := "obj-force-created"

	bucket := &types.BucketInfo{
		Owner:                      operator.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		BucketStatus:               types.BUCKET_STATUS_CREATED,
		PaymentAddress:             sample.RandAccAddress().String(), // Required for UnlockObjectStoreFee
		GlobalVirtualGroupFamilyId: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)

	// Use a fixed historical timestamp
	createTime := s.ctx.BlockTime().Unix() - 500
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          createTime,
		TotalChargeSize:    0,
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(20),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_CREATED,
		SourceType:          0,
		PayloadSize:         1,
		CreateAt:            createTime, // Use the same timestamp
		UpdatedAt:           createTime,
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// Seed versioned params covering createTime
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(createTime, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// mark as discontinue with CREATED status by manually setting the status key
	store := s.ctx.KVStore(s.storeKey)
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_CREATED))
	store.Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              1,
		OperatorAddress: operator.String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	// Add GVGFamily mock for created object test
	gvgFamily := &types2.GlobalVirtualGroupFamily{
		Id:                    0,
		VirtualPaymentAddress: operator.String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	// Add permission mocks (doDeleteObject needs these)
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetOutFlows(gomock.Any(), gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes() // Returns only one value
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes() // Required for UnlockObjectStoreFee

	// A CREATED (never sealed) object never minted an NFT, so the burn path is
	// skipped entirely and keeper.CallEVM is never reached. ForceDelete therefore
	// succeeds even though CallEVM is stubbed/disabled during the cosmos/evm
	// v0.6.0 migration.
	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().NoError(err, "ForceDelete created object should not trigger burn, so delete should succeed")
}

// TestForceDeleteDiscontinuedButOriginallySealedObjectShouldBurnNFT verifies that ForceDeleteObject
// triggers burn for objects with DISCONTINUED status but originally SEALED (the audit feedback scenario)
func (s *BurnTestSuite) TestForceDeleteDiscontinuedButOriginallySealedObjectShouldBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-discontinued-sealed"
	objectName := "obj-discontinued-sealed"

	bucket := &types.BucketInfo{
		Owner:                      operator.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		BucketStatus:               types.BUCKET_STATUS_CREATED,
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)

	priceTime := s.ctx.BlockTime().Unix() - 1000
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          priceTime,
		TotalChargeSize:    1,
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0, TotalChargeSize: 1}},
	})

	// Create object with DISCONTINUED status (simulating the state after DiscontinueObject was called)
	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(30),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_DISCONTINUED, // Current status is DISCONTINUED
		SourceType:          0,
		PayloadSize:         1,
		CreateAt:            priceTime,
		UpdatedAt:           priceTime,
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// Seed versioned params
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(priceTime, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// Manually set the discontinue status key to SEALED (simulating saveDiscontinueObjectStatus)
	// This represents the ORIGINAL status before the object was marked as DISCONTINUED
	store := s.ctx.KVStore(s.storeKey)
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_SEALED)) // Original status: SEALED
	store.Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	// Setup mocks
	gvg := &types2.GlobalVirtualGroup{Id: 0}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              1,
		OperatorAddress: operator.String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()

	gvgFamily := &types2.GlobalVirtualGroupFamily{
		Id:                    0,
		VirtualPaymentAddress: operator.String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	// This is the key test: object.ObjectStatus = DISCONTINUED, but originalStatus = SEALED.
	// The burn must still be triggered based on originalStatus. cosmos/evm v0.6.0
	// migration: the burn path now really routes through keeper.CallEVM ->
	// CallEVMWithData -> the (mock) EVM keeper. A burn call must be made (off the
	// ORIGINAL SEALED status, not the current DISCONTINUED status); we assert it
	// happens by requiring exactly one CallEVMWithData and a clean delete.
	s.evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).Times(1)

	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().NoError(err, "ForceDelete should attempt burn based on original SEALED status and remove the object")
	_, found := s.storageKeeper.GetObjectInfoById(s.ctx, object.Id)
	s.Require().False(found, "object should be deleted after a successful burn")
}

// TestDeleteEmptySealedObjectShouldNotBurnNFT verifies that deleting an empty sealed object (PayloadSize=0) does not trigger burn
// Empty objects are sealed directly on creation and never mint NFTs
func (s *BurnTestSuite) TestDeleteEmptySealedObjectShouldNotBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-empty-sealed"
	objectName := "obj-empty-sealed"

	bucket := &types.BucketInfo{
		Owner:                      operator.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(4),
		BucketStatus:               types.BUCKET_STATUS_CREATED,
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          s.ctx.BlockTime().Unix(),
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0}},
	})

	// Create an empty sealed object (PayloadSize = 0)
	// Empty objects are sealed directly on creation and never mint NFTs
	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(40),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		SourceType:          0,
		PayloadSize:         0, // Empty object
		CreateAt:            s.ctx.BlockTime().Unix(),
		UpdatedAt:           s.ctx.BlockTime().Unix(),
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// Seed versioned params
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// Mock dependencies
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroupFamily{Id: 0, PrimarySpId: 0}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&types2.GlobalVirtualGroup{
		Id: 0, FamilyId: 0, PrimarySpId: 0, SecondarySpIds: []uint32{}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	}, true).AnyTimes()
	spAddress, _, _ := sample.RandSignBytes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(&sptypes.StorageProvider{Id: 0, OperatorAddress: spAddress.String()}, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(1),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ReserveTime: 10000, ValidatorTaxRate: sdkmath.LegacyNewDec(1)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	// An empty sealed object (PayloadSize=0) never minted an NFT, so the burn path
	// is skipped (production guards on PayloadSize>0) and keeper.CallEVM is never
	// reached. Delete therefore succeeds even though CallEVM is stubbed/disabled
	// during the cosmos/evm v0.6.0 migration.
	err := s.storageKeeper.DeleteObject(s.ctx, operator, bucketName, objectName, types.DeleteObjectOptions{SourceType: 0})
	s.Require().NoError(err, "empty sealed object (PayloadSize=0) should not trigger burn, so delete should succeed")
}

// TestForceDeleteEmptySealedObjectShouldNotBurnNFT verifies that ForceDeleteObject on empty sealed objects does not trigger burn
func (s *BurnTestSuite) TestForceDeleteEmptySealedObjectShouldNotBurnNFT() {
	operator := sample.RandAccAddress()
	bucketName := "bucket-force-empty"
	objectName := "obj-force-empty"

	bucket := &types.BucketInfo{
		Owner:                      operator.String(),
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(5),
		BucketStatus:               types.BUCKET_STATUS_CREATED,
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucket)

	priceTime := s.ctx.BlockTime().Unix() - 1000
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucket.Id, &types.InternalBucketInfo{
		PriceTime:          priceTime,
		TotalChargeSize:    0, // Empty object has no charge size
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 0, GlobalVirtualGroupId: 0, TotalChargeSize: 0}},
	})

	// Create an empty sealed object (PayloadSize = 0)
	object := &types.ObjectInfo{
		Id:                  sdkmath.NewUint(50),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Owner:               operator.String(),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		SourceType:          0,
		PayloadSize:         0, // Empty object
		CreateAt:            priceTime,
		UpdatedAt:           priceTime,
		LocalVirtualGroupId: 0,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	// Seed versioned params
	prev := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockTime(time.Unix(priceTime, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})
	s.ctx = s.ctx.WithBlockTime(prev)
	_ = s.storageKeeper.SetVersionedParamsWithTS(s.ctx, types.VersionedParams{MinChargeSize: 1})

	// Mark as discontinue with SEALED status
	store := s.ctx.KVStore(s.storeKey)
	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_SEALED))
	store.Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	// Setup mocks
	gvg := &types2.GlobalVirtualGroup{Id: 0}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              1,
		OperatorAddress: operator.String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(primarySp).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(100)}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()

	gvgFamily := &types2.GlobalVirtualGroupFamily{
		Id:                    0,
		VirtualPaymentAddress: operator.String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	s.permissionKeeper.EXPECT().ExistAccountPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupPolicyForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.permissionKeeper.EXPECT().ExistGroupMemberForGroup(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	// An empty sealed object (PayloadSize=0) never minted an NFT, so the burn path
	// is skipped (production guards on PayloadSize>0) and keeper.CallEVM is never
	// reached. ForceDelete therefore succeeds even though CallEVM is
	// stubbed/disabled during the cosmos/evm v0.6.0 migration.
	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().NoError(err, "ForceDelete empty sealed object (PayloadSize=0) should not trigger burn, so delete should succeed")
}

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

// TestForceDeleteObjectMissingFamilyFailsLoud asserts the orphan-GC fallback is scoped
// to a missing primary SP only: any other resolution failure (here, a missing GVG
// family) is surfaced as an error rather than silently garbage-collecting the object.
func (s *BurnTestSuite) TestForceDeleteObjectMissingFamilyFailsLoud() {
	owner := sample.RandAccAddress()
	bucketName := "corrupt-family-bucket"

	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{
		Owner:          owner.String(),
		BucketName:     bucketName,
		Id:             sdkmath.NewUint(70),
		BucketStatus:   types.BUCKET_STATUS_DISCONTINUED,
		PaymentAddress: sample.RandAccAddress().String(),
	})
	object := &types.ObjectInfo{
		Id:           sdkmath.NewUint(700),
		BucketName:   bucketName,
		ObjectName:   "corrupt-family-obj",
		Owner:        owner.String(),
		ObjectStatus: types.OBJECT_STATUS_CREATED,
		PayloadSize:  1,
		CreateAt:     s.ctx.BlockTime().Unix(),
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, object)

	statusBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(statusBytes, uint32(types.OBJECT_STATUS_CREATED))
	s.ctx.KVStore(s.storeKey).Set(types.GetDiscontinueObjectStatusKey(object.Id), statusBytes)

	// The GVG family itself is missing -> ErrGVGFamilyNotExist, not a missing SP.
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	err := s.storageKeeper.ForceDeleteObject(s.ctx, object.Id)
	s.Require().ErrorIs(err, types2.ErrGVGFamilyNotExist, "a missing GVG family is a genuine invariant break -> must surface, not GC")
	_, found := s.storageKeeper.GetObjectInfoById(s.ctx, object.Id)
	s.Require().True(found, "object must NOT be deleted when the failure isn't a missing SP")
}
