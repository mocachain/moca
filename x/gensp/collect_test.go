package gensp_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cosmossdk.io/math"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankexported "github.com/cosmos/cosmos-sdk/x/bank/exported"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/gensp"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

const (
	testChainID    = "test-chain"
	testSPEndpoint = "http://127.0.0.1:9033"
	testGenTxFile  = "gentx.json"
)

// testDeposit is the fixed deposit MsgCreateStorageProvider fixtures request;
// individual test cases vary the genesis account's balance instead.
var testDeposit = sdk.NewInt64Coin(sptypes.DefaultDepositDenom, 100)

// failJSONEncodeCfg wraps a real TxConfig but forces TxJSONEncoder to fail,
// exercising GenAppStateFromConfig's SetGenTxsInAppGenesisState-failure
// propagation branch after CollectTxs (which only needs TxJSONDecoder) has
// already succeeded.
type failJSONEncodeCfg struct {
	client.TxConfig
}

func (failJSONEncodeCfg) TxJSONEncoder() sdk.TxEncoder {
	return func(sdk.Tx) ([]byte, error) { return nil, errors.New("boom") }
}

type doNothingUnmarshalJSON struct {
	codec.JSONCodec
}

func (dnj *doNothingUnmarshalJSON) UnmarshalJSON(_ []byte, _ proto.Message) error {
	return nil
}

type doNothingIterator struct {
	types.GenesisBalancesIterator
}

func (dni *doNothingIterator) IterateGenesisBalances(_ codec.JSONCodec, _ map[string]json.RawMessage, _ func(bankexported.GenesisBalance) bool) {
}

// Ensures that CollectTx correctly traverses directories and won't error out on encountering
// a directory during traversal of the first level. See issue https://github.com/cosmos/cosmos-sdk/issues/6788.
func TestCollectTxsHandlesDirectories(t *testing.T) {
	testDir, err := os.MkdirTemp(os.TempDir(), "testCollectTxs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)

	// 1. We'll insert a directory as the first element before JSON file.
	subDirPath := filepath.Join(testDir, "_adir")
	if err := os.MkdirAll(subDirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1b. A non-".json" file must be skipped too, not treated as a gentx.
	if err := os.WriteFile(filepath.Join(testDir, "README"), []byte("not a gentx"), 0o600); err != nil {
		t.Fatal(err)
	}

	txDecoder := sdk.TxDecoder(func(_ []byte) (sdk.Tx, error) {
		return nil, nil
	})

	// 2. Ensure that we don't encounter any error traversing the directory.
	srvCtx := server.NewDefaultContext()
	_ = srvCtx
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	genesis := &types.AppGenesis{
		ChainID:  "test-chain",
		AppState: []byte("{}"),
	}
	balItr := new(doNothingIterator)

	dnc := &doNothingUnmarshalJSON{cdc}
	if _, _, err := genutil.CollectTxs(dnc, txDecoder, "foo", testDir, genesis, balItr, types.DefaultMessageValidator); err != nil {
		t.Fatal(err)
	}

	// The above exercises upstream genutil's CollectTxs, not gensp's own
	// (narrower-signature) fork of it - call gensp.CollectTxs against the
	// same fixture so this regression test covers the module under test.
	if _, _, err := gensp.CollectTxs(dnc, txDecoder, "foo", testDir, genesis, balItr); err != nil {
		t.Fatal(err)
	}
}

// buildStorageProviderMsg returns a MsgCreateStorageProvider that passes
// ValidateBasic, with fundingAddr as its FundingAddress - the field
// CollectTxs cross-checks against genesis balances.
func buildStorageProviderMsg(fundingAddr string) *sptypes.MsgCreateStorageProvider {
	addr := sample.RandAccAddressHex()
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	return &sptypes.MsgCreateStorageProvider{
		Creator:            addr,
		SpAddress:          addr,
		FundingAddress:     fundingAddr,
		SealAddress:        addr,
		ApprovalAddress:    addr,
		GcAddress:          addr,
		MaintenanceAddress: addr,
		Description:        sptypes.NewDescription("moniker", "identity", "website", "details"),
		Endpoint:           testSPEndpoint,
		Deposit:            testDeposit,
		ReadPrice:          math.LegacyZeroDec(),
		StorePrice:         math.LegacyZeroDec(),
		BlsKey:             blsKey,
		BlsProof:           blsProof,
	}
}

// buildBankAppState wraps balances into the app_state map shape CollectTxs
// and GenAppStateFromConfig expect under the bank module key.
func buildBankAppState(cdc codec.JSONCodec, balances ...banktypes.Balance) map[string]json.RawMessage {
	bankGenesis := banktypes.GenesisState{
		Params:   banktypes.Params{DefaultSendEnabled: true},
		Balances: balances,
	}
	return map[string]json.RawMessage{
		banktypes.ModuleName: cdc.MustMarshalJSON(&bankGenesis),
	}
}

// writeGenTxFile JSON-encodes msg as an (unsigned) gentx and writes it into
// dir; CollectTxs only stateless-validates gentx messages, so no signature
// is needed.
func writeGenTxFile(t *testing.T, dir string, txCfg client.TxConfig, msg sdk.Msg) {
	txBuilder := txCfg.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))
	bz, err := txCfg.TxJSONEncoder()(txBuilder.GetTx())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, testGenTxFile), bz, 0o600))
}

