package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	pruningtypes "cosmossdk.io/store/pruning/types"
	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/mocachain/moca/v2/app"
	servercfg "github.com/mocachain/moca/v2/server/config"
	srvflags "github.com/mocachain/moca/v2/server/flags"
)

// TestInitAppConfigBakesEVMChainID asserts that initAppConfig threads the EVM
// chain ID into the rendered app.toml config, so `mocad init --chain-id
// moca_<evmid>-<epoch>` writes the correct evm.evm-chain-id with no operator step.
func TestInitAppConfigBakesEVMChainID(t *testing.T) {
	cases := []struct {
		name  string
		evmID uint64
	}{
		{"mainnet", 2288},
		{"testnet", 222888},
		{"devnet", 5151},
		{"unset keeps cosmos/evm default", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := initAppConfig(tc.evmID)
			ac, ok := cfg.(servercfg.AppConfig)
			require.True(t, ok, "unexpected app config type %T", cfg)
			require.Equal(t, tc.evmID, ac.EVM.EVMChainID)
		})
	}
}

// TestEVMChainIDFromChainID covers the init-time derivation of the EVM chain ID
// from --chain-id: any well-formed <name>_<evmid>-<epoch> yields its evmid (the
// prefix need not be "moca", matching types.ParseChainID), while empty, malformed,
// or out-of-range inputs fall back to 0 (cosmos/evm's default) rather than error
// or guess.
func TestEVMChainIDFromChainID(t *testing.T) {
	cases := []struct {
		name    string
		chainID string
		want    uint64
	}{
		{"mainnet chain-id", "moca_2288-1", 2288},
		{"testnet chain-id", "moca_222888-1", 222888},
		{"devnet chain-id", "moca_5151-1", 5151},
		// ParseChainID accepts any lowercase prefix, so a non-moca prefix with the
		// valid <name>_<evmid>-<epoch> shape still derives its evmid.
		{"non-moca prefix, valid shape", "other_2288-1", 2288},
		{"empty (no --chain-id)", "", 0},
		{"malformed (no evmid segment)", "cosmoshub-4", 0},
		{"malformed (no separators)", "not-a-chain-id", 0},
		{"non-numeric evm id", "moca_x-1", 0},
		// 2^64 + 5151: Uint64() would truncate to 5151; guard must return 0 (unset).
		{"out-of-range evm id fails closed", "moca_18446744073709556767-1", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, evmChainIDFromChainID(tc.chainID))
		})
	}
}

// TestGenesisChainID checks the minimal-struct read: it returns chain_id and
// ignores every other (possibly schema-shifted) genesis field, and errors when
// genesis.json is absent so the caller can fall back to --chain-id.
func TestGenesisChainID(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
	genesis := `{"genesis_time":"2020-01-01T00:00:00Z","chain_id":"moca_5151-1","initial_height":"1",` +
		`"app_state":{"evm":{"params":{"some_new_field":true}}},"consensus":{"params":{}}}`
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte(genesis), 0o600))

	got, err := genesisChainID(home)
	require.NoError(t, err)
	require.Equal(t, "moca_5151-1", got)

	_, err = genesisChainID(t.TempDir())
	require.Error(t, err)
}

// TestNewAppAppliesQueryGasLimit checks that newApp passes query-gas-limit
// (app.toml or --query-gas-limit) to baseapp, so the context a gRPC or REST
// query runs in is bounded by it.
func TestNewAppAppliesQueryGasLimit(t *testing.T) {
	// cosmos/evm's process-global chain config can only be set again while it
	// holds the default EVM chain ID, so use that one to keep later app
	// constructions in this test binary working.
	chainID := fmt.Sprintf("moca_%d-1", evmtypes.DefaultEVMChainID)
	appOpts := viper.New()
	appOpts.Set(flags.FlagHome, t.TempDir())
	appOpts.Set(flags.FlagChainID, chainID)
	appOpts.Set(srvflags.EVMChainID, evmtypes.DefaultEVMChainID)
	appOpts.Set(sdkserver.FlagPruning, pruningtypes.PruningOptionDefault)
	appOpts.Set(sdkserver.FlagQueryGasLimit, "50000") // app.toml renders it as a string

	mocaApp, ok := appCreator{}.newApp(log.NewNopLogger(), dbm.NewMemDB(), nil, appOpts).(*app.Moca)
	require.True(t, ok)

	// a query context needs a committed block
	stateBytes, err := json.Marshal(app.NewTestGenesisState(mocaApp))
	require.NoError(t, err)
	_, err = mocaApp.InitChain(&abci.RequestInitChain{
		ChainId:         chainID,
		ConsensusParams: app.DefaultConsensusParams,
		AppStateBytes:   stateBytes,
	})
	require.NoError(t, err)
	_, err = mocaApp.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1})
	require.NoError(t, err)
	_, err = mocaApp.Commit()
	require.NoError(t, err)

	ctx, err := mocaApp.CreateQueryContext(0, false)
	require.NoError(t, err)
	require.Equal(t, uint64(50000), ctx.GasMeter().Limit())
}

