package keeper_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestTryResumeStreamRecord_InResumingOrSettling(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	account := sample.RandAccAddress()
	// deposit to a resuming account is not allowed
	streamRecord := &types.StreamRecord{
		Account:     account.String(),
		Status:      types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate: sdkmath.NewInt(-100),
	}

	keeper.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{
		Timestamp: ctx.BlockTime().Unix() + 10,
		Addr:      account.String(),
	})

	deposit := sdkmath.NewInt(100)
	err := keeper.TryResumeStreamRecord(ctx, streamRecord, deposit)
	require.ErrorContains(t, err, "is resuming")
}

func TestTryResumeStreamRecord_ResumeInOneBlock(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// resume account in one call
	params := keeper.GetParams(ctx)
	rate := sdkmath.NewInt(100)
	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      1,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	gvg := sample.RandAccAddress()
	outFlow := &types.OutFlow{
		ToAddress: gvg.String(),
		Rate:      rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow)

	err := keeper.TryResumeStreamRecord(ctx, streamRecord, rate.MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err)

	userStreamRecord, _ := keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, userStreamRecord.NetflowRate, rate.Neg())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvgStreamRecord, _ := keeper.GetStreamRecord(ctx, gvg)
	require.True(t, gvgStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvgStreamRecord.NetflowRate, rate)
	require.Equal(t, gvgStreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())
}

func TestTryResumeStreamRecord_ResumeInMultipleBlocks(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// resume account in multiple blocks
	params := keeper.GetParams(ctx)
	params.MaxAutoResumeFlowCount = 1
	_ = keeper.SetParams(ctx, params)

	rate := sdkmath.NewInt(300)
	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      3,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	gvgAddress := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()}

	gvg1 := gvgAddress[0]
	gvg1Rate := sdkmath.NewInt(50)
	outFlow1 := &types.OutFlow{
		ToAddress: gvg1.String(),
		Rate:      gvg1Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow1)

	gvg2 := gvgAddress[1]
	gvg2Rate := sdkmath.NewInt(100)
	outFlow2 := &types.OutFlow{
		ToAddress: gvg2.String(),
		Rate:      gvg2Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow2)

	gvg3 := gvgAddress[2]
	gvg3Rate := sdkmath.NewInt(150)
	outFlow3 := &types.OutFlow{
		ToAddress: gvg3.String(),
		Rate:      gvg3Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow3)

	// try to resume stream record
	err := keeper.TryResumeStreamRecord(ctx, streamRecord, rate.SubRaw(10).MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err) // only added static balance
	found := keeper.ExistsAutoResumeRecord(ctx, ctx.BlockTime().Unix(), user)
	require.True(t, !found)
	streamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, streamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	err = keeper.TryResumeStreamRecord(ctx, streamRecord, rate.MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err)

	// still frozen
	userStreamRecord, _ := keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	_, found = keeper.GetAutoResumeRecord(ctx, ctx.BlockTime().Unix(), user)
	require.True(t, found)

	// resume in end block
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	// resume in end block
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	// resume in end block
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, userStreamRecord.NetflowRate, rate.Neg())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg1StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg1)
	require.True(t, gvg1StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg1StreamRecord.NetflowRate, gvg1Rate)
	require.Equal(t, gvg1StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg2StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg2)
	require.True(t, gvg2StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg2StreamRecord.NetflowRate, gvg2Rate)
	require.Equal(t, gvg2StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg3StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg3)
	require.True(t, gvg3StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg3StreamRecord.NetflowRate, gvg3Rate)
	require.Equal(t, gvg3StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())
}

func TestTryResumeStreamRecord_ResumeInMultipleBlocks_BalanceNotEnoughFinally(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// resume account in multiple blocks
	params := keeper.GetParams(ctx)
	params.MaxAutoResumeFlowCount = 1
	_ = keeper.SetParams(ctx, params)

	rate := sdkmath.NewInt(300)
	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      3,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	gvgAddress := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()}

	gvg1 := gvgAddress[0]
	gvg1Rate := sdkmath.NewInt(50)
	outFlow1 := &types.OutFlow{
		ToAddress: gvg1.String(),
		Rate:      gvg1Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow1)

	gvg2 := gvgAddress[1]
	gvg2Rate := sdkmath.NewInt(100)
	outFlow2 := &types.OutFlow{
		ToAddress: gvg2.String(),
		Rate:      gvg2Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow2)

	gvg3 := gvgAddress[2]
	gvg3Rate := sdkmath.NewInt(150)
	outFlow3 := &types.OutFlow{
		ToAddress: gvg3.String(),
		Rate:      gvg3Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow3)

	err := keeper.TryResumeStreamRecord(ctx, streamRecord, rate.MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err)

	// still frozen
	userStreamRecord, _ := keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	_, found := keeper.GetAutoResumeRecord(ctx, ctx.BlockTime().Unix(), user)
	require.True(t, found)

	// resume in end block
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	// resume in end block
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	// time flies
	timestamp := ctx.BlockTime().Unix() + int64(params.VersionedParams.ReserveTime)*2
	ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))

	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()
	depKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("fail to transfer")).AnyTimes()

	// resume in end block
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)

	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, userStreamRecord.NetflowRate.IsZero())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, rate.Neg())

	gvg1StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg1)
	require.True(t, gvg1StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, gvg1StreamRecord.NetflowRate.IsZero())
	require.Equal(t, gvg1StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg2StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg2)
	require.True(t, gvg2StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, gvg2StreamRecord.NetflowRate.IsZero())
	require.Equal(t, gvg2StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg3StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg3)
	require.True(t, gvg3StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, gvg3StreamRecord.NetflowRate.IsZero())
	require.Equal(t, gvg3StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	// there will be an auto settle record
	autoSettles := keeper.GetAllAutoSettleRecord(ctx)
	found = false
	for _, settle := range autoSettles {
		if settle.GetAddr() == user.String() {
			found = true
		}
	}
	require.True(t, found, "")

	keeper.AutoSettle(ctx)
	keeper.AutoSettle(ctx)
	keeper.AutoSettle(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.Equal(t, userStreamRecord.NetflowRate, sdkmath.ZeroInt())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, rate.Neg())
}

func TestAutoSettle_AccountIsInResuming(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// resume account in multiple blocks
	params := keeper.GetParams(ctx)
	params.MaxAutoResumeFlowCount = 1
	_ = keeper.SetParams(ctx, params)

	rate := sdkmath.NewInt(300)
	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      3,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	gvgAddress := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()}

	gvg1 := gvgAddress[0]
	gvg1Rate := sdkmath.NewInt(50)
	outFlow1 := &types.OutFlow{
		ToAddress: gvg1.String(),
		Rate:      gvg1Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow1)

	gvg2 := gvgAddress[1]
	gvg2Rate := sdkmath.NewInt(100)
	outFlow2 := &types.OutFlow{
		ToAddress: gvg2.String(),
		Rate:      gvg2Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow2)

	gvg3 := gvgAddress[2]
	gvg3Rate := sdkmath.NewInt(150)
	outFlow3 := &types.OutFlow{
		ToAddress: gvg3.String(),
		Rate:      gvg3Rate,
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow3)

	err := keeper.TryResumeStreamRecord(ctx, streamRecord, rate.MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err)

	// still frozen
	userStreamRecord, _ := keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	_, found := keeper.GetAutoResumeRecord(ctx, ctx.BlockTime().Unix(), user)
	require.True(t, found)

	// add auto settle record
	keeper.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix(),
		Addr:      user.String(),
	})

	keeper.AutoSettle(ctx)
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	keeper.AutoSettle(ctx)
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	keeper.AutoSettle(ctx)
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, userStreamRecord.NetflowRate, rate.Neg())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg1StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg1)
	require.True(t, gvg1StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg1StreamRecord.NetflowRate, gvg1Rate)
	require.Equal(t, gvg1StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg2StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg2)
	require.True(t, gvg2StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg2StreamRecord.NetflowRate, gvg2Rate)
	require.Equal(t, gvg2StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg3StreamRecord, _ := keeper.GetStreamRecord(ctx, gvg3)
	require.True(t, gvg3StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg3StreamRecord.NetflowRate, gvg3Rate)
	require.Equal(t, gvg3StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())
}

func TestAutoSettle_SettleInOneBlock(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()
	depKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("fail to transfer")).AnyTimes()

	// freeze account in one block
	rate := sdkmath.NewInt(100)
	user := sample.RandAccAddress()
	userStreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       rate.Neg(),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      1,
	}
	keeper.SetStreamRecord(ctx, userStreamRecord)

	gvg := sample.RandAccAddress()
	gvgStreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       rate,
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvgStreamRecord)

	outFlow := &types.OutFlow{
		ToAddress: gvg.String(),
		Rate:      rate,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	}
	keeper.SetOutFlow(ctx, user, outFlow)

	keeper.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix(),
		Addr:      user.String(),
	})

	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	keeper.AutoSettle(ctx)

	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.Equal(t, userStreamRecord.NetflowRate, sdkmath.ZeroInt())
	require.Equal(t, userStreamRecord.FrozenNetflowRate, rate.Neg())

	gvgOutFlow := keeper.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, gvg)
	require.Equal(t, gvgOutFlow.Status, types.OUT_FLOW_STATUS_FROZEN)
	require.Equal(t, gvgOutFlow.Rate, rate)
}

