package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	testutilcodec "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/gensp/client/cli"
	spmodule "github.com/mocachain/moca/v2/x/sp"
	spcli "github.com/mocachain/moca/v2/x/sp/client/cli"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// gentxTestChainID mirrors the repo-wide test chain ID convention
// (sdk/client/test.TestChainID): moca's EIP-155 signer derives a numeric EVM
// chain ID from this string, so it must follow the "<name>_<number>-<version>"
// shape rather than an arbitrary label.
const gentxTestChainID = "moca_5151-1"

// gentxFixture builds a fresh temp home (config.toml, priv_validator_key.json,
// node_key.json, genesis.json) whose genesis app_state carries a "bank"
// balance for fundingAddr and a default "staking" section, exactly what
// SPGenTxCmd's RunE reads: genutil.ValidateAccountInGenesis unmarshals the
// staking genesis for the bond denom and walks the bank balances looking for
// fundingAddr.
func gentxFixture(t *testing.T, fundingAddr string, balance sdk.Coin) (home string, encCfg sdktestutil.TestEncodingConfig) {
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
	stakingGen := stakingtypes.DefaultGenesisState()

	appState := map[string]json.RawMessage{
		banktypes.ModuleName:    encCfg.Codec.MustMarshalJSON(&bankGen),
		stakingtypes.ModuleName: encCfg.Codec.MustMarshalJSON(stakingGen),
	}
	appStateBz, err := json.Marshal(appState)
	require.NoError(t, err)

	appGenesis.AppState = appStateBz
	require.NoError(t, appGenesis.SaveAs(cmtCfg.GenesisFile()))

	return home, encCfg
}

// gentxExecContext wires a cobra command context the same way mocad's own
// root command does at runtime: a *server.Context carrying the CometBFT
// config, and a *client.Context carrying the codec/tx config/keyring, both
// reachable from RunE via server.GetServerContextFromCmd /
// client.GetClientTxContext.
func gentxExecContext(t *testing.T, home string, encCfg sdktestutil.TestEncodingConfig, kr keyring.Keyring) context.Context {
	t.Helper()

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	serverCtx := server.NewContext(viper.New(), cmtCfg, log.NewNopLogger())

	clientCtx := client.Context{}.
		WithCodec(encCfg.Codec).
		WithTxConfig(encCfg.TxConfig).
		WithHomeDir(home).
		WithKeyring(kr).
		WithOutput(io.Discard)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	return ctx
}

// newTestKey adds an eth_secp256k1 key named "sp0" to kr (moca signs with the
// Ethereum curve; SIGN_MODE_EIP_712 is the CLI's default and needs it) and
// returns its derived address. Every SPGenTxCmd test uses "sp0" as both the
// keyring uid and the [key_name] positional argument.
func newTestKey(t *testing.T, kr keyring.Keyring) sdk.AccAddress {
	t.Helper()

	record, _, err := kr.NewMnemonic("sp0", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.EthSecp256k1)
	require.NoError(t, err)

	addr, err := record.GetAddress()
	require.NoError(t, err)
	return addr
}

// spGenTxArgs assembles a full, valid flag set for SPGenTxCmd around the
// given creator/funding addresses, using SIGN_MODE_DIRECT (rather than the
// CLI's EIP-712 default) so the fixture doesn't depend on the caller
// registering EIP-712-specific globals beyond what encoding.MakeConfig sets
// up.
func spGenTxArgs(creator, funding string) []string {
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	return []string{
		fmt.Sprintf("--%s=%s", flags.FlagChainID, gentxTestChainID),
		fmt.Sprintf("--%s=%s", flags.FlagSignMode, flags.SignModeDirect),
		fmt.Sprintf("--%s=%s", spcli.FlagCreator, creator),
		fmt.Sprintf("--%s=%s", spcli.FlagOperatorAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagFundingAddress, funding),
		fmt.Sprintf("--%s=%s", spcli.FlagSealAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagApprovalAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagGcAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagMaintenanceAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagBlsPubKey, blsPubKey),
		fmt.Sprintf("--%s=%s", spcli.FlagBlsProof, blsProof),
		fmt.Sprintf("--%s=%s", spcli.FlagMoniker, "sp0"),
		fmt.Sprintf("--%s=%s", spcli.FlagEndpoint, "http://127.0.0.1:9033"),
	}
}

