package challenge_test

import (
	"errors"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/challenge/keeper"
	"github.com/mocachain/moca/v2/x/challenge/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

type TestSuite struct {
	suite.Suite

	cdc             codec.Codec
	challengeKeeper *keeper.Keeper
	storeKey        storetypes.StoreKey

	bankKeeper    *types.MockBankKeeper
	storageKeeper *types.MockStorageKeeper
	spKeeper      *types.MockSpKeeper
	stakingKeeper *types.MockStakingKeeper
	paymentKeeper *types.MockPaymentKeeper

	ctx         sdk.Context
	queryClient types.QueryClient
	msgServer   types.MsgServer
}

func (s *TestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))

	// set mock randao mix
	randaoMix := crypto.Keccak256([]byte{1})
	randaoMix = append(randaoMix, crypto.Keccak256([]byte{2})...)
	header := testCtx.Ctx.BlockHeader()
	header.RandaoMix = randaoMix
	testCtx = testutil.TestContext{
		Ctx: sdk.NewContext(testCtx.CMS, header, false, testCtx.Ctx.Logger()),
		DB:  testCtx.DB,
		CMS: testCtx.CMS,
	}

	s.ctx = testCtx.Ctx

	ctrl := gomock.NewController(s.T())

	bankKeeper := types.NewMockBankKeeper(ctrl)
	storageKeeper := types.NewMockStorageKeeper(ctrl)
	spKeeper := types.NewMockSpKeeper(ctrl)
	stakingKeeper := types.NewMockStakingKeeper(ctrl)
	paymentKeeper := types.NewMockPaymentKeeper(ctrl)

	s.challengeKeeper = keeper.NewKeeper(
		encCfg.Codec,
		key,
		key,
		bankKeeper,
		storageKeeper,
		spKeeper,
		stakingKeeper,
		paymentKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	s.cdc = encCfg.Codec
	s.storeKey = key
	s.bankKeeper = bankKeeper
	s.storageKeeper = storageKeeper
	s.spKeeper = spKeeper
	s.stakingKeeper = stakingKeeper
	s.paymentKeeper = paymentKeeper

	err := s.challengeKeeper.SetParams(s.ctx, types.DefaultParams())
	s.Require().NoError(err)

	queryHelper := baseapp.NewQueryServerTestHelper(testCtx.Ctx, encCfg.InterfaceRegistry)
	types.RegisterQueryServer(queryHelper, s.challengeKeeper)

	s.queryClient = types.NewQueryClient(queryHelper)
	s.msgServer = keeper.NewMsgServerImpl(*s.challengeKeeper)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestBeginBlocker_RemoveExpiredChallenge() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id:            100,
		ExpiredHeight: 100,
	})
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id:            200,
		ExpiredHeight: 300,
	})

	s.ctx = s.ctx.WithBlockHeight(101)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 100))
	s.Require().True(s.challengeKeeper.ExistsChallenge(s.ctx, 200))
}

func (s *TestSuite) TestBeginBlocker_RemoveSlash() {
	s.challengeKeeper.SaveSlash(s.ctx, types.Slash{
		SpId:     100,
		ObjectId: math.NewUint(100),
		Height:   100,
	})
	s.challengeKeeper.SaveSlash(s.ctx, types.Slash{
		SpId:     200,
		ObjectId: math.NewUint(200),
		Height:   200,
	})

	params := s.challengeKeeper.GetParams(s.ctx)
	params.SlashCoolingOffPeriod = 10
	_ = s.challengeKeeper.SetParams(s.ctx, params)

	s.ctx = s.ctx.WithBlockHeight(101)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, 100, math.NewUint(100)))
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, 200, math.NewUint(200)))

	s.ctx = s.ctx.WithBlockHeight(111)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().False(s.challengeKeeper.ExistsSlash(s.ctx, 100, math.NewUint(100)))
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, 200, math.NewUint(200)))

	s.ctx = s.ctx.WithBlockHeight(211)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().False(s.challengeKeeper.ExistsSlash(s.ctx, 100, math.NewUint(100)))
	s.Require().False(s.challengeKeeper.ExistsSlash(s.ctx, 200, math.NewUint(200)))
}