func TestAutoSettle_ForceSettleFreezesAllOutFlowsInOneBlock(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// A force-settled account must freeze all of its out-flows in the same block.
	params := keeper.GetParams(ctx)
	params.MaxAutoSettleFlowCount = 1
	_ = keeper.SetParams(ctx, params)

	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()
	depKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("fail to transfer")).AnyTimes()

	rate := sdkmath.NewInt(300)
	user := sample.RandAccAddress()
	userStreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       rate.Neg(),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      3,
	}
	keeper.SetStreamRecord(ctx, userStreamRecord)

	gvgAddress := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()}

	gvg1 := gvgAddress[0]
	gvg1StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg1.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(50),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg1StreamRecord)

	outFlow1 := &types.OutFlow{
		ToAddress: gvg1.String(),
		Rate:      sdkmath.NewInt(50),
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	}
	keeper.SetOutFlow(ctx, user, outFlow1)

	gvg2 := gvgAddress[1]
	gvg2StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg2.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(100),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg2StreamRecord)

	outFlow2 := &types.OutFlow{
		ToAddress: gvg2.String(),
		Rate:      sdkmath.NewInt(100),
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	}
	keeper.SetOutFlow(ctx, user, outFlow2)

	gvg3 := gvgAddress[2]
	gvg3StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg3.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(150),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg3StreamRecord)

	outFlow3 := &types.OutFlow{
		ToAddress: gvg3.String(),
		Rate:      sdkmath.NewInt(150),
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	}
	keeper.SetOutFlow(ctx, user, outFlow3)

	keeper.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix(),
		Addr:      user.String(),
	})

	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	keeper.AutoSettle(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, userStreamRecord.NetflowRate.IsZero())
	require.True(t, userStreamRecord.FrozenNetflowRate.Equal(rate.Neg()))

	gvg1OutFlow := keeper.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, gvg1)
	require.Equal(t, gvg1OutFlow.Status, types.OUT_FLOW_STATUS_FROZEN)
	require.Equal(t, gvg1OutFlow.Rate, sdkmath.NewInt(50))

	gvg2OutFlow := keeper.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, gvg2)
	require.Equal(t, gvg2OutFlow.Status, types.OUT_FLOW_STATUS_FROZEN)
	require.Equal(t, gvg2OutFlow.Rate, sdkmath.NewInt(100))

	gvg3OutFlow := keeper.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, gvg3)
	require.Equal(t, gvg3OutFlow.Status, types.OUT_FLOW_STATUS_FROZEN)
	require.Equal(t, gvg3OutFlow.Rate, sdkmath.NewInt(150))
}

func TestAutoSettle_SettleInMultipleBlocks_AutoResumeExists(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// freeze account in multiple blocks
	params := keeper.GetParams(ctx)
	params.MaxAutoSettleFlowCount = 1
	params.MaxAutoResumeFlowCount = 1
	_ = keeper.SetParams(ctx, params)

	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()
	depKeepers.BankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("fail to transfer")).AnyTimes()

	rate := sdkmath.NewInt(300)
	user := sample.RandAccAddress()
	userStreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      3,
	}
	keeper.SetStreamRecord(ctx, userStreamRecord)

	gvgAddress := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()}

	gvg1 := gvgAddress[0]
	gvg1StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg1.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg1StreamRecord)

	outFlow1 := &types.OutFlow{
		ToAddress: gvg1.String(),
		Rate:      sdkmath.NewInt(50),
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow1)

	gvg2 := gvgAddress[1]
	gvg2StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg2.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg2StreamRecord)

	outFlow2 := &types.OutFlow{
		ToAddress: gvg2.String(),
		Rate:      sdkmath.NewInt(100),
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow2)

	gvg3 := gvgAddress[2]
	gvg3StreamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg3.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: sdkmath.NewInt(0),
		OutFlowCount:      0,
	}
	keeper.SetStreamRecord(ctx, gvg3StreamRecord)

	outFlow3 := &types.OutFlow{
		ToAddress: gvg3.String(),
		Rate:      sdkmath.NewInt(150),
		Status:    types.OUT_FLOW_STATUS_FROZEN,
	}
	keeper.SetOutFlow(ctx, user, outFlow3)

	// resume the stream record
	err := keeper.TryResumeStreamRecord(ctx, userStreamRecord, rate.MulRaw(int64(params.VersionedParams.ReserveTime)))
	require.NoError(t, err) // only added static balance
	found := keeper.ExistsAutoResumeRecord(ctx, ctx.BlockTime().Unix(), user)
	require.True(t, found)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	// add auto settle record
	settleTime := ctx.BlockTime().Unix()
	keeper.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: settleTime,
		Addr:      user.String(),
	})

	keeper.AutoSettle(ctx) // this is for settle stream, it is counted
	keeper.AutoSettle(ctx)
	keeper.AutoSettle(ctx)
	keeper.AutoSettle(ctx)
	keeper.AutoSettle(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	keeper.AutoResume(ctx)
	keeper.AutoResume(ctx)
	keeper.AutoResume(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)

	timestamp := ctx.BlockTime().Unix()
	ctx = ctx.WithBlockTime(time.Unix(timestamp+10, 0))
	keeper.AutoSettle(ctx) // it will pick up the auto settle record
	autoSettles := keeper.GetAllAutoSettleRecord(ctx)
	var record types.AutoSettleRecord
	for _, settle := range autoSettles {
		if settle.GetAddr() == user.String() {
			record = settle
		}
	}
	// old settle record removed, new settle record added
	require.True(t, record.Timestamp != settleTime, "")

	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.True(t, userStreamRecord.NetflowRate.Equal(rate.Neg()))
	require.True(t, userStreamRecord.FrozenNetflowRate.IsZero())

	gvg1StreamRecord, _ = keeper.GetStreamRecord(ctx, gvg1)
	require.True(t, gvg1StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg1StreamRecord.NetflowRate, sdkmath.NewInt(50))
	require.Equal(t, gvg1StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg2StreamRecord, _ = keeper.GetStreamRecord(ctx, gvg2)
	require.True(t, gvg2StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg2StreamRecord.NetflowRate, sdkmath.NewInt(100))
	require.Equal(t, gvg2StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())

	gvg3StreamRecord, _ = keeper.GetStreamRecord(ctx, gvg3)
	require.True(t, gvg3StreamRecord.Status == types.STREAM_ACCOUNT_STATUS_ACTIVE)
	require.Equal(t, gvg3StreamRecord.NetflowRate, sdkmath.NewInt(150))
	require.Equal(t, gvg3StreamRecord.FrozenNetflowRate, sdkmath.ZeroInt())
}

func TestAutoResume_MultipleAccounts(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	params := keeper.GetParams(ctx)
	reserveTime := int64(params.VersionedParams.ReserveTime)
	now := ctx.BlockTime().Unix()

	// Set up several frozen accounts, each with its own frozen out-flows and an
	// auto-resume record queued for the current block. AutoResume opens a
	// frozen-flow iterator per account in its outer loop, so iterating multiple
	// accounts in a single call exercises the per-iteration iterator close.
	type account struct {
		addr      sdk.AccAddress
		outFlows  []*types.OutFlow
		totalRate sdkmath.Int
	}

	numAccounts := 3
	accounts := make([]account, 0, numAccounts)
	for i := 0; i < numAccounts; i++ {
		user := sample.RandAccAddress()

		gvg1 := sample.RandAccAddress()
		gvg2 := sample.RandAccAddress()
		rate1 := sdkmath.NewInt(int64(50 * (i + 1)))
		rate2 := sdkmath.NewInt(int64(70 * (i + 1)))
		totalRate := rate1.Add(rate2)

		outFlow1 := &types.OutFlow{ToAddress: gvg1.String(), Rate: rate1, Status: types.OUT_FLOW_STATUS_FROZEN}
		outFlow2 := &types.OutFlow{ToAddress: gvg2.String(), Rate: rate2, Status: types.OUT_FLOW_STATUS_FROZEN}
		keeper.SetOutFlow(ctx, user, outFlow1)
		keeper.SetOutFlow(ctx, user, outFlow2)

		// extra static balance beyond the buffer so the resumed account is not force-settled
		bufferBalance := totalRate.MulRaw(reserveTime)
		streamRecord := &types.StreamRecord{
			StaticBalance:     bufferBalance,
			BufferBalance:     bufferBalance,
			LockBalance:       sdkmath.ZeroInt(),
			Account:           user.String(),
			Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
			NetflowRate:       sdkmath.ZeroInt(),
			FrozenNetflowRate: totalRate.Neg(),
			OutFlowCount:      2,
			CrudTimestamp:     now,
		}
		keeper.SetStreamRecord(ctx, streamRecord)

		keeper.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{
			Timestamp: now,
			Addr:      user.String(),
		})

		accounts = append(accounts, account{
			addr:      user,
			outFlows:  []*types.OutFlow{outFlow1, outFlow2},
			totalRate: totalRate,
		})
	}

	// every account is queued for resume before the call
	for _, acc := range accounts {
		_, found := keeper.GetAutoResumeRecord(ctx, now, acc.addr)
		require.True(t, found)
	}

	// a single AutoResume call resumes every queued account
	keeper.AutoResume(ctx)

	for _, acc := range accounts {
		userStreamRecord, found := keeper.GetStreamRecord(ctx, acc.addr)
		require.True(t, found)
		require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, userStreamRecord.Status)
		require.Equal(t, acc.totalRate.Neg(), userStreamRecord.NetflowRate)
		require.Equal(t, sdkmath.ZeroInt(), userStreamRecord.FrozenNetflowRate)

		// the queued resume record has been consumed
		_, found = keeper.GetAutoResumeRecord(ctx, now, acc.addr)
		require.False(t, found)

		for _, outFlow := range acc.outFlows {
			toAddr := sdk.MustAccAddressFromHex(outFlow.ToAddress)

			// the out-flow is now active and no frozen out-flow remains
			activeOutFlow := keeper.GetOutFlow(ctx, acc.addr, types.OUT_FLOW_STATUS_ACTIVE, toAddr)
			require.NotNil(t, activeOutFlow)
			require.Equal(t, types.OUT_FLOW_STATUS_ACTIVE, activeOutFlow.Status)
			require.Equal(t, outFlow.Rate, activeOutFlow.Rate)
			require.Nil(t, keeper.GetOutFlow(ctx, acc.addr, types.OUT_FLOW_STATUS_FROZEN, toAddr))

			// the receiver stream record is active with the restored rate
			gvgStreamRecord, found := keeper.GetStreamRecord(ctx, toAddr)
			require.True(t, found)
			require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, gvgStreamRecord.Status)
			require.Equal(t, outFlow.Rate, gvgStreamRecord.NetflowRate)
			require.Equal(t, sdkmath.ZeroInt(), gvgStreamRecord.FrozenNetflowRate)
		}
	}
}

