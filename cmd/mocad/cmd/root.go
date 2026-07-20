package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/creachadair/tomledit/transform"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"cosmossdk.io/log"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/store"
	"cosmossdk.io/store/snapshots"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	clientcfg "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	mocaclient "github.com/mocachain/moca/v2/client"
	"github.com/mocachain/moca/v2/client/debug"
	mocaserver "github.com/mocachain/moca/v2/server"
	servercfg "github.com/mocachain/moca/v2/server/config"
	srvflags "github.com/mocachain/moca/v2/server/flags"

	"github.com/mocachain/moca/v2/app"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	mocakr "github.com/mocachain/moca/v2/crypto/keyring"
	mocatypes "github.com/mocachain/moca/v2/types"
	gensputilcli "github.com/mocachain/moca/v2/x/gensp/client/cli"
)

const EnvPrefix = "EVMOS"

type emptyAppOptions struct{}

func (ao emptyAppOptions) Get(_ string) interface{} { return nil }

var AppConfig = servercfg.NewDefaultAppConfig(cmdcfg.BaseDenom)

func ParseAppConfigInPlace(cmd *cobra.Command) error {
	newViper := viper.New()

	// Configure the viper instance
	if err := newViper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}
	if err := newViper.BindPFlags(cmd.PersistentFlags()); err != nil {
		return err
	}

	homeDir := newViper.GetString(flags.FlagHome)

	newViper.SetConfigName("app")
	newViper.SetConfigType("toml")
	newViper.AddConfigPath(homeDir)
	newViper.AddConfigPath(filepath.Join(homeDir, "config"))

	// If a config file is found, read it in.
	if err := newViper.ReadInConfig(); err != nil {
		return err
	}

	AppConfig = servercfg.NewDefaultAppConfig(cmdcfg.BaseDenom)
	err := newViper.Unmarshal(AppConfig)
	if err != nil {
		return err
	}

	srvCfg := serverconfig.DefaultConfig()
	err = newViper.Unmarshal(srvCfg)
	if err != nil {
		return err
	}
	AppConfig.Config = *srvCfg

	return nil
}

// NewRootCmd creates a new root command for mocad. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, sdktestutil.TestEncodingConfig) {
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	// and the CLI options for the modules
	// add keyring to autocli opts
	tempApp := app.NewMoca(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil, true, nil,
		tempDir(app.DefaultNodeHome),
		AppConfig,
		emptyAppOptions{},
	)
	encodingConfig := sdktestutil.TestEncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.GetTxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithBroadcastMode(flags.FlagBroadcastMode).
		WithHomeDir(app.DefaultNodeHome).
		WithKeyringOptions(mocakr.Option()).
		WithViper(EnvPrefix).
		WithLedgerHasProtobuf(true)

	rootCmd := &cobra.Command{
		Use:   app.Name,
		Short: "Moca Daemon",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = clientcfg.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			// This needs to go after ReadFromClientConfig, as that function
			// sets the RPC client needed for SIGN_MODE_TEXTUAL. This sign mode
			// is only available if the client is online.
			if !initClientCtx.Offline {
				enabledSignModes := append(tx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
				txConfigOpts := tx.ConfigOptions{
					EnabledSignModes:           enabledSignModes,
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
					// cosmos-sdk v0.53 requires a signing context with address
					// codecs; without it, NewTxConfigWithOptions falls back to
					// empty signing options and errors with "address codec is
					// required". Reuse the one already built into the registry.
					SigningContext: initClientCtx.InterfaceRegistry.SigningContext(),
				}
				txConfig, err := tx.NewTxConfigWithOptions(
					initClientCtx.Codec,
					txConfigOpts,
				)
				if err != nil {
					return err
				}

				initClientCtx = initClientCtx.WithTxConfig(txConfig)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			// override the app and tendermint configuration.
			//
			// Derive the EIP-155 EVM chain ID from --chain-id so a fresh
			// `mocad init` bakes the right evm.evm-chain-id into app.toml with no
			// operator step.
			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			customAppTemplate, customAppConfig := initAppConfig(evmChainIDFromChainID(chainID))
			customTMConfig := initTendermintConfig()

			err = sdkserver.InterceptConfigsPreRunHandler(
				cmd, customAppTemplate, customAppConfig, customTMConfig,
			)
			if err != nil {
				return err
			}

			return ParseAppConfigInPlace(cmd)
		},
	}

	cfg := sdk.GetConfig()
	cfg.Seal()

	a := appCreator{encodingConfig}

	gentxModule := tempApp.BasicModuleManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)

	rootCmd.AddCommand(
		mocaclient.ValidateChainID(
			InitCmd(tempApp.BasicModuleManager, app.DefaultNodeHome),
		),
		genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, gentxModule.GenTxValidator),
		MigrateGenesisCmd(),
		genutilcli.GenTxCmd(tempApp.BasicModuleManager, tempApp.GetTxConfig(), banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome),
		genutilcli.ValidateGenesisCmd(tempApp.BasicModuleManager),
		AddGenesisAccountCmd(app.DefaultNodeHome),
		gensputilcli.SPGenTxCmd(
			tempApp.BasicModuleManager,
			tempApp.GetTxConfig(),
			banktypes.GenesisBalancesIterator{},
			app.DefaultNodeHome),
		gensputilcli.CollectSPGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		NewTestnetCmd(tempApp.BasicModuleManager, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(a.newApp, app.DefaultNodeHome),
		snapshot.Cmd(a.newApp),
	)

	mocaserver.AddCommands(
		rootCmd,
		mocaserver.NewDefaultStartOptions(a.newApp, app.DefaultNodeHome),
		a.appExport,
		addModuleInitFlags,
	)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		sdkserver.StatusCommand(),
		queryCommand(),
		txCommand(),
		mocaclient.KeyCommands(app.DefaultNodeHome),
	)
	rootCmd, err := srvflags.AddTxFlags(rootCmd)
	if err != nil {
		panic(err)
	}

	autoCliOpts := tempApp.AutoCliOpts()
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd, encodingConfig
}