func (s *TestSuite) TestBeginBlocker_RemoveSpSlashAmount() {
	s.challengeKeeper.SetSpSlashAmount(s.ctx, 100, math.NewInt(100))
	s.challengeKeeper.SetSpSlashAmount(s.ctx, 200, math.NewInt(200))

	params := s.challengeKeeper.GetParams(s.ctx)
	params.SpSlashCountingWindow = 10
	_ = s.challengeKeeper.SetParams(s.ctx, params)

	s.ctx = s.ctx.WithBlockHeight(101)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().True(s.challengeKeeper.GetSpSlashAmount(s.ctx, 100).Int64() == 100)
	s.Require().True(s.challengeKeeper.GetSpSlashAmount(s.ctx, 200).Int64() == 200)

	s.ctx = s.ctx.WithBlockHeight(100)
	challenge.BeginBlocker(s.ctx, *s.challengeKeeper)
	s.Require().False(s.challengeKeeper.GetSpSlashAmount(s.ctx, 100).Int64() == 100)
	s.Require().False(s.challengeKeeper.GetSpSlashAmount(s.ctx, 200).Int64() == 200)
}

// BeginBlocker must not panic when params are unset (zero SpSlashCountingWindow).
func (s *TestSuite) TestBeginBlocker_ZeroSpSlashCountingWindow_NoPanic() {
	s.ctx.KVStore(s.storeKey).Delete(types.ParamsKey)
	s.Require().Equal(uint64(0), s.challengeKeeper.GetParams(s.ctx).SpSlashCountingWindow)

	s.ctx = s.ctx.WithBlockHeight(1)
	s.Require().NotPanics(func() {
		s.Require().NoError(challenge.BeginBlocker(s.ctx, *s.challengeKeeper))
	})
}

func (s *TestSuite) TestEndBlocker_NoRandomChallenge() {
	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)

	params := s.challengeKeeper.GetParams(s.ctx)
	params.ChallengeCountPerBlock = 0
	_ = s.challengeKeeper.SetParams(s.ctx, params)

	challenge.EndBlocker(s.ctx, *s.challengeKeeper)
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_ObjectNotExists() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(0))

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	challenge.EndBlocker(s.ctx, *s.challengeKeeper)
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SuccessRandomChallenge() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))
	s.storageKeeper.EXPECT().MaxSegmentSize(gomock.Any(), gomock.Any()).Return(uint64(10000), nil).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(64),
		BucketName:   "bucketname",
		ObjectName:   "objectname",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(existObject.Id)).
		Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{
		BucketName: existObject.BucketName,
		Id:         math.NewUint(10),
	}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(existBucket.BucketName)).
		Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{
		1,
	}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()

	// the auto-raised challenge must bind the sp and lock its deposit, the same as a
	// submitted one: this is the path that raises most challenges on a live chain.
	s.spKeeper.EXPECT().SetDepositLockUntil(gomock.Any(), gomock.Eq(sp.Id), gomock.Any()).Times(1)

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	challenge.EndBlocker(s.ctx, *s.challengeKeeper)
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID-1)

	boundSpID, bound := s.challengeKeeper.GetChallengeSpID(s.ctx, afterChallengeID)
	s.Require().True(bound, "the auto-raised challenge must record the sp it names")
	s.Require().Equal(sp.Id, boundSpID)
}

func (s *TestSuite) TestEndBlocker_SkipsUnsealedOrMissingObject() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SkipsEmptyPayload() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	emptyObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "empty-payload-bucket",
		ObjectName:   "empty-payload-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  0,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(emptyObject, true).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SkipsMissingBucket() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "missing-bucket",
		ObjectName:   "missing-bucket-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SkipsMissingGVG() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "missing-gvg-bucket",
		ObjectName:   "missing-gvg-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