func TestUpdateStreamRecord_FrozenAccountLockBalance(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.NewInt(1000),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.NewInt(0),
		FrozenNetflowRate: sdkmath.NewInt(100).Neg(),
		OutFlowCount:      1,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	// update fail when no force flag
	change := types.NewDefaultStreamRecordChangeWithAddr(user).
		WithLockBalanceChange(streamRecord.LockBalance.Neg())
	_, err := keeper.UpdateStreamRecordByAddr(ctx, change)
	require.ErrorContains(t, err, "is frozen")

	// update success when there is force flag
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	change = types.NewDefaultStreamRecordChangeWithAddr(user).
		WithLockBalanceChange(streamRecord.LockBalance.Neg())
	_, err = keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)

	streamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, streamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, streamRecord.LockBalance.IsZero())
	require.True(t, streamRecord.StaticBalance.Int64() == 1000)
}

func TestSettleStreamRecord(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	user := sample.RandAccAddress()
	rate := sdkmath.NewInt(-100)
	staticBalance := sdkmath.NewInt(1e10)
	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithRateChange(rate).WithStaticBalanceChange(staticBalance)
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
	_, err := keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)
	// check
	streamRecord, found := keeper.GetStreamRecord(ctx, user)
	require.True(t, found)
	t.Logf("stream record: %+v", streamRecord)
	// 345 seconds pass
	var seconds int64 = 345
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Duration(seconds) * time.Second))
	change = types.NewDefaultStreamRecordChangeWithAddr(user)
	_, err = keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)
	userStreamRecord2, _ := keeper.GetStreamRecord(ctx, user)
	t.Logf("stream record after %d seconds: %+v", seconds, userStreamRecord2)
	require.Equal(t, userStreamRecord2.StaticBalance, streamRecord.StaticBalance.Add(rate.Mul(sdkmath.NewInt(seconds))))
	require.Equal(t, userStreamRecord2.BufferBalance, streamRecord.BufferBalance)
	require.Equal(t, userStreamRecord2.NetflowRate, streamRecord.NetflowRate)
	require.Equal(t, userStreamRecord2.CrudTimestamp, streamRecord.CrudTimestamp+seconds)
}

// TestUpdateStreamRecord_SettleTimestampOverflow_ForcedSaturates reproduces the
// original chain-halt: a forced EndBlocker update (ForceDeleteObject during
// discontinue) decreases the netflow rate on a large-balance account, pushing
// payDuration past MaxInt64. The EndBlocker cannot surface an error (abci.go
// panics on any error from DeleteDiscontinueObjectsUntil), so the settle
// timestamp must saturate to MaxInt64 instead of overflowing.
func TestUpdateStreamRecord_SettleTimestampOverflow_ForcedSaturates(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)

	params := k.GetParams(ctx)
	// Buffer reserved for the starting rate (-2); after the rate drops to -1 the
	// buffer recomputes to 1×ReserveTime. payDuration overflows either way.
	bufferBalance := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime).MulRaw(2)

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewIntFromUint64(math.MaxUint64),
		BufferBalance:     bufferBalance,
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-2),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	// Forced rate decrease (-2 → -1), as when an object is force-deleted.
	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithRateChange(sdkmath.NewInt(1))
	require.NotPanics(t, func() {
		err := k.UpdateStreamRecord(ctx, sr, change)
		require.NoError(t, err, "forced path must not return an error (would panic EndBlocker)")
	})
	require.Equal(t, int64(math.MaxInt64), sr.SettleTimestamp,
		"forced overflow must saturate to MaxInt64, not panic")
}

// TestUpdateStreamRecord_SettleTimestampUnderflow_ForcedSaturates covers the low
// side: a forced update on a deeply indebted active account (a large rate that
// collapsed to a tiny one while the balance was negative) makes payDuration
// hugely negative, so currentTimestamp - forcedSettleTime + payDuration underflows
// int64. Int64() would panic inside the EndBlocker and halt the chain; instead the
// settle timestamp must saturate to MinInt64. This is reachable only when forced —
// a non-forced update returns early once payDuration drops below ForcedSettleTime.
func TestUpdateStreamRecord_SettleTimestampUnderflow_ForcedSaturates(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1_000_000_000, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)

	// No bank account, so the negative-balance auto-transfer (for the user and for
	// the governance account inside ForceSettle) is skipped.
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	params := k.GetParams(ctx)
	// Buffer matches |rate|=1 so the buffer recompute does not adjust staticBalance.
	bufferBalance := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime)
	// staticBalance far below MinInt64 → payDuration (÷1) underflows int64.
	staticBalance := sdkmath.NewInt(math.MinInt64).MulRaw(2)

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     staticBalance,
		BufferBalance:     bufferBalance,
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	change := types.NewDefaultStreamRecordChangeWithAddr(user)
	require.NotPanics(t, func() {
		err := k.UpdateStreamRecord(ctx, sr, change)
		require.NoError(t, err, "forced path must not return an error (would panic EndBlocker)")
	})
	require.Equal(t, int64(math.MinInt64), sr.SettleTimestamp,
		"forced underflow must saturate to MinInt64, not panic")
}

// TestUpdateStreamRecord_SettleTimestampOverflow_UserDepositRejected covers the
// user-initiated deposit path: a positive static-balance change with no rate
// change that pushes payDuration past MaxInt64 is rejected with
// ErrSettleTimestampOverflow, so the depositor can simply deposit less. This is
// the one path where the degenerate state is genuinely user-created.
func TestUpdateStreamRecord_SettleTimestampOverflow_UserDepositRejected(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	// No ForceUpdateStreamRecordKey → forced=false (user-initiated path).

	params := k.GetParams(ctx)
	bufferBalance := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime)

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewIntFromUint64(math.MaxUint64),
		BufferBalance:     bufferBalance,
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	// A real deposit: positive static-balance change, no rate change.
	change := types.NewDefaultStreamRecordChangeWithAddr(user).
		WithStaticBalanceChange(sdkmath.NewInt(1_000))
	err := k.UpdateStreamRecord(ctx, sr, change)
	require.ErrorIs(t, err, types.ErrSettleTimestampOverflow,
		"a deposit that overflows the settle timestamp must be rejected")
}

// TestUpdateStreamRecord_SettleTimestampOverflow_RateDecreaseSaturates covers a
// non-forced rate decrease — a user MsgDeleteObject, or lazy re-pricing when the
// SP price falls. Removing rate increases payDuration and can overflow int64, but
// this is a legitimate action (not over-funding), so it must NOT be rejected: the
// settle timestamp saturates to MaxInt64 and the operation succeeds. This is the
// asymmetry B fixes — a user can always delete their own object.
func TestUpdateStreamRecord_SettleTimestampOverflow_RateDecreaseSaturates(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	// No ForceUpdateStreamRecordKey → forced=false (user-initiated path).

	params := k.GetParams(ctx)
	bufferBalance := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime).MulRaw(2)

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewIntFromUint64(math.MaxUint64),
		BufferBalance:     bufferBalance,
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-2),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	// Rate decrease (-2 → -1), as when a user deletes an object. No static-balance
	// change, so this is not a user deposit and must not be rejected.
	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithRateChange(sdkmath.NewInt(1))
	require.NotPanics(t, func() {
		err := k.UpdateStreamRecord(ctx, sr, change)
		require.NoError(t, err, "a legitimate rate decrease must not be rejected")
	})
	require.Equal(t, int64(math.MaxInt64), sr.SettleTimestamp,
		"rate-decrease overflow must saturate to MaxInt64, not reject")
}

// TestTryResumeStreamRecord_SettleTimestampInt64Overflow covers the Deposit
// path to a frozen account. TryResumeStreamRecord is only reachable from
// MsgDeposit (never from an EndBlocker), so an overflow must be rejected with
// ErrSettleTimestampOverflow; the user should reduce their deposit amount.
func TestTryResumeStreamRecord_SettleTimestampInt64Overflow(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.NewInt(-1),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, sr)

	deposit := sdkmath.NewIntFromUint64(math.MaxUint64)
	err := k.TryResumeStreamRecord(ctx, sr, deposit)
	require.ErrorIs(t, err, types.ErrSettleTimestampOverflow,
		"deposit that overflows settle timestamp must be rejected, not silently absorbed")
}