// addModuleInitFlags is intentionally a no-op after the x/crisis module was
// removed. The hook is still wired through mocaserver.AddCommands because that
// signature mandates a non-nil types.ModuleInitFlags callback; future modules
// that need to inject CLI flags into the start command can register them here.
func addModuleInitFlags(_ *cobra.Command) {
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		sdkserver.QueryBlockCmd(),
		authcmd.QueryTxCmd(),
		sdkserver.QueryBlockResultsCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// evmChainIDFromChainID derives the EIP-155 EVM chain ID from a cosmos chain-id
// (any well-formed <name>_<evmid>-<epoch>, per types.ParseChainID — not
// moca-prefix-specific) for init-time app.toml rendering. It returns 0 for an
// empty, malformed, or out-of-range chain-id, so `mocad init` with an
// unparseable chain-id keeps the existing default rather than guessing.
func evmChainIDFromChainID(chainID string) uint64 {
	if chainID == "" {
		return 0
	}
	id, err := mocatypes.ParseChainID(chainID)
	if err != nil {
		return 0
	}
	// ParseChainID accepts an unbounded integer; leave the value unset rather
	// than truncating an out-of-range id into a wrong EVM chain ID.
	if !id.IsUint64() {
		return 0
	}
	return id.Uint64()
}

// initAppConfig helps to override default appConfig template and configs.
// evmChainID is baked into the rendered app.toml (evm.evm-chain-id) so a fresh
// `mocad init` needs no operator edit; 0 leaves the default in place.
func initAppConfig(evmChainID uint64) (string, interface{}) {
	customAppTemplate, customAppConfig := servercfg.NewAppConfig(cmdcfg.BaseDenom)

	srvCfg, ok := customAppConfig.(servercfg.AppConfig)
	if !ok {
		panic(fmt.Errorf("unknown app config type %T", customAppConfig))
	}

	srvCfg.StateSync.SnapshotInterval = 5000
	srvCfg.StateSync.SnapshotKeepRecent = 2
	srvCfg.IAVLDisableFastNode = false
	srvCfg.EVM.EVMChainID = evmChainID

	return customAppTemplate, srvCfg
}

// genesisChainID reads only the chain_id from <home>/config/genesis.json,
// decoding a minimal struct rather than the full genesis document so it stays
// robust to genesis schema changes across upgrades — only chain_id is needed to
// derive the EVM chain ID, and it is the field CometBFT validates against.
func genesisChainID(home string) (string, error) {
	data, err := os.ReadFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil {
		return "", err
	}
	var g struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return "", err
	}
	return g.ChainID, nil
}

