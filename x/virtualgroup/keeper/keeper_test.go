package keeper_test

import (
	"errors"
	stdmath "math"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// newSP returns an in-service storage provider fixture with a random funding
// address; SwapAsPrimarySP/SwapOutAsSecondarySP/DeleteGVG all resolve funding
// addresses through it, so it must always be a valid hex address.
func newSP(id uint32) *sptypes.StorageProvider {
	return &sptypes.StorageProvider{Id: id, Status: sptypes.STATUS_IN_SERVICE, FundingAddress: sample.RandAccAddress().String()}
}

// newExitingSP is newSP(1) with a caller-chosen status, for the SwapIn precondition
// that the target SP must be graceful- or forced-exiting.
func newExitingSP(status sptypes.Status) *sptypes.StorageProvider {
	return &sptypes.StorageProvider{Id: 1, Status: status, FundingAddress: sample.RandAccAddress().String()}
}

// setStats seeds a GVGStatisticsWithinSP record. Several keeper.go functions
// (SwapAsPrimarySP, SwapOutAsSecondarySP, completeSwapInGVG) unconditionally
// Must-fetch this once their preceding checks pass.
func (s *TestSuite) setStats(spID, primaryCount, secondaryCount uint32) {
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{
		StorageProviderId: spID, PrimaryCount: primaryCount, SecondaryCount: secondaryCount,
	})
}

// setFamilyStats seeds a GVGFamilyStatisticsWithinSP record, Must-fetched by
// SwapAsPrimarySP once ownership of the family is confirmed.
func (s *TestSuite) setFamilyStats(spID uint32, familyIDs ...uint32) {
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: familyIDs,
	})
}

// stubZeroSettlement makes every SettleAndDistributeGVG(Family) call a no-op
// success, for tests that must clear settlement without exercising it.
func (s *TestSuite) stubZeroSettlement() {
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).AnyTimes()
}

// ---- GetAuthority / Logger / GenNextGVGFamilyID ----

func (s *TestSuite) TestGetAuthority() {
	require.Equal(s.T(), authtypes.NewModuleAddress(govtypes.ModuleName).String(), s.virtualgroupKeeper.GetAuthority())
}

func (s *TestSuite) TestLogger() {
	require.NotNil(s.T(), s.virtualgroupKeeper.Logger(s.ctx))
}

func (s *TestSuite) TestGenNextGVGFamilyID() {
	first := s.virtualgroupKeeper.GenNextGVGFamilyID(s.ctx)
	second := s.virtualgroupKeeper.GenNextGVGFamilyID(s.ctx)
	require.NotEqual(s.T(), first, second)
}

// ---- SetGVGAndEmitUpdateEvent ----

func (s *TestSuite) TestSetGVGAndEmitUpdateEvent() {
	gvg := &types.GlobalVirtualGroup{Id: 90, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String()}
	err := s.virtualgroupKeeper.SetGVGAndEmitUpdateEvent(s.ctx, gvg)
	require.NoError(s.T(), err)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 90)
	require.True(s.T(), found)
	require.Equal(s.T(), gvg.Id, stored.Id)
}

// ---- GetGVGFamily ----

func (s *TestSuite) TestGetGVGFamily_NotFound() {
	_, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, 12345)
	require.False(s.T(), found)
}

// ---- GetAndCheckGVGFamilyAvailableForNewBucket ----

func (s *TestSuite) TestGetAndCheckGVGFamilyAvailableForNewBucket_FamilyNotFound() {
	_, err := s.virtualgroupKeeper.GetAndCheckGVGFamilyAvailableForNewBucket(s.ctx, 999)
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestGetAndCheckGVGFamilyAvailableForNewBucket_ExceedsLimit() {
	params := types.DefaultParams()
	params.MaxStoreSizePerFamily = 50
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))

	gvg := &types.GlobalVirtualGroup{Id: 100, FamilyId: 101, StoredSize: 60, TotalDeposit: math.ZeroInt()}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 101, GlobalVirtualGroupIds: []uint32{100}})

	_, err := s.virtualgroupKeeper.GetAndCheckGVGFamilyAvailableForNewBucket(s.ctx, 101)
	require.ErrorIs(s.T(), err, types.ErrLimitationExceed)
}

func (s *TestSuite) TestGetAndCheckGVGFamilyAvailableForNewBucket_Available() {
	gvg := &types.GlobalVirtualGroup{Id: 102, FamilyId: 103, StoredSize: 10, TotalDeposit: math.ZeroInt()}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 103, GlobalVirtualGroupIds: []uint32{102}})

	family, err := s.virtualgroupKeeper.GetAndCheckGVGFamilyAvailableForNewBucket(s.ctx, 103)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint32(103), family.Id)
}

// ---- GetOrCreateEmptyGVGFamily ----

func (s *TestSuite) TestGetOrCreateEmptyGVGFamily_NoSpecifiedIDCreatesNew() {
	family, err := s.virtualgroupKeeper.GetOrCreateEmptyGVGFamily(s.ctx, types.NoSpecifiedFamilyID, 7)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint32(7), family.PrimarySpId)
	require.NotEmpty(s.T(), family.VirtualPaymentAddress)

	stat, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, 7)
	require.True(s.T(), found)
	require.Contains(s.T(), stat.GlobalVirtualGroupFamilyIds, family.Id)
}