// TestUpdateStreamRecord_SettleTimestampSilentWrap catches the secondary overflow:
// even when payDuration < MaxInt64 (so Int64() would not have panicked), the full
// expression currentTimestamp - forcedSettleTime + payDuration can itself overflow
// int64 and silently wrap to a large negative value. A negative settle timestamp
// makes the account look overdue and drives repeated force-settle attempts.
func TestUpdateStreamRecord_SettleTimestampSilentWrap(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	params := k.GetParams(ctx)
	reserveTime := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime)

	timestamp := int64(1_000_000_000)
	ctx = ctx.WithBlockTime(time.Unix(timestamp, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)

	// payDuration = MaxInt64 - 500 fits in int64, so Int64() would not panic
	// pre-fix. But timestamp + payDuration - forcedSettleTime overflows int64.
	payDuration := sdkmath.NewInt(math.MaxInt64).SubRaw(500)
	// With |rate|=1 and bufferBal=reserveTime: staticBal = payDuration - reserveTime.
	staticBal := payDuration.Sub(reserveTime)
	bufferBal := reserveTime

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     staticBal,
		BufferBalance:     bufferBal,
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     timestamp,
	}

	change := types.NewDefaultStreamRecordChangeWithAddr(user)
	require.NotPanics(t, func() {
		err := k.UpdateStreamRecord(ctx, sr, change)
		require.NoError(t, err)
	})
	require.Equal(t, int64(math.MaxInt64), sr.SettleTimestamp,
		"settle timestamp must saturate to MaxInt64 when full expression overflows int64")
	require.Greater(t, sr.SettleTimestamp, int64(0),
		"settle timestamp must not silently wrap to negative")
}

func TestAutoForceSettle(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	t.Logf("depKeepers: %+v", depKeepers)
	params := keeper.GetParams(ctx)
	var startTime int64 = 100
	ctx = ctx.WithBlockTime(time.Unix(startTime, 0))
	user := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)
	sp := sample.RandAccAddress()
	userInitBalance := sdkmath.NewInt(int64(100*params.VersionedParams.ReserveTime) + 1) // just enough for reserve
	// init balance
	streamRecordChanges := []types.StreamRecordChange{
		*types.NewDefaultStreamRecordChangeWithAddr(user).WithStaticBalanceChange(userInitBalance),
	}
	err := keeper.ApplyStreamRecordChanges(ctx, streamRecordChanges)
	require.NoError(t, err)
	userStreamRecord, found := keeper.GetStreamRecord(ctx, user)
	t.Logf("user stream record: %+v", userStreamRecord)
	require.True(t, found)
	flowChanges := []types.OutFlow{
		{ToAddress: sp.String(), Rate: rate},
	}
	userFlows := types.UserFlows{Flows: flowChanges, From: user}
	err = keeper.ApplyUserFlowsList(ctx, []types.UserFlows{userFlows})
	require.NoError(t, err)
	userStreamRecord, found = keeper.GetStreamRecord(ctx, user)
	t.Logf("user stream record: %+v", userStreamRecord)
	require.True(t, found)
	outFlows := keeper.GetOutFlows(ctx, user)
	require.Equal(t, 1, len(outFlows))
	require.Equal(t, outFlows[0].ToAddress, sp.String())
	spStreamRecord, found := keeper.GetStreamRecord(ctx, sp)
	t.Logf("sp stream record: %+v", spStreamRecord)
	require.True(t, found)
	require.Equal(t, spStreamRecord.NetflowRate, rate)
	require.Equal(t, spStreamRecord.StaticBalance, sdkmath.ZeroInt())
	require.Equal(t, spStreamRecord.BufferBalance, sdkmath.ZeroInt())
	// check auto settle queue
	autoSettleQueue := keeper.GetAllAutoSettleRecord(ctx)
	t.Logf("auto settle queue: %+v", autoSettleQueue)
	require.Equal(t, len(autoSettleQueue), 1)
	require.Equal(t, autoSettleQueue[0].Addr, user.String())
	require.Equal(t, autoSettleQueue[0].Timestamp, startTime+int64(params.VersionedParams.ReserveTime)-int64(params.ForcedSettleTime))
	// 1 day pass
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Duration(86400) * time.Second))
	// update and deposit to user for extra 100s
	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	userAddBalance := rate.MulRaw(100)
	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithStaticBalanceChange(userAddBalance)
	ret, err := keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)
	userStreamRecord = ret
	t.Logf("user stream record: %+v", userStreamRecord)
	require.True(t, found)
	require.True(t, userStreamRecord.StaticBalance.IsNegative())
	change = types.NewDefaultStreamRecordChangeWithAddr(sp)
	_, err = keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)
	spStreamRecord, _ = keeper.GetStreamRecord(ctx, sp)
	t.Logf("sp stream record: %+v", spStreamRecord)
	autoSettleQueue2 := keeper.GetAllAutoSettleRecord(ctx)
	t.Logf("auto settle queue: %+v", autoSettleQueue2)
	require.Equal(t, autoSettleQueue[0].Timestamp+100, autoSettleQueue2[0].Timestamp)
	// reserve time - forced settle time - 1 day + 101s pass
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Duration(params.VersionedParams.ReserveTime-params.ForcedSettleTime-86400+101) * time.Second))
	usrBeforeForceSettle, _ := keeper.GetStreamRecord(ctx, user)
	t.Logf("usrBeforeForceSettle: %s", usrBeforeForceSettle)

	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	time.Sleep(1 * time.Second)
	keeper.AutoSettle(ctx)

	usrAfterForceSettle, found := keeper.GetStreamRecord(ctx, user)
	require.True(t, found)
	t.Logf("usrAfterForceSettle: %s", usrAfterForceSettle)
	// user has been force settled
	require.Equal(t, usrAfterForceSettle.StaticBalance, sdkmath.ZeroInt())
	require.Equal(t, usrAfterForceSettle.BufferBalance, sdkmath.ZeroInt())
	require.Equal(t, usrAfterForceSettle.NetflowRate, sdkmath.ZeroInt())
	require.Equal(t, usrAfterForceSettle.Status, types.STREAM_ACCOUNT_STATUS_FROZEN)
	change = types.NewDefaultStreamRecordChangeWithAddr(sp)
	_, err = keeper.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)
	spStreamRecord, _ = keeper.GetStreamRecord(ctx, sp)
	t.Logf("sp stream record: %+v", spStreamRecord)
	autoSettleQueue3 := keeper.GetAllAutoSettleRecord(ctx)
	t.Logf("auto settle queue: %+v", autoSettleQueue3)
	require.Equal(t, len(autoSettleQueue3), 0)
	govStreamRecord, found := keeper.GetStreamRecord(ctx, types.GovernanceAddress)
	require.True(t, found)
	t.Logf("gov stream record: %+v", govStreamRecord)
	require.Equal(t, govStreamRecord.StaticBalance.Add(spStreamRecord.StaticBalance), userInitBalance.Add(userAddBalance))
}

func TestForceSettle_FreezesAllActiveOutFlows(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	payer := sample.RandAccAddress()
	recipient1 := sample.RandAccAddress()
	recipient2 := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)

	payerRecord := types.NewStreamRecord(payer, ctx.BlockTime().Unix())
	payerRecord.NetflowRate = rate.MulRaw(-2)
	payerRecord.OutFlowCount = 2
	keeper.SetStreamRecord(ctx, payerRecord)

	governanceRecord := types.NewStreamRecord(types.GovernanceAddress, ctx.BlockTime().Unix())
	keeper.SetStreamRecord(ctx, governanceRecord)

	for _, recipient := range []sdk.AccAddress{recipient1, recipient2} {
		recipientRecord := types.NewStreamRecord(recipient, ctx.BlockTime().Unix())
		recipientRecord.NetflowRate = rate
		keeper.SetStreamRecord(ctx, recipientRecord)
		keeper.SetOutFlow(ctx, payer, &types.OutFlow{
			ToAddress: recipient.String(),
			Rate:      rate,
			Status:    types.OUT_FLOW_STATUS_ACTIVE,
		})
	}

	_, err := keeper.UpdateStreamRecordByAddr(ctx, types.NewDefaultStreamRecordChangeWithAddr(payer))
	require.NoError(t, err)
	payerRecord, _ = keeper.GetStreamRecord(ctx, payer)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, payerRecord.Status)
	require.True(t, payerRecord.NetflowRate.IsZero())
	require.Equal(t, rate.MulRaw(-2), payerRecord.FrozenNetflowRate)

	for _, recipient := range []sdk.AccAddress{recipient1, recipient2} {
		require.Nil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_ACTIVE, recipient))
		require.NotNil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_FROZEN, recipient))

		recipientRecord, _ := keeper.GetStreamRecord(ctx, recipient)
		require.True(t, recipientRecord.NetflowRate.IsZero())
	}
}

