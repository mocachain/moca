package keeper_test

import (
	"errors"
	"math"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
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

func TestAutoSettle_SettleInMultipleBlocks(t *testing.T) {
	keeper, ctx, depKeepers := makePaymentKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// freeze account in multiple blocks
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
	keeper.AutoSettle(ctx) // this is for settle stream, it is counted
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)

	keeper.AutoSettle(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, !userStreamRecord.NetflowRate.IsZero())
	require.True(t, !userStreamRecord.FrozenNetflowRate.IsZero())

	keeper.AutoSettle(ctx)
	userStreamRecord, _ = keeper.GetStreamRecord(ctx, user)
	require.True(t, userStreamRecord.Status == types.STREAM_ACCOUNT_STATUS_FROZEN)
	require.True(t, !userStreamRecord.NetflowRate.IsZero())
	require.True(t, !userStreamRecord.FrozenNetflowRate.IsZero())

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
