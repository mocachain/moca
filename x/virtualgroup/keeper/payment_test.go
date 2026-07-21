package keeper_test

import (
	"errors"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

type TestSuite struct {
	suite.Suite

	cdc                codec.Codec
	virtualgroupKeeper *keeper.Keeper

	bankKeeper    *types.MockBankKeeper
	accountKeeper *types.MockAccountKeeper
	spKeeper      *types.MockSpKeeper
	paymentKeeper *types.MockPaymentKeeper

	ctx sdk.Context
}

func (s *TestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(s.T())

	bankKeeper := types.NewMockBankKeeper(ctrl)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	spKeeper := types.NewMockSpKeeper(ctrl)
	paymentKeeper := types.NewMockPaymentKeeper(ctrl)

	s.ctx = testCtx.Ctx
	s.virtualgroupKeeper = keeper.NewKeeper(
		encCfg.Codec,
		key,
		key,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		spKeeper,
		accountKeeper,
		bankKeeper,
		paymentKeeper,
	)

	s.cdc = encCfg.Codec
	s.bankKeeper = bankKeeper
	s.accountKeeper = accountKeeper
	s.spKeeper = spKeeper
	s.paymentKeeper = paymentKeeper

	err := s.virtualgroupKeeper.SetParams(s.ctx, types.DefaultParams())
	s.Require().NoError(err)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestSettleAndDistributeGVGFamily() {
	sp := &sptypes.StorageProvider{Id: 1, FundingAddress: sample.RandAccAddress().String()}
	family := &types.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.ZeroInt(), nil)
	err := s.virtualgroupKeeper.SettleAndDistributeGVGFamily(s.ctx, sp, family)
	require.NoError(s.T(), err)

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.ZeroInt(), errors.New("error"))
	err = s.virtualgroupKeeper.SettleAndDistributeGVGFamily(s.ctx, sp, family)
	require.Error(s.T(), err)

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1024), nil)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	err = s.virtualgroupKeeper.SettleAndDistributeGVGFamily(s.ctx, sp, family)
	require.NoError(s.T(), err)
}

func (s *TestSuite) TestSettleAndDistributeGVG() {
	gvg := &types.GlobalVirtualGroup{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		SecondarySpIds:        []uint32{3, 6, 9},
	}

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.ZeroInt(), nil)
	err := s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg)
	require.NoError(s.T(), err)

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.ZeroInt(), errors.New("error"))
	err = s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg)
	require.Error(s.T(), err)

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1024), nil).AnyTimes()
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	sp := &sptypes.StorageProvider{Id: 1, FundingAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	err = s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg)
	require.NoError(s.T(), err)
}

// TestStorageProviderExitable asserts the exit precondition: an SP that is still the
// primary of a GVG family must not be exitable -- including the case where its GVG
// counts are already zero but a residual (empty) family still references it. That
// zero-count-with-residual-family case is the gap this guards against.
func (s *TestSuite) TestStorageProviderExitable() {
	const spID = uint32(7)

	// No GVG stats and no families -> exitable.
	require.NoError(s.T(), s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID))

	// GVG counts are zero, but a family still references the SP -> NOT exitable.
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{
		StorageProviderId: spID,
		PrimaryCount:      0,
		SecondaryCount:    0,
	})
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId:                        spID,
		GlobalVirtualGroupFamilyIds: []uint32{1},
	})
	require.ErrorIs(s.T(), s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID), types.ErrSPCanNotExit)

	// Once the family is swapped out (empty list) -> exitable again.
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId:                        spID,
		GlobalVirtualGroupFamilyIds: []uint32{},
	})
	require.NoError(s.T(), s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID))
}