func (s *TestSuite) TestGetOrCreateEmptyGVGFamily_SpecifiedIDNotFound() {
	_, err := s.virtualgroupKeeper.GetOrCreateEmptyGVGFamily(s.ctx, 555, 7)
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestGetOrCreateEmptyGVGFamily_SpecifiedIDNotOwned() {
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 110, PrimarySpId: 1})
	_, err := s.virtualgroupKeeper.GetOrCreateEmptyGVGFamily(s.ctx, 110, 2)
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotOwned)
}

func (s *TestSuite) TestGetOrCreateEmptyGVGFamily_SpecifiedIDOwnedSucceeds() {
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 111, PrimarySpId: 3})
	family, err := s.virtualgroupKeeper.GetOrCreateEmptyGVGFamily(s.ctx, 111, 3)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint32(111), family.Id)
}

// ---- GetAvailableStakingTokens ----

func (s *TestSuite) TestGetAvailableStakingTokens() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(10)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))

	gvg := &types.GlobalVirtualGroup{StoredSize: 5, TotalDeposit: math.NewInt(100)}
	require.Equal(s.T(), math.NewInt(50), s.virtualgroupKeeper.GetAvailableStakingTokens(s.ctx, gvg))
}

// ---- StorageProviderExitable (extends payment_test.go's TestStorageProviderExitable) ----

func (s *TestSuite) TestStorageProviderExitable_PrimaryCountNonZeroErrors() {
	const spID = uint32(230)
	s.setStats(spID, 1, 0)
	err := s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID)
	require.ErrorIs(s.T(), err, types.ErrSPCanNotExit)
}

func (s *TestSuite) TestStorageProviderExitable_SecondaryCountNonZeroErrors() {
	const spID = uint32(231)
	s.setStats(spID, 0, 1)
	err := s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID)
	require.ErrorIs(s.T(), err, types.ErrSPCanNotExit)
}

// ---- SwapAsPrimarySP ----

func (s *TestSuite) TestSwapAsPrimarySP_FamilyNotFound() {
	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, newSP(1), newSP(2), 999, false)
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestSwapAsPrimarySP_FamilyNotOwnedByPrimarySP() {
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 5, PrimarySpId: 99})
	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, newSP(1), newSP(2), 5, false)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapAsPrimarySP_GVGNotFound() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{123}})

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestSwapAsPrimarySP_GVGPrimaryMismatch_SwapOut() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 50, FamilyId: 7, PrimarySpId: 999, TotalDeposit: math.ZeroInt()})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{50}})

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapAsPrimarySP_GVGPrimaryMismatch_SwapIn() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 51, FamilyId: 7, PrimarySpId: 999, TotalDeposit: math.ZeroInt()})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{51}})

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, true)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapAsPrimarySP_SuccessorAlreadySecondary_SwapOutRejected() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 52, FamilyId: 7, PrimarySpId: src.Id, SecondarySpIds: []uint32{dst.Id, 3}, TotalDeposit: math.ZeroInt(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{52}})

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 52)
	require.True(s.T(), found)
	require.Equal(s.T(), src.Id, stored.PrimarySpId, "rejected swap must not mutate the gvg")
}

func (s *TestSuite) TestSwapAsPrimarySP_SuccessorAlreadySecondary_SwapInBreaksRedundancy() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 53, FamilyId: 7, PrimarySpId: src.Id, SecondarySpIds: []uint32{dst.Id, 3}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{53}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, true)
	require.NoError(s.T(), err)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 53)
	require.True(s.T(), found)
	require.Equal(s.T(), dst.Id, stored.PrimarySpId)

	dstStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, dst.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(1), dstStat.BreakRedundancyReqmtGvgCount)
}

func (s *TestSuite) TestSwapAsPrimarySP_DepositSwapBankTransferFails() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 54, FamilyId: 7, PrimarySpId: src.Id, TotalDeposit: math.NewInt(500),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{54}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.bankKeeper.EXPECT().SendCoins(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("bank down"))

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.ErrorContains(s.T(), err, "bank down")
}

func (s *TestSuite) TestSwapAsPrimarySP_SettleFailure_SwapOut() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, VirtualPaymentAddress: sample.RandAccAddress().String()})
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("query failed"))

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapAsPrimarySP_SettleFailure_SwapIn() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 1, 0)
	s.setFamilyStats(src.Id, 7)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: src.Id, VirtualPaymentAddress: sample.RandAccAddress().String()})
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("query failed"))

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, true)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapAsPrimarySP_Success() {
	src, dst := newSP(1), newSP(2)
	s.setStats(src.Id, 2, 0)
	s.setFamilyStats(src.Id, 7)
	s.stubZeroSettlement()

	gvgA := &types.GlobalVirtualGroup{Id: 60, FamilyId: 7, PrimarySpId: src.Id, TotalDeposit: math.NewInt(200), VirtualPaymentAddress: sample.RandAccAddress().String()}
	gvgB := &types.GlobalVirtualGroup{Id: 61, FamilyId: 7, PrimarySpId: src.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvgA)
	s.virtualgroupKeeper.SetGVG(s.ctx, gvgB)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 7, PrimarySpId: src.Id, GlobalVirtualGroupIds: []uint32{60, 61}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.bankKeeper.EXPECT().
		SendCoins(gomock.Any(), sdk.MustAccAddressFromHex(dst.FundingAddress), sdk.MustAccAddressFromHex(src.FundingAddress), gomock.Any()).
		Return(nil)

	err := s.virtualgroupKeeper.SwapAsPrimarySP(s.ctx, src, dst, 7, false)
	require.NoError(s.T(), err)

	family, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, 7)
	require.True(s.T(), found)
	require.Equal(s.T(), dst.Id, family.PrimarySpId)

	for _, id := range []uint32{60, 61} {
		g, found := s.virtualgroupKeeper.GetGVG(s.ctx, id)
		require.True(s.T(), found)
		require.Equal(s.T(), dst.Id, g.PrimarySpId)
	}

	srcStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, src.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(0), srcStat.PrimaryCount)
	dstStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, dst.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(2), dstStat.PrimaryCount)

	srcFamilyStat, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, src.Id)
	require.True(s.T(), found)
	require.NotContains(s.T(), srcFamilyStat.GlobalVirtualGroupFamilyIds, uint32(7))
	dstFamilyStat, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, dst.Id)
	require.True(s.T(), found)
	require.Contains(s.T(), dstFamilyStat.GlobalVirtualGroupFamilyIds, uint32(7))
}

