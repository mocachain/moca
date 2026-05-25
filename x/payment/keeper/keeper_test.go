package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"go.uber.org/mock/gomock"

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
