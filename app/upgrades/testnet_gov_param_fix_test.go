package upgrades_test

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/app/upgrades"
	"github.com/evmos/evmos/v12/utils"
	feemarkettypes "github.com/evmos/evmos/v12/x/feemarket/types"
	"github.com/stretchr/testify/require"
)

func TestTestnetGovParamFix_UpdatesGovAndEvmParams(t *testing.T) {
	evmosApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), utils.TestnetChainID+"-1")
	sdkCtx := evmosApp.BaseApp.NewContext(false)
	ctx := sdk.WrapSDKContext(sdkCtx)

	// Force gov param away from the desired value to prove the upgrade changes it.
	govParams, err := evmosApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	govParams.MinDepositRatio = "0.500000000000000000"
	require.NoError(t, govParams.ValidateBasic())
	require.NoError(t, evmosApp.GovKeeper.Params.Set(ctx, govParams))

	// Force EVM param away from the desired value to prove the upgrade changes it.
	evmParams := evmosApp.EvmKeeper.GetParams(sdkCtx)
	evmParams.AllowUnprotectedTxs = false
	require.NoError(t, evmosApp.EvmKeeper.SetParams(sdkCtx, evmParams))

	mm := module.NewManager()
	configurator := module.NewConfigurator(evmosApp.AppCodec(), evmosApp.MsgServiceRouter(), evmosApp.GRPCQueryRouter())

	handler := upgrades.TestnetGovParamFix(&evmosApp.GovKeeper, evmosApp.EvmKeeper, mm, configurator)
	_, err = handler(ctx, upgradetypes.Plan{Name: "testnet-gov-param-fix"}, module.VersionMap{})
	require.NoError(t, err)

	updatedGovParams, err := evmosApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "0.010000000000000000", updatedGovParams.MinDepositRatio)

	updatedEvmParams := evmosApp.EvmKeeper.GetParams(sdkCtx)
	require.True(t, updatedEvmParams.AllowUnprotectedTxs)
}