// ---- SwapOutAsSecondarySP ----

func (s *TestSuite) TestSwapOutAsSecondarySP_GVGNotFound() {
	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, newSP(1), newSP(2), 999)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_SuccessorIsPrimary() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 70, PrimarySpId: successorSP.Id, SecondarySpIds: []uint32{secondarySP.Id}, TotalDeposit: math.ZeroInt(),
	})

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 70)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_SuccessorAlreadySecondary() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 71, PrimarySpId: 9, SecondarySpIds: []uint32{secondarySP.Id, successorSP.Id}, TotalDeposit: math.ZeroInt(),
	})

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 71)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_NotACurrentSecondary() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 72, PrimarySpId: 9, SecondarySpIds: []uint32{5, 6}, TotalDeposit: math.ZeroInt(),
	})

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 72)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_SettleFailure() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 73, PrimarySpId: 9, SecondarySpIds: []uint32{secondarySP.Id}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("boom"))

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 73)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_Success_NewSuccessorStats() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.setStats(secondarySP.Id, 0, 1)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 74, PrimarySpId: 9, SecondarySpIds: []uint32{secondarySP.Id, 6}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 74)
	require.NoError(s.T(), err)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 74)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{successorSP.Id, 6}, stored.SecondarySpIds)

	originStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, secondarySP.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(0), originStat.SecondaryCount)
	successorStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, successorSP.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(1), successorStat.SecondaryCount)
}

func (s *TestSuite) TestSwapOutAsSecondarySP_Success_PreexistingSuccessorStats() {
	secondarySP, successorSP := newSP(1), newSP(2)
	s.setStats(secondarySP.Id, 0, 1)
	s.setStats(successorSP.Id, 0, 3)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 75, PrimarySpId: 9, SecondarySpIds: []uint32{secondarySP.Id}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})

	err := s.virtualgroupKeeper.SwapOutAsSecondarySP(s.ctx, secondarySP, successorSP, 75)
	require.NoError(s.T(), err)

	successorStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, successorSP.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(4), successorStat.SecondaryCount)
}

// ---- GetOrCreateGVGStatisticsWithinSP / MustGetGVGStatisticsWithinSP ----

func (s *TestSuite) TestGetOrCreateGVGStatisticsWithinSP_NotFoundReturnsZeroValue() {
	stat := s.virtualgroupKeeper.GetOrCreateGVGStatisticsWithinSP(s.ctx, 200)
	require.Equal(s.T(), uint32(200), stat.StorageProviderId)
	require.Equal(s.T(), uint32(0), stat.SecondaryCount)
}

func (s *TestSuite) TestGetOrCreateGVGStatisticsWithinSP_Found() {
	s.setStats(201, 3, 4)
	stat := s.virtualgroupKeeper.GetOrCreateGVGStatisticsWithinSP(s.ctx, 201)
	require.Equal(s.T(), uint32(3), stat.PrimaryCount)
	require.Equal(s.T(), uint32(4), stat.SecondaryCount)
}

func (s *TestSuite) TestMustGetGVGStatisticsWithinSP_PanicsWhenMissing() {
	require.Panics(s.T(), func() {
		s.virtualgroupKeeper.MustGetGVGStatisticsWithinSP(s.ctx, 9999)
	})
}

func (s *TestSuite) TestMustGetGVGStatisticsWithinSP_Found() {
	s.setStats(202, 1, 2)
	stat := s.virtualgroupKeeper.MustGetGVGStatisticsWithinSP(s.ctx, 202)
	require.Equal(s.T(), uint32(1), stat.PrimaryCount)
}

// ---- GetOrCreateGVGFamilyStatisticsWithinSP / DeleteSpecificGVGFamilyStatisticsFromSP / MustGetGVGFamilyStatisticsWithinSP ----

