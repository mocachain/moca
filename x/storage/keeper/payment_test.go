package keeper_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/app"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"github.com/stretchr/testify/require"
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

// seedLimitedBucket stores a bucket that pays its own read fee (owner ==
// payment address) and drives it into the rate-limited state via
// SetBucketFlowRateLimit's own over-limit path (a limit of 1 against a much
// larger bill), so k.IsBucketRateLimited(...) is true afterward.
func seedLimitedBucket(s *TestSuite, paymentAddr, bucketName string) *types.BucketInfo {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	bucketInfo := &types.BucketInfo{
		Owner: paymentAddr, PaymentAddress: paymentAddr, BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id, ChargedReadQuota: 100,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	ownerAddr := sdk.MustAccAddressFromHex(paymentAddr)
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, ownerAddr, bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)
	s.Require().True(s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName), "test setup should rate-limit the bucket")

	return bucketInfo
}

// newStoreFeeFixture seeds a bucket owning a single LVG bound to a GVG, and
// mocks every dependency ChargeViaObjectChange (and, through it,
// ChargeObjectStoreFee / UnlockAndChargeObjectStoreFee) needs for a clean
// happy-path run: GVG family + GVG lookup, a flat store price, zero
// validator tax, payment-account-owner (so the flow rate limit check is
// bypassed) and successful stream-record updates. It returns the bucket, its
// internal info, and the LVG id an ObjectInfo should target.
func newStoreFeeFixture(s *TestSuite, ownerAddr string) (bucketInfo *types.BucketInfo, internalBucketInfo *types.InternalBucketInfo, lvgID uint32) {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String(), SecondarySpIds: []uint32{101}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).
		DoAndReturn(func(flows []paymenttypes.OutFlow) []paymenttypes.OutFlow { return flows }).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.ZeroInt()}, nil).AnyTimes()

	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: gvg.Id, TotalChargeSize: 0}
	bucketInfo = &types.BucketInfo{
		Owner: ownerAddr, PaymentAddress: ownerAddr, BucketName: "storefee-fixture-bucket",
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id,
	}
	internalBucketInfo = &types.InternalBucketInfo{LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)

	return bucketInfo, internalBucketInfo, lvg.Id
}

func (s *TestSuite) TestChargeBucketReadFee_ZeroQuota() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 0}
	internalBucketInfo := &types.InternalBucketInfo{PriceTime: -1}

	err := s.storageKeeper.ChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	s.Require().Equal(int64(-1), internalBucketInfo.PriceTime, "zero-quota short circuit must return before touching internalBucketInfo")
}

func (s *TestSuite) TestChargeBucketReadFee_GetBillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "chargereadfee-billerror-bucket",
		ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 999,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.ChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "charge bucket read fee failed, get bucket bill failed")
}

func (s *TestSuite) TestChargeBucketReadFee_OverLimit() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
		BucketName: "chargereadfee-overlimit-bucket", ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	err := s.storageKeeper.ChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "the flow rate limit is not set for the bucket")
}

func (s *TestSuite) TestChargeBucketReadFee_ApplyError() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
		BucketName: "chargereadfee-applyerror-bucket", ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))

	err := s.storageKeeper.ChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestChargeBucketReadFee_Success() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
		BucketName: "chargereadfee-success-bucket", ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil)

	err := s.storageKeeper.ChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	s.Require().Equal(s.ctx.BlockTime().Unix(), internalBucketInfo.PriceTime)
}

func (s *TestSuite) TestGetBucketReadBill_GVGFamilyNotFound() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 999}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	_, err := s.storageKeeper.GetBucketReadBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().ErrorContains(err, "get GVG family failed")
}

func (s *TestSuite) TestGetBucketReadBill_PriceError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.GetBucketReadBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().ErrorContains(err, "get storage price failed")
}

func (s *TestSuite) TestGetBucketReadBill_VersionedParamsError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.GetBucketReadBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().ErrorContains(err, "failed to get validator tax rate")
}

func (s *TestSuite) TestGetBucketReadBill_ZeroRatesNoAppend() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyNewDecWithPrec(5, 2)}, nil).AnyTimes()

	flows, err := s.storageKeeper.GetBucketReadBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().NoError(err)
	s.Require().Empty(flows.Flows, "a zero read price must skip both the primary and validator-tax appends")
}

func (s *TestSuite) TestUnChargeBucketReadFee_TotalChargeSizeError() {
	bucketInfo := &types.BucketInfo{BucketName: "unchargereadfee-sizeerror-bucket"}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 1}

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "unexpected total store charge size")
}

func (s *TestSuite) TestUnChargeBucketReadFee_RateLimited() {
	paymentAddr := sample.RandAccAddress().String()
	bucketInfo := seedLimitedBucket(s, paymentAddr, "unchargereadfee-limited-bucket")
	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
}

func (s *TestSuite) TestUnChargeBucketReadFee_BillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargereadfee-billerror-bucket",
		ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 999,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "uncharge bucket read fee failed, get bucket bill failed")
}

func (s *TestSuite) TestUnChargeBucketReadFee_EmptyFlows() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargereadfee-empty-bucket", ChargedReadQuota: 0}
	internalBucketInfo := &types.InternalBucketInfo{}

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
}

