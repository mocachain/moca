package app

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	crosschaintypes "github.com/cosmos/cosmos-sdk/x/crosschain/types"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/app/upgrades"
	evmostypes "github.com/evmos/evmos/v12/types"
)

// TestUpgradeHandlersRegistered pins the handler names the network upgrades
// through. A plan whose name has no handler halts every node at the upgrade
// height, and nothing else in the build fails when one is missing, so the set is
// asserted here rather than discovered on the chain.
func TestUpgradeHandlersRegistered(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	for _, name := range []string{"v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0", "v1.5.0"} {
		require.True(t, mocaApp.UpgradeKeeper.HasHandler(name),
			"no upgrade handler registered for %s: a plan with this name would halt the chain", name)
	}
}

// TestV150UpgradeHandler_BurnsCrosschainGenesisMint: the v1.5.0 upgrade empties the
// crosschain module account and lowers the supply by exactly the genesis mint.
func TestV150UpgradeHandler_BurnsCrosschainGenesisMint(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false).WithBlockHeight(100)

	crosschainAddr := authtypes.NewModuleAddress(crosschaintypes.ModuleName)
	mint := sdk.NewCoins(sdk.NewCoin(evmostypes.AttoEvmos, upgrades.CrosschainGenesisMint))
	require.NoError(t, mocaApp.BankKeeper.MintCoins(ctx, crosschaintypes.ModuleName, mint))
	supply := mocaApp.BankKeeper.GetSupply(ctx, evmostypes.AttoEvmos).Amount

	require.NoError(t, mocaApp.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: "v1.5.0", Height: ctx.BlockHeight()}))

	require.True(t, mocaApp.BankKeeper.GetBalance(ctx, crosschainAddr, evmostypes.AttoEvmos).IsZero())
	require.Equal(t, supply.Sub(upgrades.CrosschainGenesisMint).String(),
		mocaApp.BankKeeper.GetSupply(ctx, evmostypes.AttoEvmos).Amount.String())
}
