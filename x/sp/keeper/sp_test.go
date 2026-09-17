package keeper_test

import (
	"errors"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	types2 "github.com/mocachain/moca/v2/sdk/types"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func (s *KeeperTestSuite) TestSetGetStorageProvider() {
	keeper := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{Id: 100}
	spAccStr := sample.RandAccAddressHex()
	spAcc := sdk.MustAccAddressFromHex(spAccStr)
	sp.OperatorAddress = spAcc.String()

	keeper.SetStorageProvider(ctx, sp)
	_, found := keeper.GetStorageProvider(ctx, 100)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)
}

// TestStorageProviderBasics tests GetStorageProviderByOperatorAddr, GetStorageProviderByFundingAddr,
// GetStorageProviderBySealAddr, GetStorageProviderByApprovalAddr, GetStorageProviderByBlsKey
func (s *KeeperTestSuite) TestStorageProviderBasics() {
	k := s.spKeeper
	ctx := s.ctx
	spAccStr := sample.RandAccAddressHex()
	spAcc := sdk.MustAccAddressFromHex(spAccStr)

	fundingAccStr := sample.RandAccAddressHex()
	fundingAcc := sdk.MustAccAddressFromHex(fundingAccStr)

	sealAccStr := sample.RandAccAddressHex()
	sealAcc := sdk.MustAccAddressFromHex(sealAccStr)

	approvalAccStr := sample.RandAccAddressHex()
	approvalAcc := sdk.MustAccAddressFromHex(approvalAccStr)

	blsPubKey := sample.RandBlsPubKey()
	sp := &types.StorageProvider{
		Id:              100,
		OperatorAddress: spAcc.String(),
		FundingAddress:  fundingAcc.String(),
		SealAddress:     sealAcc.String(),
		ApprovalAddress: approvalAcc.String(),
		BlsKey:          blsPubKey,
	}

	k.SetStorageProvider(ctx, sp)
	_, found := k.GetStorageProvider(ctx, 100)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)

	k.SetStorageProviderByFundingAddr(ctx, sp)
	_, found = k.GetStorageProviderByFundingAddr(ctx, fundingAcc)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)

	k.SetStorageProviderBySealAddr(ctx, sp)
	_, found = k.GetStorageProviderBySealAddr(ctx, sealAcc)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)

	k.SetStorageProviderByApprovalAddr(ctx, sp)
	_, found = k.GetStorageProviderByApprovalAddr(ctx, approvalAcc)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)

	k.SetStorageProviderByBlsKey(ctx, sp)
	_, found = k.GetStorageProviderByBlsKey(ctx, blsPubKey)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)
}

func (s *KeeperTestSuite) TestGetAllStorageProviders() {
	k := s.spKeeper
	ctx := s.ctx

	ids := []uint32{100, 200, 300}
	for _, id := range ids {
		sp := &types.StorageProvider{
			Id:              id,
			OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		}
		k.SetStorageProvider(ctx, sp)
	}

	sps := k.GetAllStorageProviders(ctx)
	require.Len(s.T(), sps, len(ids))

	got := make(map[uint32]bool, len(sps))
	for _, sp := range sps {
		got[sp.Id] = true
	}
	for _, id := range ids {
		require.True(s.T(), got[id], "storage provider %d not returned", id)
	}
}

