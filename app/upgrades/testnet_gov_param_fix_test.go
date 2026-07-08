package upgrades_test

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/app/upgrades"
	"github.com/mocachain/moca/v2/utils"
	"github.com/stretchr/testify/require"
)

func TestTestnetGovParamFix_UpdatesGovParams(t *testing.T) {
	mocaApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), utils.TestnetChainID+"-1")
	sdkCtx := mocaApp.BaseApp.NewContext(false)
	ctx := sdk.WrapSDKContext(sdkCtx)

	// Force gov param away from the desired value to prove the upgrade changes it.
	govParams, err := mocaApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	govParams.MinDepositRatio = "0.500000000000000000"
	require.NoError(t, govParams.ValidateBasic())
	require.NoError(t, mocaApp.GovKeeper.Params.Set(ctx, govParams))

	// NOTE(cosmos/evm v0.6.0 migration): the EVM AllowUnprotectedTxs param was
	// dropped from evmtypes.Params, and TestnetGovParamFix no longer touches EVM
	// params (see app/upgrades/testnet_gov_param_fix.go). The upgrade now only
	// fixes the gov MinDepositRatio, so this test asserts that surviving behavior.

	mm := module.NewManager()
	configurator := module.NewConfigurator(mocaApp.AppCodec(), mocaApp.MsgServiceRouter(), mocaApp.GRPCQueryRouter())

	handler := upgrades.TestnetGovParamFix(&mocaApp.GovKeeper, mocaApp.EvmKeeper, mm, configurator)
	_, err = handler(ctx, upgradetypes.Plan{Name: "testnet-gov-param-fix"}, module.VersionMap{})
	require.NoError(t, err)

	updatedGovParams, err := mocaApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "0.010000000000000000", updatedGovParams.MinDepositRatio)
}
