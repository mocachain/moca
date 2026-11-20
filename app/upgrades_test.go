package app

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/app/upgrades"
	mocatypes "github.com/mocachain/moca/v2/types"
)

// TestUpgradeHandlersRegistered fails the build if any planned upgrade name
// lacks a handler -- a missing one would halt the chain at that height.
func TestUpgradeHandlersRegistered(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	for _, name := range []string{"v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0", upgradeNameV150, upgradeNameV2} {
		require.True(t, mocaApp.UpgradeKeeper.HasHandler(name),
			"no upgrade handler registered for %s: a plan with this name would halt the chain", name)
	}
}

// TestV150UpgradeHandler_BurnsCrosschainGenesisMint: the v1.5.0 upgrade empties the
// crosschain module account and lowers the supply by exactly the genesis mint.
func TestV150UpgradeHandler_BurnsCrosschainGenesisMint(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false).WithBlockHeight(100)

	crosschainAddr := authtypes.NewModuleAddress("crosschain")
	mint := sdk.NewCoins(sdk.NewCoin(mocatypes.AttoMoca, upgrades.CrosschainGenesisMint))
	require.NoError(t, mocaApp.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, mint))
	require.NoError(t, mocaApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, crosschainAddr, mint))
	supply := mocaApp.BankKeeper.GetSupply(ctx, mocatypes.AttoMoca).Amount

	require.NoError(t, mocaApp.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: upgradeNameV150, Height: ctx.BlockHeight()}))

	require.True(t, mocaApp.BankKeeper.GetBalance(ctx, crosschainAddr, mocatypes.AttoMoca).IsZero())
	require.Equal(t, supply.Sub(upgrades.CrosschainGenesisMint).String(),
		mocaApp.BankKeeper.GetSupply(ctx, mocatypes.AttoMoca).Amount.String())
}
