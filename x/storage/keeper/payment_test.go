package keeper_test

import (
	"encoding/hex"
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
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/x/challenge"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
	paymenttypes "github.com/evmos/evmos/v12/x/payment/types"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
	"github.com/evmos/evmos/v12/x/storage/keeper"
	"github.com/evmos/evmos/v12/x/storage/types"
	storagetypes "github.com/evmos/evmos/v12/x/storage/types"
	virtualgroupmoduletypes "github.com/evmos/evmos/v12/x/virtualgroup/types"
)

type TestSuite struct {
	suite.Suite

	cdc           codec.Codec
	storageKeeper *keeper.Keeper
	storeKey      storetypes.StoreKey

	accountKeeper      *types.MockAccountKeeper
	spKeeper           *types.MockSpKeeper
	permissionKeeper   *types.MockPermissionKeeper
	crossChainKeeper   *types.MockCrossChainKeeper
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
	crossChainKeeper := types.NewMockCrossChainKeeper(ctrl)
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
		crossChainKeeper,
		virtualGroupKeeper,
		evmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	evmKeeper.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).Return(&evmtypes.EstimateGasResponse{Gas: 100000}, nil).AnyTimes()
	evmKeeper.EXPECT().ApplyMessage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&evmtypes.MsgEthereumTxResponse{}, nil).AnyTimes()

	s.cdc = encCfg.Codec
	s.accountKeeper = accountKeeper
	s.spKeeper = spKeeper
	s.permissionKeeper = permissionKeeper
	s.crossChainKeeper = crossChainKeeper
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

// The per-block delete-GC bookkeeping lives in the regular KV store, so the only
// thing keeping it off the app hash is EndBlocker draining it within the block.
// These run against the real app and inspect the committed IAVL store directly,
// rather than a cache layer.

type commitProbe struct {
	t   *testing.T
	app *app.Evmos
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
