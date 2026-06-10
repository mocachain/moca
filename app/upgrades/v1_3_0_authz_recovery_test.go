package upgrades_test

import (
	"encoding/hex"
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/app/upgrades"
	feemarkettypes "github.com/mocachain/moca/v2/x/feemarket/types"
	"github.com/stretchr/testify/require"
)

// mainnet chain-id of the live network whose authz store the iavl bug damaged.
// NOTE: this is the genesis chain-id returned by ctx.ChainID() on the running
// mainnet (confirmed via RPC). It is intentionally NOT utils.MainnetChainID,
// whose constant differs from the deployed network.
const mainnetChainID = "moca_2288-1"

func mustAcc(t *testing.T, hexAddr string) sdk.AccAddress {
	t.Helper()
	b, err := hex.DecodeString(hexAddr)
	require.NoError(t, err)
	require.Len(t, b, 20)
	return sdk.AccAddress(b)
}

// On the mainnet chain-id, the handler must re-insert the 4 recorded grants so
// each becomes retrievable through the authz keeper.
func TestV1_3_0AuthzRecovery_Mainnet_ReinsertsGrants(t *testing.T) {
	mocaApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), mainnetChainID)
	// NewContext does not carry the genesis chain-id; the handler keys off
	// ctx.ChainID() (set from the block header in production), so set it here.
	sdkCtx := mocaApp.BaseApp.NewContext(false).WithChainID(mainnetChainID)
	ctx := sdk.WrapSDKContext(sdkCtx)

	type grant struct {
		granter, grantee, msg string
	}
	want := []grant{
		{"75233ac05854d55ea2f6c2bbd37c15c21907b891", "7b5fe22b5446f7c62ea27b8bd71cef94e03f3df2", "/moca.sp.MsgDeposit"},
		{"9da842f6ed097372f5dad1c0c57ef5033109fd11", "7b5fe22b5446f7c62ea27b8bd71cef94e03f3df2", "/moca.sp.MsgDeposit"},
		{"ac1960e1d340095bea998cfa6ff6601c0feca62e", "9a7559666d4117a6b0ba2525629e6be171eb0095", "/cosmos.gov.v1.MsgVote"},
		{"fbc7a5930f64a1bdb30d4878902ca7ec7956c09f", "7b5fe22b5446f7c62ea27b8bd71cef94e03f3df2", "/moca.sp.MsgDeposit"},
	}

	// Pre-condition: none of the grants exist on a fresh app.
	for _, g := range want {
		got, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, mustAcc(t, g.grantee), mustAcc(t, g.granter), g.msg)
		require.Nil(t, got, "grant %s->%s should be absent before recovery", g.granter, g.grantee)
	}

	mm := module.NewManager()
	configurator := module.NewConfigurator(mocaApp.AppCodec(), mocaApp.MsgServiceRouter(), mocaApp.GRPCQueryRouter())
	handler := upgrades.V1_3_0AuthzRecovery(mocaApp.AuthzKeeper, mm, configurator)
	_, err := handler(ctx, upgradetypes.Plan{Name: upgrades.V1_3_0UpgradeName}, module.VersionMap{})
	require.NoError(t, err)

	// Post-condition: all 4 grants are retrievable with the right msg type.
	for _, g := range want {
		got, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx, mustAcc(t, g.grantee), mustAcc(t, g.granter), g.msg)
		require.NotNil(t, got, "grant %s->%s should exist after recovery", g.granter, g.grantee)
		require.Equal(t, g.msg, got.MsgTypeURL())
	}
}

// On a chain-id with no recorded damage (devnet), the handler is a no-op and
// must not error or add any grants.
func TestV1_3_0AuthzRecovery_OtherChain_NoOp(t *testing.T) {
	mocaApp := app.Setup(false, feemarkettypes.DefaultGenesisState(), "moca_5151-1") // devnet
	sdkCtx := mocaApp.BaseApp.NewContext(false).WithChainID("moca_5151-1")
	ctx := sdk.WrapSDKContext(sdkCtx)

	mm := module.NewManager()
	configurator := module.NewConfigurator(mocaApp.AppCodec(), mocaApp.MsgServiceRouter(), mocaApp.GRPCQueryRouter())
	handler := upgrades.V1_3_0AuthzRecovery(mocaApp.AuthzKeeper, mm, configurator)
	_, err := handler(ctx, upgradetypes.Plan{Name: upgrades.V1_3_0UpgradeName}, module.VersionMap{})
	require.NoError(t, err)

	// devnet's recorded grant set is empty; the mainnet grant must NOT appear.
	got, _ := mocaApp.AuthzKeeper.GetAuthorization(ctx,
		mustAcc(t, "7b5fe22b5446f7c62ea27b8bd71cef94e03f3df2"),
		mustAcc(t, "75233ac05854d55ea2f6c2bbd37c15c21907b891"),
		"/moca.sp.MsgDeposit")
	require.Nil(t, got)
}
