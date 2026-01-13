package keeper_test

import (
	"errors"
	"sort"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/x/payment/types"
)

func TestApplyFlowChanges(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	user := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)
	sp := sample.RandAccAddress()
	userInitBalance := sdkmath.NewInt(1e10)
	flowChanges := []types.StreamRecordChange{
		*types.NewDefaultStreamRecordChangeWithAddr(user).WithStaticBalanceChange(userInitBalance).WithRateChange(rate.Neg()),
		*types.NewDefaultStreamRecordChangeWithAddr(sp).WithRateChange(rate),
	}
	sr := &types.StreamRecord{
		Account:           user.String(),
		OutFlowCount:      1,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
	}
	keeper.SetStreamRecord(ctx, sr)
	err := keeper.ApplyStreamRecordChanges(ctx, flowChanges)
	require.NoError(t, err)
	userStreamRecord, found := keeper.GetStreamRecord(ctx, user)
	require.True(t, found)
	require.Equal(t, userStreamRecord.StaticBalance.Add(userStreamRecord.BufferBalance), userInitBalance)
	require.Equal(t, userStreamRecord.NetflowRate, rate.Neg())
	t.Logf("user stream record: %+v", userStreamRecord)
	spStreamRecord, found := keeper.GetStreamRecord(ctx, sp)
	require.Equal(t, spStreamRecord.NetflowRate, rate)
	require.Equal(t, spStreamRecord.StaticBalance, sdkmath.ZeroInt())
	require.Equal(t, spStreamRecord.BufferBalance, sdkmath.ZeroInt())
	require.True(t, found)
	t.Logf("sp stream record: %+v", spStreamRecord)
}

func TestMergeStreamRecordChanges(t *testing.T) {
	users := []sdk.AccAddress{
		sample.RandAccAddress(),
		sample.RandAccAddress(),
		sample.RandAccAddress(),
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].String() < users[j].String()
	})
	user1 := users[0]
	user2 := users[1]
	user3 := users[2]
	base := []types.StreamRecordChange{
		*types.NewDefaultStreamRecordChangeWithAddr(user1).WithRateChange(sdkmath.NewInt(100)).WithStaticBalanceChange(sdkmath.NewInt(1e10)),
		*types.NewDefaultStreamRecordChangeWithAddr(user2).WithRateChange(sdkmath.NewInt(200)).WithStaticBalanceChange(sdkmath.NewInt(2e10)),
	}
	changes := []types.StreamRecordChange{
		*types.NewDefaultStreamRecordChangeWithAddr(user1).WithRateChange(sdkmath.NewInt(100)).WithStaticBalanceChange(sdkmath.NewInt(1e10)),
		*types.NewDefaultStreamRecordChangeWithAddr(user3).WithRateChange(sdkmath.NewInt(200)).WithStaticBalanceChange(sdkmath.NewInt(2e10)),
	}
	k, _, _ := makePaymentKeeper(t)
	merged := k.MergeStreamRecordChanges(append(base, changes...))
	t.Logf("merged: %+v", merged)
	require.Equal(t, len(merged), 3)
	require.Equal(t, merged, []types.StreamRecordChange{
		*types.NewDefaultStreamRecordChangeWithAddr(user1).WithRateChange(sdkmath.NewInt(200)).WithStaticBalanceChange(sdkmath.NewInt(2e10)),
		*types.NewDefaultStreamRecordChangeWithAddr(user2).WithRateChange(sdkmath.NewInt(200)).WithStaticBalanceChange(sdkmath.NewInt(2e10)),
		*types.NewDefaultStreamRecordChangeWithAddr(user3).WithRateChange(sdkmath.NewInt(200)).WithStaticBalanceChange(sdkmath.NewInt(2e10)),
	})
}