func TestForceSettle_SelfOutFlowDoesNotReenter(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	payer := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)
	payerRecord := types.NewStreamRecord(payer, ctx.BlockTime().Unix())
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1
	keeper.SetStreamRecord(ctx, payerRecord)

	governanceRecord := types.NewStreamRecord(types.GovernanceAddress, ctx.BlockTime().Unix())
	keeper.SetStreamRecord(ctx, governanceRecord)
	keeper.SetOutFlow(ctx, payer, &types.OutFlow{
		ToAddress: payer.String(),
		Rate:      rate,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})

	_, err := keeper.UpdateStreamRecordByAddr(ctx, types.NewDefaultStreamRecordChangeWithAddr(payer))
	require.NoError(t, err)
	payerRecord, _ = keeper.GetStreamRecord(ctx, payer)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, payerRecord.Status)
	require.True(t, payerRecord.NetflowRate.IsZero())
	require.NotNil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_FROZEN, payer))
}

// Freezing a payer removes its rate from each recipient, which can push a
// recipient that was only solvent on that inflow into a force settle of its
// own. That second freeze must cascade to the recipient's own out-flows.
func TestForceSettle_FreezesOutFlowsAcrossCascade(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	payer := sample.RandAccAddress()
	intermediary := sample.RandAccAddress()
	leaf := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)

	governanceRecord := types.NewStreamRecord(types.GovernanceAddress, ctx.BlockTime().Unix())
	keeper.SetStreamRecord(ctx, governanceRecord)

	payerRecord := types.NewStreamRecord(payer, ctx.BlockTime().Unix())
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1
	keeper.SetStreamRecord(ctx, payerRecord)
	keeper.SetOutFlow(ctx, payer, &types.OutFlow{
		ToAddress: intermediary.String(),
		Rate:      rate,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})

	// Solvent only while the payer's inflow covers its own out-flow.
	intermediaryRecord := types.NewStreamRecord(intermediary, ctx.BlockTime().Unix())
	intermediaryRecord.NetflowRate = sdkmath.ZeroInt()
	intermediaryRecord.OutFlowCount = 1
	keeper.SetStreamRecord(ctx, intermediaryRecord)
	keeper.SetOutFlow(ctx, intermediary, &types.OutFlow{
		ToAddress: leaf.String(),
		Rate:      rate,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})

	leafRecord := types.NewStreamRecord(leaf, ctx.BlockTime().Unix())
	leafRecord.NetflowRate = rate
	keeper.SetStreamRecord(ctx, leafRecord)

	_, err := keeper.UpdateStreamRecordByAddr(ctx, types.NewDefaultStreamRecordChangeWithAddr(payer))
	require.NoError(t, err)

	payerRecord, _ = keeper.GetStreamRecord(ctx, payer)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, payerRecord.Status)
	require.True(t, payerRecord.NetflowRate.IsZero())
	require.Equal(t, rate.Neg(), payerRecord.FrozenNetflowRate)
	require.Nil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_ACTIVE, intermediary))
	require.NotNil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_FROZEN, intermediary))

	intermediaryRecord, _ = keeper.GetStreamRecord(ctx, intermediary)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, intermediaryRecord.Status)
	require.True(t, intermediaryRecord.NetflowRate.IsZero())
	require.Equal(t, rate.Neg(), intermediaryRecord.FrozenNetflowRate)
	require.Nil(t, keeper.GetOutFlow(ctx, intermediary, types.OUT_FLOW_STATUS_ACTIVE, leaf))
	require.NotNil(t, keeper.GetOutFlow(ctx, intermediary, types.OUT_FLOW_STATUS_FROZEN, leaf))

	// The leaf keeps no stale rate from the frozen intermediary.
	leafRecord, _ = keeper.GetStreamRecord(ctx, leaf)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, leafRecord.Status)
	require.True(t, leafRecord.NetflowRate.IsZero())
}

// A cascade that loops back to the payer updates the payer's record from a
// nested call, so the freeze must apply its rate delta to that stored state
// rather than to a copy read before the cascade ran.
func TestForceSettle_FreezesOutFlowsAcrossCycle(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0))
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)
	depKeepers.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	payer := sample.RandAccAddress()
	peer := sample.RandAccAddress()
	out := sdkmath.NewInt(100)
	back := sdkmath.NewInt(50)

	governanceRecord := types.NewStreamRecord(types.GovernanceAddress, ctx.BlockTime().Unix())
	keeper.SetStreamRecord(ctx, governanceRecord)

	payerRecord := types.NewStreamRecord(payer, ctx.BlockTime().Unix())
	payerRecord.NetflowRate = back.Sub(out)
	payerRecord.OutFlowCount = 1
	keeper.SetStreamRecord(ctx, payerRecord)
	keeper.SetOutFlow(ctx, payer, &types.OutFlow{
		ToAddress: peer.String(),
		Rate:      out,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})

	peerRecord := types.NewStreamRecord(peer, ctx.BlockTime().Unix())
	peerRecord.NetflowRate = out.Sub(back)
	peerRecord.OutFlowCount = 1
	keeper.SetStreamRecord(ctx, peerRecord)
	keeper.SetOutFlow(ctx, peer, &types.OutFlow{
		ToAddress: payer.String(),
		Rate:      back,
		Status:    types.OUT_FLOW_STATUS_ACTIVE,
	})

	_, err := keeper.UpdateStreamRecordByAddr(ctx, types.NewDefaultStreamRecordChangeWithAddr(payer))
	require.NoError(t, err)

	// Both flows end frozen, so neither account may retain a live rate.
	payerRecord, _ = keeper.GetStreamRecord(ctx, payer)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, payerRecord.Status)
	require.True(t, payerRecord.NetflowRate.IsZero(),
		"payer retains a rate with no active out-flow: %s", payerRecord.NetflowRate)
	require.Equal(t, out.Neg(), payerRecord.FrozenNetflowRate)

	peerRecord, _ = keeper.GetStreamRecord(ctx, peer)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, peerRecord.Status)
	require.True(t, peerRecord.NetflowRate.IsZero(),
		"peer retains a rate with no active out-flow: %s", peerRecord.NetflowRate)
	require.Equal(t, back.Neg(), peerRecord.FrozenNetflowRate)

	require.Nil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_ACTIVE, peer))
	require.NotNil(t, keeper.GetOutFlow(ctx, payer, types.OUT_FLOW_STATUS_FROZEN, peer))
	require.Nil(t, keeper.GetOutFlow(ctx, peer, types.OUT_FLOW_STATUS_ACTIVE, payer))
	require.NotNil(t, keeper.GetOutFlow(ctx, peer, types.OUT_FLOW_STATUS_FROZEN, payer))
}

// TestAutoResume_MultiBatch_ChargesOnlyRealActiveWindow reproduces MOCA-1073.
//
// When an account's auto-resume spans multiple blocks (OutFlowCount exceeds
// MaxAutoResumeFlowCount), AutoResume raises NetflowRate one batch per block but
// leaves CrudTimestamp pinned at resume-start until the final batch flips the
// account back to ACTIVE and settles. That final settlement debits the
// fully-restored rate across the whole elapsed window, charging the payer for
// time during which most flows were not yet active. The excess is destroyed:
// debited from the payer but credited to no recipient.
//
// The existing multi-block tests never advance block time between AutoResume
// calls, so the stale window is zero and the bug is invisible. This test
// advances time one block per batch, the way a real chain does.
func TestAutoResume_MultiBatch_ChargesOnlyRealActiveWindow(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)

	params := keeper.GetParams(ctx)
	params.MaxAutoResumeFlowCount = 1 // restore one frozen flow per block
	_ = keeper.SetParams(ctx, params)
	reserveTime := sdkmath.NewIntFromUint64(params.VersionedParams.ReserveTime)

	const (
		flowRate    = int64(100)
		numFlows    = 3
		blockStride = int64(100) // seconds between resume batches
	)
	fullRate := sdkmath.NewInt(flowRate * numFlows) // -(-300) magnitude

	t0 := int64(1_000_000_000)
	ctx = ctx.WithBlockTime(time.Unix(t0, 0))

	user := sample.RandAccAddress()
	// Extra static beyond the reserved buffer so the account stays solvent and no
	// force-settle noise obscures the measurement.
	extraStatic := sdkmath.NewInt(1_000_000_000)

	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: fullRate.Neg(),
		OutFlowCount:      numFlows,
		CrudTimestamp:     t0,
	}
	keeper.SetStreamRecord(ctx, streamRecord)

	recipients := make([]sdk.AccAddress, numFlows)
	for i := 0; i < numFlows; i++ {
		recipients[i] = sample.RandAccAddress()
		keeper.SetOutFlow(ctx, user, &types.OutFlow{
			ToAddress: recipients[i].String(),
			Rate:      sdkmath.NewInt(flowRate),
			Status:    types.OUT_FLOW_STATUS_FROZEN,
		})
	}

	// Deposit exactly the reserve buffer plus the extra static.
	deposit := fullRate.Mul(reserveTime).Add(extraStatic)
	err := keeper.TryResumeStreamRecord(ctx, streamRecord, deposit)
	require.NoError(t, err)

	sr, _ := keeper.GetStreamRecord(ctx, user)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, sr.Status,
		"account must enqueue for multi-batch resume, not resume directly")
	require.Equal(t, extraStatic, sr.StaticBalance,
		"after reserving the buffer, static balance is the extra deposit")
	staticAtResumeStart := sr.StaticBalance

	// Drive one resume batch per block, advancing block time each block.
	var tn int64
	for b := 1; b <= numFlows; b++ {
		tn = t0 + blockStride*int64(b)
		ctx = ctx.WithBlockTime(time.Unix(tn, 0))
		keeper.AutoResume(ctx)
	}

	sr, _ = keeper.GetStreamRecord(ctx, user)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, sr.Status)
	require.Equal(t, fullRate.Neg(), sr.NetflowRate)
	require.True(t, sr.FrozenNetflowRate.IsZero())

	// Correct charge: flow restored at block (t0 + stride*b) is active only over
	// [restore_b, tn]. Equal rates make the total order-independent.
	correctCharge := int64(0)
	for b := 1; b <= numFlows; b++ {
		restore := t0 + blockStride*int64(b)
		correctCharge += flowRate * (tn - restore)
	}
	correctStatic := staticAtResumeStart.SubRaw(correctCharge)

	buggyCharge := fullRate.Int64() * (tn - t0) // full rate across the whole window
	destroyed := buggyCharge - correctCharge

	t.Logf("resume-start t0=%d  final settle tn=%d  window=%ds", t0, tn, tn-t0)
	t.Logf("static at resume-start            : %s", staticAtResumeStart)
	t.Logf("correct charge (per-flow window)  : %d  -> correct static %s", correctCharge, correctStatic)
	t.Logf("buggy charge (fullRate x window)  : %d -> buggy static  %s", buggyCharge, staticAtResumeStart.SubRaw(buggyCharge))
	t.Logf("payer static (actual)             : %s", sr.StaticBalance)
	t.Logf("funds destroyed (debited, credited to nobody): %d", destroyed)

	// Recipients only ever accrue from the block their own flow was restored, so
	// the total credited across recipients equals the correct charge — never the
	// buggy full-window charge. This holds before and after the fix and is the
	// proof the excess debit is destroyed rather than redistributed.
	creditedToRecipients := int64(0)
	for _, r := range recipients {
		_, err := keeper.UpdateStreamRecordByAddr(ctx, types.NewDefaultStreamRecordChangeWithAddr(r))
		require.NoError(t, err)
		rr, _ := keeper.GetStreamRecord(ctx, r)
		creditedToRecipients += rr.StaticBalance.Int64()
	}
	require.Equal(t, correctCharge, creditedToRecipients,
		"recipients are credited only for their real active window")

	// Conservation: everything the payer holds (static + reserved buffer) plus
	// everything the recipients were credited must equal the deposit that entered
	// the system. The buggy path leaves this short by exactly `destroyed`.
	systemHeld := sr.StaticBalance.Add(sr.BufferBalance).AddRaw(creditedToRecipients)
	require.Equal(t, deposit, systemHeld,
		"payment accounting must conserve the deposit; short by %d amoca (destroyed)", destroyed)

	// The regression assertion: the payer must be charged only for the windows
	// each flow was actually active.
	require.Equal(t, correctStatic, sr.StaticBalance,
		"payer overcharged by %d amoca over the stale resume window (funds destroyed)", destroyed)
}