func TestSPGenTxCmd_Success(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	deposit := sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000))
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))

	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", deposit.String(),
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	require.NoError(t, cmd.ExecuteContext(ctx))

	entries, err := os.ReadDir(filepath.Join(home, "config", "gentx"))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	gentxBz, err := os.ReadFile(filepath.Join(home, "config", "gentx", entries[0].Name()))
	require.NoError(t, err)

	genTx, err := encCfg.TxConfig.TxJSONDecoder()(gentxBz)
	require.NoError(t, err)
	msgs := genTx.GetMsgs()
	require.Len(t, msgs, 1)
	createMsg, ok := msgs[0].(*sptypes.MsgCreateStorageProvider)
	require.True(t, ok)
	require.Equal(t, fundingAddr, createMsg.FundingAddress)
	require.Equal(t, creatorAddr.String(), createMsg.Creator)
	require.Equal(t, deposit, createMsg.Deposit)

	// The file must carry a real signature, not just an unsigned envelope.
	var rawTx map[string]interface{}
	require.NoError(t, json.Unmarshal(gentxBz, &rawTx))
	sigs, ok := rawTx["signatures"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, sigs)
}

func TestSPGenTxCmd_OutputDocumentFlag(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	deposit := sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000))
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))

	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	outputDocument := filepath.Join(t.TempDir(), "gentx-sp0.json")

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", deposit.String(),
		"--" + flags.FlagHome, home,
		"--" + flags.FlagOutputDocument, outputDocument,
		"--" + spcli.FlagNodeID, "custom-node-id",
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	require.NoError(t, cmd.ExecuteContext(ctx))
	require.FileExists(t, outputDocument)

	// The default <home>/config/gentx directory must be untouched.
	_, err := os.Stat(filepath.Join(home, "config", "gentx"))
	require.True(t, os.IsNotExist(err))
}

func TestSPGenTxCmd_OutputDocumentAlreadyExists(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	deposit := sdk.NewCoin(bondDenom, sdkmath.NewInt(1_000_000))
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))

	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	outputDocument := filepath.Join(t.TempDir(), "gentx-sp0.json")
	require.NoError(t, os.WriteFile(outputDocument, []byte("{}"), 0o600))

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", deposit.String(),
		"--" + flags.FlagHome, home,
		"--" + flags.FlagOutputDocument, outputDocument,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write signed gen tx")
}

func TestSPGenTxCmd_MissingGenesisFile(t *testing.T) {
	home := t.TempDir()
	encCfg := testutilcodec.MakeTestEncodingConfig(spmodule.AppModuleBasic{})

	_, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + sdk.DefaultBondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), sample.RandAccAddressHex())...)
	cmd.SetArgs(args)

	err = cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read genesis doc file")
}

func TestSPGenTxCmd_UnknownKey(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	// No key named "sp0" is ever added to kr.

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(sample.RandAccAddressHex(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to fetch 'sp0' from the keyring")
}

func TestSPGenTxCmd_BadAmount(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "not-a-coin",
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse coins")
}

func TestSPGenTxCmd_FundingAddressNotInGenesis(t *testing.T) {
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	// The bank balance in genesis belongs to a different address than the
	// one passed via --funding-address below.
	home, encCfg := gentxFixture(t, sample.RandAccAddressHex(), balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), sample.RandAccAddressHex())...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate account in genesis")
}

func TestSPGenTxCmd_InvalidCreateConfig(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	// A malformed bls-pub-key makes PrepareConfigForTxCreateStorageProvider
	// itself fail, before a message is ever built. Appended last so it wins
	// over the valid one spGenTxArgs already set (pflag keeps the last value).
	args = append(args, "--"+spcli.FlagBlsPubKey, "not-a-valid-bls-key")
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error creating configuration to create storage provider msg")
}

func TestSPGenTxCmd_InvalidMsg(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	// An empty --endpoint makes ValidateEndpointURL reject the resulting
	// MsgCreateStorageProvider inside RunE's own ValidateBasic check.
	args := []string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
		fmt.Sprintf("--%s=%s", flags.FlagChainID, gentxTestChainID),
		fmt.Sprintf("--%s=%s", flags.FlagSignMode, flags.SignModeDirect),
		fmt.Sprintf("--%s=%s", spcli.FlagCreator, creatorAddr.String()),
		fmt.Sprintf("--%s=%s", spcli.FlagOperatorAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagFundingAddress, fundingAddr),
		fmt.Sprintf("--%s=%s", spcli.FlagSealAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagApprovalAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagGcAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagMaintenanceAddress, sample.RandAccAddressHex()),
		fmt.Sprintf("--%s=%s", spcli.FlagMoniker, "sp0"),
		fmt.Sprintf("--%s=%s", spcli.FlagEndpoint, ""),
	}
	blsPubKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	args = append(args,
		fmt.Sprintf("--%s=%s", spcli.FlagBlsPubKey, blsPubKey),
		fmt.Sprintf("--%s=%s", spcli.FlagBlsProof, blsProof),
	)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid endpoint")
}

func TestSPGenTxCmd_ClientTxContextError(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	// An unparsable --fee-payer makes client.GetClientTxContext itself fail,
	// before RunE reaches any gensp/sp-specific logic.
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
		"--" + flags.FlagFeePayer, "not-a-hex-address",
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
}