func TestApplyUserFlows_ActiveStreamRecord(t *testing.T) {
	keeper, ctx, deepKeepers := makePaymentKeeper(t)
	ctx = ctx.WithIsCheckTx(true)

	from := sample.RandAccAddress()
	userFlows := types.UserFlows{
		From: from,
	}

	toAddr1 := sample.RandAccAddress()
	outFlow1 := types.OutFlow{
		ToAddress: toAddr1.String(),
		Rate:      sdkmath.NewInt(100),
	}
	userFlows.Flows = append(userFlows.Flows, outFlow1)

	toAddr2 := sample.RandAccAddress()
	outFlow2 := types.OutFlow{
		ToAddress: toAddr2.String(),
		Rate:      sdkmath.NewInt(200),
	}
	userFlows.Flows = append(userFlows.Flows, outFlow2)

	// no bank account
	deepKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(false).Times(1)
	err := keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.ErrorContains(t, err, "balance not enough")

	// has bank account, but balance is not enough
	deepKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()
	deepKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("transfer error")).Times(1)
	err = keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.ErrorContains(t, err, "balance not enough")

	// has bank account, and balance is enough
	deepKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	err = keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.NoError(t, err)

	fromRecord, _ := keeper.GetStreamRecord(ctx, from)
	require.True(t, fromRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, fromRecord.NetflowRate.Int64() == -300)
	require.True(t, fromRecord.StaticBalance.Int64() == 0)
	require.True(t, fromRecord.FrozenNetflowRate.Int64() == 0)
	require.True(t, fromRecord.LockBalance.Int64() == 0)
	require.True(t, fromRecord.BufferBalance.Int64() > 0)

	to1Record, _ := keeper.GetStreamRecord(ctx, toAddr1)
	require.True(t, to1Record.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, to1Record.NetflowRate.Int64() == 100)
	require.True(t, to1Record.StaticBalance.Int64() == 0)
	require.True(t, to1Record.FrozenNetflowRate.Int64() == 0)
	require.True(t, to1Record.LockBalance.Int64() == 0)
	require.True(t, to1Record.BufferBalance.Int64() == 0)

	to2Record, _ := keeper.GetStreamRecord(ctx, toAddr2)
	require.True(t, to2Record.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, to2Record.NetflowRate.Int64() == 200)
	require.True(t, to2Record.StaticBalance.Int64() == 0)
	require.True(t, to2Record.FrozenNetflowRate.Int64() == 0)
	require.True(t, to2Record.LockBalance.Int64() == 0)
	require.True(t, to2Record.BufferBalance.Int64() == 0)
}

func TestApplyUserFlows_Frozen(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)

	from := sample.RandAccAddress()
	toAddr1 := sample.RandAccAddress()
	toAddr2 := sample.RandAccAddress()

	// the account is frozen, and during auto settle or auto resume
	fromStreamRecord := types.NewStreamRecord(from, ctx.BlockTime().Unix())
	fromStreamRecord.Status = types.STREAM_ACCOUNT_STATUS_FROZEN
	fromStreamRecord.NetflowRate = sdkmath.NewInt(-100)
	fromStreamRecord.FrozenNetflowRate = sdkmath.NewInt(-200)
	fromStreamRecord.StaticBalance = sdkmath.ZeroInt()
	fromStreamRecord.OutFlowCount = 4
	keeper.SetStreamRecord(ctx, fromStreamRecord)

	keeper.SetOutFlow(ctx, from, &types.OutFlow{
		ToAddress: toAddr1.String(),
		Rate:      sdkmath.NewInt(40),
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})
	keeper.SetOutFlow(ctx, from, &types.OutFlow{
		ToAddress: sample.RandAccAddress().String(),
		Rate:      sdkmath.NewInt(60),
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})
	keeper.SetOutFlow(ctx, from, &types.OutFlow{
		ToAddress: toAddr2.String(),
		Rate:      sdkmath.NewInt(120),
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	})
	keeper.SetOutFlow(ctx, from, &types.OutFlow{
		ToAddress: sample.RandAccAddress().String(),
		Rate:      sdkmath.NewInt(80),
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	})

	to1StreamRecord := types.NewStreamRecord(toAddr1, ctx.BlockTime().Unix())
	to1StreamRecord.NetflowRate = sdkmath.NewInt(300)
	to1StreamRecord.StaticBalance = sdkmath.NewInt(300)
	keeper.SetStreamRecord(ctx, to1StreamRecord)

	to2StreamRecord := types.NewStreamRecord(toAddr2, ctx.BlockTime().Unix())
	to2StreamRecord.NetflowRate = sdkmath.NewInt(400)
	to2StreamRecord.StaticBalance = sdkmath.NewInt(400)
	keeper.SetStreamRecord(ctx, to2StreamRecord)

	userFlows := types.UserFlows{
		From: from,
	}

	outFlow1 := types.OutFlow{
		ToAddress: toAddr1.String(),
		Rate:      sdkmath.NewInt(-40),
	}
	userFlows.Flows = append(userFlows.Flows, outFlow1)

	outFlow2 := types.OutFlow{
		ToAddress: toAddr2.String(),
		Rate:      sdkmath.NewInt(-60),
	}
	userFlows.Flows = append(userFlows.Flows, outFlow2)

	// update frozen stream record needs force flag
	err := keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.ErrorContains(t, err, "frozen")

	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	err = keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.NoError(t, err)

	fromRecord, _ := keeper.GetStreamRecord(ctx, from)
	require.True(t, fromRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, fromRecord.StaticBalance.Int64() == 0)
	require.True(t, fromRecord.NetflowRate.Int64() == -60)
	require.True(t, fromRecord.FrozenNetflowRate.Int64() == -140)
	require.True(t, fromRecord.LockBalance.Int64() == 0)
	require.True(t, fromRecord.BufferBalance.Int64() == 0)

	outFlows := keeper.GetOutFlows(ctx, from)
	require.True(t, len(outFlows) == 3)
	// the out flow to toAddr1 should be deleted
	// the out flow to toAddr2 should be still there
	to1Found := false
	for _, outFlow := range outFlows {
		if outFlow.ToAddress == toAddr1.String() {
			to1Found = true
		}
		if outFlow.ToAddress == toAddr2.String() {
			require.True(t, outFlow.Rate.Int64() == 60)
			require.True(t, outFlow.Status == types.OUT_FLOW_STATUS_FROZEN)
		}
	}
	require.True(t, !to1Found)

	to1Record, _ := keeper.GetStreamRecord(ctx, toAddr1)
	require.True(t, to1Record.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, to1Record.NetflowRate.Int64() == 260)
	require.True(t, to1Record.FrozenNetflowRate.Int64() == 0)
	require.True(t, to1Record.LockBalance.Int64() == 0)
	require.True(t, to1Record.BufferBalance.Int64() == 0)

	to2Record, _ := keeper.GetStreamRecord(ctx, toAddr2)
	require.True(t, to2Record.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, to2Record.NetflowRate.Int64() == 400) // the outflow is frozen, which means the flow had been deduced
	require.True(t, to2Record.FrozenNetflowRate.Int64() == 0)
	require.True(t, to2Record.LockBalance.Int64() == 0)
	require.True(t, to2Record.BufferBalance.Int64() == 0)
}