func (s *TestSuite) TestUnChargeBucketReadFee_Success() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargereadfee-success-bucket",
		ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()

	var captured []paymenttypes.UserFlows
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, flows []paymenttypes.UserFlows) error {
			captured = flows
			return nil
		})

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	s.Require().Len(captured, 1)
	s.Require().True(captured[0].Flows[0].Rate.IsNegative(), "uncharging must negate the read-fee flow")
}

func (s *TestSuite) TestUnChargeBucketReadFee_ApplyError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargereadfee-applyerror-bucket",
		ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))

	err := s.storageKeeper.UnChargeBucketReadFee(s.ctx, bucketInfo, internalBucketInfo)
	// UnChargeBucketReadFee (unlike UnChargeBucketReadStoreFee) logs and returns
	// the underlying ApplyUserFlowsList error unwrapped.
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestLockShadowObjectStoreFee_Success() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(), BucketName: "lockshadow-success-bucket"}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(1)}, nil)

	shadowObjectInfo := &types.ShadowObjectInfo{PayloadSize: 2048, UpdatedAt: s.ctx.BlockTime().Unix() + 1}
	err := s.storageKeeper.LockShadowObjectStoreFee(s.ctx, bucketInfo, shadowObjectInfo, "shadow-object")
	s.Require().NoError(err)
}

func (s *TestSuite) TestLockShadowObjectStoreFee_InsufficientBalance() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(), BucketName: "lockshadow-insufficient-bucket"}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(-1)}, nil)

	shadowObjectInfo := &types.ShadowObjectInfo{PayloadSize: 2048, UpdatedAt: s.ctx.BlockTime().Unix() + 1}
	err := s.storageKeeper.LockShadowObjectStoreFee(s.ctx, bucketInfo, shadowObjectInfo, "shadow-object")
	s.Require().ErrorContains(err, "static balance is not enough")
}

func (s *TestSuite) TestLockObjectStoreFee_RateLimitChecked_UnderLimit() {
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner: bucketOwner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		PaymentAddress: paymentAccount.String(), ChargedReadQuota: 100,
	}
	prepareReadStoreBill(s, bucketInfo)

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	// prepareReadStoreBill's fixed prices (1000/500) make even a modest payload's
	// lock fee huge once GetObjectChargeSize's MinChargeSize floor kicks in, so
	// the limit here just needs to comfortably exceed any such bill, not match
	// it precisely.
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, bucketOwner, bucketOwner, paymentAccount, bucketName, sdkmath.NewInt(1_000_000_000_000_000_000))
	s.Require().NoError(err)

	objectInfo := &types.ObjectInfo{ObjectName: "ratelimit-checked-object", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	streamRecord := paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(1)}
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&streamRecord, nil)

	err = s.storageKeeper.LockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().NoError(err)
}

func (s *TestSuite) TestLockObjectStoreFee_RateLimitChecked_ExceedsLimit() {
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner: bucketOwner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
		PaymentAddress: paymentAccount.String(), ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, bucketOwner, bucketOwner, paymentAccount, bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1000), SecondaryStorePrice: sdkmath.LegacyNewDec(500)}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.DefaultParams().VersionedParams, nil).AnyTimes()

	objectInfo := &types.ObjectInfo{ObjectName: "ratelimit-exceeds-object", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	err = s.storageKeeper.LockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorContains(err, "greater than the flow rate limit")
}

func (s *TestSuite) TestUnlockShadowObjectStoreFee_LockFeeError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String()}
	shadowObjectInfo := &types.ShadowObjectInfo{PayloadSize: 1024, UpdatedAt: s.ctx.BlockTime().Unix() + 1}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	err := s.storageKeeper.UnlockShadowObjectStoreFee(s.ctx, bucketInfo, shadowObjectInfo)
	s.Require().ErrorContains(err, "get shadow object store fee rate failed")
}

func (s *TestSuite) TestUnlockShadowObjectStoreFee_UpdateError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String()}
	shadowObjectInfo := &types.ShadowObjectInfo{PayloadSize: 1024, UpdatedAt: s.ctx.BlockTime().Unix() + 1}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom"))

	err := s.storageKeeper.UnlockShadowObjectStoreFee(s.ctx, bucketInfo, shadowObjectInfo)
	s.Require().ErrorContains(err, "update stream record failed")
}

func (s *TestSuite) TestUnlockShadowObjectStoreFee_Success() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String()}
	shadowObjectInfo := &types.ShadowObjectInfo{PayloadSize: 1024, UpdatedAt: s.ctx.BlockTime().Unix() + 1}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	// ReserveTime must be positive: GetObjectLockFee's amount = rate * ReserveTime,
	// and a zero amount would make the negation-is-negative assertion below
	// meaningless (zero is neither negative nor positive).
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ReserveTime: 100, ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()

	var captured *paymenttypes.StreamRecordChange
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, change *paymenttypes.StreamRecordChange) (*paymenttypes.StreamRecord, error) {
			captured = change
			return &paymenttypes.StreamRecord{}, nil
		})

	err := s.storageKeeper.UnlockShadowObjectStoreFee(s.ctx, bucketInfo, shadowObjectInfo)
	s.Require().NoError(err)
	s.Require().True(captured.LockBalanceChange.IsNegative(), "unlocking must negate the locked balance")
}

