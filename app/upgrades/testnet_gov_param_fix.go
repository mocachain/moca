package upgrades

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	evmkeeper "github.com/evmos/evmos/v12/x/evm/keeper"
)

// TestnetGovParamFix is the upgrade handler for the `testnet-gov-param-fix`
// upgrade which fixes critical testnet issue where the minimum deposit ratio
// was not set correctly, preventing proposals from being submitted.
//
// Changes:
//   - Minimum deposit ratio is set to 0.01
//   - Allow unprotected (non EIP155 signed) txs at the protocol level
func TestnetGovParamFix(govKeeper *govkeeper.Keeper, evmKeeper *evmkeeper.Keeper, mm *module.Manager, configurator module.Configurator) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {

		// Set minimum deposit ratio to 0.01
		govParams, err := govKeeper.Params.Get(ctx)
		if err != nil {
			return nil, err
		}

		govParams.MinDepositRatio = "0.010000000000000000"
		if err := govParams.ValidateBasic(); err != nil {
			return nil, err
		}

		if err := govKeeper.Params.Set(ctx, govParams); err != nil {
			return nil, err
		}

		// Allow unprotected (non EIP155 signed) txs at the protocol level.
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		evmParams := evmKeeper.GetParams(sdkCtx)
		evmParams.AllowUnprotectedTxs = true
		if err := evmKeeper.SetParams(sdkCtx, evmParams); err != nil {
			return nil, err
		}

		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