// TestEndBlocker_SuccessRandomChallenge_PrimarySp covers the primary-sp
// branch (spOperatorID = gvg.PrimarySpId), which the existing success test
// never reaches. An empty SecondarySpIds forces sps==1, so
// RandomRedundancyIndex's mod-1 is always 0 regardless of the seed,
// deterministically resolving to the primary sp every time.
func (s *TestSuite) TestEndBlocker_SuccessRandomChallenge_PrimarySp() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))
	s.storageKeeper.EXPECT().MaxSegmentSize(gomock.Any(), gomock.Any()).Return(uint64(10000), nil).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "primary-sp-bucket",
		ObjectName:   "primary-sp-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: gvg.PrimarySpId, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(gvg.PrimarySpId)).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().SetDepositLockUntil(gomock.Any(), gomock.Eq(sp.Id), gomock.Any()).Times(1)

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID-1)

	boundSpID, bound := s.challengeKeeper.GetChallengeSpID(s.ctx, afterChallengeID)
	s.Require().True(bound, "the auto-raised challenge must record the sp it names")
	s.Require().Equal(sp.Id, boundSpID)
}

func (s *TestSuite) TestEndBlocker_SkipsMissingStorageProvider() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "no-sp-bucket",
		ObjectName:   "no-sp-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{1}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SkipsInvalidStorageProviderStatus() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "jailed-sp-bucket",
		ObjectName:   "jailed-sp-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{1}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_JAILED}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

// TestEndBlocker_DedupSkipsRepeatedPair covers the objectMap dedup branch by
// requiring 2 challenges per block while only ever offering one deterministic
// (sp, object) pair: the empty SecondarySpIds trick from
// TestEndBlocker_SuccessRandomChallenge_PrimarySp pins spOperatorID to the
// same value on every iteration, so the map key this loop dedups on never
// changes. The 1st iteration creates a challenge; every later one must skip.
func (s *TestSuite) TestEndBlocker_DedupSkipsRepeatedPair() {
	params := s.challengeKeeper.GetParams(s.ctx)
	params.ChallengeCountPerBlock = 2
	s.Require().NoError(s.challengeKeeper.SetParams(s.ctx, params))

	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))
	s.storageKeeper.EXPECT().MaxSegmentSize(gomock.Any(), gomock.Any()).Return(uint64(10000), nil).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "dedup-bucket",
		ObjectName:   "dedup-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: gvg.PrimarySpId, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().SetDepositLockUntil(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID-1, "exactly one challenge should be created despite needing 2")
}

func (s *TestSuite) TestEndBlocker_SkipsExistingSlash() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "slashed-bucket",
		ObjectName:   "slashed-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{1}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	// Real store, not a mock: seed the exact (sp, object) pair EndBlocker
	// will look up (GetStorageProvider/GetObjectInfoById above always return
	// this same sp/object regardless of the queried id), so ExistsSlash short
	// -circuits every iteration.
	s.challengeKeeper.SaveSlash(s.ctx, types.Slash{
		SpId:     sp.Id,
		ObjectId: existObject.Id,
		Height:   uint64(s.ctx.BlockHeight()),
	})

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}

func (s *TestSuite) TestEndBlocker_SkipsOnSegmentSizeError() {
	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(100))
	s.storageKeeper.EXPECT().MaxSegmentSize(gomock.Any(), gomock.Any()).
		Return(uint64(0), errors.New("boom")).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		BucketName:   "segment-error-bucket",
		ObjectName:   "segment-error-object",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(existObject, true).AnyTimes()

	existBucket := &storagetypes.BucketInfo{BucketName: existObject.BucketName, Id: math.NewUint(1)}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Any()).Return(existBucket, true).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: 100, SecondarySpIds: []uint32{1}}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Any(), gomock.Any()).Return(gvg, true).AnyTimes()

	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	preChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().NoError(challenge.EndBlocker(s.ctx, *s.challengeKeeper))
	afterChallengeID := s.challengeKeeper.GetChallengeId(s.ctx)
	s.Require().True(preChallengeID == afterChallengeID)
}