func (s *TestSuite) TestUnlockAndChargeObjectStoreFee_UnlockError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "unlockcharge-error-bucket", Id: sdkmath.NewUint(1)}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	err := s.storageKeeper.UnlockAndChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "unlock store fee failed")
}

func (s *TestSuite) TestUnlockAndChargeObjectStoreFee_Success() {
	ownerAddr := sample.RandAccAddress().String()
	bucketInfo, internalBucketInfo, lvgID := newStoreFeeFixture(s, ownerAddr)

	objectInfo := &types.ObjectInfo{
		ObjectName: "obj", PayloadSize: 1024, LocalVirtualGroupId: lvgID,
		CreateAt: s.ctx.BlockTime().Unix() + 1,
	}

	// The default MinChargeSize param floors small payloads, so derive the
	// expected charge size the same way the keeper does instead of asserting
	// against the raw (pre-floor) payload size.
	wantChargeSize, err := s.storageKeeper.GetObjectChargeSize(s.ctx, objectInfo.PayloadSize, objectInfo.GetLatestUpdatedTime())
	s.Require().NoError(err)

	err = s.storageKeeper.UnlockAndChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().NoError(err)
	s.Require().Equal(wantChargeSize, internalBucketInfo.LocalVirtualGroups[0].TotalChargeSize)
}

func (s *TestSuite) TestIsPriceChanged_PrePriceError() {
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom"))

	_, _, _, _, _, err := s.storageKeeper.IsPriceChanged(s.ctx, 1, 0)
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestIsPriceChanged_CurrentPriceError() {
	gomock.InOrder(
		s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, nil),
		s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")),
	)

	_, _, _, _, _, err := s.storageKeeper.IsPriceChanged(s.ctx, 1, 0)
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestIsPriceChanged_PreParamsError() {
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom"))

	_, _, _, _, _, err := s.storageKeeper.IsPriceChanged(s.ctx, 1, 0)
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestIsPriceChanged_CurrentParamsError() {
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, nil).AnyTimes()
	gomock.InOrder(
		s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{}, nil),
		s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")),
	)

	_, _, _, _, _, err := s.storageKeeper.IsPriceChanged(s.ctx, 1, 0)
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestChargeObjectStoreFee_ChargeSizeError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "chargefee-sizeerror-bucket"}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: 0}

	err := s.storageKeeper.ChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "get charge size failed")
}

func (s *TestSuite) TestChargeObjectStoreFee_PriceChangedError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "chargefee-pricechangederror-bucket"}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	err := s.storageKeeper.ChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "check whether price changed failed")
}

func (s *TestSuite) TestChargeObjectStoreFee_NotChanged_ChargeViaObjectChangeError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "chargefee-cvoerror-bucket", GlobalVirtualGroupFamilyId: 999,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.ChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "apply object store bill failed")
}

func (s *TestSuite) TestChargeObjectStoreFee_NotChanged_Success() {
	ownerAddr := sample.RandAccAddress().String()
	bucketInfo, internalBucketInfo, lvgID := newStoreFeeFixture(s, ownerAddr)

	objectInfo := &types.ObjectInfo{
		ObjectName: "obj", PayloadSize: 1024, LocalVirtualGroupId: lvgID, CreateAt: s.ctx.BlockTime().Unix() + 1,
	}

	// The default MinChargeSize param floors small payloads, so derive the
	// expected charge size the same way the keeper does instead of asserting
	// against the raw (pre-floor) payload size.
	wantChargeSize, err := s.storageKeeper.GetObjectChargeSize(s.ctx, objectInfo.PayloadSize, objectInfo.GetLatestUpdatedTime())
	s.Require().NoError(err)

	err = s.storageKeeper.ChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().NoError(err)
	s.Require().Equal(wantChargeSize, internalBucketInfo.LocalVirtualGroups[0].TotalChargeSize,
		"the matching LVG's charge size must accumulate the object's charge size")
}

func (s *TestSuite) TestChargeObjectStoreFeeForEarlyDeletion_PerFlowError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "earlydel-flowerror-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj"}
	flows := []paymenttypes.OutFlow{{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(10)}}
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom"))

	err := s.storageKeeper.ChargeObjectStoreFeeForEarlyDeletion(s.ctx, flows, bucketInfo, objectInfo, 100)
	s.Require().ErrorContains(err, "pay address")
}

func (s *TestSuite) TestChargeObjectStoreFeeForEarlyDeletion_FinalError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "earlydel-finalerror-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj"}
	flows := []paymenttypes.OutFlow{{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(10)}}
	gomock.InOrder(
		s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil),
		s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom")),
	)

	err := s.storageKeeper.ChargeObjectStoreFeeForEarlyDeletion(s.ctx, flows, bucketInfo, objectInfo, 100)
	s.Require().ErrorContains(err, "subtracting from payment account failed")
}