func (s *TestSuite) TestGetOrCreateGVGFamilyStatisticsWithinSP_NotFoundReturnsZeroValue() {
	stat := s.virtualgroupKeeper.GetOrCreateGVGFamilyStatisticsWithinSP(s.ctx, 210)
	require.Equal(s.T(), uint32(210), stat.SpId)
	require.Empty(s.T(), stat.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestGetOrCreateGVGFamilyStatisticsWithinSP_Found() {
	s.setFamilyStats(211, 1, 2, 3)
	stat := s.virtualgroupKeeper.GetOrCreateGVGFamilyStatisticsWithinSP(s.ctx, 211)
	require.Equal(s.T(), []uint32{1, 2, 3}, stat.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestDeleteSpecificGVGFamilyStatisticsFromSP_RemovesMatchingID() {
	s.setFamilyStats(220, 5, 6, 7)
	s.virtualgroupKeeper.DeleteSpecificGVGFamilyStatisticsFromSP(s.ctx, 220, 6)

	stat, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, 220)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{5, 7}, stat.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestDeleteSpecificGVGFamilyStatisticsFromSP_NoMatchIsNoop() {
	s.setFamilyStats(221, 5, 6)
	s.virtualgroupKeeper.DeleteSpecificGVGFamilyStatisticsFromSP(s.ctx, 221, 999)

	stat, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, 221)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{5, 6}, stat.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestMustGetGVGFamilyStatisticsWithinSP_PanicsWhenMissing() {
	require.Panics(s.T(), func() {
		s.virtualgroupKeeper.MustGetGVGFamilyStatisticsWithinSP(s.ctx, 9998)
	})
}

// ---- MigrateGlobalVirtualGroupFamiliesForSP ----

func (s *TestSuite) TestMigrateGlobalVirtualGroupFamiliesForSP() {
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 230, PrimarySpId: 1})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 231, PrimarySpId: 1})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 232, PrimarySpId: 2})

	s.virtualgroupKeeper.MigrateGlobalVirtualGroupFamiliesForSP(s.ctx)

	stat1, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, 1)
	require.True(s.T(), found)
	require.ElementsMatch(s.T(), []uint32{230, 231}, stat1.GlobalVirtualGroupFamilyIds)

	stat2, found := s.virtualgroupKeeper.GetGVGFamilyStatisticsWithinSP(s.ctx, 2)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{232}, stat2.GlobalVirtualGroupFamilyIds)
}

// ---- GetStoreSizeOfFamily / GetTotalStakingStoreSize / GetGlobalVirtualFamilyTotalStakingAndStoredSize / GetGlobalVirtualGroupIfAvailable ----

func (s *TestSuite) TestGetStoreSizeOfFamily_SumsAcrossGVGs() {
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 240, StoredSize: 100, TotalDeposit: math.ZeroInt()})
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 241, StoredSize: 200, TotalDeposit: math.ZeroInt()})
	family := &types.GlobalVirtualGroupFamily{Id: 242, GlobalVirtualGroupIds: []uint32{240, 241}}

	require.Equal(s.T(), uint64(300), s.virtualgroupKeeper.GetStoreSizeOfFamily(s.ctx, family))
}

func (s *TestSuite) TestGetStoreSizeOfFamily_PanicsOnMissingGVG() {
	family := &types.GlobalVirtualGroupFamily{Id: 243, GlobalVirtualGroupIds: []uint32{99999}}
	require.Panics(s.T(), func() {
		s.virtualgroupKeeper.GetStoreSizeOfFamily(s.ctx, family)
	})
}

func (s *TestSuite) TestGetTotalStakingStoreSize_Normal() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(10)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))

	gvg := &types.GlobalVirtualGroup{TotalDeposit: math.NewInt(1000)}
	require.Equal(s.T(), uint64(100), s.virtualgroupKeeper.GetTotalStakingStoreSize(s.ctx, gvg))
}

func (s *TestSuite) TestGetTotalStakingStoreSize_OverflowClampsToMaxUint64() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(1)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))

	gvg := &types.GlobalVirtualGroup{TotalDeposit: math.NewIntFromUint64(stdmath.MaxUint64).AddRaw(1)}
	require.Equal(s.T(), uint64(stdmath.MaxUint64), s.virtualgroupKeeper.GetTotalStakingStoreSize(s.ctx, gvg))
}

func (s *TestSuite) TestGetGlobalVirtualFamilyTotalStakingAndStoredSize_Success() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(1)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 250, StoredSize: 10, TotalDeposit: math.NewInt(20)})
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 251, StoredSize: 30, TotalDeposit: math.NewInt(40)})
	family := &types.GlobalVirtualGroupFamily{GlobalVirtualGroupIds: []uint32{250, 251}}

	staking, stored, err := s.virtualgroupKeeper.GetGlobalVirtualFamilyTotalStakingAndStoredSize(s.ctx, family)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(60), staking)
	require.Equal(s.T(), uint64(40), stored)
}

func (s *TestSuite) TestGetGlobalVirtualFamilyTotalStakingAndStoredSize_GVGNotFoundErrors() {
	family := &types.GlobalVirtualGroupFamily{GlobalVirtualGroupIds: []uint32{99998}}
	_, _, err := s.virtualgroupKeeper.GetGlobalVirtualFamilyTotalStakingAndStoredSize(s.ctx, family)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestGetGlobalVirtualGroupIfAvailable_NotFound() {
	_, err := s.virtualgroupKeeper.GetGlobalVirtualGroupIfAvailable(s.ctx, 99997, 10)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestGetGlobalVirtualGroupIfAvailable_InsufficientStaking() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(1)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 260, StoredSize: 90, TotalDeposit: math.NewInt(100)})

	_, err := s.virtualgroupKeeper.GetGlobalVirtualGroupIfAvailable(s.ctx, 260, 20)
	require.ErrorIs(s.T(), err, types.ErrInsufficientStaking)
}

func (s *TestSuite) TestGetGlobalVirtualGroupIfAvailable_Success() {
	params := types.DefaultParams()
	params.GvgStakingPerBytes = math.NewInt(1)
	require.NoError(s.T(), s.virtualgroupKeeper.SetParams(s.ctx, params))
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 261, StoredSize: 10, TotalDeposit: math.NewInt(100)})

	got, err := s.virtualgroupKeeper.GetGlobalVirtualGroupIfAvailable(s.ctx, 261, 20)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint32(261), got.Id)
}