// TestSettleAndDistributeGVG_DistributesEqualShares: each secondary gets an equal
// share (1024/3 = 341); the indivisible remainder stays in the account.
func (s *TestSuite) TestSettleAndDistributeGVG_DistributesEqualShares() {
	gvg := &types.GlobalVirtualGroup{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		SecondarySpIds:        []uint32{3, 6, 9},
	}
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1024), nil).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, id uint32) (*sptypes.StorageProvider, bool) {
			return &sptypes.StorageProvider{Id: id, FundingAddress: sample.RandAccAddress().String()}, true
		}).AnyTimes()

	var amounts []math.Int
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _, _ sdk.AccAddress, amount math.Int) error {
			amounts = append(amounts, amount)
			return nil
		}).AnyTimes()

	err := s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg)
	require.NoError(s.T(), err)

	require.Len(s.T(), amounts, 3) // one equal payout per secondary SP
	for _, a := range amounts {
		require.Equal(s.T(), math.NewInt(341), a) // 1024 / 3, dust not distributed
	}
}

// TestDeleteGVG_DrainsVPAAndSweepsRemainder: DeleteGVG pays equal shares and sweeps
// the remainder to the gov address, leaving the account at exactly zero.
func (s *TestSuite) TestDeleteGVG_DrainsVPAAndSweepsRemainder() {
	const (
		gvgID       = uint32(1)
		familyID    = uint32(1)
		primarySpID = uint32(2)
	)
	secondaries := []uint32{3, 6, 9}
	gvg := &types.GlobalVirtualGroup{
		Id:                    gvgID,
		FamilyId:              familyID,
		PrimarySpId:           primarySpID,
		SecondarySpIds:        secondaries,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		StoredSize:            0,
		TotalDeposit:          math.ZeroInt(),
	}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id:                    familyID,
		PrimarySpId:           primarySpID,
		GlobalVirtualGroupIds: []uint32{gvgID},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: primarySpID, PrimaryCount: 1})
	for _, id := range secondaries {
		s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: id, SecondaryCount: 1})
	}

	// Model the account draining so the sweep sees the real residual.
	vpaBalance := math.NewInt(1024)
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ sdk.AccAddress) (math.Int, error) {
			return vpaBalance, nil
		}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, id uint32) (*sptypes.StorageProvider, bool) {
			return &sptypes.StorageProvider{Id: id, FundingAddress: sample.RandAccAddress().String()}, true
		}).AnyTimes()
	toSecondaries := math.ZeroInt()
	sweptToGov := math.ZeroInt()
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _, to sdk.AccAddress, amount math.Int) error {
			vpaBalance = vpaBalance.Sub(amount)
			if to.Equals(paymenttypes.GovernanceAddress) {
				sweptToGov = sweptToGov.Add(amount)
			} else {
				toSecondaries = toSecondaries.Add(amount)
			}
			return nil
		}).AnyTimes()

	primarySp := &sptypes.StorageProvider{Id: primarySpID, FundingAddress: sample.RandAccAddress().String()}
	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, primarySp, gvgID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), math.NewInt(1023), toSecondaries) // 341 x 3 equal shares
	require.Equal(s.T(), math.NewInt(1), sweptToGov)       // indivisible remainder swept out
	require.True(s.T(), vpaBalance.IsZero())               // account fully drained to 0
	_, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.False(s.T(), found) // GVG deleted
}

// TestSettleAndDistributeGVG_Guards asserts the balance guards: zero is a no-op,
// a negative balance is rejected (invariant violation), and a positive balance
// with no secondary SPs is rejected rather than stranded.
func (s *TestSuite) TestSettleAndDistributeGVG_Guards() {
	gvg := &types.GlobalVirtualGroup{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		SecondarySpIds:        []uint32{3, 6, 9},
	}
	// zero balance -> nil (nothing to distribute)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil)
	require.NoError(s.T(), s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg))

	// negative balance -> error
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(-5), nil)
	require.Error(s.T(), s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, gvg))

	// positive balance but no secondary SPs -> error
	noSecondaries := &types.GlobalVirtualGroup{
		Id:                    2,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		SecondarySpIds:        nil,
	}
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(100), nil)
	require.Error(s.T(), s.virtualgroupKeeper.SettleAndDistributeGVG(s.ctx, noSecondaries))
}