func (s *TestSuite) TestChargeObjectStoreFeeForEarlyDeletion_Success() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "earlydel-success-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj"}
	flows := []paymenttypes.OutFlow{
		{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(10)},
		{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(-5)},
	}

	var final *paymenttypes.StreamRecordChange
	gomock.InOrder(
		s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil),
		s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).Return(&paymenttypes.StreamRecord{}, nil),
		s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ sdk.Context, change *paymenttypes.StreamRecordChange) (*paymenttypes.StreamRecord, error) {
				final = change
				return &paymenttypes.StreamRecord{}, nil
			}),
	)

	err := s.storageKeeper.ChargeObjectStoreFeeForEarlyDeletion(s.ctx, flows, bucketInfo, objectInfo, 100)
	s.Require().NoError(err)
	// per-flow static balance changes: |10|*100=1000, |-5|*100=500 -> total 1500, negated on the payer.
	s.Require().Equal(sdkmath.NewInt(-1500), final.StaticBalanceChange)
}

func (s *TestSuite) TestChargeViaBucketChange_GetPrevBillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvbc-prevbillerror-bucket",
		ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 999,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(_ *types.BucketInfo, _ *types.InternalBucketInfo) error { return nil })
	s.Require().ErrorContains(err, "charge via bucket change failed, get bucket bill failed")
}

func (s *TestSuite) TestChargeViaBucketChange_ChangeFuncError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvbc-changefuncerror-bucket", ChargedReadQuota: 0}
	internalBucketInfo := &types.InternalBucketInfo{}

	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo,
		func(_ *types.BucketInfo, _ *types.InternalBucketInfo) error { return fmt.Errorf("boom") })
	s.Require().ErrorContains(err, "change bucket internal info failed")
}

func (s *TestSuite) TestChargeViaBucketChange_NewBillError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvbc-newbillerror-bucket", ChargedReadQuota: 0}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.ChargedReadQuota = 100
		bi.GlobalVirtualGroupFamilyId = 999
		return nil
	})
	s.Require().ErrorContains(err, "get new bucket bill failed")
}

func (s *TestSuite) TestChargeViaBucketChange_SameAddressButLimited() {
	paymentAddr := sample.RandAccAddress().String()
	bucketInfo := seedLimitedBucket(s, paymentAddr, "cvbc-samelimited-bucket")
	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)

	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo,
		func(_ *types.BucketInfo, _ *types.InternalBucketInfo) error { return nil })
	s.Require().ErrorContains(err, "payment account is not changed but the bucket is limited")
}

func (s *TestSuite) TestChargeViaBucketChange_PreviouslyLimited_Resumes() {
	paymentAddr := sample.RandAccAddress().String()
	bucketInfo := seedLimitedBucket(s, paymentAddr, "cvbc-resumes-bucket")
	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	newPaymentAddr := sample.RandAccAddress().String()

	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.PaymentAddress = newPaymentAddr
		return nil
	})
	s.Require().NoError(err)
	s.Require().False(s.storageKeeper.IsBucketRateLimited(s.ctx, bucketInfo.BucketName), "resuming charge must clear the limited status")
}

func (s *TestSuite) TestChargeViaBucketChange_OverLimit() {
	ownerAddr := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: ownerAddr.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 50,
	}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	// current bill (rate 50) is under the stored limit of 60, so seeding does not trip the limiter.
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, ownerAddr, bucketName, sdkmath.NewInt(60))
	s.Require().NoError(err)
	s.Require().False(s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName))

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	err = s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.ChargedReadQuota = 200 // pushes the new bill (rate 200) over the stored limit of 60
		return nil
	})
	s.Require().ErrorContains(err, "greater than the flow rate limit")
}

func (s *TestSuite) TestChargeViaBucketChange_Success() {
	ownerAddr := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: ownerAddr.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 50,
	}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, ownerAddr, bucketName, sdkmath.NewInt(1000))
	s.Require().NoError(err)

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	err = s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.ChargedReadQuota = 60 // still comfortably under the limit of 1000
		return nil
	})
	s.Require().NoError(err)
	s.Require().False(s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName))
}

func (s *TestSuite) TestApplyBillChanges_BothNil() {
	var captured []paymenttypes.UserFlows
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, flows []paymenttypes.UserFlows) error {
			captured = flows
			return nil
		})

	err := s.storageKeeper.ApplyBillChanges(s.ctx, nil, nil)
	s.Require().NoError(err)
	s.Require().Empty(captured)
}

func (s *TestSuite) TestApplyBillChanges_PrevOnly() {
	prev := &paymenttypes.UserFlows{
		From:  sdk.MustAccAddressFromHex(sample.RandAccAddress().String()),
		Flows: []paymenttypes.OutFlow{{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(50)}},
	}
	var captured []paymenttypes.UserFlows
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, flows []paymenttypes.UserFlows) error {
			captured = flows
			return nil
		})

	err := s.storageKeeper.ApplyBillChanges(s.ctx, prev, nil)
	s.Require().NoError(err)
	s.Require().Len(captured, 1)
	s.Require().Equal(sdkmath.NewInt(-50), captured[0].Flows[0].Rate, "the previous bill's flows must be negated")
}

func (s *TestSuite) TestApplyBillChanges_CurrentOnly() {
	current := &paymenttypes.UserFlows{
		From:  sdk.MustAccAddressFromHex(sample.RandAccAddress().String()),
		Flows: []paymenttypes.OutFlow{{ToAddress: sample.RandAccAddress().String(), Rate: sdkmath.NewInt(80)}},
	}
	var captured []paymenttypes.UserFlows
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, flows []paymenttypes.UserFlows) error {
			captured = flows
			return nil
		})

	err := s.storageKeeper.ApplyBillChanges(s.ctx, nil, current)
	s.Require().NoError(err)
	s.Require().Len(captured, 1)
	s.Require().Equal(sdkmath.NewInt(80), captured[0].Flows[0].Rate, "the current bill's flows must be left untouched")
}