// ---- SetSwapOutInfo ----

func (s *TestSuite) TestSetSwapOutInfo_Family_FreshSucceeds() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 10, nil, 1, 2))
}

func (s *TestSuite) TestSetSwapOutInfo_Family_AlreadyExistsErrors() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 10, nil, 1, 2))
	err := s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 10, nil, 1, 3)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSetSwapOutInfo_GVGs_FreshSucceedsForAll() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{20, 21}, 1, 2))

	// both entries were written: re-setting either now reports "already exists".
	require.ErrorIs(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{20}, 1, 2), types.ErrSwapOutFailed)
	require.ErrorIs(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{21}, 1, 2), types.ErrSwapOutFailed)
}

func (s *TestSuite) TestSetSwapOutInfo_GVGs_StopsAtFirstExisting() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{30}, 1, 2))

	err := s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{30, 31}, 1, 2)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)

	// gvg 31 must never have been written since the loop returned on gvg 30.
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{31}, 1, 2))
}

// ---- DeleteSwapOutInfo ----

func (s *TestSuite) TestDeleteSwapOutInfo_Family_NoInfoErrors() {
	err := s.virtualgroupKeeper.DeleteSwapOutInfo(s.ctx, 40, nil, 1)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestDeleteSwapOutInfo_Family_WrongSpIdErrors() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 41, nil, 1, 2))
	err := s.virtualgroupKeeper.DeleteSwapOutInfo(s.ctx, 41, nil, 99)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestDeleteSwapOutInfo_Family_Succeeds() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 42, nil, 1, 2))
	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapOutInfo(s.ctx, 42, nil, 1))

	// deleted: setting it again must succeed (no "already exists").
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 42, nil, 1, 2))
}

func (s *TestSuite) TestDeleteSwapOutInfo_GVGs_WrongSpIdErrors() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{50}, 1, 2))
	err := s.virtualgroupKeeper.DeleteSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{50}, 99)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestDeleteSwapOutInfo_GVGs_Succeeds() {
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{51, 52}, 1, 2))
	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{51, 52}, 1))
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{51, 52}, 1, 2))
}

// ---- CompleteSwapOut ----

func (s *TestSuite) TestCompleteSwapOut_Family_InfoNotFound() {
	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 60, nil, newSP(2))
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCompleteSwapOut_Family_SuccessorMismatch() {
	srcSP, successorSP, other := newSP(1), newSP(2), newSP(3)
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 61, nil, srcSP.Id, other.Id))

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 61, nil, successorSP)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCompleteSwapOut_Family_SpNotFound() {
	srcSP, successorSP := newSP(1), newSP(2)
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 62, nil, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(nil, false)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 62, nil, successorSP)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteSwapOut_Family_Success() {
	srcSP, successorSP := newSP(1), newSP(2)
	s.setStats(srcSP.Id, 1, 0)
	s.setFamilyStats(srcSP.Id, 63)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 63, PrimarySpId: srcSP.Id, VirtualPaymentAddress: sample.RandAccAddress().String()})
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 63, nil, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(srcSP, true)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 63, nil, successorSP)
	require.NoError(s.T(), err)

	family, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, 63)
	require.True(s.T(), found)
	require.Equal(s.T(), successorSP.Id, family.PrimarySpId)

	// swap-out info key was deleted: completing again now reports "not found".
	err = s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 63, nil, successorSP)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCompleteSwapOut_GVGs_InfoNotFound() {
	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, types.NoSpecifiedFamilyID, []uint32{70}, newSP(2))
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCompleteSwapOut_GVGs_SuccessorMismatch() {
	srcSP, successorSP, other := newSP(1), newSP(2), newSP(3)
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{71}, srcSP.Id, other.Id))

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, types.NoSpecifiedFamilyID, []uint32{71}, successorSP)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCompleteSwapOut_GVGs_SpNotFound() {
	srcSP, successorSP := newSP(1), newSP(2)
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{72}, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(nil, false)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, types.NoSpecifiedFamilyID, []uint32{72}, successorSP)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteSwapOut_GVGs_Success() {
	srcSP, successorSP := newSP(1), newSP(2)
	s.setStats(srcSP.Id, 0, 1)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 73, PrimarySpId: 9, SecondarySpIds: []uint32{srcSP.Id}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{73}, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(srcSP, true)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, types.NoSpecifiedFamilyID, []uint32{73}, successorSP)
	require.NoError(s.T(), err)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 73)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{successorSP.Id}, stored.SecondarySpIds)
}

// TestCompleteSwapOut_Family_DelegateFailurePropagates covers CompleteSwapOut's own
// "err := k.SwapAsPrimarySP(...); if err != nil { return err }" branch: the family
// references a gvg that was never created, so the delegated call itself fails.
func (s *TestSuite) TestCompleteSwapOut_Family_DelegateFailurePropagates() {
	srcSP, successorSP := newSP(1), newSP(2)
	s.setStats(srcSP.Id, 1, 0)
	s.setFamilyStats(srcSP.Id, 64)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 64, PrimarySpId: srcSP.Id, GlobalVirtualGroupIds: []uint32{999}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 64, nil, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(srcSP, true)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, 64, nil, successorSP)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