// TestCollectTxs covers CollectTxs' error branches (bad genTxsDir, malformed
// app state, an undecodable gentx, missing/insufficient funding balance) and
// its happy path, none of which the pre-existing directory-traversal fixture
// (TestCollectTxsHandlesDirectories) reaches.
func TestCollectTxs(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(genutil.AppModuleBasic{})
	banktypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	sptypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	cdc := encCfg.Codec

	validAddr := sample.RandAccAddressHex()

	testCases := []struct {
		name          string
		setup         func(t *testing.T) (genTxsDir string, appGenesis *types.AppGenesis)
		wantErrSubstr string
	}{
		{
			name: "genTxsDir does not exist",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				appState, err := json.Marshal(buildBankAppState(cdc,
					banktypes.Balance{Address: validAddr, Coins: sdk.Coins{testDeposit}}))
				require.NoError(t, err)
				return filepath.Join(t.TempDir(), "missing"), &types.AppGenesis{ChainID: testChainID, AppState: appState}
			},
			wantErrSubstr: "no such file or directory",
		},
		{
			name: "malformed app state json",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				return t.TempDir(), &types.AppGenesis{ChainID: testChainID, AppState: []byte("not json")}
			},
			wantErrSubstr: "invalid character",
		},
		{
			name: "gentx file fails to decode",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, testGenTxFile), []byte("{not valid json"), 0o600))
				return dir, &types.AppGenesis{ChainID: testChainID, AppState: []byte("{}")}
			},
			wantErrSubstr: "failed to decode gentx",
		},
		{
			name: "gentx file is unreadable",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				dir := t.TempDir()
				// A dangling symlink named *.json passes the extension
				// filter but fails to open; unlike a permission-bit fixture
				// this also fails when tests run as root.
				require.NoError(t, os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, testGenTxFile)))
				return dir, &types.AppGenesis{ChainID: testChainID, AppState: []byte("{}")}
			},
			wantErrSubstr: "no such file or directory",
		},
		{
			name: "funding address balance missing from genesis",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				dir := t.TempDir()
				writeGenTxFile(t, dir, encCfg.TxConfig, buildStorageProviderMsg(validAddr))
				appState, err := json.Marshal(buildBankAppState(cdc))
				require.NoError(t, err)
				return dir, &types.AppGenesis{ChainID: testChainID, AppState: appState}
			},
			wantErrSubstr: "balance not in genesis state",
		},
		{
			name: "funding address normalized form missing from genesis",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				dir := t.TempDir()
				lower := strings.ToLower(validAddr)
				writeGenTxFile(t, dir, encCfg.TxConfig, buildStorageProviderMsg(lower))
				appState, err := json.Marshal(buildBankAppState(cdc,
					banktypes.Balance{Address: lower, Coins: sdk.Coins{testDeposit}}))
				require.NoError(t, err)
				return dir, &types.AppGenesis{ChainID: testChainID, AppState: appState}
			},
			wantErrSubstr: "balance not in genesis state",
		},
		{
			name: "insufficient deposit funds",
			setup: func(t *testing.T) (string, *types.AppGenesis) {
				dir := t.TempDir()
				writeGenTxFile(t, dir, encCfg.TxConfig, buildStorageProviderMsg(validAddr))
				short := sdk.NewInt64Coin(sptypes.DefaultDepositDenom, 1)
				appState, err := json.Marshal(buildBankAppState(cdc,
					banktypes.Balance{Address: validAddr, Coins: sdk.Coins{short}}))
				require.NoError(t, err)
				return dir, &types.AppGenesis{ChainID: testChainID, AppState: appState}
			},
			wantErrSubstr: "insufficient fund for delegation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			genTxsDir, appGenesis := tc.setup(t)

			_, _, err := gensp.CollectTxs(
				cdc, encCfg.TxConfig.TxJSONDecoder(), "moniker", genTxsDir, appGenesis, banktypes.GenesisBalancesIterator{},
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErrSubstr)
		})
	}

	t.Run("happy path collects the gentx", func(t *testing.T) {
		dir := t.TempDir()
		writeGenTxFile(t, dir, encCfg.TxConfig, buildStorageProviderMsg(validAddr))
		appState, err := json.Marshal(buildBankAppState(cdc,
			banktypes.Balance{Address: validAddr, Coins: sdk.Coins{testDeposit}}))
		require.NoError(t, err)
		appGenesis := &types.AppGenesis{ChainID: testChainID, AppState: appState}

		appGenTxs, persistentPeers, err := gensp.CollectTxs(
			cdc, encCfg.TxConfig.TxJSONDecoder(), "moniker", dir, appGenesis, banktypes.GenesisBalancesIterator{},
		)
		require.NoError(t, err)
		require.Len(t, appGenTxs, 1)
		require.Empty(t, persistentPeers)
	})
}