func (s *TestSuite) TestApplyBillChanges_Error() {
	current := &paymenttypes.UserFlows{From: sdk.MustAccAddressFromHex(sample.RandAccAddress().String())}
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))

	err := s.storageKeeper.ApplyBillChanges(s.ctx, nil, current)
	s.Require().ErrorContains(err, "apply user flows list failed")
}

func (s *TestSuite) TestUnChargeBucketReadStoreFee_BillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargestore-billerror-bucket", GlobalVirtualGroupFamilyId: 999,
	}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 1}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.UnChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "get bucket bill failed")
}

func (s *TestSuite) TestUnChargeBucketReadStoreFee_ApplyError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargestore-applyerror-bucket", GlobalVirtualGroupFamilyId: 1,
	}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))

	err := s.storageKeeper.UnChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "apply user flows list failed")
}

func (s *TestSuite) TestChargeBucketReadStoreFee_BillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), BucketName: "chargestore-billerror-bucket", GlobalVirtualGroupFamilyId: 999, ChargedReadQuota: 100,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.ChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "get bucket bill failed")
}

func (s *TestSuite) TestChargeBucketReadStoreFee_RateLimitedError() {
	paymentAddr := sample.RandAccAddress().String()
	bucketInfo := seedLimitedBucket(s, paymentAddr, "chargestore-limited-bucket")
	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)

	err := s.storageKeeper.ChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "bucket is rate limited")
}

func (s *TestSuite) TestChargeBucketReadStoreFee_OverLimitError() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
		BucketName: "chargestore-overlimit-bucket", GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 100,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	err := s.storageKeeper.ChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "the flow rate limit is not set for the bucket")
}

func (s *TestSuite) TestChargeBucketReadStoreFee_ApplyError() {
	bucketInfo := &types.BucketInfo{
		Owner: sample.RandAccAddress().String(), PaymentAddress: sample.RandAccAddress().String(),
		BucketName: "chargestore-applyerror-bucket", GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 100,
	}
	internalBucketInfo := &types.InternalBucketInfo{}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))

	err := s.storageKeeper.ChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "apply user flows list failed")
}

func (s *TestSuite) TestGetBucketReadBill_ValidatorTaxAppended() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(params.VersionedParams, nil).AnyTimes()

	flows, err := s.storageKeeper.GetBucketReadBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().NoError(err)
	readRate := price.ReadPrice.MulInt64(int64(bucketInfo.ChargedReadQuota)).TruncateInt()
	s.Require().Equal(gvgFamily.VirtualPaymentAddress, flows.Flows[0].ToAddress)
	s.Require().Equal(readRate, flows.Flows[0].Rate)
	taxRate := params.VersionedParams.ValidatorTaxRate.MulInt(readRate).TruncateInt()
	s.Require().True(taxRate.IsPositive(), "test setup must exercise the positive validator-tax branch")
	s.Require().Equal(paymenttypes.ValidatorTaxPoolAddress.String(), flows.Flows[1].ToAddress)
	s.Require().Equal(taxRate, flows.Flows[1].Rate)
}

func (s *TestSuite) TestLockObjectStoreFee_GetLockFeeError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(), BucketName: "lockfee-getlockfeeerror-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	err := s.storageKeeper.LockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorContains(err, "get object store fee rate failed")
}

// TestLockObjectStoreFee_CheckTxEmitsPreviewEvent covers the ctx.IsCheckTx() branch: during
// CheckTx, lockObjectStoreFee must emit an EventFeePreview before touching the store.
func (s *TestSuite) TestLockObjectStoreFee_CheckTxEmitsPreviewEvent() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(), BucketName: "lockfee-checktx-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	// bucketInfo.Owner == PaymentAddress and no explicit per-bucket rate limit is stored, so
	// shouldCheckRateLimit's IsPaymentAccountOwner fallback must be true to keep this test
	// focused on the CheckTx event branch, not the rate-limit-check block.
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return(&paymenttypes.StreamRecord{StaticBalance: sdkmath.NewInt(1)}, nil)

	checkTxCtx := s.ctx.WithIsCheckTx(true)
	err := s.storageKeeper.LockObjectStoreFee(checkTxCtx, bucketInfo, objectInfo)
	s.Require().NoError(err)

	found := false
	for _, e := range checkTxCtx.EventManager().Events() {
		if e.Type == "moca.payment.EventFeePreview" {
			found = true
		}
	}
	s.Require().True(found, "CheckTx must emit a fee-preview event")
}

func (s *TestSuite) TestLockObjectStoreFee_RateLimitCheckBillError() {
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: bucketOwner.String(), PaymentAddress: paymentAccount.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 999, ChargedReadQuota: 100,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	// A non-zero ChargedReadQuota keeps GetBucketReadStoreBill from short-circuiting before it
	// ever reaches the GVG family lookup below. Different bucket owner and payment account, and
	// no explicit per-bucket rate limit stored, so shouldCheckRateLimit's fallback
	// (!IsPaymentAccountOwner) must be true.
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	err := s.storageKeeper.LockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorContains(err, "get bucket bill failed")
}