// TestCompleteSwapOut_GVGs_DelegateFailurePropagates covers the analogous
// "err := k.SwapOutAsSecondarySP(...); if err != nil { return err }" branch.
func (s *TestSuite) TestCompleteSwapOut_GVGs_DelegateFailurePropagates() {
	srcSP, successorSP := newSP(1), newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 74, PrimarySpId: 9, SecondarySpIds: []uint32{5, 6}, TotalDeposit: math.ZeroInt(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, types.NoSpecifiedFamilyID, []uint32{74}, srcSP.Id, successorSP.Id))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), srcSP.Id).Return(srcSP, true)

	err := s.virtualgroupKeeper.CompleteSwapOut(s.ctx, types.NoSpecifiedFamilyID, []uint32{74}, successorSP)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

// ---- DeleteGVG (additional branches beyond the pre-existing payment_test.go coverage) ----

func (s *TestSuite) TestDeleteGVG_NotFound() {
	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, newSP(1), 999)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestDeleteGVG_StoredSizeNonZeroErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 80, PrimarySpId: sp.Id, StoredSize: 100, TotalDeposit: math.ZeroInt()})

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 80)
	require.ErrorIs(s.T(), err, types.ErrGVGNotEmpty)
}

func (s *TestSuite) TestDeleteGVG_NonEmptyNetFlowErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 81, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(false)

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 81)
	require.ErrorIs(s.T(), err, types.ErrGVGNotEmpty)
}

func (s *TestSuite) TestDeleteGVG_SettleFailureErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 82, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("settle boom"))

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 82)
	require.Error(s.T(), err)
}

func (s *TestSuite) TestDeleteGVG_ResidualQueryErrorErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 83, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil)                         // settlement no-op
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("residual boom")) // sweep

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 83)
	require.Error(s.T(), err)
}

func (s *TestSuite) TestDeleteGVG_ResidualWithdrawErrorErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 84, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(50), nil)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("withdraw boom"))

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 84)
	require.Error(s.T(), err)
}

func (s *TestSuite) TestDeleteGVG_DepositRefundBankErrorErrors() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 85, PrimarySpId: sp.Id, TotalDeposit: math.NewInt(10), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).AnyTimes()
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), types.ModuleName, gomock.Any(), gomock.Any()).Return(errors.New("bank boom"))

	err := s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 85)
	require.Error(s.T(), err)
}

func (s *TestSuite) TestDeleteGVG_FamilyNotFoundPanics() {
	sp := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 86, FamilyId: 999, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).AnyTimes()

	require.Panics(s.T(), func() {
		_ = s.virtualgroupKeeper.DeleteGVG(s.ctx, sp, 86)
	})
}

// ---- SwapIn ----

func (s *TestSuite) TestSwapIn_Family_TargetNotExitingErrors() {
	err := s.virtualgroupKeeper.SwapIn(s.ctx, 300, 0, 2, newSP(1), s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderWrongStatus)
}

func (s *TestSuite) TestSwapIn_Family_NotFoundErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	err := s.virtualgroupKeeper.SwapIn(s.ctx, 301, 0, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestSwapIn_Family_PrimaryMismatchErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 302, PrimarySpId: 999})

	err := s.virtualgroupKeeper.SwapIn(s.ctx, 302, 0, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_GVG_NotFoundErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 310, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestSwapIn_GVG_SuccessorAlreadyPrimaryErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 311, PrimarySpId: 2, TotalDeposit: math.ZeroInt()})

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 311, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_GVG_SuccessorAlreadySecondaryErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 312, PrimarySpId: 9, SecondarySpIds: []uint32{1, 2}, TotalDeposit: math.ZeroInt()})

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 312, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_GVG_TargetNotASecondaryErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 313, PrimarySpId: 9, SecondarySpIds: []uint32{5, 6}, TotalDeposit: math.ZeroInt()})

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 313, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_GVG_ExitingTargetSucceeds() {
	target := newExitingSP(sptypes.STATUS_FORCED_EXITING)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 314, PrimarySpId: 9, SecondarySpIds: []uint32{1, 6}, TotalDeposit: math.ZeroInt()})
	expiration := s.ctx.BlockTime().Unix() + 100

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 314, 2, target, expiration)
	require.NoError(s.T(), err)

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 314)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(2), info.SuccessorSpId)
	require.Equal(s.T(), uint32(1), info.TargetSpId)
}

func (s *TestSuite) TestSwapIn_GVG_NotExitingAndUniqueSecondariesErrors() {
	target := newSP(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 315, PrimarySpId: 9, SecondarySpIds: []uint32{1, 6}, TotalDeposit: math.ZeroInt()})

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 315, 2, target, s.ctx.BlockTime().Unix()+100)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_GVG_NotExitingButBreaksRedundancySucceeds() {
	target := newSP(1)
	// target(1) is both primary and one of its own secondaries -- the "breaks redundancy" case.
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 316, PrimarySpId: 1, SecondarySpIds: []uint32{1, 6}, TotalDeposit: math.ZeroInt()})
	expiration := s.ctx.BlockTime().Unix() + 100

	err := s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 316, 2, target, expiration)
	require.NoError(s.T(), err)

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 316)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(2), info.SuccessorSpId)
}

// setSwapInInfo's expiry/successor branches, driven through SwapIn's family path.

