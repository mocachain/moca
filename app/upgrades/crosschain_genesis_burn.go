package upgrades

import (
	"context"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

const crosschainModuleName = "crosschain"

// CrosschainGenesisMint is the amount x/crosschain minted to its module account at
// genesis: its default init_module_balance of 2,000,000 MOCA.
var CrosschainGenesisMint = sdkmath.NewIntWithDecimal(2_000_000, 18)

// BurnCrosschainGenesisMint moves CrosschainGenesisMint of denom from the crosschain
// module account to the gov module account and burns it there. It burns nothing if
// the crosschain module account holds less than that.
//
// Returns the burned amount.
func BurnCrosschainGenesisMint(ctx context.Context, bk bankkeeper.Keeper, denom string) (sdkmath.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	crosschainAddr := authtypes.NewModuleAddress(crosschainModuleName)

	balance := bk.GetBalance(ctx, crosschainAddr, denom).Amount
	if balance.LT(CrosschainGenesisMint) {
		sdkCtx.Logger().Info("crosschain: genesis mint not burned", "balance", balance)
		return sdkmath.ZeroInt(), nil
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, CrosschainGenesisMint))
	if err := bk.SendCoinsFromAccountToModule(ctx, crosschainAddr, govtypes.ModuleName, coins); err != nil {
		return sdkmath.ZeroInt(), err
	}
	if err := bk.BurnCoins(ctx, govtypes.ModuleName, coins); err != nil {
		return sdkmath.ZeroInt(), err
	}

	sdkCtx.Logger().Info("crosschain: burned the genesis mint", "amount", coins)
	return CrosschainGenesisMint, nil
}