func (s *TestSuite) TestLockObjectStoreFee_UpdateStreamRecordFails() {
	ownerAddr := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: ownerAddr.String(), BucketName: bucketName, Id: sdkmath.NewUint(1),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom"))

	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	err := s.storageKeeper.LockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorContains(err, "update stream record failed")
}

func (s *TestSuite) TestUnlockObjectStoreFee_UpdateError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "unlockobj-updateerror-bucket"}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom"))

	err := s.storageKeeper.UnlockObjectStoreFee(s.ctx, bucketInfo, objectInfo)
	s.Require().ErrorContains(err, "update stream record failed")
}

// TestChargeObjectStoreFee_PriceChanged_Success covers ChargeObjectStoreFee's priceChanged==true
// branch (it delegates to ChargeViaBucketChange instead of ChargeViaObjectChange). The price at
// internalBucketInfo's stale PriceTime differs from the price at the current block time, which is
// exactly what IsPriceChanged detects.
func (s *TestSuite) TestChargeObjectStoreFee_PriceChanged_Success() {
	ownerAddr := sample.RandAccAddress().String()
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String(), SecondarySpIds: []uint32{101}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	oldPriceTime := s.ctx.BlockTime().Unix() - 1000
	oldPrice := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	newPrice := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(2), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), oldPriceTime).Return(oldPrice, nil).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Not(oldPriceTime)).Return(newPrice, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: gvg.Id, TotalChargeSize: 0}
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr, PaymentAddress: ownerAddr, BucketName: "pricechanged-bucket",
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id,
	}
	internalBucketInfo := &types.InternalBucketInfo{PriceTime: oldPriceTime, LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)

	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, LocalVirtualGroupId: lvg.Id, CreateAt: s.ctx.BlockTime().Unix() + 1}
	wantChargeSize, err := s.storageKeeper.GetObjectChargeSize(s.ctx, objectInfo.PayloadSize, objectInfo.GetLatestUpdatedTime())
	s.Require().NoError(err)

	err = s.storageKeeper.ChargeObjectStoreFee(s.ctx, 1, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().NoError(err)
	s.Require().Equal(wantChargeSize, internalBucketInfo.LocalVirtualGroups[0].TotalChargeSize,
		"a price-changed recompute must still accumulate the object's charge size via ChargeViaBucketChange")
}

func (s *TestSuite) TestUnChargeObjectStoreFee_ChargeSizeError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargeobj-sizeerror-bucket"}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: 0}

	err := s.storageKeeper.UnChargeObjectStoreFee(s.ctx, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "get charge size failed")
}

func (s *TestSuite) TestUnChargeObjectStoreFee_ChargeViaObjectChangeError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "unchargeobj-cvcerror-bucket", GlobalVirtualGroupFamilyId: 999}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, CreateAt: s.ctx.BlockTime().Unix() + 1}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	err := s.storageKeeper.UnChargeObjectStoreFee(s.ctx, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "apply object store bill failed")
}

func (s *TestSuite) TestUnChargeObjectStoreFee_VersionedParamsError() {
	ownerAddr := sample.RandAccAddress().String()
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String(), SecondarySpIds: []uint32{101}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).
		DoAndReturn(func(flows []paymenttypes.OutFlow) []paymenttypes.OutFlow { return flows }).AnyTimes()
	// ChargeViaObjectChange makes two GetVersionedParamsWithTs calls of its own before returning
	// (once for the LVG store-bill delta, once inside its not-forced GetBucketReadStoreBill
	// rate-limit check) — both must succeed. Only UnChargeObjectStoreFee's own, separate call
	// (for the early-deletion window) fails.
	gomock.InOrder(
		s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
			Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil),
		s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
			Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil),
		s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
			Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")),
	)

	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: gvg.Id, TotalChargeSize: 10_000_000}
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr, PaymentAddress: ownerAddr, BucketName: "uncharge-verparamerr-bucket",
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id,
	}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 10_000_000, LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)

	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, LocalVirtualGroupId: lvg.Id, CreateAt: s.ctx.BlockTime().Unix() + 1}

	err := s.storageKeeper.UnChargeObjectStoreFee(s.ctx, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "failed to get versioned params")
}

func (s *TestSuite) TestUnChargeObjectStoreFee_EarlyDeletionError() {
	ownerAddr := sample.RandAccAddress().String()
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String(), SecondarySpIds: []uint32{101}}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	// A huge ReserveTime keeps timeToPay positive so UnChargeObjectStoreFee reaches the
	// early-deletion charge below.
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec(), ReserveTime: 1_000_000}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).
		DoAndReturn(func(flows []paymenttypes.OutFlow) []paymenttypes.OutFlow { return flows }).AnyTimes()
	s.paymentKeeper.EXPECT().UpdateStreamRecordByAddr(gomock.Any(), gomock.Any()).
		Return((*paymenttypes.StreamRecord)(nil), fmt.Errorf("boom")).AnyTimes()

	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: gvg.Id, TotalChargeSize: 10_000_000}
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr, PaymentAddress: ownerAddr, BucketName: "uncharge-earlydel-bucket",
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id,
	}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 10_000_000, LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)

	objectInfo := &types.ObjectInfo{ObjectName: "obj", PayloadSize: 1024, LocalVirtualGroupId: lvg.Id, CreateAt: s.ctx.BlockTime().Unix() + 1}

	err := s.storageKeeper.UnChargeObjectStoreFee(s.ctx, bucketInfo, internalBucketInfo, objectInfo)
	s.Require().ErrorContains(err, "pay for early deletion failed")
}