// TestHealAppTomlEVMChainID checks that the self-heal writes a valid app.toml with
// the derived value across every shape it must handle — key present, key missing,
// section missing, a commented header, and the quoted table/key forms the reviewer
// flagged — while preserving the operator's other keys and comments.
func TestHealAppTomlEVMChainID(t *testing.T) {
	const (
		marker = "# operator note: keep me"
		keepJR = "json-rpc.enable"
		tailJR = "\n\n[json-rpc]\nenable = true\n"
	)
	cases := []struct {
		name    string
		initial string
		keep    string
	}{
		{"key present as 0", "[evm]\ntracer = \"\"\n" + marker + "\nevm-chain-id = 0" + tailJR, keepJR},
		{"section present, key missing", "[evm]\n" + marker + "\ntracer = \"\"" + tailJR, keepJR},
		{"no evm section (pre-cosmos/evm upgrade)", marker + "\n\n[api]\nenable = true\n", "api.enable"},
		{"header has a trailing comment", "[evm] " + marker + "\ntracer = \"\"" + tailJR, keepJR},
		// reviewer repro 1: a quoted key must be recognized, not duplicated.
		{"quoted key", "[evm]\n" + marker + "\n\"evm-chain-id\" = 0" + tailJR, keepJR},
		// reviewer repro 2: a quoted table name must be recognized, not duplicated.
		{"quoted table name", "[\"evm\"]\n" + marker + "\ntracer = \"\"" + tailJR, keepJR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
			p := filepath.Join(home, "config", "app.toml")
			require.NoError(t, os.WriteFile(p, []byte(tc.initial), 0o600))

			require.NoError(t, healAppTomlEVMChainID(home, 2288))

			out, err := os.ReadFile(p)
			require.NoError(t, err)
			v := viper.New()
			v.SetConfigType("toml")
			require.NoError(t, v.ReadConfig(bytes.NewReader(out)), "healed app.toml must be valid TOML:\n%s", out)
			require.Equal(t, uint64(2288), v.GetUint64("evm.evm-chain-id"))
			require.True(t, v.GetBool(tc.keep), "operator key %q must survive", tc.keep)
			require.Contains(t, string(out), marker, "operator comment must survive")
			require.Equal(t, 1, strings.Count(string(out), "evm-chain-id"), "must not duplicate the key:\n%s", out)
		})
	}

	t.Run("idempotent", func(t *testing.T) {
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
		p := filepath.Join(home, "config", "app.toml")
		require.NoError(t, os.WriteFile(p, []byte("[evm]\nevm-chain-id = 2288\n"), 0o600))
		require.NoError(t, healAppTomlEVMChainID(home, 2288))
		require.NoError(t, healAppTomlEVMChainID(home, 2288))
		out, _ := os.ReadFile(p)
		require.Equal(t, 1, strings.Count(string(out), "evm-chain-id"))
	})

	t.Run("preserves file mode", func(t *testing.T) {
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
		p := filepath.Join(home, "config", "app.toml")
		require.NoError(t, os.WriteFile(p, []byte("[evm]\ntracer = \"\"\n"), 0o600))
		require.NoError(t, os.Chmod(p, 0o600))
		require.NoError(t, healAppTomlEVMChainID(home, 2288))
		fi, err := os.Stat(p)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	})

	t.Run("writes through a symlink", func(t *testing.T) {
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
		realPath := filepath.Join(home, "real-app.toml")
		require.NoError(t, os.WriteFile(realPath, []byte("[evm]\ntracer = \"\"\n"), 0o600))
		link := filepath.Join(home, "config", "app.toml")
		require.NoError(t, os.Symlink(realPath, link))
		require.NoError(t, healAppTomlEVMChainID(home, 2288))
		fi, err := os.Lstat(link)
		require.NoError(t, err)
		require.NotZero(t, fi.Mode()&os.ModeSymlink, "app.toml must stay a symlink")
		out, err := os.ReadFile(realPath)
		require.NoError(t, err)
		require.Contains(t, string(out), "evm-chain-id = 2288")
	})
}