func (s *KeeperTestSuite) TestSlashBasic() {
	// mock
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	k := s.spKeeper
	ctx := s.ctx
	spAccStr := sample.RandAccAddressHex()
	spAcc := sdk.MustAccAddressFromHex(spAccStr)

	fundingAccStr := sample.RandAccAddressHex()
	fundingAcc := sdk.MustAccAddressFromHex(fundingAccStr)

	sealAccStr := sample.RandAccAddressHex()
	sealAcc := sdk.MustAccAddressFromHex(sealAccStr)

	approvalAccStr := sample.RandAccAddressHex()
	approvalAcc := sdk.MustAccAddressFromHex(approvalAccStr)

	blsPubKey := sample.RandBlsPubKey()

	sp := &types.StorageProvider{
		Id:              100,
		OperatorAddress: spAcc.String(),
		FundingAddress:  fundingAcc.String(),
		SealAddress:     sealAcc.String(),
		ApprovalAddress: approvalAcc.String(),
		BlsKey:          blsPubKey,
		TotalDeposit:    math.NewIntWithDecimal(2010, types2.DecimalMOCA),
	}

	k.SetStorageProvider(ctx, sp)
	_, found := k.GetStorageProvider(ctx, 100)
	if !found {
		fmt.Printf("no such sp: %s", spAcc)
	}
	require.EqualValues(s.T(), found, true)

	rewardInfo := types.RewardInfo{
		Address: sample.RandAccAddressHex(),
		Amount:  sdk.NewCoin(types2.Denom, math.NewIntWithDecimal(10, types2.DecimalMOCA)),
	}

	err := k.Slash(ctx, sp.Id, []types.RewardInfo{rewardInfo})
	require.NoError(s.T(), err)

	spAfterSlash, found := k.GetStorageProvider(ctx, 100)
	require.True(s.T(), found)
	s.T().Logf("%s", spAfterSlash.TotalDeposit.String())
	require.True(s.T(), spAfterSlash.TotalDeposit.Equal(math.NewIntWithDecimal(2000, types2.DecimalMOCA)))
}

// TestGetStorageProviderNotFound exercises GetStorageProvider's own not-found
// branch directly. The By...Addr getters short-circuit on their own index miss
// before ever reaching this lookup, so this path needs a direct call.
func (s *KeeperTestSuite) TestGetStorageProviderNotFound() {
	_, found := s.spKeeper.GetStorageProvider(s.ctx, 999999)
	require.False(s.T(), found)
}

func (s *KeeperTestSuite) TestMustGetStorageProvider() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{Id: 500}
	k.SetStorageProvider(ctx, sp)

	got := k.MustGetStorageProvider(ctx, 500)
	require.Equal(s.T(), sp.Id, got.Id)

	require.Panics(s.T(), func() {
		k.MustGetStorageProvider(ctx, 999999)
	})
}

// TestDepositLockUntil covers the SetDepositLockUntil/ReleaseDepositLockUntil/
// GetDepositLockUntil trio: the watermark-only-moves-forward behavior of Set,
// the unconditional overwrite (including delete-on-zero) behavior of Release,
// and the zero-value default of Get on an unset key.
func (s *KeeperTestSuite) TestDepositLockUntil() {
	k := s.spKeeper
	ctx := s.ctx
	spID := uint32(700)

	require.EqualValues(s.T(), 0, k.GetDepositLockUntil(ctx, spID))

	k.SetDepositLockUntil(ctx, spID, 100)
	require.EqualValues(s.T(), 100, k.GetDepositLockUntil(ctx, spID))

	// A lower height must not move the watermark backward.
	k.SetDepositLockUntil(ctx, spID, 50)
	require.EqualValues(s.T(), 100, k.GetDepositLockUntil(ctx, spID))

	// A higher height moves it forward.
	k.SetDepositLockUntil(ctx, spID, 150)
	require.EqualValues(s.T(), 150, k.GetDepositLockUntil(ctx, spID))

	// ReleaseDepositLockUntil sets unconditionally, even backward.
	k.ReleaseDepositLockUntil(ctx, spID, 30)
	require.EqualValues(s.T(), 30, k.GetDepositLockUntil(ctx, spID))

	// A zero height deletes the entry.
	k.ReleaseDepositLockUntil(ctx, spID, 0)
	require.EqualValues(s.T(), 0, k.GetDepositLockUntil(ctx, spID))
}

func (s *KeeperTestSuite) TestSlashNotFound() {
	err := s.spKeeper.Slash(s.ctx, 999999, []types.RewardInfo{})
	require.ErrorIs(s.T(), err, types.ErrStorageProviderNotFound)
}