func TestMergeUserFlows(t *testing.T) {
	k, _, _ := makePaymentKeeper(t)

	// Test case 1: Empty list
	t.Run("EmptyList", func(t *testing.T) {
		result := k.MergeUserFlows([]types.UserFlows{})
		require.Empty(t, result)
	})

	// Test case 2: Single UserFlows
	t.Run("SingleUserFlows", func(t *testing.T) {
		from := sample.RandAccAddress()
		to1 := sample.RandAccAddress()
		to2 := sample.RandAccAddress()

		userFlows := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows})
		require.Len(t, result, 1)
		require.Equal(t, from.String(), result[0].From.String())
		require.Len(t, result[0].Flows, 2)
	})

	// Test case 3: Multiple UserFlows with different From addresses
	t.Run("DifferentFromAddresses", func(t *testing.T) {
		from1 := sample.RandAccAddress()
		from2 := sample.RandAccAddress()
		to1 := sample.RandAccAddress()
		to2 := sample.RandAccAddress()

		userFlows1 := types.UserFlows{
			From: from1,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
			},
		}

		userFlows2 := types.UserFlows{
			From: from2,
			Flows: []types.OutFlow{
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows1, userFlows2})
		require.Len(t, result, 2)

		// Check that results are sorted by From address
		require.True(t, result[0].From.String() < result[1].From.String())
	})

	// Test case 4: Multiple UserFlows with same From address - positive and negative flows
	t.Run("SameFromAddressWithPositiveAndNegativeFlows", func(t *testing.T) {
		from := sample.RandAccAddress()
		to1 := sample.RandAccAddress()
		to2 := sample.RandAccAddress()

		// First UserFlows: positive rates
		userFlows1 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		// Second UserFlows: negative rates (to cancel previous charges)
		userFlows2 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(-50)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(-100)},
			},
		}

		// Third UserFlows: new positive rates
		userFlows3 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(30)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(60)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows1, userFlows2, userFlows3})
		require.Len(t, result, 1)
		require.Equal(t, from.String(), result[0].From.String())
		require.Len(t, result[0].Flows, 2)

		// Check merged rates: to1: 100 + (-50) + 30 = 80, to2: 200 + (-100) + 60 = 160
		flows := result[0].Flows

		// Find flows by ToAddress instead of assuming order
		to1Flow := findFlowByToAddress(flows, to1.String())
		to2Flow := findFlowByToAddress(flows, to2.String())

		require.NotNil(t, to1Flow)
		require.Equal(t, sdkmath.NewInt(80), to1Flow.Rate)
		require.NotNil(t, to2Flow)
		require.Equal(t, sdkmath.NewInt(160), to2Flow.Rate)
	})

	// Test case 5: Same From address with flows that cancel out to zero
	t.Run("SameFromAddressWithZeroResult", func(t *testing.T) {
		from := sample.RandAccAddress()
		to1 := sample.RandAccAddress()
		to2 := sample.RandAccAddress()

		// First UserFlows: positive rates
		userFlows1 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		// Second UserFlows: negative rates that exactly cancel out
		userFlows2 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(-100)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(-200)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows1, userFlows2})
		require.Len(t, result, 1)
		require.Equal(t, from.String(), result[0].From.String())
		require.Empty(t, result[0].Flows) // All flows should be filtered out as they sum to zero
	})

	// Test case 6: Complex scenario with multiple From addresses and mixed flows
	t.Run("ComplexScenario", func(t *testing.T) {
		from1 := sample.RandAccAddress()
		from2 := sample.RandAccAddress()
		to1 := sample.RandAccAddress()
		to2 := sample.RandAccAddress()
		to3 := sample.RandAccAddress()

		// Multiple UserFlows for from1
		userFlows1a := types.UserFlows{
			From: from1,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		userFlows1b := types.UserFlows{
			From: from1,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(-50)},
				{ToAddress: to3.String(), Rate: sdkmath.NewInt(150)},
			},
		}

		// Single UserFlows for from2
		userFlows2 := types.UserFlows{
			From: from2,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(300)},
				{ToAddress: to2.String(), Rate: sdkmath.NewInt(400)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows1a, userFlows1b, userFlows2})
		require.Len(t, result, 2)

		// Results should be sorted by From address
		require.True(t, result[0].From.String() < result[1].From.String())

		// Find the results for each From address
		var from1Result, from2Result types.UserFlows
		if result[0].From.String() == from1.String() {
			from1Result = result[0]
			from2Result = result[1]
		} else {
			from1Result = result[1]
			from2Result = result[0]
		}

		// Check from1 result: to1: 100 + (-50) = 50, to2: 200, to3: 150
		require.Len(t, from1Result.Flows, 3)
		from1Flows := from1Result.Flows
		sort.Slice(from1Flows, func(i, j int) bool {
			return from1Flows[i].ToAddress < from1Flows[j].ToAddress
		})

		// Find flows by ToAddress instead of assuming order
		to1Flow := findFlowByToAddress(from1Flows, to1.String())
		to2Flow := findFlowByToAddress(from1Flows, to2.String())
		to3Flow := findFlowByToAddress(from1Flows, to3.String())

		require.NotNil(t, to1Flow)
		require.Equal(t, sdkmath.NewInt(50), to1Flow.Rate)
		require.NotNil(t, to2Flow)
		require.Equal(t, sdkmath.NewInt(200), to2Flow.Rate)
		require.NotNil(t, to3Flow)
		require.Equal(t, sdkmath.NewInt(150), to3Flow.Rate)

		// Check from2 result: to1: 300, to2: 400
		require.Len(t, from2Result.Flows, 2)
		from2Flows := from2Result.Flows
		sort.Slice(from2Flows, func(i, j int) bool {
			return from2Flows[i].ToAddress < from2Flows[j].ToAddress
		})

		// Find flows by ToAddress instead of assuming order
		to1Flow2 := findFlowByToAddress(from2Flows, to1.String())
		to2Flow2 := findFlowByToAddress(from2Flows, to2.String())

		require.NotNil(t, to1Flow2)
		require.Equal(t, sdkmath.NewInt(300), to1Flow2.Rate)
		require.NotNil(t, to2Flow2)
		require.Equal(t, sdkmath.NewInt(400), to2Flow2.Rate)
	})

	// Test case 7: Edge case with duplicate ToAddress in same UserFlows
	t.Run("DuplicateToAddressInSameUserFlows", func(t *testing.T) {
		from := sample.RandAccAddress()
		to1 := sample.RandAccAddress()

		// Create two UserFlows with the same From address to trigger merging
		userFlows1 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(100)},
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(200)},
			},
		}

		userFlows2 := types.UserFlows{
			From: from,
			Flows: []types.OutFlow{
				{ToAddress: to1.String(), Rate: sdkmath.NewInt(-50)},
			},
		}

		result := k.MergeUserFlows([]types.UserFlows{userFlows1, userFlows2})
		require.Len(t, result, 1)

		// The MergeOutFlows should merge the duplicate ToAddress flows
		// Let's check if MergeOutFlows is working correctly
		allFlows := append(userFlows1.Flows, userFlows2.Flows...)
		mergedFlows := k.MergeOutFlows(allFlows)
		require.Len(t, mergedFlows, 1)
		require.Equal(t, to1.String(), mergedFlows[0].ToAddress)
		require.Equal(t, sdkmath.NewInt(250), mergedFlows[0].Rate) // 100 + 200 + (-50) = 250

		// Now check the result from MergeUserFlows
		require.Len(t, result[0].Flows, 1)
		require.Equal(t, to1.String(), result[0].Flows[0].ToAddress)
		require.Equal(t, sdkmath.NewInt(250), result[0].Flows[0].Rate)
	})
}

// Helper function to find flow by ToAddress
func findFlowByToAddress(flows []types.OutFlow, toAddress string) *types.OutFlow {
	for _, flow := range flows {
		if flow.ToAddress == toAddress {
			return &flow
		}
	}
	return nil
}