// healAppTomlEVMChainID sets evm.evm-chain-id in <home>/config/app.toml to the
// derived value, editing the file surgically with tomledit (comment- and
// format-preserving; it recognizes bare and quoted table/key forms natively and
// never re-renders the whole file). Best-effort: the caller has already set the
// runtime value, so a write error is non-fatal.
func healAppTomlEVMChainID(home string, evmChainID uint64) error {
	path := filepath.Join(home, "config", "app.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := tomledit.Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}
	value := parser.MustValue(strconv.FormatUint(evmChainID, 10))
	switch e := doc.First("evm", "evm-chain-id"); {
	case e != nil && e.KeyValue != nil:
		// present: set the value in place, preserving the key's comment and form
		e.Value = value
	default:
		kv := &parser.KeyValue{Name: parser.Key{"evm-chain-id"}, Value: value}
		if tab := transform.FindTable(doc, "evm"); tab != nil {
			transform.InsertMapping(tab.Section, kv, true) // [evm] present, key absent: add it
		} else {
			doc.Sections = append(doc.Sections, &tomledit.Section{ // [evm] absent: create it
				Heading: &parser.Heading{Name: parser.Key{"evm"}},
				Items:   []parser.Item{kv},
			})
		}
	}
	var buf bytes.Buffer
	if err := tomledit.Format(&buf, doc); err != nil {
		return err
	}
	return writeConfigFile(path, buf.Bytes())
}

// writeConfigFile writes content to path preserving the file's existing mode and
// following a symlink, so healing never widens a 0600 app.toml to world-readable
// nor replaces an operator's symlink with a regular file.
func writeConfigFile(path string, content []byte) error {
	target := path
	if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil {
		target = resolved
	}
	mode := os.FileMode(0o600)
	if fi, serr := os.Stat(target); serr == nil {
		mode = fi.Mode().Perm()
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil { // WriteFile is subject to umask; set the exact mode
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, target)
}

type appCreator struct {
	encCfg sdktestutil.TestEncodingConfig
}

// newApp is an appCreator
func (a appCreator) newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	var cache storetypes.MultiStorePersistentCache

	if cast.ToBool(appOpts.Get(sdkserver.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(sdkserver.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}

	pruningOpts, err := sdkserver.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}

	home := cast.ToString(appOpts.Get(flags.FlagHome))
	snapshotDir := filepath.Join(home, "data", "snapshots")
	snapshotDB, err := dbm.NewDB("metadata", sdkserver.GetAppDBBackend(appOpts), snapshotDir)
	if err != nil {
		panic(err)
	}

	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	if err != nil {
		panic(err)
	}

	snapshotOptions := snapshottypes.NewSnapshotOptions(
		cast.ToUint64(appOpts.Get(sdkserver.FlagStateSyncSnapshotInterval)),
		cast.ToUint32(appOpts.Get(sdkserver.FlagStateSyncSnapshotKeepRecent)),
	)

	// Setup chainId
	chainID := cast.ToString(appOpts.Get(flags.FlagChainID))
	if len(chainID) == 0 {
		v := viper.New()
		v.AddConfigPath(filepath.Join(home, "config"))
		v.SetConfigName("client")
		v.SetConfigType("toml")
		if err := v.ReadInConfig(); err != nil {
			panic(err)
		}
		conf := new(clientcfg.ClientConfig)
		if err := v.Unmarshal(conf); err != nil {
			panic(err)
		}
		chainID = conf.ChainID
	}

	// Ensure the cosmos/evm keeper is built with the correct EIP-155 EVM chain ID
	// on every start. cosmos/evm reads it from app.toml (evm.evm-chain-id) and
	// silently falls back to 262144 when unset; app.toml is NOT rewritten on
	// upgrade, so an in-place-upgraded validator would otherwise run on the wrong
	// chain ID. Derive it from the genesis chain-id (the id CometBFT validates the
	// node against, so a mistyped --chain-id can't poison it), fail fast on a
	// consensus-critical mismatch or when it can't be resolved, and self-heal
	// app.toml when it is unset so operators need no manual step.
	configuredEVMChainID := cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	genesisCID := chainID
	if gcid, gerr := genesisChainID(home); gerr == nil && gcid != "" {
		genesisCID = gcid
	} else if gerr != nil {
		logger.Error("evm-chain-id: could not read genesis chain-id; deriving from --chain-id/client.toml instead", "err", gerr)
	}
	derivedEVMChainID := evmChainIDFromChainID(genesisCID)
	switch {
	case configuredEVMChainID != 0:
		// Operator value is authoritative — but one that disagrees with the
		// derivable genesis id is consensus-critical, so refuse to start (fail
		// fast) rather than let this node fork. When genesis is not derivable we
		// cannot compare, so the configured value stands.
		if derivedEVMChainID != 0 && configuredEVMChainID != derivedEVMChainID {
			panic(fmt.Errorf("evm.evm-chain-id=%d in app.toml disagrees with the id %d derived from genesis chain-id %q; refusing to start (fix app.toml or genesis)", configuredEVMChainID, derivedEVMChainID, genesisCID))
		}
	case derivedEVMChainID != 0:
		// Unset: adopt the derived value for the keeper/RPC and self-heal app.toml.
		if v, ok := appOpts.(*viper.Viper); ok {
			v.Set(srvflags.EVMChainID, derivedEVMChainID)
		}
		switch herr := healAppTomlEVMChainID(home, derivedEVMChainID); {
		case herr == nil:
			logger.Info("evm-chain-id: was unset in app.toml, derived from genesis chain-id and persisted",
				"evm_chain_id", derivedEVMChainID, "chain_id", genesisCID)
		case errors.Is(herr, syscall.EROFS):
			// A read-only config mount (e.g. a Kubernetes ConfigMap) is expected:
			// the derived value is authoritative and re-derived every start, so
			// only the on-disk persist is skipped — not worth an error.
			logger.Info("evm-chain-id: config is read-only, running with the derived value (not persisted to app.toml)",
				"evm_chain_id", derivedEVMChainID, "chain_id", genesisCID)
		default:
			logger.Warn("evm-chain-id: derived but could not persist to app.toml; running with the derived value",
				"evm_chain_id", derivedEVMChainID, "chain_id", genesisCID, "err", herr)
		}
	default:
		// Unset and not derivable: never silently run on cosmos/evm's 262144 default.
		panic(fmt.Errorf("evm.evm-chain-id is unset and cannot be derived from genesis chain-id %q; set evm.evm-chain-id in app.toml or pass --chain-id", genesisCID))
	}

	mocaApp := app.NewMoca(
		logger, db, traceStore, true, skipUpgradeHeights,
		cast.ToString(appOpts.Get(flags.FlagHome)),
		AppConfig,
		appOpts,
		baseapp.SetPruning(pruningOpts),
		baseapp.SetEventing(cast.ToString(appOpts.Get(sdkserver.FlagEventing))),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(sdkserver.FlagMinGasPrices))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(sdkserver.FlagMinRetainBlocks))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltTime))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(sdkserver.FlagMinRetainBlocks))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(sdkserver.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(sdkserver.FlagIndexEvents))),
		baseapp.SetSnapshot(snapshotStore, snapshotOptions),
		baseapp.SetIAVLCacheSize(cast.ToInt(appOpts.Get(sdkserver.FlagIAVLCacheSize))),
		baseapp.SetIAVLDisableFastNode(cast.ToBool(appOpts.Get(sdkserver.FlagDisableIAVLFastNode))),
		baseapp.SetChainID(chainID),
		baseapp.SetEnableUnsafeQuery(cast.ToBool(appOpts.Get(sdkserver.FlagEnableUnsafeQuery))),
		baseapp.SetEnablePlainStore(cast.ToBool(appOpts.Get(sdkserver.FlagEnablePlainStore))),
	)

	return mocaApp
}

// appExport creates a new simapp (optionally at a given height)
// and exports state.
func (a appCreator) appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var mocaApp *app.Moca
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	if height != -1 {
		mocaApp = app.NewMoca(logger, db, traceStore, false, map[int64]bool{}, "", AppConfig, appOpts)

		if err := mocaApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		mocaApp = app.NewMoca(logger, db, traceStore, true, map[int64]bool{}, "", AppConfig, appOpts)
	}

	return mocaApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// initTendermintConfig helps to override default Tendermint Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initTendermintConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()
	cfg.Consensus.TimeoutCommit = time.Second * 3

	// to put a higher strain on node memory, use these values:
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

func tempDir(defaultHome string) string {
	dir, err := os.MkdirTemp("", "moca")
	if err != nil {
		dir = defaultHome
	}
	defer os.RemoveAll(dir)

	return dir
}