// TestGenAppStateFromConfig covers the no-gentx error, a propagated
// CollectTxs failure, and the happy path where a genesis.json gets written
// with a populated gensp module entry.
func TestGenAppStateFromConfig(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(genutil.AppModuleBasic{})
	banktypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	sptypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	cdc := encCfg.Codec
	validAddr := sample.RandAccAddressHex()

	newConfig := func(t *testing.T) *cfg.Config {
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config", "gentx"), 0o755))
		config := cfg.DefaultConfig()
		config.SetRoot(home)
		config.Moniker = "test-moniker"
		return config
	}

	initConfigFor := func(config *cfg.Config) types.InitConfig {
		return types.NewInitConfig(testChainID, filepath.Join(config.RootDir, "config", "gentx"), "test-node", nil)
	}

	t.Run("no gentx files returns an error", func(t *testing.T) {
		config := newConfig(t)
		appState, err := json.Marshal(buildBankAppState(cdc))
		require.NoError(t, err)
		genesis := &types.AppGenesis{ChainID: testChainID, AppState: appState, Consensus: &types.ConsensusGenesis{}}

		_, err = gensp.GenAppStateFromConfig(
			cdc, encCfg.TxConfig, config, initConfigFor(config), genesis, banktypes.GenesisBalancesIterator{},
		)
		require.ErrorContains(t, err, "there must be at least one genesis tx")
	})

	t.Run("a CollectTxs failure propagates", func(t *testing.T) {
		config := newConfig(t)
		genesis := &types.AppGenesis{ChainID: testChainID, AppState: []byte("not json")}

		_, err := gensp.GenAppStateFromConfig(
			cdc, encCfg.TxConfig, config, initConfigFor(config), genesis, banktypes.GenesisBalancesIterator{},
		)
		require.Error(t, err)
	})

	t.Run("a SetGenTxsInAppGenesisState failure propagates", func(t *testing.T) {
		config := newConfig(t)
		writeGenTxFile(t, filepath.Join(config.RootDir, "config", "gentx"), encCfg.TxConfig, buildStorageProviderMsg(validAddr))
		appStateBz, err := json.Marshal(buildBankAppState(cdc,
			banktypes.Balance{Address: validAddr, Coins: sdk.Coins{testDeposit}}))
		require.NoError(t, err)
		genesis := &types.AppGenesis{ChainID: testChainID, AppState: appStateBz, Consensus: &types.ConsensusGenesis{}}

		// CollectTxs only needs TxJSONDecoder (unaffected), so this reaches
		// GenAppStateFromConfig's own SetGenTxsInAppGenesisState call before failing.
		_, err = gensp.GenAppStateFromConfig(
			cdc, failJSONEncodeCfg{encCfg.TxConfig}, config, initConfigFor(config), genesis, banktypes.GenesisBalancesIterator{},
		)
		require.Error(t, err)
	})

	t.Run("happy path writes the genesis file with a populated gensp entry", func(t *testing.T) {
		config := newConfig(t)
		writeGenTxFile(t, filepath.Join(config.RootDir, "config", "gentx"), encCfg.TxConfig, buildStorageProviderMsg(validAddr))
		appStateBz, err := json.Marshal(buildBankAppState(cdc,
			banktypes.Balance{Address: validAddr, Coins: sdk.Coins{testDeposit}}))
		require.NoError(t, err)
		genesis := &types.AppGenesis{ChainID: testChainID, AppState: appStateBz, Consensus: &types.ConsensusGenesis{}}

		appState, err := gensp.GenAppStateFromConfig(
			cdc, encCfg.TxConfig, config, initConfigFor(config), genesis, banktypes.GenesisBalancesIterator{},
		)
		require.NoError(t, err)

		var appStateMap map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(appState, &appStateMap))
		require.NotEmpty(t, appStateMap[gensptypes.ModuleName])

		var genState gensptypes.GenesisState
		require.NoError(t, cdc.UnmarshalJSON(appStateMap[gensptypes.ModuleName], &genState))
		require.Len(t, genState.GenspTxs, 1)

		require.FileExists(t, config.GenesisFile())
	})
}