func (s *KeeperTestSuite) TestSlashInvalidDenom() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{
		Id:              900,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		TotalDeposit:    math.NewIntWithDecimal(100, types2.DecimalMOCA),
	}
	k.SetStorageProvider(ctx, sp)

	rewardInfo := types.RewardInfo{
		Address: sample.RandAccAddressHex(),
		Amount:  sdk.NewCoin("wrongdenom", math.NewInt(1)),
	}
	err := k.Slash(ctx, sp.Id, []types.RewardInfo{rewardInfo})
	require.ErrorIs(s.T(), err, types.ErrInvalidDenom)
}

func (s *KeeperTestSuite) TestSlashInsufficientDeposit() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{
		Id:              901,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		TotalDeposit:    math.NewIntWithDecimal(10, types2.DecimalMOCA),
	}
	k.SetStorageProvider(ctx, sp)

	rewardInfo := types.RewardInfo{
		Address: sample.RandAccAddressHex(),
		Amount:  sdk.NewCoin(types2.Denom, math.NewIntWithDecimal(20, types2.DecimalMOCA)),
	}
	err := k.Slash(ctx, sp.Id, []types.RewardInfo{rewardInfo})
	require.ErrorIs(s.T(), err, types.ErrInsufficientDepositAmount)
}

func (s *KeeperTestSuite) TestSlashInvalidRewardAddress() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{
		Id:              903,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		TotalDeposit:    math.NewIntWithDecimal(100, types2.DecimalMOCA),
	}
	k.SetStorageProvider(ctx, sp)

	rewardInfo := types.RewardInfo{
		Address: "", // sdk.AccAddressFromHexUnsafe rejects the empty address outright
		Amount:  sdk.NewCoin(types2.Denom, math.NewIntWithDecimal(10, types2.DecimalMOCA)),
	}
	err := k.Slash(ctx, sp.Id, []types.RewardInfo{rewardInfo})
	require.Error(s.T(), err)
}

func (s *KeeperTestSuite) TestSlashSendCoinsError() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{
		Id:              902,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		TotalDeposit:    math.NewIntWithDecimal(100, types2.DecimalMOCA),
	}
	k.SetStorageProvider(ctx, sp)

	wantErr := errors.New("send failed")
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)

	rewardInfo := types.RewardInfo{
		Address: sample.RandAccAddressHex(),
		Amount:  sdk.NewCoin(types2.Denom, math.NewIntWithDecimal(10, types2.DecimalMOCA)),
	}
	err := k.Slash(ctx, sp.Id, []types.RewardInfo{rewardInfo})
	require.ErrorIs(s.T(), err, wantErr)
}

// Exit has to remove the BLS-key index entry it wrote. The entry is only
// reachable through the raw store: GetStorageProviderByBlsKey resolves the index
// and then loads the storage provider, which Exit does delete, so a stale index
// entry reads back as not-found and is invisible from the keeper API.
func (s *KeeperTestSuite) TestExitDeletesBlsKeyIndex() {
	k := s.spKeeper
	ctx := s.ctx

	blsPubKey := sample.RandBlsPubKey()
	sp := &types.StorageProvider{
		Id:              200,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		FundingAddress:  sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		SealAddress:     sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		ApprovalAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		GcAddress:       sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		BlsKey:          blsPubKey,
	}

	k.SetStorageProvider(ctx, sp)
	k.SetStorageProviderByBlsKey(ctx, sp)

	indexKey := types.GetStorageProviderByBlsKeyKey(blsPubKey)
	require.NotNil(s.T(), ctx.KVStore(s.storeKey).Get(indexKey),
		"the index entry must exist before the exit")

	require.NoError(s.T(), k.Exit(ctx, sp))

	require.Nil(s.T(), ctx.KVStore(s.storeKey).Get(indexKey),
		"exit must delete the BLS-key index entry")
}
