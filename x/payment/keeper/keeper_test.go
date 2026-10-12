package keeper_test

import (
	"errors"
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	"github.com/mocachain/moca/v2/x/payment/types"
)

type DepKeepers struct {
	BankKeeper    *types.MockBankKeeper
	AccountKeeper *types.MockAccountKeeper
}

func makePaymentKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, DepKeepers) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	bankKeeper := types.NewMockBankKeeper(ctrl)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		bankKeeper,
		accountKeeper,
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)
	err := k.SetParams(testCtx.Ctx, types.DefaultParams())
	if err != nil {
		panic(err)
	}

	depKeepers := DepKeepers{
		BankKeeper:    bankKeeper,
		AccountKeeper: accountKeeper,
	}

	return k, testCtx.Ctx, depKeepers
}

func TestKeeperLogger(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	require.NotNil(t, k.Logger(ctx))
}

func TestQueryDynamicBalance(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)

	// not found -> zero, no error
	amount, err := k.QueryDynamicBalance(ctx, sample.RandAccAddress())
	require.NoError(t, err)
	require.True(t, amount.IsZero())

	// found -> settled static balance is returned
	addr := sample.RandAccAddress()
	record := types.NewStreamRecord(addr, ctx.BlockTime().Unix())
	record.StaticBalance = sdkmath.NewInt(500)
	k.SetStreamRecord(ctx, record)

	amount, err = k.QueryDynamicBalance(ctx, addr)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(500), amount)
}

func newFundedStreamRecord(k *keeper.Keeper, ctx sdk.Context, balance sdkmath.Int) sdk.AccAddress {
	addr := sample.RandAccAddress()
	record := types.NewStreamRecord(addr, ctx.BlockTime().Unix())
	record.StaticBalance = balance
	k.SetStreamRecord(ctx, record)
	return addr
}

func TestKeeperWithdraw(t *testing.T) {
	k, ctx, dep := makePaymentKeeper(t)
	// The over-withdraw case below pushes the balance negative, which makes
	// UpdateStreamRecord probe for a bank account to auto-cover the shortfall.
	dep.AccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	// not found -> ErrStreamRecordNotFound
	err := k.Withdraw(ctx, sample.RandAccAddress(), sample.RandAccAddress(), sdkmath.NewInt(100))
	require.ErrorIs(t, err, types.ErrStreamRecordNotFound)

	// UpdateStreamRecord error: withdrawing far more than the account holds
	from := newFundedStreamRecord(k, ctx, sdkmath.NewInt(1000))
	err = k.Withdraw(ctx, from, sample.RandAccAddress(), sdkmath.NewInt(10_000))
	require.Error(t, err)

	// success: regular recipient via SendCoinsFromModuleToAccount
	from = newFundedStreamRecord(k, ctx, sdkmath.NewInt(1000))
	to := sample.RandAccAddress()
	dep.BankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(1)
	err = k.Withdraw(ctx, from, to, sdkmath.NewInt(100))
	require.NoError(t, err)
	updated, _ := k.GetStreamRecord(ctx, from)
	require.Equal(t, sdkmath.NewInt(900), updated.StaticBalance)

	// SendCoinsFromModuleToAccount error branch
	from = newFundedStreamRecord(k, ctx, sdkmath.NewInt(1000))
	dep.BankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("send failed")).Times(1)
	err = k.Withdraw(ctx, from, to, sdkmath.NewInt(50))
	require.Error(t, err)

	// success: distribution module recipient bypasses the blocklist via SendCoinsFromModuleToModule
	from = newFundedStreamRecord(k, ctx, sdkmath.NewInt(1000))
	distAddr := authtypes.NewModuleAddress(distrtypes.ModuleName)
	dep.BankKeeper.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), types.ModuleName, distrtypes.ModuleName, gomock.Any()).
		Return(nil).Times(1)
	err = k.Withdraw(ctx, from, distAddr, sdkmath.NewInt(50))
	require.NoError(t, err)

	// SendCoinsFromModuleToModule error branch
	from = newFundedStreamRecord(k, ctx, sdkmath.NewInt(1000))
	dep.BankKeeper.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), types.ModuleName, distrtypes.ModuleName, gomock.Any()).
		Return(errors.New("send failed")).Times(1)
	err = k.Withdraw(ctx, from, distAddr, sdkmath.NewInt(50))
	require.Error(t, err)
}