// TestCheckStreamRecord_PanicsOnInvalidRecord drives every defensive panic
// branch in CheckStreamRecord. SetStreamRecord already exercises the "valid
// record" (false) side of each check on every successful call throughout this
// file; this covers the panic (true) side, one invalid field at a time.
func TestCheckStreamRecord_PanicsOnInvalidRecord(t *testing.T) {
	k, _, _ := makePaymentKeeper(t)

	valid := func() *types.StreamRecord {
		return &types.StreamRecord{
			Account:           sample.RandAccAddress().String(),
			Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
			StaticBalance:     sdkmath.ZeroInt(),
			NetflowRate:       sdkmath.ZeroInt(),
			FrozenNetflowRate: sdkmath.ZeroInt(),
			LockBalance:       sdkmath.ZeroInt(),
			BufferBalance:     sdkmath.ZeroInt(),
			OutFlowCount:      0,
		}
	}
	// sanity check: the base fixture itself must not panic
	require.NotPanics(t, func() { k.CheckStreamRecord(valid()) })

	tests := []struct {
		name    string
		mutate  func(*types.StreamRecord)
		wantMsg string
	}{
		{"wrong length account", func(r *types.StreamRecord) { r.Account = "0x1234" }, "invalid streamRecord account"},
		{"invalid status", func(r *types.StreamRecord) { r.Status = 99 }, "invalid streamRecord status"},
		{"nil static balance", func(r *types.StreamRecord) { r.StaticBalance = sdkmath.Int{} }, "invalid streamRecord staticBalance"},
		{"nil netflow rate", func(r *types.StreamRecord) { r.NetflowRate = sdkmath.Int{} }, "invalid streamRecord netflowRate"},
		{"positive frozen netflow rate", func(r *types.StreamRecord) { r.FrozenNetflowRate = sdkmath.NewInt(1) }, "invalid streamRecord frozenNetflowRate"},
		{"nil lock balance", func(r *types.StreamRecord) { r.LockBalance = sdkmath.Int{} }, "invalid streamRecord lockBalance"},
		{"negative lock balance", func(r *types.StreamRecord) { r.LockBalance = sdkmath.NewInt(-1) }, "invalid streamRecord lockBalance"},
		{"nil buffer balance", func(r *types.StreamRecord) { r.BufferBalance = sdkmath.Int{} }, "invalid streamRecord bufferBalance"},
		{"negative buffer balance", func(r *types.StreamRecord) { r.BufferBalance = sdkmath.NewInt(-1) }, "invalid streamRecord bufferBalance"},
		{"negative rate with zero out-flow count", func(r *types.StreamRecord) {
			r.NetflowRate = sdkmath.NewInt(-1)
			r.OutFlowCount = 0
		}, "when outFlowCount is zero"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := valid()
			tt.mutate(record)
			defer func() {
				r := recover()
				require.NotNil(t, r)
				require.Contains(t, fmt.Sprintf("%v", r), tt.wantMsg)
			}()
			k.CheckStreamRecord(record)
		})
	}

	require.PanicsWithValue(t, "streamRecord is nil", func() { k.CheckStreamRecord(nil) })
}

func TestIsEmptyNetFlow(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)

	// not found -> treated as empty
	require.True(t, k.IsEmptyNetFlow(ctx, sample.RandAccAddress()))

	// found with zero rates -> empty
	zeroAddr := sample.RandAccAddress()
	k.SetStreamRecord(ctx, types.NewStreamRecord(zeroAddr, ctx.BlockTime().Unix()))
	require.True(t, k.IsEmptyNetFlow(ctx, zeroAddr))

	// found with a non-zero netflow rate -> not empty
	activeAddr := sample.RandAccAddress()
	activeRecord := types.NewStreamRecord(activeAddr, ctx.BlockTime().Unix())
	activeRecord.NetflowRate = sdkmath.NewInt(100)
	activeRecord.OutFlowCount = 1
	k.SetStreamRecord(ctx, activeRecord)
	require.False(t, k.IsEmptyNetFlow(ctx, activeAddr))

	// found with only a non-zero frozen netflow rate -> not empty
	frozenAddr := sample.RandAccAddress()
	frozenRecord := types.NewStreamRecord(frozenAddr, ctx.BlockTime().Unix())
	frozenRecord.Status = types.STREAM_ACCOUNT_STATUS_FROZEN
	frozenRecord.FrozenNetflowRate = sdkmath.NewInt(-50)
	k.SetStreamRecord(ctx, frozenRecord)
	require.False(t, k.IsEmptyNetFlow(ctx, frozenAddr))
}

func TestGetAllStreamRecord(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)

	require.Empty(t, k.GetAllStreamRecord(ctx))

	addr1 := sample.RandAccAddress()
	addr2 := sample.RandAccAddress()
	k.SetStreamRecord(ctx, types.NewStreamRecord(addr1, ctx.BlockTime().Unix()))
	k.SetStreamRecord(ctx, types.NewStreamRecord(addr2, ctx.BlockTime().Unix()))

	all := k.GetAllStreamRecord(ctx)
	require.Len(t, all, 2)
	accounts := []string{all[0].Account, all[1].Account}
	require.ElementsMatch(t, []string{addr1.String(), addr2.String()}, accounts)
}

// TestUpdateFrozenStreamRecord_LockBalanceClampsAtZeroWhenOverdrawn covers the
// clamp when a forced lock-balance decrease removes more than is locked: the
// shortfall is clamped to zero rather than going negative.
func TestUpdateFrozenStreamRecord_LockBalanceClampsAtZeroWhenOverdrawn(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())
	ctx = ctx.WithValue(types.ForceUpdateStreamRecordKey, true)

	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.NewInt(1000),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.NewInt(500),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
	}
	k.SetStreamRecord(ctx, streamRecord)

	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithLockBalanceChange(sdkmath.NewInt(-800))
	_, err := k.UpdateStreamRecordByAddr(ctx, change)
	require.NoError(t, err)

	updated, _ := k.GetStreamRecord(ctx, user)
	require.True(t, updated.LockBalance.IsZero(), "lock balance must clamp at zero, not go negative")
	require.Equal(t, sdkmath.NewInt(1500), updated.StaticBalance, "static+lock is conserved: the full lock balance releases into static")
}

// TestUpdateStreamRecord_ActiveLockBalanceClampsAtZeroWhenOverdrawn covers the
// same clamp on the non-frozen path.
func TestUpdateStreamRecord_ActiveLockBalanceClampsAtZeroWhenOverdrawn(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	user := sample.RandAccAddress()
	streamRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.NewInt(1000),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.NewInt(500),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	change := types.NewDefaultStreamRecordChangeWithAddr(user).WithLockBalanceChange(sdkmath.NewInt(-800))
	err := k.UpdateStreamRecord(ctx, streamRecord, change)
	require.NoError(t, err)
	require.True(t, streamRecord.LockBalance.IsZero(), "lock balance must clamp at zero, not go negative")
	require.Equal(t, sdkmath.NewInt(1500), streamRecord.StaticBalance, "static+lock is conserved: the full lock balance releases into static")
}