// TestChargeViaBucketChange_PreviouslyLimited_ApplyError covers the isPreviousBucketLimited==true
// branch's ApplyBillChanges error (a bespoke fixture: seedLimitedBucket's own ApplyUserFlowsList
// mock is AnyTimes-success, so it cannot be overridden after the fact for just the second call).
func (s *TestSuite) TestChargeViaBucketChange_PreviouslyLimited_ApplyError() {
	paymentAddr := sample.RandAccAddress().String()
	bucketName := "cvbc-limitedapplyerror-bucket"
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	bucketInfo := &types.BucketInfo{
		Owner: paymentAddr, PaymentAddress: paymentAddr, BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: gvgFamily.Id, ChargedReadQuota: 100,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	// First call: tripping the bucket into "limited" (must succeed). Second call: this test's
	// actual ChargeViaBucketChange invocation, resuming via a changed payment address (must fail).
	gomock.InOrder(
		s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil),
		s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom")),
	)

	ownerAddr := sdk.MustAccAddressFromHex(paymentAddr)
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, ownerAddr, bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)
	s.Require().True(s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName))

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	newPaymentAddr := sample.RandAccAddress().String()
	err = s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.PaymentAddress = newPaymentAddr
		return nil
	})
	s.Require().ErrorContains(err, "boom")
}

func (s *TestSuite) TestChargeViaBucketChange_NotLimited_ApplyError() {
	ownerAddr := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: ownerAddr.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 50,
	}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	err := s.storageKeeper.ChargeViaBucketChange(s.ctx, bucketInfo, internalBucketInfo, func(bi *types.BucketInfo, _ *types.InternalBucketInfo) error {
		bi.ChargedReadQuota = 60
		return nil
	})
	s.Require().ErrorContains(err, "apply user flows list failed")
}

func (s *TestSuite) TestChargeViaObjectChange_PriceError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvoc-priceerror-bucket", GlobalVirtualGroupFamilyId: 1}
	internalBucketInfo := &types.InternalBucketInfo{}
	objectInfo := &types.ObjectInfo{ObjectName: "obj"}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.ChargeViaObjectChange(s.ctx, bucketInfo, internalBucketInfo, objectInfo, 1024, false)
	s.Require().ErrorContains(err, "get storage price failed")
}

func (s *TestSuite) TestChargeViaObjectChange_GVGNotFound() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvoc-gvgnotfound-bucket", GlobalVirtualGroupFamilyId: 1}
	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: 999}
	internalBucketInfo := &types.InternalBucketInfo{LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", LocalVirtualGroupId: lvg.Id}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroup)(nil), false).AnyTimes()

	_, err := s.storageKeeper.ChargeViaObjectChange(s.ctx, bucketInfo, internalBucketInfo, objectInfo, 1024, false)
	s.Require().ErrorContains(err, "get GVG failed")
}

func (s *TestSuite) TestChargeViaObjectChange_VersionedParamsError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), BucketName: "cvoc-verparamerror-bucket", GlobalVirtualGroupFamilyId: 1}
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String(), SecondarySpIds: []uint32{101}}
	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: gvg.Id}
	internalBucketInfo := &types.InternalBucketInfo{LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}
	objectInfo := &types.ObjectInfo{ObjectName: "obj", LocalVirtualGroupId: lvg.Id}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.ChargeViaObjectChange(s.ctx, bucketInfo, internalBucketInfo, objectInfo, 1024, false)
	s.Require().ErrorContains(err, "failed to get validator tax rate")
}

func (s *TestSuite) TestGetBucketReadStoreBill_PriceError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().ErrorContains(err, "get storage price failed")
}

func (s *TestSuite) TestGetBucketReadStoreBill_VersionedParamsError() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")).AnyTimes()

	_, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, &types.InternalBucketInfo{})
	s.Require().ErrorContains(err, "failed to get validator tax rate")
}

func (s *TestSuite) TestGetBucketReadStoreBill_LVGGVGNotFound() {
	bucketInfo := &types.BucketInfo{PaymentAddress: sample.RandAccAddress().String(), ChargedReadQuota: 0, GlobalVirtualGroupFamilyId: 1}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroup)(nil), false).AnyTimes()

	lvg := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: 999, TotalChargeSize: 100}
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 100, LocalVirtualGroups: []*types.LocalVirtualGroup{lvg}}

	_, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().ErrorContains(err, "get GVG failed")
}

func (s *TestSuite) TestUnChargeBucketReadStoreFee_RateLimited() {
	paymentAddr := sample.RandAccAddress().String()
	bucketInfo := seedLimitedBucket(s, paymentAddr, "unchargestore-ratelimited-bucket")
	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)

	err := s.storageKeeper.UnChargeBucketReadStoreFee(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
}