func (s *TestSuite) TestSwapIn_Family_UnexpiredReservationBlocksNewRequest() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 320, PrimarySpId: target.Id})
	now := s.ctx.BlockTime()
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 320, 0, 2, target, now.Unix()+1000))

	err := s.virtualgroupKeeper.SwapIn(s.ctx, 320, 0, 3, target, now.Unix()+1000)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, 320, 0)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(2), info.SuccessorSpId, "the unexpired reservation must not be overwritten")
}

func (s *TestSuite) TestSwapIn_Family_ExpiredSameSuccessorIsRejected() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 321, PrimarySpId: target.Id})
	now := s.ctx.BlockTime()
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 321, 0, 2, target, now.Unix()+10))

	expiredCtx := s.ctx.WithBlockTime(now.Add(20 * time.Second))
	err := s.virtualgroupKeeper.SwapIn(expiredCtx, 321, 0, 2, target, now.Unix()+1000)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_Family_ExpiredDifferentSuccessorOverrides() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 322, PrimarySpId: target.Id})
	now := s.ctx.BlockTime()
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 322, 0, 2, target, now.Unix()+10))

	expiredCtx := s.ctx.WithBlockTime(now.Add(20 * time.Second))
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(expiredCtx, 322, 0, 3, target, now.Unix()+1000))

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, 322, 0)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(3), info.SuccessorSpId)
}

// ---- DeleteSwapInInfo ----

func (s *TestSuite) TestDeleteSwapInInfo_Family_NotFoundErrors() {
	err := s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 330, 0, 2)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestDeleteSwapInInfo_Family_MismatchErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 331, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 331, 0, 2, target, s.ctx.BlockTime().Unix()+100))

	err := s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 331, 0, 3)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestDeleteSwapInInfo_Family_Succeeds() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 332, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 332, 0, 2, target, s.ctx.BlockTime().Unix()+100))

	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 332, 0, 2))

	// Cancel tombstones the record (expires it immediately) instead of removing it.
	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, 332, 0)
	require.True(s.T(), found)
	require.LessOrEqual(s.T(), info.ExpirationTime, uint64(s.ctx.BlockTime().Unix())) //nolint:gosec // block time is never negative
}

func (s *TestSuite) TestDeleteSwapInInfo_GVG_NotFoundErrors() {
	err := s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 340, 2)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestDeleteSwapInInfo_GVG_Succeeds() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 341, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 341, 2, target, s.ctx.BlockTime().Unix()+100))

	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 341, 2))

	// Cancel tombstones the record (expires it immediately) instead of removing it.
	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 341)
	require.True(s.T(), found)
	require.LessOrEqual(s.T(), info.ExpirationTime, uint64(s.ctx.BlockTime().Unix())) //nolint:gosec // block time is never negative
}

// ---- Cancel then reserve ----
// The tombstone written by DeleteSwapInInfo must keep setSwapInInfo's same-successor guard
// engaged: the successor that just canceled cannot immediately re-reserve, only a different
// successor can.

func (s *TestSuite) TestSwapIn_Family_CancelThenSameSuccessorReserveIsRejected() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 323, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 323, 0, 2, target, s.ctx.BlockTime().Unix()+1000))
	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 323, 0, 2))

	err := s.virtualgroupKeeper.SwapIn(s.ctx, 323, 0, 2, target, s.ctx.BlockTime().Unix()+1000)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestSwapIn_Family_CancelThenDifferentSuccessorReserveSucceeds() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 324, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 324, 0, 2, target, s.ctx.BlockTime().Unix()+1000))
	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 324, 0, 2))

	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 324, 0, 3, target, s.ctx.BlockTime().Unix()+1000))

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, 324, 0)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(3), info.SuccessorSpId)
}

// ---- CompleteSwapIn / completeSwapInGVG ----

func (s *TestSuite) TestCompleteSwapIn_Family_NotFoundErrors() {
	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 350, 0, newSP(2))
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCompleteSwapIn_Family_MismatchErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor, other := newSP(2), newSP(3)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 351, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 351, 0, other.Id, target, s.ctx.BlockTime().Unix()+100))

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 351, 0, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCompleteSwapIn_Family_TargetSpNotFoundErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 352, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 352, 0, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(nil, false)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 352, 0, successor)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteSwapIn_Family_Success() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.setStats(target.Id, 1, 0)
	s.setFamilyStats(target.Id, 353)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 353, PrimarySpId: target.Id, VirtualPaymentAddress: sample.RandAccAddress().String()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 353, 0, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 353, 0, successor)
	require.NoError(s.T(), err)

	family, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, 353)
	require.True(s.T(), found)
	require.Equal(s.T(), successor.Id, family.PrimarySpId)

	_, found = s.virtualgroupKeeper.GetSwapInInfo(s.ctx, 353, 0)
	require.False(s.T(), found)
}

// TestCompleteSwapIn_Family_DelegateFailurePropagates covers CompleteSwapIn's own
// "if err := k.SwapAsPrimarySP(...); err != nil { return err }" branch: the family
// references a gvg that was never created, so the delegated call itself fails.
func (s *TestSuite) TestCompleteSwapIn_Family_DelegateFailurePropagates() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.setStats(target.Id, 1, 0)
	s.setFamilyStats(target.Id, 354)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 354, PrimarySpId: target.Id, GlobalVirtualGroupIds: []uint32{999}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 354, 0, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 354, 0, successor)
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestCompleteSwapIn_GVG_NotFoundErrors() {
	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 360, newSP(2))
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCompleteSwapIn_GVG_MismatchErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor, other := newSP(2), newSP(3)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 361, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 361, other.Id, target, s.ctx.BlockTime().Unix()+100))

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 361, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCompleteSwapIn_GVG_TargetSpNotFoundErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 362, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 362, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(nil, false)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 362, successor)
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// TestCompleteSwapIn_GVG_AlreadyPrimaryErrors simulates the successor having become
// primary (e.g. via a separate SwapAsPrimarySP) between reservation and completion --
// completeSwapInGVG's own defensive guard against that race.
func (s *TestSuite) TestCompleteSwapIn_GVG_AlreadyPrimaryErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 363, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 363, successor.Id, target, s.ctx.BlockTime().Unix()+100))

	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 363, PrimarySpId: successor.Id, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 363, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