func TestSPGenTxCmd_BadNodeKey(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	// gentxFixture (via ExecInitCmd) already wrote a valid node_key.json;
	// corrupt it so genutil.InitializeNodeValidatorFiles's LoadOrGenNodeKey
	// takes the load-and-fail path instead of generating a fresh key.
	nodeKeyFile := filepath.Join(home, "config", "node_key.json")
	require.NoError(t, os.WriteFile(nodeKeyFile, []byte("not-json"), 0o600))

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize node validator files")
}

func TestSPGenTxCmd_InvalidGenesisAppState(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	appGenesis, err := genutiltypes.AppGenesisFromFile(cmtCfg.GenesisFile())
	require.NoError(t, err)
	// A JSON array is a syntactically valid standalone value (so writing it
	// back out via SaveAs, and later re-reading the genesis file, both
	// succeed), but it isn't a JSON object and so cannot unmarshal into the
	// map[string]json.RawMessage RunE expects.
	appGenesis.AppState = json.RawMessage(`[]`)
	require.NoError(t, appGenesis.SaveAs(cmtCfg.GenesisFile()))

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err = cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal genesis state")
}

func TestSPGenTxCmd_InvalidGenesisState(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))

	home := t.TempDir()
	encCfg := testutilcodec.MakeTestEncodingConfig(spmodule.AppModuleBasic{})
	require.NoError(t, genutiltest.ExecInitCmd(nil, home, encCfg.Codec))

	cmtCfg, err := genutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)
	appGenesis, err := genutiltypes.AppGenesisFromFile(cmtCfg.GenesisFile())
	require.NoError(t, err)

	// Deliberately omit the "staking" section: staking.AppModuleBasic{}'s own
	// ValidateGenesis, run below via mbm, unmarshals a nil json.RawMessage and
	// fails before RunE ever reaches genutil.ValidateAccountInGenesis (which
	// would otherwise need a well-formed staking genesis to read the bond
	// denom from).
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

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	mbm := module.NewBasicManager(staking.AppModuleBasic{})
	cmd := cli.SPGenTxCmd(mbm, encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err = cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate genesis state")
}

func TestSPGenTxCmd_OfflineMissingAccountFlags(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
		"--" + flags.FlagOffline,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account-number and sequence must be set in offline mode")
}

func TestSPGenTxCmd_OfflineAutoGas(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	// Offline + auto-gas is the one combination NewFactoryCLI accepts (given
	// explicit account-number/sequence) but PrintUnsignedTx itself refuses,
	// since estimating gas needs a live node.
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
		"--" + flags.FlagOffline,
		"--" + flags.FlagAccountNumber, "0",
		"--" + flags.FlagSequence, "0",
		"--" + flags.FlagGas, "auto",
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to print unsigned std tx")
}

func TestSPGenTxCmd_MultiDenomAmount(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	// sdk.ParseCoinsNormalized (used for the genesis-balance check just
	// above) accepts a multi-denom coin list, but BuildCreateStorageProviderMsg
	// re-parses the very same raw string with the single-coin
	// ParseCoinNormalized, which rejects it.
	amount := fmt.Sprintf("100%s,1other", bondDenom)
	args := append([]string{
		"sp0", amount,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to build create-validator message")
}

func TestSPGenTxCmd_CreatorMismatch(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	kr := keyring.NewInMemory(encCfg.Codec)
	// "sp0" is a real keyring key, but --creator below names a different,
	// unrelated address: SignTx's isTxSigner check must reject the mismatch.
	newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(sample.RandAccAddressHex(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to sign std tx")
}

func TestSPGenTxCmd_GentxDirIsFile(t *testing.T) {
	fundingAddr := sample.RandAccAddressHex()
	bondDenom := sdk.DefaultBondDenom
	balance := sdk.NewCoin(bondDenom, sdkmath.NewInt(10_000_000))
	home, encCfg := gentxFixture(t, fundingAddr, balance)

	// Block the default output directory: makeOutputFilepath's EnsureDir
	// must fail when "config/gentx" already exists as a regular file.
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "gentx"), []byte("not-a-dir"), 0o600))

	kr := keyring.NewInMemory(encCfg.Codec)
	creatorAddr := newTestKey(t, kr)

	ctx := gentxExecContext(t, home, encCfg, kr)
	cmd := cli.SPGenTxCmd(module.NewBasicManager(), encCfg.TxConfig, banktypes.GenesisBalancesIterator{}, home)
	args := append([]string{
		"sp0", "1000000" + bondDenom,
		"--" + flags.FlagHome, home,
	}, spGenTxArgs(creatorAddr.String(), fundingAddr)...)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create output file path")
}