// TestUpdateStreamRecord_NonForcedLowRunwayRejected covers a non-forced update
// (a live tx, never an EndBlocker) that would leave the account with a still
// negative rate and less runway than ForcedSettleTime: it must be rejected
// rather than silently forced into settlement.
func TestUpdateStreamRecord_NonForcedLowRunwayRejected(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	user := sample.RandAccAddress()
	sr := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewInt(100),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-100),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}

	// A zero-value "poke" change (isPay == false) still hits the runway check,
	// which is unconditional on the netflow rate rather than gated by isPay.
	change := types.NewDefaultStreamRecordChangeWithAddr(user)
	err := k.UpdateStreamRecord(ctx, sr, change)
	require.ErrorContains(t, err, "lacks of")
}

// TestAutoSettle_FutureRecordSkipped covers the queue's earliest entry being
// scheduled after the current block time: AutoSettle must stop rather than
// process it early.
func TestAutoSettle_FutureRecordSkipped(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	user := sample.RandAccAddress()
	record := types.NewStreamRecord(user, ctx.BlockTime().Unix())
	record.NetflowRate = sdkmath.NewInt(-1)
	record.OutFlowCount = 1
	k.SetStreamRecord(ctx, record)
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix() + 100,
		Addr:      user.String(),
	})

	k.AutoSettle(ctx)

	// nothing was touched: the record is unchanged and still queued
	after, _ := k.GetStreamRecord(ctx, user)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, after.Status)
	require.True(t, hasAutoSettleRecord(k, ctx, user))
}

// TestAutoSettle_StreamRecordNotFoundSkipsEntry covers the defensive
// "should not happen" path: a queued auto-settle entry whose stream record no
// longer exists is logged and skipped rather than causing a crash.
func TestAutoSettle_StreamRecordNotFoundSkipsEntry(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	ghost := sample.RandAccAddress()
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix() - 1,
		Addr:      ghost.String(),
	})

	require.NotPanics(t, func() { k.AutoSettle(ctx) })
}

// TestAutoSettle_UpdateStreamRecordErrorSkipsEntry covers a non-forced
// AutoSettle call whose own settle-poke on an ACTIVE record returns an error:
// the entry is logged and skipped (left queued) rather than settled.
func TestAutoSettle_UpdateStreamRecordErrorSkipsEntry(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	// forced is deliberately NOT set: AutoSettle can run mid-block too, and a
	// non-forced settle-poke on a too-low-runway account must be rejected.

	user := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewInt(100),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-100),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}
	k.SetStreamRecord(ctx, record)
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{
		Timestamp: ctx.BlockTime().Unix() - 1,
		Addr:      user.String(),
	})

	require.NotPanics(t, func() { k.AutoSettle(ctx) })

	// left untouched: still active, still queued
	after, _ := k.GetStreamRecord(ctx, user)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, after.Status)
	require.True(t, hasAutoSettleRecord(k, ctx, user))
}

// hasAutoSettleRecord reports whether addr has any queued auto-settle record;
// the keeper only exposes a bulk getter, not a lookup by address.
func hasAutoSettleRecord(k *keeper.Keeper, ctx sdk.Context, addr sdk.AccAddress) bool {
	for _, r := range k.GetAllAutoSettleRecord(ctx) {
		if r.Addr == addr.String() {
			return true
		}
	}
	return false
}

// TestAutoSettle_SettleActiveOutFlows_MultipleFrozenAccounts covers
// settleActiveOutFlows finding and freezing genuinely active out-flows: two
// already-FROZEN accounts (queued for settlement, e.g. from a force-delete that
// changed their flows without going through the normal freeze-everything path)
// each still carrying one ACTIVE out-flow that AutoSettle must finish freezing.
func TestAutoSettle_SettleActiveOutFlows_MultipleFrozenAccounts(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	addrs := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress()}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i].String() < addrs[j].String() })

	rates := []sdkmath.Int{sdkmath.NewInt(30), sdkmath.NewInt(70)}
	receivers := make([]sdk.AccAddress, 2)
	for i, addr := range addrs {
		receiver := sample.RandAccAddress()
		receivers[i] = receiver
		// the receiver already shows this flow's rate as live inflow, matching
		// the payer's still-ACTIVE out-flow to it; losing the flow must bring
		// this back to exactly zero rather than dip negative.
		receiverRecord := types.NewStreamRecord(receiver, ctx.BlockTime().Unix())
		receiverRecord.NetflowRate = rates[i]
		k.SetStreamRecord(ctx, receiverRecord)

		record := &types.StreamRecord{
			Account:       addr.String(),
			Status:        types.STREAM_ACCOUNT_STATUS_FROZEN,
			StaticBalance: sdkmath.ZeroInt(),
			BufferBalance: sdkmath.ZeroInt(),
			LockBalance:   sdkmath.ZeroInt(),
			// still negative: this flow has not been frozen yet, only queued to be
			NetflowRate:       rates[i].Neg(),
			FrozenNetflowRate: sdkmath.ZeroInt(),
			OutFlowCount:      1,
		}
		k.SetStreamRecord(ctx, record)
		k.SetOutFlow(ctx, addr, &types.OutFlow{
			ToAddress: receiver.String(),
			Rate:      rates[i],
			Status:    types.OUT_FLOW_STATUS_ACTIVE,
		})
		k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{Timestamp: ctx.BlockTime().Unix(), Addr: addr.String()})
	}

	k.AutoSettle(ctx)

	for i, addr := range addrs {
		require.Nil(t, k.GetOutFlow(ctx, addr, types.OUT_FLOW_STATUS_ACTIVE, receivers[i]))
		frozen := k.GetOutFlow(ctx, addr, types.OUT_FLOW_STATUS_FROZEN, receivers[i])
		require.NotNil(t, frozen)
		require.Equal(t, rates[i], frozen.Rate)

		receiverRecord, _ := k.GetStreamRecord(ctx, receivers[i])
		require.True(t, receiverRecord.NetflowRate.IsZero(), "receiver must lose the now-frozen inflow rate")

		require.False(t, hasAutoSettleRecord(k, ctx, addr), "fully settled entries are dequeued")
	}
}

// TestAutoSettle_SettleActiveOutFlows_BoundedAcrossBlocks covers the count/max
// boundary inside settleActiveOutFlows (a mid-loop break) and the
// same-address-frozen termination, by bounding MaxAutoSettleFlowCount below the
// number of active out-flows on a single already-frozen account.
func TestAutoSettle_SettleActiveOutFlows_BoundedAcrossBlocks(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	params := k.GetParams(ctx)
	params.MaxAutoSettleFlowCount = 2
	require.NoError(t, k.SetParams(ctx, params))

	user := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:       user.String(),
		Status:        types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance: sdkmath.ZeroInt(),
		BufferBalance: sdkmath.ZeroInt(),
		LockBalance:   sdkmath.ZeroInt(),
		// -60: still negative for the 3 not-yet-frozen flows (10+20+30)
		NetflowRate:       sdkmath.NewInt(-60),
		FrozenNetflowRate: sdkmath.NewInt(-40), // one flow (rate 40) already frozen earlier
		OutFlowCount:      4,
	}
	k.SetStreamRecord(ctx, record)

	// one already-frozen out-flow, positioned right after the active ones for
	// this same address in key order, to also exercise the same-address frozen
	// termination on the second call.
	alreadyFrozenReceiver := sample.RandAccAddress()
	k.SetOutFlow(ctx, user, &types.OutFlow{
		ToAddress: alreadyFrozenReceiver.String(), Rate: sdkmath.NewInt(40), Status: types.OUT_FLOW_STATUS_FROZEN,
	})

	receivers := make([]sdk.AccAddress, 3)
	rates := []sdkmath.Int{sdkmath.NewInt(10), sdkmath.NewInt(20), sdkmath.NewInt(30)}
	for i := range receivers {
		receivers[i] = sample.RandAccAddress()
		receiverRecord := types.NewStreamRecord(receivers[i], ctx.BlockTime().Unix())
		receiverRecord.NetflowRate = rates[i] // matches the still-ACTIVE inflow from user
		k.SetStreamRecord(ctx, receiverRecord)
		k.SetOutFlow(ctx, user, &types.OutFlow{
			ToAddress: receivers[i].String(), Rate: rates[i], Status: types.OUT_FLOW_STATUS_ACTIVE,
		})
	}
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{Timestamp: ctx.BlockTime().Unix(), Addr: user.String()})

	// first call: only 2 of the 3 active flows may be frozen (max == 2)
	k.AutoSettle(ctx)
	frozenCount := 0
	for i := range receivers {
		if k.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, receivers[i]) != nil {
			frozenCount++
		}
	}
	require.Equal(t, 2, frozenCount, "the bounded call must freeze exactly max flows")
	require.True(t, hasAutoSettleRecord(k, ctx, user), "partially settled accounts stay queued")

	// second call: the remaining active flow is frozen, and the iterator's own
	// already-frozen entry for this address terminates the batch cleanly
	k.AutoSettle(ctx)
	for i := range receivers {
		require.NotNil(t, k.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_FROZEN, receivers[i]))
		require.Nil(t, k.GetOutFlow(ctx, user, types.OUT_FLOW_STATUS_ACTIVE, receivers[i]))
	}
	require.False(t, hasAutoSettleRecord(k, ctx, user), "fully settled accounts are dequeued")

	final, _ := k.GetStreamRecord(ctx, user)
	require.True(t, final.NetflowRate.IsZero())
	require.True(t, final.FrozenNetflowRate.Equal(sdkmath.NewInt(-100)), "all four flows (40+10+20+30) end frozen")
}