// TestCompleteSwapIn_GVG_IndexNotCorrectPanics simulates the reserved target no longer
// being a secondary of the gvg by completion time (e.g. swapped out via a separate
// SwapOutAsSecondarySP in between) -- completeSwapInGVG's index-consistency panic.
func (s *TestSuite) TestCompleteSwapIn_GVG_IndexNotCorrectPanics() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 364, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 364, successor.Id, target, s.ctx.BlockTime().Unix()+100))

	// The reserved target(1) is no longer a secondary by completion time; settlement must
	// still clear (needs a valid VirtualPaymentAddress) before the index check panics.
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 364, PrimarySpId: 9, SecondarySpIds: []uint32{7, 8}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.stubZeroSettlement()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	require.Panics(s.T(), func() {
		_ = s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 364, successor)
	})
}

func (s *TestSuite) TestCompleteSwapIn_GVG_SettleFailureErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 365, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 365, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("settle boom"))

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 365, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCompleteSwapIn_GVG_Success_NewSuccessorStats() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.setStats(target.Id, 0, 1)
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 366, PrimarySpId: 9, SecondarySpIds: []uint32{1, 6}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 366, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 366, successor)
	require.NoError(s.T(), err)

	stored, found := s.virtualgroupKeeper.GetGVG(s.ctx, 366)
	require.True(s.T(), found)
	require.Equal(s.T(), []uint32{successor.Id, 6}, stored.SecondarySpIds)

	originStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, target.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(0), originStat.SecondaryCount)
	successorStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, successor.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(1), successorStat.SecondaryCount)

	_, found = s.virtualgroupKeeper.GetSwapInInfo(s.ctx, types.NoSpecifiedFamilyID, 366)
	require.False(s.T(), found)
}

// TestCompleteSwapIn_GVG_Success_BreaksRedundancyDecrementsCount covers the case where
// the swapped-out target was also the gvg's primary (the "breaks redundancy" case):
// completing the swap must decrement BreakRedundancyReqmtGvgCount, not just SecondaryCount.
func (s *TestSuite) TestCompleteSwapIn_GVG_Success_BreaksRedundancyDecrementsCount() {
	target := newSP(1) // in-service: reaches the redundancy-break branch of SwapIn's reservation.
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{
		StorageProviderId: target.Id, SecondaryCount: 1, BreakRedundancyReqmtGvgCount: 1,
	})
	s.stubZeroSettlement()
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 367, PrimarySpId: target.Id, SecondarySpIds: []uint32{target.Id}, TotalDeposit: math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 367, successor.Id, target, s.ctx.BlockTime().Unix()+100))
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), target.Id).Return(target, true)

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, types.NoSpecifiedFamilyID, 367, successor)
	require.NoError(s.T(), err)

	originStat, found := s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, target.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), uint32(0), originStat.SecondaryCount)
	require.Equal(s.T(), uint32(0), originStat.BreakRedundancyReqmtGvgCount)
}

// ---- CompleteSwapIn expiry ----

func (s *TestSuite) TestCompleteSwapIn_Family_ExpiredErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 355, PrimarySpId: target.Id})
	now := s.ctx.BlockTime()
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 355, 0, successor.Id, target, now.Unix()+10))

	expiredCtx := s.ctx.WithBlockTime(now.Add(20 * time.Second))
	err := s.virtualgroupKeeper.CompleteSwapIn(expiredCtx, 355, 0, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInExpired)
}

func (s *TestSuite) TestCompleteSwapIn_GVG_ExpiredErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: 368, PrimarySpId: 9, SecondarySpIds: []uint32{1}, TotalDeposit: math.ZeroInt()})
	now := s.ctx.BlockTime()
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 368, successor.Id, target, now.Unix()+10))

	expiredCtx := s.ctx.WithBlockTime(now.Add(20 * time.Second))
	err := s.virtualgroupKeeper.CompleteSwapIn(expiredCtx, types.NoSpecifiedFamilyID, 368, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInExpired)
}

// TestCompleteSwapIn_Family_CanceledErrors covers CompleteSwapIn against a tombstoned
// (canceled) reservation: the successor id still matches, so only the expiry check added
// alongside the cancel-tombstone stops a canceled reservation from being completed.
func (s *TestSuite) TestCompleteSwapIn_Family_CanceledErrors() {
	target := newExitingSP(sptypes.STATUS_GRACEFUL_EXITING)
	successor := newSP(2)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 356, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, 356, 0, successor.Id, target, s.ctx.BlockTime().Unix()+1000))
	require.NoError(s.T(), s.virtualgroupKeeper.DeleteSwapInInfo(s.ctx, 356, 0, successor.Id))

	err := s.virtualgroupKeeper.CompleteSwapIn(s.ctx, 356, 0, successor)
	require.ErrorIs(s.T(), err, types.ErrSwapInExpired)
}