func (s *TestSuite) TestGetObjectLockFee_ChargeSizeError() {
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()

	_, _, err := s.storageKeeper.GetObjectLockFee(s.ctx, 0, 1024)
	s.Require().ErrorContains(err, "get charge size failed")
}

func (s *TestSuite) TestGetObjectLockFee_VersionedParamsError() {
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{}, fmt.Errorf("boom")).AnyTimes()

	priceTime := s.ctx.BlockTime().Unix() + 1
	_, _, err := s.storageKeeper.GetObjectLockFee(s.ctx, priceTime, 1024)
	s.Require().ErrorContains(err, "get versioned reserve time error")
}

// The per-block delete-GC bookkeeping lives in the regular KV store, so the only
// thing keeping it off the app hash is EndBlocker draining it within the block.
// These run against the real app and inspect the committed IAVL store directly,
// rather than a cache layer.

type commitProbe struct {
	t   *testing.T
	app *app.Moca
	key *storetypes.KVStoreKey
}

func newCommitProbe(t *testing.T) *commitProbe {
	t.Helper()
	a := app.EthSetupWithDB(false, nil, dbm.NewMemDB())
	return &commitProbe{t: t, app: a, key: a.GetKey(storagetypes.StoreKey)}
}

// block branches the commit multi-store, runs body as the block's txs would,
// optionally runs the storage EndBlocker, then writes and commits. It returns
// the app hash.
func (p *commitProbe) block(height int64, body func(sdk.Context), runEndBlocker bool) []byte {
	p.t.Helper()
	cms := p.app.CommitMultiStore()
	ms := cms.CacheMultiStore()
	ctx := sdk.NewContext(ms, tmproto.Header{Height: height}, false, log.NewNopLogger()).
		WithGasMeter(storetypes.NewInfiniteGasMeter())

	if body != nil {
		body(ctx)
	}
	if runEndBlocker {
		require.NoError(p.t, keeper.EndBlocker(ctx, p.app.StorageKeeper))
	}
	ms.Write()
	return cms.Commit().Hash
}

// committed reads the key out of the committed IAVL store, past every cache.
func (p *commitProbe) committed() []byte {
	return p.app.CommitMultiStore().GetCommitKVStore(p.key).
		Get(storagetypes.CurrentBlockDeleteStalePoliciesKey)
}

func (p *commitProbe) storageStoreHash() []byte {
	return p.app.CommitMultiStore().GetCommitKVStore(p.key).LastCommitID().Hash
}

// writeDeleteInfo writes what appendResourceIDForGarbageCollection writes for a
// single deleted group.
func writeDeleteInfo(t *testing.T, ctx sdk.Context, key *storetypes.KVStoreKey) {
	t.Helper()
	di := &storagetypes.DeleteInfo{
		BucketIds: &storagetypes.Ids{},
		ObjectIds: &storagetypes.Ids{},
		GroupIds:  &storagetypes.Ids{Id: []sdkmath.Uint{sdkmath.NewUint(7)}},
	}
	bz, err := di.Marshal()
	require.NoError(t, err)
	ctx.KVStore(key).Set(storagetypes.CurrentBlockDeleteStalePoliciesKey, bz)
}

// TestDeleteInfoNeverReachesCommittedState is the invariant the whole change
// rests on.
func TestDeleteInfoNeverReachesCommittedState(t *testing.T) {
	p := newCommitProbe(t)

	p.block(2, func(ctx sdk.Context) {
		writeDeleteInfo(t, ctx, p.key)
		require.NotNil(t, ctx.KVStore(p.key).Get(storagetypes.CurrentBlockDeleteStalePoliciesKey),
			"precondition: the bookkeeping was written during the block")
	}, true)

	require.Nil(t, p.committed(), "the delete-GC bookkeeping key must not be in committed state")
}

// TestDeleteInfoLeakChangesAppHash is the negative control: if EndBlocker ever
// does not run, the key lands in IAVL and the app hash moves.
func TestDeleteInfoLeakChangesAppHash(t *testing.T) {
	leaky := newCommitProbe(t)
	leakedHash := leaky.block(2, func(ctx sdk.Context) { writeDeleteInfo(t, ctx, leaky.key) }, false)
	require.NotNil(t, leaky.committed(), "sanity: without EndBlocker the key is committed")

	clean := newCommitProbe(t)
	cleanHash := clean.block(2, func(ctx sdk.Context) { writeDeleteInfo(t, ctx, clean.key) }, true)
	require.Nil(t, clean.committed())

	require.NotEqual(t, hex.EncodeToString(cleanHash), hex.EncodeToString(leakedHash),
		"a leaked bookkeeping key changes the app hash")
}

// TestEndBlockerDeleteIsHashNeutral covers the other direction: EndBlocker
// deletes the key on every block, including blocks where nothing wrote it. That
// must not perturb the store.
func TestEndBlockerDeleteIsHashNeutral(t *testing.T) {
	withEB := newCommitProbe(t)
	noEB := newCommitProbe(t)
	for h := int64(2); h <= 6; h++ {
		withEB.block(h, nil, true)
		noEB.block(h, nil, false)
	}
	require.Equal(t,
		hex.EncodeToString(noEB.storageStoreHash()),
		hex.EncodeToString(withEB.storageStoreHash()),
		"deleting an absent key every block must be hash-neutral")
	require.Nil(t, withEB.committed())
}
