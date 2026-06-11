package upgrades_test

import (
	"encoding/hex"
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/app/upgrades"
	feemarkettypes "github.com/mocachain/moca/v2/x/feemarket/types"
	"github.com/stretchr/testify/require"
)

func mustAcc(t *testing.T, hexAddr string) sdk.AccAddress {
	t.Helper()
	b, err := hex.DecodeString(hexAddr)
	require.NoError(t, err)
	require.Len(t, b, 20)
	return sdk.AccAddress(b)
}

// The handler must re-grant each validator's SelfDelAddress -> gov : MsgDelegate
// (Generic), and leave unrelated grants untouched.
func TestV1_3_0RestoreGovGrants(t *testing.T) {
	const chainID = "moca_5151-1"
	mocaApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), chainID)
	sdkCtx := mocaApp.BaseApp.NewContext(false).WithChainID(chainID)
	ctx := sdk.WrapSDKContext(sdkCtx)

	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)
	delegateMsg := sdk.MsgTypeURL(&stakingtypes.MsgDelegate{})

	// Give the genesis validator a SelfDelAddress (the test harness leaves it
	// empty; production populates it at create-validator time).
	vals, err := mocaApp.StakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	selfDel := mustAcc(t, "70f0f0cfd8b32d64db8871108229615bc80aaa3a")
	v := vals[0]
	v.SelfDelAddress = selfDel.String()
	require.NoError(t, mocaApp.StakingKeeper.SetValidator(ctx, v))

	// An unrelated grant the handler must NOT touch.
	other := mustAcc(t, "2ab5683d20f32356daa4bcaf5129d64fe338302e")
	require.NoError(t, mocaApp.AuthzKeeper.SaveGrant(ctx, other, selfDel,
		authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend"), nil))

	// Pre: the validator's gov-delegate grant is absent.
	got, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, govAddr, selfDel, delegateMsg)
	require.Nil(t, got)

	mm := module.NewManager()
	configurator := module.NewConfigurator(mocaApp.AppCodec(), mocaApp.MsgServiceRouter(), mocaApp.GRPCQueryRouter())
	handler := upgrades.V1_3_0RestoreGovGrants(
		mocaApp.AuthzKeeper, mocaApp.StakingKeeper, mocaApp.SpKeeper, mm, configurator)
	_, err = handler(ctx, upgradetypes.Plan{Name: upgrades.V1_3_0UpgradeName}, module.VersionMap{})
	require.NoError(t, err)

	// Post 1: the validator's self-del account now has a Generic MsgDelegate grant to gov.
	a, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, govAddr, selfDel, delegateMsg)
	require.NotNil(t, a, "validator self-del account should have a gov-delegate grant")
	require.Equal(t, delegateMsg, a.MsgTypeURL())
	_, ok := a.(*authz.GenericAuthorization)
	require.True(t, ok, "restored grant should be a GenericAuthorization")

	// Post 2: the unrelated grant is untouched (restore-only does not purge).
	stillThere, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, other, selfDel, "/cosmos.bank.v1beta1.MsgSend")
	require.NotNil(t, stillThere, "unrelated grant must be left intact")
}
