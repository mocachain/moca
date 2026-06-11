package upgrades_test

import (
	"encoding/hex"
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
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

// The handler must (1) purge every pre-existing grant and (2) restore each
// validator's SelfDelAddress -> gov : MsgDelegate grant.
func TestV1_3_0ResetAuthzGrants(t *testing.T) {
	const chainID = "moca_5151-1"
	mocaApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), chainID)
	sdkCtx := mocaApp.BaseApp.NewContext(false).WithChainID(chainID)
	ctx := sdk.WrapSDKContext(sdkCtx)

	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)
	delegateMsg := sdk.MsgTypeURL(&stakingtypes.MsgDelegate{})

	// The test app's genesis validator has no SelfDelAddress set; give it one so
	// the restore phase has a real validator to grant for (mirrors production,
	// where SelfDelAddress is populated at create-validator time).
	vals, err := mocaApp.StakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals, "expected a genesis validator")
	selfDel := mustAcc(t, "70f0f0cfd8b32d64db8871108229615bc80aaa3a")
	v := vals[0]
	v.SelfDelAddress = selfDel.String()
	require.NoError(t, mocaApp.StakingKeeper.SetValidator(ctx, v))

	// Pre-state: the validator's gov-delegate grant is absent, plus a junk grant.
	junkGranter := mustAcc(t, "72ce2b75aa57e5a87ca5255d83971aef096e14e4")
	junkGrantee := mustAcc(t, "2ab5683d20f32356daa4bcaf5129d64fe338302e")
	require.NoError(t, mocaApp.AuthzKeeper.SaveGrant(ctx, junkGrantee, junkGranter,
		authz.NewGenericAuthorization("/cosmos.gov.v1.MsgVote"), nil))

	got, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, govAddr, selfDel, delegateMsg)
	require.Nil(t, got, "validator gov-delegate grant should be absent before the upgrade")

	// Run the handler.
	mm := module.NewManager()
	configurator := module.NewConfigurator(mocaApp.AppCodec(), mocaApp.MsgServiceRouter(), mocaApp.GRPCQueryRouter())
	handler := upgrades.V1_3_0ResetAuthzGrants(
		mocaApp.GetKey(authzkeeper.StoreKey), mocaApp.AuthzKeeper, mocaApp.StakingKeeper, mocaApp.SpKeeper, mm, configurator)
	_, err = handler(ctx, upgradetypes.Plan{Name: upgrades.V1_3_0UpgradeName}, module.VersionMap{})
	require.NoError(t, err)

	// Post-state 1: the junk grant is purged.
	gotJunk, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, junkGrantee, junkGranter, "/cosmos.gov.v1.MsgVote")
	require.Nil(t, gotJunk, "junk grant should be purged")

	// Post-state 2: the validator's self-del account now has a Generic
	// MsgDelegate grant to the gov module.
	a, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, govAddr, selfDel, delegateMsg)
	require.NotNil(t, a, "validator self-del account should have a gov-delegate grant after the upgrade")
	require.Equal(t, delegateMsg, a.MsgTypeURL())
	_, ok := a.(*authz.GenericAuthorization)
	require.True(t, ok, "restored grant should be a GenericAuthorization")
}
