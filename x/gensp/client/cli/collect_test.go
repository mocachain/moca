package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	testutilcodec "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/gensp/client/cli"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
	spmodule "github.com/mocachain/moca/v2/x/sp"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// collectFixture builds a fresh temp home (config.toml, priv_validator_key.json,
// node_key.json, genesis.json) whose genesis app_state carries a "bank" balance
// for fundingAddr, exactly what CollectSPGenTxsCmd's RunE reads via
// genBalIterator before it will accept a gentx that spends from that address.
func collectFixture(t *testing.T, fundingAddr string, balance sdk.Coin) (home string, encCfg sdktestutil.TestEncodingConfig) {
	t.Helper()

	home = t.TempDir()
	encCfg = testutilcodec.MakeTestEncodingConfig(spmodule.AppModuleBasic{})

	require.NoError(t, genutiltest.ExecInitCmd(nil, home, encCfg.Codec))

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	appGenesis, err := genutiltypes.AppGenesisFromFile(cmtCfg.GenesisFile())
	require.NoError(t, err)

	bankGen := banktypes.GenesisState{
		Params: banktypes.DefaultParams(),
		Balances: []banktypes.Balance{
			{Address: fundingAddr, Coins: sdk.NewCoins(balance)},
		},
	}
	appState := map[string]json.RawMessage{
		banktypes.ModuleName: encCfg.Codec.MustMarshalJSON(&bankGen),
	}
	appStateBz, err := json.Marshal(appState)
	require.NoError(t, err)

	appGenesis.AppState = appStateBz
	require.NoError(t, appGenesis.SaveAs(cmtCfg.GenesisFile()))

	return home, encCfg
}

// collectExecContext wires a cobra command context the same way mocad's own
// root command does at runtime: a *server.Context carrying the CometBFT
// config, and a *client.Context carrying the codec/tx config, both reachable
// from RunE via server.GetServerContextFromCmd / client.GetClientContextFromCmd.
func collectExecContext(t *testing.T, home string, encCfg sdktestutil.TestEncodingConfig) context.Context {
	t.Helper()

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	serverCtx := server.NewContext(viper.New(), cmtCfg, log.NewNopLogger())

	clientCtx := client.Context{}.
		WithCodec(encCfg.Codec).
		WithTxConfig(encCfg.TxConfig).
		WithHomeDir(home).
		WithOutput(io.Discard)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	return ctx
}

// writeGenTxFile builds an (unsigned) MsgCreateStorageProvider gentx and drops
// it in <home>/config/gentx/. CollectTxs only performs stateless validation
// (ValidateAndGetGenTx / ValidateBasic) on genesis transactions, so a real
// signature is not required to exercise it.
func writeGenTxFile(t *testing.T, home string, encCfg sdktestutil.TestEncodingConfig, fundingAddr string, deposit sdk.Coin, name string) {
	t.Helper()

	creator := sample.RandAccAddressHex()
	creatorAddr, err := sdk.AccAddressFromHexUnsafe(creator)
	require.NoError(t, err)
	fundingAccAddr, err := sdk.AccAddressFromHexUnsafe(fundingAddr)
	require.NoError(t, err)
	operatorAddr, err := sdk.AccAddressFromHexUnsafe(sample.RandAccAddressHex())
	require.NoError(t, err)
	sealAddr, err := sdk.AccAddressFromHexUnsafe(sample.RandAccAddressHex())
	require.NoError(t, err)
	approvalAddr, err := sdk.AccAddressFromHexUnsafe(sample.RandAccAddressHex())
	require.NoError(t, err)
	gcAddr, err := sdk.AccAddressFromHexUnsafe(sample.RandAccAddressHex())
	require.NoError(t, err)
	maintenanceAddr, err := sdk.AccAddressFromHexUnsafe(sample.RandAccAddressHex())
	require.NoError(t, err)
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	msg, err := sptypes.NewMsgCreateStorageProvider(
		creatorAddr, operatorAddr, fundingAccAddr, sealAddr, approvalAddr, gcAddr, maintenanceAddr,
		sptypes.NewDescription("sp0", "", "", ""),
		"http://127.0.0.1:9033", deposit,
		sdkmath.LegacyNewDec(100), 100000000, sdkmath.LegacyNewDec(10000),
		blsPubKey, blsProof,
	)
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic())

	txBuilder := encCfg.TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))

	genTxBz, err := encCfg.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
	require.NoError(t, err)

	gentxDir := filepath.Join(home, "config", "gentx")
	require.NoError(t, os.MkdirAll(gentxDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gentxDir, name), genTxBz, 0o600))
}

func TestCollectSPGenTxsCmd_MissingGenesisFile(t *testing.T) {
	home := t.TempDir()
	encCfg := testutilcodec.MakeTestEncodingConfig(spmodule.AppModuleBasic{})

	// A fresh home has config/data directories (from CreateDefaultCometConfig)
	// but no genesis.json yet.
	_, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	ctx := collectExecContext(t, home, encCfg)
	cmd := cli.CollectSPGenTxsCmd(banktypes.GenesisBalancesIterator{}, home)
	cmd.SetArgs([]string{
		"--" + flags.FlagHome, home,
	})

	err = cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read genesis doc from file")
}

func TestCollectSPGenTxsCmd_BadNodeKey(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	home, encCfg := collectFixture(t, fundingAddr, sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))

	// ExecInitCmd already wrote a valid node_key.json; corrupt it so
	// genutil.InitializeNodeValidatorFiles's LoadOrGenNodeKey takes the
	// load-and-fail path instead of generating a fresh key.
	nodeKeyFile := filepath.Join(home, "config", "node_key.json")
	require.NoError(t, os.WriteFile(nodeKeyFile, []byte("not-json"), 0o600))

	ctx := collectExecContext(t, home, encCfg)
	cmd := cli.CollectSPGenTxsCmd(banktypes.GenesisBalancesIterator{}, home)
	cmd.SetArgs([]string{
		"--" + flags.FlagHome, home,
	})

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize node validator files")
}

func TestCollectSPGenTxsCmd_NoGenTxFiles(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	home, encCfg := collectFixture(t, fundingAddr, sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000)))

	// config/gentx exists but is empty: len(appGenTxs) == 0.
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config", "gentx"), 0o755))

	ctx := collectExecContext(t, home, encCfg)
	cmd := cli.CollectSPGenTxsCmd(banktypes.GenesisBalancesIterator{}, home)
	cmd.SetArgs([]string{
		"--" + flags.FlagHome, home,
	})

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "there must be at least one genesis tx")
}

func TestCollectSPGenTxsCmd_Success(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	deposit := sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000))
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))

	home, encCfg := collectFixture(t, fundingAddr, balance)
	writeGenTxFile(t, home, encCfg, fundingAddr, deposit, "gentx-sp0.json")

	ctx := collectExecContext(t, home, encCfg)
	cmd := cli.CollectSPGenTxsCmd(banktypes.GenesisBalancesIterator{}, home)
	cmd.SetArgs([]string{
		"--" + flags.FlagHome, home,
	})

	require.NoError(t, cmd.ExecuteContext(ctx))

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	appGenesis, err := genutiltypes.AppGenesisFromFile(cmtCfg.GenesisFile())
	require.NoError(t, err)

	var appState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(appGenesis.AppState, &appState))
	require.Contains(t, appState, gensptypes.ModuleName)

	var genState gensptypes.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(appState[gensptypes.ModuleName], &genState))
	require.Len(t, genState.GenspTxs, 1)
}
