package keeper_test

import (
	"fmt"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
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
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type TestSuite struct {
	suite.Suite

	cdc           codec.Codec
	storageKeeper *keeper.Keeper
	storeKey      storetypes.StoreKey

	accountKeeper      *types.MockAccountKeeper
	spKeeper           *types.MockSpKeeper
	permissionKeeper   *types.MockPermissionKeeper
	paymentKeeper      *types.MockPaymentKeeper
	virtualGroupKeeper *types.MockVirtualGroupKeeper

	ctx         sdk.Context
	queryClient types.QueryClient
	msgServer   types.MsgServer
}

func (s *TestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	s.storeKey = key
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

	accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	// cosmos/evm v0.6.0 migration: the production mint/burn path (keeper.CallEVM
	// / CallEVMWithData in x/storage/keeper/evm.go) now really executes and routes
	// the ERC-721 mint/burn call through the EVM keeper. Stub both entrypoints to
	// return a non-failed response so bucket/object/group create/delete succeed.
	evmKeeper.EXPECT().CallEVMWithData(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()
	evmKeeper.EXPECT().CallEVM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()

	s.cdc = encCfg.Codec
	s.accountKeeper = accountKeeper
	s.spKeeper = spKeeper
	s.permissionKeeper = permissionKeeper
	s.paymentKeeper = paymentKeeper
	s.virtualGroupKeeper = virtualGroupKeeper

	err := s.storageKeeper.SetParams(s.ctx, types.DefaultParams())
	s.Require().NoError(err)

	queryHelper := baseapp.NewQueryServerTestHelper(testCtx.Ctx, encCfg.InterfaceRegistry)
	types.RegisterQueryServer(queryHelper, s.storageKeeper)

	s.queryClient = types.NewQueryClient(queryHelper)
	s.msgServer = keeper.NewMsgServerImpl(*s.storageKeeper)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// TestRunPaymentCheck is a regression test for the per-bucket object-iterator
// leak fixed in RunPaymentCheck. RunPaymentCheck opens one object iterator per
// bucket; before the fix that iterator was closed only when the whole function
// returned, leaking one open iterator per bucket. The fix moved the iteration
// into a closure with its own `defer it.Close()`.
//
// This seeds a real bucket with a real CREATED object so the inner object
// iterator actually iterates and the lock-fee branch the fix wraps
// (GetObjectInfoById + GetObjectLockFee) is exercised end-to-end. A matching
// stream record is supplied so the lock-balance comparison succeeds, and we
// assert RunPaymentCheck returns nil (no error, no panic).
func (s *TestSuite) TestRunPaymentCheck() {
	paymentAddr := "0x1111111111111111111111111111111111111111"
	bucketName := "payment-check-bucket"
	objectName := "payment-check-object"

	// A CREATED object's lock fee is priced at its latest-updated time, which
	// must be strictly after the block time so the versioned params seeded at
	// block time in SetupTest are found by the reverse-iterator lookup.
	priceTime := s.ctx.BlockTime().Unix() + 1

	bucketInfo := &types.BucketInfo{
		Owner:                      paymentAddr,
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             paymentAddr,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	// Empty internal bucket info (no read/store charge) so GetBucketReadStoreBill
	// short-circuits and the test focuses on the object-iterator loop.
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id:           sdkmath.NewUint(1),
		Owner:        paymentAddr,
		BucketName:   bucketName,
		ObjectName:   objectName,
		PayloadSize:  1024,
		ObjectStatus: types.OBJECT_STATUS_CREATED,
		CreateAt:     priceTime,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	// Price/param mocks for the lock-fee calculation of the CREATED object.
	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyZeroDec(),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(price, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{
		ReserveTime:      100,
		ValidatorTaxRate: sdkmath.LegacyZeroDec(),
	}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(payVer, nil).AnyTimes()

	// Expected lock balance from the production calculation, used to build the
	// matching stream record so the lock-balance comparison passes.
	expectedLockBalance, _, err := s.storageKeeper.GetObjectLockFee(s.ctx, objectInfo.GetLatestUpdatedTime(), objectInfo.PayloadSize)
	s.Require().NoError(err)
	s.Require().True(expectedLockBalance.IsPositive(), "test setup should produce a positive lock balance")

	streamRecord := paymenttypes.StreamRecord{
		Account:           paymentAddr,
		NetflowRate:       sdkmath.ZeroInt(),
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       expectedLockBalance,
		FrozenNetflowRate: sdkmath.ZeroInt(),
		Status:            paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE,
	}
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{streamRecord}).AnyTimes()

	err = s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().NoError(err)
}

// TestRunPaymentCheck_ShadowObjectNotFound covers the first abort branch of the
// per-bucket object loop the fix restructured (payment_check.go: the "shadow
// object not found" case that does `return expectedLockBalance, true` ->
// `if abort { continue Exit }`). An updating object with no shadow object entry
// triggers it; GetObjectLockFee is never reached, so no price mocks are needed.
func (s *TestSuite) TestRunPaymentCheck_ShadowObjectNotFound() {
	paymentAddr := "0x2222222222222222222222222222222222222222"
	bucketName := "abort-shadow-bucket"
	objectName := "abort-shadow-object"

	bucketInfo := &types.BucketInfo{
		Owner:                      paymentAddr,
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             paymentAddr,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	// Empty internal bucket info so GetBucketReadStoreBill short-circuits.
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	// Updating object with no shadow object stored -> GetShadowObjectInfo returns
	// not found, taking the "shadow object not found" abort path.
	objectInfo := &types.ObjectInfo{
		Id:           sdkmath.NewUint(1),
		Owner:        paymentAddr,
		BucketName:   bucketName,
		ObjectName:   objectName,
		PayloadSize:  1024,
		ObjectStatus: types.OBJECT_STATUS_SEALED,
		IsUpdating:   true,
		CreateAt:     s.ctx.BlockTime().Unix() + 1,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	// Empty stream records so the post-loop comparison does not run (the loop's
	// abort error is returned directly via the early `if result != nil` return).
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "shadow object not found")
}

// TestRunPaymentCheck_GetObjectLockFeeError covers the second abort branch of the
// per-bucket object loop the fix restructured (payment_check.go: the "get object
// lock fee failed" case that does `return expectedLockBalance, true` ->
// `if abort { continue Exit }`). A CREATED object reaches GetObjectLockFee, whose
// first call (spKeeper.GetGlobalSpStorePriceByTime) is mocked to fail.
func (s *TestSuite) TestRunPaymentCheck_GetObjectLockFeeError() {
	paymentAddr := "0x3333333333333333333333333333333333333333"
	bucketName := "abort-lockfee-bucket"
	objectName := "abort-lockfee-object"

	bucketInfo := &types.BucketInfo{
		Owner:                      paymentAddr,
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             paymentAddr,
		GlobalVirtualGroupFamilyId: 1,
		ChargedReadQuota:           0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	// Empty internal bucket info so GetBucketReadStoreBill short-circuits.
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id:           sdkmath.NewUint(1),
		Owner:        paymentAddr,
		BucketName:   bucketName,
		ObjectName:   objectName,
		PayloadSize:  1024,
		ObjectStatus: types.OBJECT_STATUS_CREATED,
		CreateAt:     s.ctx.BlockTime().Unix() + 1,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	// GetObjectLockFee calls GetGlobalSpStorePriceByTime first; make it fail so
	// GetObjectLockFee errors and the loop takes the lock-fee abort path.
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	// Empty stream records so the post-loop comparison does not run (the loop's
	// abort error is returned directly via the early `if result != nil` return).
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "get object lock fee failed")
}

func (s *TestSuite) TestGetObjectLockFee() {
	primarySp := &sptypes.StorageProvider{Status: sptypes.STATUS_IN_SERVICE, Id: 100, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(primarySp.Id)).
		Return(primarySp, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(params.VersionedParams, nil).AnyTimes()

	// verify lock fee calculation
	timeNow := time.Now().Unix() + 1
	payloadSize := int64(10 * 1024 * 1024)
	amount, _, err := s.storageKeeper.GetObjectLockFee(s.ctx, timeNow, uint64(payloadSize))
	s.Require().NoError(err)
	secondarySPNum := int64(s.storageKeeper.GetExpectSecondarySPNumForECObject(s.ctx, timeNow))
	spRate := price.PrimaryStorePrice.Add(price.SecondaryStorePrice.MulInt64(secondarySPNum)).MulInt64(payloadSize)
	validatorTaxRate := params.VersionedParams.ValidatorTaxRate.MulInt(spRate.TruncateInt())
	expectedAmount := spRate.Add(validatorTaxRate).MulInt64(int64(params.VersionedParams.ReserveTime)).TruncateInt()
	s.Require().True(amount.Equal(expectedAmount))
}

func (s *TestSuite) TestGetBucketReadBill() {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return(gvgFamily, true).AnyTimes()

	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              100,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(primarySp.Id)).
		Return(primarySp, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(params.VersionedParams, nil).AnyTimes()

	// empty bucket, zero read quota
	bucketInfo := &types.BucketInfo{
		Owner:                      "",
		BucketName:                 "bucket_name",
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: gvgFamily.Id,
		ChargedReadQuota:           0,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	flows, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	s.Require().True(len(flows.Flows) == 0)

	// empty bucket
	bucketInfo = &types.BucketInfo{
		Owner:                      "",
		BucketName:                 "bucket_name",
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: gvgFamily.Id,
		ChargedReadQuota:           100,
	}
	internalBucketInfo = &types.InternalBucketInfo{}
	flows, err = s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	readRate := price.ReadPrice.MulInt64(int64(bucketInfo.ChargedReadQuota)).TruncateInt()
	s.Require().Equal(flows.Flows[0].ToAddress, gvgFamily.VirtualPaymentAddress)
	s.Require().Equal(flows.Flows[0].Rate, readRate)
	taxPoolRate := params.VersionedParams.ValidatorTaxRate.MulInt(readRate).TruncateInt()
	s.Require().Equal(flows.Flows[1].ToAddress, paymenttypes.ValidatorTaxPoolAddress.String())
	s.Require().Equal(flows.Flows[1].Rate, taxPoolRate)
}

func (s *TestSuite) TestGetBucketReadStoreBill() {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return(gvgFamily, true).AnyTimes()

	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              100,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(primarySp.Id)).
		Return(primarySp, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(params.VersionedParams, nil).AnyTimes()

	// none empty bucket
	bucketInfo := &types.BucketInfo{
		Owner:                      "",
		BucketName:                 "bucket_name",
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: gvgFamily.Id,
		ChargedReadQuota:           100,
	}

	lvg1 := &types.LocalVirtualGroup{
		Id:                   1,
		TotalChargeSize:      100,
		GlobalVirtualGroupId: 1,
	}
	lvg2 := &types.LocalVirtualGroup{
		Id:                   2,
		TotalChargeSize:      200,
		GlobalVirtualGroupId: 2,
	}
	internalBucketInfo := &types.InternalBucketInfo{
		TotalChargeSize: 300,
		LocalVirtualGroups: []*types.LocalVirtualGroup{
			lvg1, lvg2,
		},
	}

	gvg1 := &virtualgroupmoduletypes.GlobalVirtualGroup{
		Id:                    1,
		PrimarySpId:           primarySp.Id,
		SecondarySpIds:        []uint32{101, 102, 103, 104, 105, 106},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	gvg2 := &virtualgroupmoduletypes.GlobalVirtualGroup{
		Id:                    2,
		PrimarySpId:           primarySp.Id,
		SecondarySpIds:        []uint32{201, 202, 203, 204, 205, 206},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gvg1.Id).
		Return(gvg1, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gvg2.Id).
		Return(gvg2, true).AnyTimes()

	flows, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)

	// read rate to gvg family
	s.Require().Equal(flows.Flows[0].ToAddress, gvgFamily.VirtualPaymentAddress)
	readRate := price.ReadPrice.MulInt64(int64(bucketInfo.ChargedReadQuota)).TruncateInt()
	s.Require().Equal(flows.Flows[0].Rate, readRate)

	// read rate to validator tax pool
	s.Require().Equal(flows.Flows[1].ToAddress, paymenttypes.ValidatorTaxPoolAddress.String())
	taxPoolRate := params.VersionedParams.ValidatorTaxRate.MulInt(readRate).TruncateInt()
	s.Require().Equal(flows.Flows[1].Rate, taxPoolRate)

	// first gvg
	// store rate to gvg family
	s.Require().Equal(flows.Flows[2].ToAddress, gvgFamily.VirtualPaymentAddress)
	primaryStoreRate := price.PrimaryStorePrice.MulInt64(int64(lvg1.TotalChargeSize)).TruncateInt()
	s.Require().Equal(flows.Flows[2].Rate, primaryStoreRate)

	// store rate to gvg
	gvg1StoreSize := lvg1.TotalChargeSize * uint64(len(gvg1.SecondarySpIds))
	gvg1StoreRate := price.SecondaryStorePrice.MulInt64(int64(gvg1StoreSize)).TruncateInt()
	s.Require().Equal(flows.Flows[3].ToAddress, gvg1.VirtualPaymentAddress)
	s.Require().Equal(flows.Flows[3].Rate, gvg1StoreRate)

	// store rate to validator tax pool
	s.Require().Equal(flows.Flows[4].ToAddress, paymenttypes.ValidatorTaxPoolAddress.String())
	taxPoolRate = params.VersionedParams.ValidatorTaxRate.MulInt(primaryStoreRate.Add(gvg1StoreRate)).TruncateInt()
	s.Require().Equal(flows.Flows[4].Rate, taxPoolRate)

	// secondary gvg
	// store rate to gvg family
	s.Require().Equal(flows.Flows[5].ToAddress, gvgFamily.VirtualPaymentAddress)
	primaryStoreRate = price.PrimaryStorePrice.MulInt64(int64(lvg2.TotalChargeSize)).TruncateInt()
	s.Require().Equal(flows.Flows[5].Rate, primaryStoreRate)

	// store rate to gvg
	gvg2StoreSize := lvg2.TotalChargeSize * uint64(len(gvg2.SecondarySpIds))
	gvg2StoreRate := price.SecondaryStorePrice.MulInt64(int64(gvg2StoreSize)).TruncateInt()
	s.Require().Equal(flows.Flows[6].ToAddress, gvg2.VirtualPaymentAddress)
	s.Require().Equal(flows.Flows[6].Rate, gvg2StoreRate)

	// store rate to validator tax pool
	s.Require().Equal(flows.Flows[7].ToAddress, paymenttypes.ValidatorTaxPoolAddress.String())
	taxPoolRate = params.VersionedParams.ValidatorTaxRate.MulInt(primaryStoreRate.Add(gvg2StoreRate)).TruncateInt()
	s.Require().Equal(flows.Flows[7].Rate, taxPoolRate)
}
