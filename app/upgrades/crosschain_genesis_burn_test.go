package upgrades_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/mocachain/moca/v2/app/upgrades"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
)

const (
	burnTestDenom  = "amoca"
	burnTestMinter = "minter"
)

func setupBurnTest(t *testing.T) (bankkeeper.BaseKeeper, sdk.Context) {
	t.Helper()

	authKey := storetypes.NewKVStoreKey(authtypes.StoreKey)
	bankKey := storetypes.NewKVStoreKey(banktypes.StoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	cms.MountStoreWithDB(authKey, storetypes.StoreTypeIAVL, db)
	cms.MountStoreWithDB(bankKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, cmtproto.Header{ChainID: "moca_test-1", Time: time.Now()}, false, log.NewNopLogger())

	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	ak := authkeeper.NewAccountKeeper(cdc, runtime.NewKVStoreService(authKey), authtypes.ProtoBaseAccount,
		map[string][]string{govtypes.ModuleName: {authtypes.Burner}, burnTestMinter: {authtypes.Minter}},
		cmdcfg.NewMultiPrefixBech32AccCodec(), authority)
	bk := bankkeeper.NewBaseKeeper(cdc, runtime.NewKVStoreService(bankKey), ak, map[string]bool{}, authority, log.NewNopLogger())
	return bk, ctx
}

func TestBurnCrosschainGenesisMint(t *testing.T) {
	crosschainAddr := authtypes.NewModuleAddress("crosschain")
	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)
	deposit := sdkmath.NewIntWithDecimal(5, 18)
	transfers := sdkmath.NewInt(780000000001000)

	tests := []struct {
		name       string
		crosschain sdkmath.Int
		burned     sdkmath.Int
	}{
		{"genesis mint", upgrades.CrosschainGenesisMint, upgrades.CrosschainGenesisMint},
		{"genesis mint plus transfers", upgrades.CrosschainGenesisMint.Add(transfers), upgrades.CrosschainGenesisMint},
		{"below genesis mint", upgrades.CrosschainGenesisMint.QuoRaw(2), sdkmath.ZeroInt()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bk, ctx := setupBurnTest(t)
			mint := sdk.NewCoins(sdk.NewCoin(burnTestDenom, tc.crosschain.Add(deposit)))
			require.NoError(t, bk.MintCoins(ctx, burnTestMinter, mint))
			require.NoError(t, bk.SendCoinsFromModuleToAccount(ctx, burnTestMinter, crosschainAddr,
				sdk.NewCoins(sdk.NewCoin(burnTestDenom, tc.crosschain))))
			require.NoError(t, bk.SendCoinsFromModuleToModule(ctx, burnTestMinter, govtypes.ModuleName,
				sdk.NewCoins(sdk.NewCoin(burnTestDenom, deposit))))
			supply := bk.GetSupply(ctx, burnTestDenom).Amount

			burned, err := upgrades.BurnCrosschainGenesisMint(ctx, bk, burnTestDenom)
			require.NoError(t, err)
			require.Equal(t, tc.burned.String(), burned.String())
			require.Equal(t, supply.Sub(tc.burned).String(), bk.GetSupply(ctx, burnTestDenom).Amount.String())
			require.Equal(t, tc.crosschain.Sub(tc.burned).String(), bk.GetBalance(ctx, crosschainAddr, burnTestDenom).Amount.String())
			require.Equal(t, deposit.String(), bk.GetBalance(ctx, govAddr, burnTestDenom).Amount.String())

			burned, err = upgrades.BurnCrosschainGenesisMint(ctx, bk, burnTestDenom)
			require.NoError(t, err)
			require.True(t, burned.IsZero())
		})
	}
}