// TestTryResumeStreamRecord_NotFrozen covers calling resume on an account that
// is not currently frozen.
func TestTryResumeStreamRecord_NotFrozen(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	user := sample.RandAccAddress()
	record := types.NewStreamRecord(user, ctx.BlockTime().Unix())

	err := k.TryResumeStreamRecord(ctx, record, sdkmath.NewInt(100))
	require.ErrorContains(t, err, "status is not frozen")
}

// TestTryResumeStreamRecord_NoOutstandingRateResumesImmediately covers an idle
// frozen account (no active or frozen rate at all): any deposit resumes it to
// ACTIVE immediately, bypassing the balance/buffer math entirely.
func TestTryResumeStreamRecord_NoOutstandingRateResumesImmediately(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(5000, 0))

	user := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		CrudTimestamp:     1000,
	}

	err := k.TryResumeStreamRecord(ctx, record, sdkmath.NewInt(50))
	require.NoError(t, err)

	updated, found := k.GetStreamRecord(ctx, user)
	require.True(t, found)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, sdkmath.NewInt(50), updated.StaticBalance)
	require.Equal(t, ctx.BlockTime().Unix(), updated.CrudTimestamp)
	require.Equal(t, int64(0), updated.SettleTimestamp)
}

// TestTryResumeStreamRecord_FrozenFlowsAddrMismatchStopsBatch covers the
// direct-resume loop's addr-mismatch termination: once addr's own frozen
// out-flows are exhausted, a different address's frozen entry must stop the
// scan rather than being (mis)treated as belonging to addr.
func TestTryResumeStreamRecord_FrozenFlowsAddrMismatchStopsBatch(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())
	params := k.GetParams(ctx)

	// Sorted by raw bytes, not String(): the checksum-cased hex string does not
	// sort the same as the address bytes the store actually orders keys by.
	addrs := []sdk.AccAddress{sample.RandAccAddress(), sample.RandAccAddress()}
	sort.Slice(addrs, func(i, j int) bool { return bytes.Compare(addrs[i].Bytes(), addrs[j].Bytes()) < 0 })
	user, other := addrs[0], addrs[1]

	rate := sdkmath.NewInt(100)
	gvg := sample.RandAccAddress()
	k.SetOutFlow(ctx, user, &types.OutFlow{ToAddress: gvg.String(), Rate: rate, Status: types.OUT_FLOW_STATUS_FROZEN})
	// belongs to a different, lexicographically-later address: must not be
	// mistaken for one of user's own frozen out-flows.
	k.SetOutFlow(ctx, other, &types.OutFlow{ToAddress: gvg.String(), Rate: rate, Status: types.OUT_FLOW_STATUS_FROZEN})

	record := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: rate.Neg(),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, record)

	err := k.TryResumeStreamRecord(ctx, record, rate.MulRaw(int64(params.VersionedParams.ReserveTime))) //nolint:gosec // G115
	require.NoError(t, err)

	updated, _ := k.GetStreamRecord(ctx, user)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, rate.Neg(), updated.NetflowRate)

	// other's own frozen out-flow was left untouched by user's resume
	otherFlow := k.GetOutFlow(ctx, other, types.OUT_FLOW_STATUS_FROZEN, gvg)
	require.NotNil(t, otherFlow)
}

// TestTryResumeStreamRecord_ReceiverUpdateError covers a receiver whose own
// resulting rate stays deeply negative with too little runway: crediting it
// during resume must surface as a wrapped error rather than a silent partial
// resume, and this is a live-tx path (never forced) so it cannot force-settle
// its way past the guard.
func TestTryResumeStreamRecord_ReceiverUpdateError(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	params := k.GetParams(ctx)

	user := sample.RandAccAddress()
	receiver := sample.RandAccAddress()
	creditedRate := sdkmath.NewInt(100)

	// the receiver's own outgoing rate (-1000) dwarfs the inflow being restored,
	// and its balance is only enough for an instant's runway.
	receiverRecord := &types.StreamRecord{
		Account:           receiver.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewInt(1),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1000),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}
	k.SetStreamRecord(ctx, receiverRecord)

	k.SetOutFlow(ctx, user, &types.OutFlow{ToAddress: receiver.String(), Rate: creditedRate, Status: types.OUT_FLOW_STATUS_FROZEN})
	userRecord := &types.StreamRecord{
		StaticBalance:     creditedRate.MulRaw(int64(params.VersionedParams.ReserveTime)), //nolint:gosec // G115
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: creditedRate.Neg(),
		OutFlowCount:      1,
	}

	err := k.TryResumeStreamRecord(ctx, userRecord, sdkmath.ZeroInt())
	require.ErrorContains(t, err, "try resume, update receiver stream record failed")
}

// TestAutoResume_StreamRecordNotFoundSkipsEntry covers the defensive
// "should not happen" path: a queued auto-resume entry whose stream record no
// longer exists is logged and skipped.
func TestAutoResume_StreamRecordNotFoundSkipsEntry(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ghost := sample.RandAccAddress()
	k.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{Timestamp: ctx.BlockTime().Unix(), Addr: ghost.String()})

	require.NotPanics(t, func() { k.AutoResume(ctx) })
}

// TestAutoResume_ReceiverUpdateErrorPanics covers AutoResume's own receiver
// credit call: unlike TryResumeStreamRecord (a live tx, which returns the
// error), AutoResume runs from the EndBlocker and deliberately panics instead
// of silently leaving the batch half-applied.
func TestAutoResume_ReceiverUpdateErrorPanics(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	user := sample.RandAccAddress()
	receiver := sample.RandAccAddress()
	creditedRate := sdkmath.NewInt(100)

	receiverRecord := &types.StreamRecord{
		Account:           receiver.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewInt(1),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1000),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}
	k.SetStreamRecord(ctx, receiverRecord)
	k.SetOutFlow(ctx, user, &types.OutFlow{ToAddress: receiver.String(), Rate: creditedRate, Status: types.OUT_FLOW_STATUS_FROZEN})

	userRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: creditedRate.Neg(),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, userRecord)
	k.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{Timestamp: ctx.BlockTime().Unix(), Addr: user.String()})

	require.PanicsWithValue(t, "should not happen", func() { k.AutoResume(ctx) })
}

// TestAutoResume_FrozenNetflowNotZeroPanics covers the invariant guard on the
// finishing side: a queued resume whose FrozenNetflowRate does not return to
// exactly zero once its (empty, in this fixture) frozen out-flows are drained
// indicates corrupted bookkeeping, so AutoResume panics rather than silently
// leaving the account active with a dangling frozen rate.
func TestAutoResume_FrozenNetflowNotZeroPanics(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	user := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.NewInt(-10), // inconsistent: no frozen out-flow backs this
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}
	k.SetStreamRecord(ctx, record)
	k.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{Timestamp: ctx.BlockTime().Unix(), Addr: user.String()})

	require.PanicsWithValue(t, "should not happen", func() { k.AutoResume(ctx) })
}

// TestAutoSettle_FrozenNetflowNotZeroPanics mirrors
// TestAutoResume_FrozenNetflowNotZeroPanics for the settle side: an
// already-frozen, no-active-out-flow record whose NetflowRate was never
// actually drained is a bookkeeping inconsistency AutoSettle must not paper
// over.
func TestAutoSettle_FrozenNetflowNotZeroPanics(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))

	user := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-10), // inconsistent: no active out-flow backs this
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, record)
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{Timestamp: ctx.BlockTime().Unix(), Addr: user.String()})

	require.PanicsWithValue(t, "should not happen", func() { k.AutoSettle(ctx) })
}

// TestAutoSettle_SettleActiveOutFlows_ReceiverUpdateErrorPanics covers
// settleActiveOutFlows's own receiver-credit call failing: like AutoResume,
// AutoSettle runs from the EndBlocker and panics rather than leaving the
// account half-settled.
func TestAutoSettle_SettleActiveOutFlows_ReceiverUpdateErrorPanics(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Unix(1000, 0))
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	user := sample.RandAccAddress()
	receiver := sample.RandAccAddress()
	rate := sdkmath.NewInt(100)

	receiverRecord := &types.StreamRecord{
		Account:           receiver.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance:     sdkmath.NewInt(1),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       sdkmath.NewInt(-1000),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
		CrudTimestamp:     ctx.BlockTime().Unix(),
	}
	k.SetStreamRecord(ctx, receiverRecord)

	userRecord := &types.StreamRecord{
		Account:           user.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		NetflowRate:       rate.Neg(), // still active, not yet frozen
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, userRecord)
	k.SetOutFlow(ctx, user, &types.OutFlow{ToAddress: receiver.String(), Rate: rate, Status: types.OUT_FLOW_STATUS_ACTIVE})
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{Timestamp: ctx.BlockTime().Unix(), Addr: user.String()})

	require.PanicsWithValue(t, "should not happen", func() { k.AutoSettle(ctx) })
}
