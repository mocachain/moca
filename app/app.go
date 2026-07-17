package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/gogoproto/proto"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/spf13/cast"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/iavl"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	testdata_pulsar "github.com/cosmos/cosmos-sdk/testutil/testdata/testpb"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	sigtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"

	// The moca-specific EVM-tx ante decorators that once lived in
	// app/ante/evm were deleted with the cosmos/evm v0.6.0 migration; ante
	// construction below uses cosmos/evm's MonoDecorator directly via
	// app/ante/handler_options.go. The ante keeper-interface aliases now
	// live in app/ante/evmiface.
	"github.com/mocachain/moca/v2/app/upgrades"
	upgradev2 "github.com/mocachain/moca/v2/app/upgrades/v2"
	"github.com/mocachain/moca/v2/encoding"
	servercfg "github.com/mocachain/moca/v2/server/config"
	srvflags "github.com/mocachain/moca/v2/server/flags"
	mocatypes "github.com/mocachain/moca/v2/types"
	// cosmos/evm v0.6.0 EVM keeper + moca's chain-specific precompiles, registered
	// into the EVM via WithStaticPrecompiles in EvmPrecompiled().
	evmante "github.com/cosmos/evm/ante"
	evmantetypes "github.com/cosmos/evm/ante/types"
	feemarketmodule "github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmmodule "github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	precompilesauthz "github.com/mocachain/moca/v2/precompiles/authz"
	precompilesbank "github.com/mocachain/moca/v2/precompiles/bank"
	precompilesgov "github.com/mocachain/moca/v2/precompiles/gov"
	precompilespayment "github.com/mocachain/moca/v2/precompiles/payment"
	precompilespermission "github.com/mocachain/moca/v2/precompiles/permission"
	precompilesstorage "github.com/mocachain/moca/v2/precompiles/storage"
	precompilessp "github.com/mocachain/moca/v2/precompiles/storageprovider"
	precompilesvirtualgroup "github.com/mocachain/moca/v2/precompiles/virtualgroup"

	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"

	// unnamed import of statik for swagger UI support
	_ "github.com/mocachain/moca/v2/client/docs/statik"

	"github.com/mocachain/moca/v2/app/ante"

	// Force-load the tracer engines to trigger registration due to Go-Ethereum v1.10.15 changes
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	precompilesdistribution "github.com/mocachain/moca/v2/precompiles/distribution"
	precompilesslashing "github.com/mocachain/moca/v2/precompiles/slashing"
	precompilesstaking "github.com/mocachain/moca/v2/precompiles/staking"
	challengemodule "github.com/mocachain/moca/v2/x/challenge"
	challengemodulekeeper "github.com/mocachain/moca/v2/x/challenge/keeper"
	challengemoduletypes "github.com/mocachain/moca/v2/x/challenge/types"
	"github.com/mocachain/moca/v2/x/gensp"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
	paymentmodule "github.com/mocachain/moca/v2/x/payment"
	paymentmodulekeeper "github.com/mocachain/moca/v2/x/payment/keeper"
	paymentmoduletypes "github.com/mocachain/moca/v2/x/payment/types"
	permissionmodule "github.com/mocachain/moca/v2/x/permission"
	permissionmodulekeeper "github.com/mocachain/moca/v2/x/permission/keeper"
	permissionmoduletypes "github.com/mocachain/moca/v2/x/permission/types"
	spmodule "github.com/mocachain/moca/v2/x/sp"
	spmodulekeeper "github.com/mocachain/moca/v2/x/sp/keeper"
	spmoduletypes "github.com/mocachain/moca/v2/x/sp/types"
	storagemodule "github.com/mocachain/moca/v2/x/storage"
	storagemodulekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagemoduletypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmodule "github.com/mocachain/moca/v2/x/virtualgroup"
	virtualgroupmodulekeeper "github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// Name defines the application binary name
const (
	Name      = "mocad"
	ShortName = "mocad"
)

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:         nil,
		distrtypes.ModuleName:              nil,
		stakingtypes.BondedPoolName:        {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName:     {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:                {authtypes.Burner},
		evmtypes.ModuleName:                {authtypes.Minter, authtypes.Burner}, // used for secure addition and subtraction of balance using module account
		paymentmoduletypes.ModuleName:      {authtypes.Burner, authtypes.Staking},
		permissionmoduletypes.ModuleName:   nil,
		spmoduletypes.ModuleName:           {authtypes.Staking},
		virtualgroupmoduletypes.ModuleName: nil,
	}
)

var (
	_ servertypes.Application = (*Moca)(nil)
	_ runtime.AppI            = (*Moca)(nil)
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, "."+ShortName)

	// manually update the power reduction by replacing micro (u) -> atto (a) evmos
	sdk.DefaultPowerReduction = mocatypes.PowerReduction
	// modify fee market parameter defaults through global
	feemarkettypes.DefaultMinGasPrice = MainnetMinGasPrices
	feemarkettypes.DefaultMinGasMultiplier = MainnetMinGasMultiplier
	// modify default min commission to 5%
	stakingtypes.DefaultMinCommissionRate = sdkmath.LegacyNewDecWithPrec(5, 2)

	// cosmos/evm defaults to aatom; moca is an 18-decimal native chain with base
	// denom amoca (base == extended). Override the package defaults so the EVM
	// module's DefaultGenesis (used by `mocad init`, testnet, etc. via the basic
	// module manager) yields moca's denom and activates moca's static precompiles.
	// Without this, a real-network genesis carries evm_denom=aatom with no bank
	// metadata and the node panics in x/vm InitGenesis on startup.
	evmtypes.DefaultEVMDenom = cmdcfg.BaseDenom
	evmtypes.DefaultEVMExtendedDenom = cmdcfg.BaseDenom
	evmtypes.DefaultStaticPrecompiles = MocaActiveStaticPrecompiles()
}

// Evmos implements an extended ABCI application. It is an application
// that may process transactions through Ethereum's EVM running atop of
// Tendermint consensus.
type Moca struct {
	*baseapp.BaseApp

	// encoding
	cdc               *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry
	txConfig          client.TxConfig

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	AuthzKeeper           authzkeeper.Keeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper

	SpKeeper           spmodulekeeper.Keeper
	PaymentKeeper      paymentmodulekeeper.Keeper
	ChallengeKeeper    challengemodulekeeper.Keeper
	PermissionKeeper   permissionmodulekeeper.Keeper
	VirtualgroupKeeper virtualgroupmodulekeeper.Keeper
	StorageKeeper      storagemodulekeeper.Keeper

	// Ethermint keepers
	EvmKeeper       *evmkeeper.Keeper
	FeeMarketKeeper feemarketkeeper.Keeper

	// the module manager
	mm                 *module.Manager
	BasicModuleManager module.BasicManager

	// invariantChecker collects RegisterInvariants outputs from every module so
	// that ExportAppStateAndValidators can run a final state self-check before
	// serializing genesis. It replaces the x/crisis module which was removed
	// because moca does not use the runtime invariant loop or the
	// MsgVerifyInvariant governance handler. See app/invariants.go.
	invariantChecker *exportInvariantRegistry

	// the configurator
	configurator module.Configurator

	// simulation manager
	sm *module.SimulationManager

	tpsCounter *tpsCounter
	// app config
	appConfig *servercfg.AppConfig

	// pendingTxListeners are invoked (in CheckTx, via the ante
	// TxListenerDecorator) for every pending EVM tx hash. The JSON-RPC
	// newPendingTransactions subscription registers cosmos/evm's stream here
	// through RegisterPendingTxListener at server startup.
	pendingTxListeners []evmante.PendingTxListener
}

// SimulationManager implements runtime.AppI
func (app *Moca) SimulationManager() *module.SimulationManager {
	return app.sm
}

// NewEvmos returns a reference to a new initialized Ethermint application.
func NewMoca(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	customAppConfig *servercfg.AppConfig,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *Moca {
	encodingConfig := encoding.MakeConfig()
	appCodec := encodingConfig.Codec
	cdc := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	// Setup Mempool and Proposal Handlers
	baseAppOptions = append(baseAppOptions, func(app *baseapp.BaseApp) {
		mempool := mempool.NoOpMempool{}
		app.SetMempool(mempool)
		handler := baseapp.NewDefaultProposalHandler(mempool, app)
		app.SetPrepareProposal(handler.PrepareProposalHandler())
		app.SetProcessProposal(handler.ProcessProposalHandler())
	})

	// NOTE we use custom transaction decoder that supports the sdk.Tx interface instead of sdk.StdTx
	bApp := baseapp.NewBaseApp(
		Name,
		logger,
		db,
		encodingConfig.TxConfig.TxDecoder(),
		baseAppOptions...,
	)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)

	keys := storetypes.NewKVStoreKeys(
		// SDK keys
		authtypes.StoreKey, authzkeeper.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, consensusparamtypes.StoreKey,
		feegrant.StoreKey,
		spmoduletypes.StoreKey,
		virtualgroupmoduletypes.StoreKey,
		paymentmoduletypes.StoreKey,
		permissionmoduletypes.StoreKey,
		storagemoduletypes.StoreKey,
		challengemoduletypes.StoreKey,
		reconStoreKey,
		// ethermint keys
		evmtypes.StoreKey, feemarkettypes.StoreKey,
	)

	// Add the EVM transient store key
	tkeys := storetypes.NewTransientStoreKeys(evmtypes.TransientKey, feemarkettypes.TransientKey, challengemoduletypes.TStoreKey, storagemoduletypes.TStoreKey, virtualgroupmoduletypes.TStoreKey)
	memKeys := storetypes.NewMemoryStoreKeys(challengemoduletypes.MemStoreKey)

	app := &Moca{
		BaseApp:           bApp,
		cdc:               cdc,
		appCodec:          appCodec,
		appConfig:         customAppConfig,
		interfaceRegistry: interfaceRegistry,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	// get authority address
	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// set the BaseApp's parameter store
	app.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		authAddr,
		runtime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	// use custom Ethermint account for contracts
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec, runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		mocatypes.ProtoAccount, maccPerms,
		cmdcfg.NewMultiPrefixBech32AccCodec(),
		authAddr,
	)
	app.AuthzKeeper = authzkeeper.NewKeeper(runtime.NewKVStoreService(keys[authzkeeper.StoreKey]), appCodec, app.MsgServiceRouter(), app.AccountKeeper)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		app.BlockedAccountAddrs(),
		authAddr,
		logger,
	)
	app.AuthzKeeper = app.AuthzKeeper.SetBankKeeper(app.BankKeeper)
	// optional: enable sign mode textual by overwriting the default tx config (after setting the bank keeper)
	enabledSignModes := append(authtx.DefaultSignModes, sigtypes.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes: enabledSignModes,
		// cosmos-sdk v0.53: ConfigOptions needs a signing context with
		// address codecs, else NewTxConfigWithOptions panics.
		SigningContext:             interfaceRegistry.SigningContext(),
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.BankKeeper),
	}
	txConfig, err := authtx.NewTxConfigWithOptions(
		appCodec,
		txConfigOpts,
	)
	if err != nil {
		panic(err)
	}
	app.txConfig = txConfig
	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.AuthzKeeper,
		app.BankKeeper,
		authAddr,
		cmdcfg.NewMultiPrefixBech32ValCodec(),
		cmdcfg.NewMultiPrefixBech32ConsCodec(),
	)
	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)
	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		app.LegacyAmino(),
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authAddr,
	)
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keys[feegrant.StoreKey]), app.AccountKeeper)
	app.UpgradeKeeper = upgradekeeper.NewKeeper(skipUpgradeHeights, runtime.NewKVStoreService(keys[upgradetypes.StoreKey]), appCodec, homePath, app.BaseApp, authAddr)

	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	// Create Ethermint keepers
	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec, authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
		tkeys[feemarkettypes.TransientKey],
	)

	// cosmos/evm's sealed process-global EVM coin info / opcode activators are
	// set by the x/vm module itself — at genesis via InitGenesis and on every
	// restart via PreBlock (both guarded by the module's shared sync.Once) — so
	// the app must NOT seal them here (doing so panics with "EVM coin info
	// already set" when the module then tries). We only register a non-sealing
	// fallback so coin-info reads before the first PreBlock (RPC, keeper setup)
	// don't nil-deref. moca is an 18-decimal native chain: base == extended.
	mocaEvmCoinInfo := evmtypes.EvmCoinInfo{
		Denom:         cmdcfg.BaseDenom,
		ExtendedDenom: cmdcfg.BaseDenom,
		DisplayDenom:  cmdcfg.DisplayDenom,
		Decimals:      uint32(evmtypes.EighteenDecimals),
	}
	evmtypes.SetDefaultEvmCoinInfo(mocaEvmCoinInfo)

	// evmkeeper.NewKeeper internally calls types.SetChainConfig, which
	// populates the global eth chain config that the ante GasWantedDecorator
	// (and EVM tx processing) read via evmtypes.GetEthChainConfig().
	//
	// The EVM chain ID is read from appOpts (srvflags.EVMChainID). It MUST NOT
	// be hardcoded: the root command builds a throwaway app with empty
	// appOpts, so it gets 0 -> DefaultChainConfig(0) -> the cosmos/evm default
	// (262144). cosmos/evm's SetChainConfig only allows the global config to
	// be (re)written while it still equals that default, so the second
	// construction (the real app, which may carry a configured EVMChainID)
	// transitions default -> configured cleanly. Hardcoding a non-default
	// value on both constructions trips the "chainConfig already set" guard.
	evmChainID := cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	app.EvmKeeper = evmkeeper.NewKeeper(
		appCodec,
		keys[evmtypes.StoreKey],
		tkeys[evmtypes.TransientKey],
		keys,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.FeeMarketKeeper,
		app.ConsensusParamsKeeper,
		erc20StubKeeper{},
		evmChainID,
		tracer,
	)
	// Register the keeper's fallback coin info (also sets the package-level
	// default); the x/vm module's PreBlock/InitGenesis seal the authoritative
	// value at runtime.
	app.EvmKeeper.WithDefaultEvmCoinInfo(mocaEvmCoinInfo)

	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[govtypes.StoreKey]), app.AccountKeeper, app.BankKeeper,
		app.StakingKeeper, app.DistrKeeper, app.MsgServiceRouter(), govConfig, authAddr,
	)

	// Evmos Keeper

	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	// NOTE: Distr, Slashing and Claim must be created before calling the Hooks method to avoid returning a Keeper without its table generated
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			app.DistrKeeper.Hooks(),
			app.SlashingKeeper.Hooks(),
		),
	)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(),
	)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper,
		app.SlashingKeeper,
		cmdcfg.NewMultiPrefixBech32AccCodec(),
		runtime.ProvideCometInfoService(),
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	app.SpKeeper = *spmodulekeeper.NewKeeper(
		appCodec,
		keys[spmoduletypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		app.AuthzKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	spModule := spmodule.NewAppModule(appCodec, app.SpKeeper, app.AccountKeeper, app.BankKeeper)

	app.PaymentKeeper = *paymentmodulekeeper.NewKeeper(
		appCodec,
		keys[paymentmoduletypes.StoreKey],
		app.BankKeeper,
		app.AccountKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	paymentModule := paymentmodule.NewAppModule(appCodec, app.PaymentKeeper, app.AccountKeeper, app.BankKeeper)

	app.VirtualgroupKeeper = *virtualgroupmodulekeeper.NewKeeper(
		appCodec,
		keys[virtualgroupmoduletypes.StoreKey],
		tkeys[virtualgroupmoduletypes.TStoreKey],
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.SpKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		app.PaymentKeeper,
	)

	app.PermissionKeeper = *permissionmodulekeeper.NewKeeper(
		appCodec,
		keys[permissionmoduletypes.StoreKey],
		app.AccountKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	permissionModule := permissionmodule.NewAppModule(appCodec, app.PermissionKeeper, app.AccountKeeper, app.BankKeeper)

	app.StorageKeeper = *storagemodulekeeper.NewKeeper(
		appCodec,
		keys[storagemoduletypes.StoreKey],
		tkeys[storagemoduletypes.TStoreKey],
		app.AccountKeeper,
		app.SpKeeper,
		app.PaymentKeeper,
		app.PermissionKeeper,
		app.VirtualgroupKeeper,
		app.EvmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	storageModule := storagemodule.NewAppModule(appCodec, app.StorageKeeper, app.AccountKeeper, app.BankKeeper, app.SpKeeper)

	app.VirtualgroupKeeper.SetStorageKeeper(&app.StorageKeeper)
	virtualgroupModule := virtualgroupmodule.NewAppModule(appCodec, app.VirtualgroupKeeper, app.SpKeeper)

	app.ChallengeKeeper = *challengemodulekeeper.NewKeeper(
		appCodec,
		keys[challengemoduletypes.StoreKey],
		tkeys[challengemoduletypes.TStoreKey],
		app.BankKeeper,
		app.StorageKeeper,
		app.SpKeeper,
		app.StakingKeeper,
		app.PaymentKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	challengeModule := challengemodule.NewAppModule(appCodec, app.ChallengeKeeper, app.AccountKeeper, app.BankKeeper)
	/****  Module Options ****/

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(
		// SDK app modules
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper,
			app, app.txConfig,
		),
		gensp.NewAppModule(app.AccountKeeper, app.StakingKeeper, app, app.txConfig),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.PaymentKeeper, nil),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		upgrade.NewAppModule(app.UpgradeKeeper, cmdcfg.NewMultiPrefixBech32AccCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		spModule,
		virtualgroupModule,
		paymentModule,
		permissionModule,
		storageModule,
		challengeModule,
		// cosmos/evm v0.6.0 EVM (x/vm) and feemarket app modules. Registering
		// them runs their RegisterServices (x/vm MsgServer + QueryServer, the
		// feemarket EndBlocker) and gives both InitGenesis. The keepers are
		// constructed above (app.EvmKeeper, app.FeeMarketKeeper).
		evmmodule.NewAppModule(app.EvmKeeper, app.AccountKeeper, app.BankKeeper, cmdcfg.NewMultiPrefixBech32AccCodec()),
		feemarketmodule.NewAppModule(app.FeeMarketKeeper),
	)

	// BasicModuleManager defines the module BasicManager which is in charge of setting up basic,
	// non-dependant module elements, such as codec registration and genesis verification.
	// By default, it is composed of all the modules from the module manager.
	// Additionally, app module basics can be overwritten by passing them as an argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.mm,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			stakingtypes.ModuleName: staking.AppModule{AppModuleBasic: staking.AppModuleBasic{}},
			govtypes.ModuleName:     gov.NewAppModuleBasic([]govclient.ProposalHandler{}),
			// register the native denom metadata in the default genesis so every
			// genesis path (mocad init, testnet, mainnet) satisfies cosmos/evm's
			// InitEvmCoinInfo, which resolves the EVM coin info from bank metadata.
			banktypes.ModuleName: mocaBankModuleBasic{},
		},
	)
	app.BasicModuleManager.RegisterLegacyAminoCodec(cdc)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// NOTE: upgrade module is required to be prioritized
	app.mm.SetOrderPreBlockers(
		upgradetypes.ModuleName,
		// cosmos/evm x/vm implements HasPreBlocker (refreshes the EVM
		// base-fee/chain-config cache); runs after the upgrade module.
		evmtypes.ModuleName,
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0.
	app.mm.SetOrderBeginBlockers(
		feemarkettypes.ModuleName,
		evmtypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		spmoduletypes.ModuleName,
		virtualgroupmoduletypes.ModuleName,
		paymentmoduletypes.ModuleName,
		permissionmoduletypes.ModuleName,
		storagemoduletypes.ModuleName,
		gensptypes.ModuleName,
		challengemoduletypes.ModuleName,
	)

	// NOTE: fee market module must go last in order to retrieve the block gas used.
	app.mm.SetOrderEndBlockers(
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		// Evmos modules
		spmoduletypes.ModuleName,
		virtualgroupmoduletypes.ModuleName,
		paymentmoduletypes.ModuleName,
		permissionmoduletypes.ModuleName,
		storagemoduletypes.ModuleName,
		gensptypes.ModuleName,
		challengemoduletypes.ModuleName,
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	app.mm.SetOrderInitGenesis(
		// SDK modules
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		// NOTE: staking requires the claiming hook
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		// Ethermint modules
		// evm module denomination is used by the revenue module, in AnteHandle
		evmtypes.ModuleName,
		// NOTE: feemarket module needs to be initialized before genutil module:
		// gentx transactions use MinGasPriceDecorator.AnteHandle
		feemarkettypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		upgradetypes.ModuleName,
		// Evmos modules
		spmoduletypes.ModuleName,
		virtualgroupmoduletypes.ModuleName,
		paymentmoduletypes.ModuleName,
		permissionmoduletypes.ModuleName,
		storagemoduletypes.ModuleName,
		gensptypes.ModuleName,
		challengemoduletypes.ModuleName,
	)

	// Collect every module's invariants into a local registry so that
	// ExportAppStateAndValidators can run them before genesis is serialized.
	// This replaces app.mm.RegisterInvariants(&app.CrisisKeeper) without
	// re-introducing the x/crisis module surface; see app/invariants.go.
	app.invariantChecker = &exportInvariantRegistry{}
	app.mm.RegisterInvariants(app.invariantChecker)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	err = app.mm.RegisterServices(app.configurator)
	if err != nil {
		panic(err)
	}

	// add test gRPC service for testing gRPC queries in isolation
	// testdata.RegisterTestServiceServer(app.GRPCQueryRouter(), testdata.TestServiceImpl{})

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.mm.Modules, overrideModules)

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.mm.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)
	// add test gRPC service for testing gRPC queries in isolation
	testdata_pulsar.RegisterQueryServer(app.GRPCQueryRouter(), testdata_pulsar.QueryImpl{})

	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)

	// load state streaming if enabled
	if err := app.RegisterStreamingServices(appOpts, keys); err != nil {
		fmt.Printf("failed to load state streaming: %s", err)
		os.Exit(1)
	}

	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)

	maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))

	app.setAnteHandler(app.txConfig, maxGasWanted)
	app.setPostHandler()
	app.SetEndBlocker(app.EndBlocker)
	app.setupUpgradeHandlers()
	app.EvmPrecompiled()

	// RegisterUpgradeHandlers is used for registering any on-chain upgrades.
	// err = app.RegisterUpgradeHandlers(app.ChainID(), &app.appConfig.Config)
	// if err != nil {
	// 	panic(err)
	// }
	ms := app.CommitMultiStore()
	ctx := sdk.NewContext(ms, tmproto.Header{ChainID: app.ChainID(), Height: app.LastBlockHeight()}, true, app.Logger())
	// At startup, after all modules have been registered, check that all prot
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		// Once we switch to using protoreflect-based antehandlers, we might
		// want to panic here instead of logging a warning.
		fmt.Fprintln(os.Stderr, err.Error())
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			logger.Error("error on loading last version", "err", err)
			os.Exit(1)
		}
		// Execute the upgraded register, such as the newly added Msg type
		// ex.
		// app.GovKeeper.Router().RegisterService(...)
		// err = app.UpgradeKeeper.InitUpgraded(ctx)
		// if err != nil {
		// 	panic(err)
		// }
	}
	if app.IsIavlStore() {
		// enable diff for reconciliation
		bankIavl, ok := ms.GetCommitStore(keys[banktypes.StoreKey]).(*iavl.Store)
		if !ok {
			os.Exit(1)
		}
		bankIavl.EnableDiff()
		paymentIavl, ok := ms.GetCommitStore(keys[paymentmoduletypes.StoreKey]).(*iavl.Store)
		if !ok {
			os.Exit(1)
		}
		paymentIavl.EnableDiff()
	}
	app.initModules(ctx)

	// Finally start the tpsCounter.
	app.tpsCounter = newTPSCounter(logger)
	go func() {
		// Unfortunately golangci-lint is so pedantic
		// so we have to ignore this error explicitly.
		_ = app.tpsCounter.start(context.Background())
	}()

	return app
}

func (app *Moca) initModules(_ sdk.Context) {
	app.initStorage()
}

func (app *Moca) initStorage() {
	storagemodulekeeper.InitPaymentCheck(app.StorageKeeper, app.appConfig.PaymentCheck.Enabled,
		app.appConfig.PaymentCheck.Interval)
}

// Name returns the name of the App
func (app *Moca) Name() string { return app.BaseApp.Name() }

// RegisterPendingTxListener registers a listener that is invoked with every
// pending EVM tx hash observed in CheckTx. cosmos/evm's JSON-RPC server calls
// this at startup to feed its newPendingTransactions subscription stream.
func (app *Moca) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

// onPendingTx fans a pending EVM tx hash out to every registered listener. It
// is wired into the EVM ante chain via HandlerOptions.PendingTxListener, so it
// is always non-nil; it is a no-op until a listener is registered.
func (app *Moca) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}

func (app *Moca) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
	options := ante.HandlerOptions{
		Cdc:                    app.appCodec,
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: evmantetypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EvmKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		DistributionKeeper:     app.DistrKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		// PendingTxListener feeds the JSON-RPC newPendingTransactions stream
		// from CheckTx; the decorator is appended in newEVMAnteHandler.
		PendingTxListener: app.onPendingTx,
		// TxFeeChecker is left nil; moca's NewDeductFeeDecorator falls back to
		// checkTxFeeWithValidatorMinGasPrices. cosmos/evm v0.6.0's
		// NewDynamicFeeChecker takes per-call feemarket params and is wired
		// in newCosmosAnteHandler/newEVMAnteHandler when needed.
	}

	if err := options.Validate(); err != nil {
		panic(err)
	}

	app.SetAnteHandler(ante.NewAnteHandler(options))
}

func (app *Moca) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}

	app.SetPostHandler(postHandler)
}

// BeginBlocker runs the Tendermint ABCI BeginBlock logic. It executes state changes at the beginning
// of the new block for every registered module. If there is a registered fork at the current height,
// BeginBlocker will schedule the upgrade plan and perform the state migration (if any).
func (app *Moca) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	// Perform any scheduled forks before executing the modules logic
	app.ScheduleForkUpgrade(ctx)
	return app.mm.BeginBlock(ctx)
}

// EndBlocker updates every end block
func (app *Moca) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	resp, err := app.mm.EndBlock(ctx)
	if err != nil {
		return sdk.EndBlock{}, err
	}
	if app.IsIavlStore() {
		bankIavl, _ := app.CommitMultiStore().GetCommitStore(app.GetKey(banktypes.StoreKey)).(*iavl.Store)
		paymentIavl, _ := app.CommitMultiStore().GetCommitStore(app.GetKey(paymentmoduletypes.StoreKey)).(*iavl.Store)

		reconCtx, _ := ctx.CacheContext()
		reconCtx = reconCtx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		app.reconcile(reconCtx, bankIavl, paymentIavl)
	}
	return resp, nil
}

// The DeliverTx method is intentionally decomposed to calculate the transactions per second.
func (app *Moca) FinalizeBlock(req *abci.RequestFinalizeBlock) (res *abci.ResponseFinalizeBlock, err error) {
	defer func() {
		// TODO: Record the count along with the code and or reason so as to display
		// in the transactions per second live dashboards.
		// BaseApp.FinalizeBlock returns (nil, err) on rejection (e.g. invalid
		// height); without this guard, dereferencing res would mask the real
		// error with a nil-pointer panic.
		if res == nil {
			return
		}
		for _, txRes := range res.TxResults {
			if txRes.IsErr() {
				app.tpsCounter.incrementFailure()
			} else {
				app.tpsCounter.incrementSuccess()
			}
		}
	}()
	res, err = app.BaseApp.FinalizeBlock(req)
	return
}

// InitChainer updates at chain initialization
func (app *Moca) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState mocatypes.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap()); err != nil {
		panic(err)
	}

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

func (app *Moca) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.mm.PreBlock(ctx)
}

// LoadHeight loads state at a particular height
func (app *Moca) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *Moca) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)

	accs := make([]string, 0, len(maccPerms))
	for k := range maccPerms {
		accs = append(accs, k)
	}
	sort.Strings(accs)

	for _, acc := range accs {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// BlockedAccountAddrs returns all the app's module account and precompile addresses that are not
// allowed to receive external tokens.
func (app *Moca) BlockedAccountAddrs() map[string]bool {
	blockedAddrs := app.ModuleAccountAddrs()

	blockedPrecompilesHex := []string{
		mocatypes.BankAddress,
		mocatypes.AuthAddress,
		mocatypes.GovAddress,
		mocatypes.StakingAddress,
		mocatypes.DistributionAddress,
		mocatypes.SlashingAddress,
		mocatypes.EvidenceAddress,
		mocatypes.DeprecatedEpochsAddress,
		mocatypes.AuthzAddress,
		mocatypes.FeemarketAddress,
		mocatypes.PaymentAddress,
		mocatypes.PermissionAddress,
		mocatypes.DeprecatedErc20Address,
		mocatypes.VirtualGroupAddress,
		mocatypes.StorageAddress,
		mocatypes.SpAddress,
	}
	for _, addr := range vm.PrecompiledAddressesBerlin {
		blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
	}

	for _, precompileAddr := range blockedPrecompilesHex {
		blockedAddrs[precompileAddr] = true
	}

	return blockedAddrs
}

// LegacyAmino returns Evmos's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *Moca) LegacyAmino() *codec.LegacyAmino {
	return app.cdc
}

// AppCodec returns Evmos's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *Moca) AppCodec() codec.Codec {
	return app.appCodec
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's,
// patched for moca's cosmos/evm wiring. cosmos/evm's x/vm reads the EVM denom
// from genesis params and, at InitGenesis, resolves its coin info (decimals /
// display) from the bank denom metadata for that denom (keeper.LoadEvmCoinInfo).
// moca is an 18-decimal EVM-native chain (base "amoca", display "moca"), so we
// set EvmDenom to the base denom and register its bank metadata; without this
// the EVM module's InitGenesis panics ("denom metadata aatom could not be found").
func (app *Moca) DefaultGenesis() mocatypes.GenesisState {
	// The cosmos/evm-specific genesis customizations (EVM denom = amoca + active
	// static precompiles via the evmtypes package defaults set in init(), and the
	// bank denom metadata via mocaBankModuleBasic) live in the basic module
	// manager, so they apply uniformly to every genesis path (mocad init,
	// testnet, mainnet) — not just this app-level helper.
	return app.BasicModuleManager.DefaultGenesis(app.appCodec)
}

// InterfaceRegistry returns Evmos's InterfaceRegistry
func (app *Moca) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *Moca) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// GetTKey returns the TransientStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *Moca) GetTKey(storeKey string) *storetypes.TransientStoreKey {
	return app.tkeys[storeKey]
}

// GetMemKey returns the MemStoreKey for the provided mem key.
//
// NOTE: This is solely used for testing purposes.
func (app *Moca) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	return app.memKeys[storeKey]
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *Moca) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx

	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register node gRPC service for grpc-gateway.
	node.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register legacy and grpc-gateway routes for all modules.
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		RegisterSwaggerAPI(clientCtx, apiSvr.Router)
	}
}

func (app *Moca) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *Moca) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

// RegisterNodeService registers the node gRPC service on the provided
// application gRPC query router.
func (app *Moca) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	node.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// IBC Go TestingApp functions

// GetBaseApp implements the TestingApp interface.
func (app *Moca) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetStakingKeeper implements the TestingApp interface.
// func (app *Evmos) GetStakingKeeper() ibctestingtypes.StakingKeeper {
// 	return app.StakingKeeper
// }

// GetStakingKeeperSDK implements the TestingApp interface.
func (app *Moca) GetStakingKeeperSDK() stakingkeeper.Keeper {
	return *app.StakingKeeper
}

// GetTxConfig implements the TestingApp interface.
func (app *Moca) GetTxConfig() client.TxConfig {
	return app.txConfig
}

// AutoCliOpts returns the autocli options for the app.
func (app *Moca) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.mm.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:       modules,
		ModuleOptions: runtimeservices.ExtractAutoCLIOptions(app.mm.Modules),
	}
}

// RegisterSwaggerAPI registers swagger route with API Server
func RegisterSwaggerAPI(_ client.Context, rtr *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	rtr.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", staticServer))
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	dupMaccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		dupMaccPerms[k] = v
	}

	return dupMaccPerms
}

// mocaStaticPrecompiles builds moca's chain-specific EVM precompiles, keyed by
// address. cosmos/evm v0.6.0 uses a build-once static model (precompiles are no
// longer reconstructed per-tx); each precompile pulls the live SDK context from
// the EVM StateDB at Run time instead of binding it at construction.
func (app *Moca) mocaStaticPrecompiles() map[common.Address]vm.PrecompiledContract {
	return map[common.Address]vm.PrecompiledContract{
		precompilesbank.GetAddress(): precompilesbank.NewPrecompile(
			bankkeeper.NewMsgServerImpl(app.BankKeeper, app.PaymentKeeper),
			app.BankKeeper,
		),
		precompilesauthz.GetAddress(): precompilesauthz.NewPrecompiledContract(app.AuthzKeeper, app.BankKeeper),
		precompilesgov.GetAddress(): precompilesgov.NewPrecompile(
			govkeeper.NewMsgServerImpl(&app.GovKeeper),
			govkeeper.NewQueryServer(&app.GovKeeper),
			app.AccountKeeper,
			app.BankKeeper,
			app.appCodec,
		),
		precompilespayment.GetAddress():    precompilespayment.NewPrecompiledContract(app.PaymentKeeper, app.BankKeeper),
		precompilespermission.GetAddress(): precompilespermission.NewPrecompiledContract(app.PermissionKeeper, app.BankKeeper),
		precompilesstaking.GetAddress(): precompilesstaking.NewPrecompile(
			stakingkeeper.NewMsgServerImpl(app.StakingKeeper),
			stakingkeeper.Querier{Keeper: app.StakingKeeper},
			app.BankKeeper,
		),
		precompilesdistribution.GetAddress(): precompilesdistribution.NewPrecompile(
			distrkeeper.NewMsgServerImpl(app.DistrKeeper),
			distrkeeper.Querier{Keeper: app.DistrKeeper},
			app.BankKeeper,
		),
		precompilesslashing.GetAddress(): precompilesslashing.NewPrecompile(
			slashingkeeper.NewMsgServerImpl(app.SlashingKeeper),
			app.SlashingKeeper,
			app.BankKeeper,
		),
		precompilesstorage.GetAddress():      precompilesstorage.NewPrecompiledContract(app.StorageKeeper, app.BankKeeper),
		precompilesvirtualgroup.GetAddress(): precompilesvirtualgroup.NewPrecompiledContract(app.VirtualgroupKeeper, app.BankKeeper),
		precompilessp.GetAddress():           precompilessp.NewPrecompiledContract(app.SpKeeper, app.BankKeeper),
	}
}

// MocaActiveStaticPrecompiles returns the sorted hex addresses of moca's
// precompiles. cosmos/evm only dispatches a static precompile whose address is
// listed (sorted) in x/vm Params.ActiveStaticPrecompiles, so this is used both
// in DefaultGenesis and in the v2 upgrade handler.
func MocaActiveStaticPrecompiles() []string {
	addrs := []string{
		precompilesbank.GetAddress().Hex(),
		precompilesauthz.GetAddress().Hex(),
		precompilesgov.GetAddress().Hex(),
		precompilespayment.GetAddress().Hex(),
		precompilespermission.GetAddress().Hex(),
		precompilesstaking.GetAddress().Hex(),
		precompilesdistribution.GetAddress().Hex(),
		precompilesslashing.GetAddress().Hex(),
		precompilesstorage.GetAddress().Hex(),
		precompilesvirtualgroup.GetAddress().Hex(),
		precompilessp.GetAddress().Hex(),
	}
	sort.Strings(addrs)
	return addrs
}

// EvmPrecompiled registers moca's static precompiles with the EVM keeper. Called
// once during app construction, after the dependency keepers are built.
func (app *Moca) EvmPrecompiled() {
	app.EvmKeeper.WithStaticPrecompiles(app.mocaStaticPrecompiles())
}

// mocaDenomMetadata is the bank denom metadata for moca's native token. The EVM
// module (cosmos/evm) resolves its coin info (decimals/display) from this, so it
// must be present both in genesis and after the v2 in-place upgrade.
func mocaDenomMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: "The native staking and EVM token of the Moca chain",
		Base:        cmdcfg.BaseDenom,
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: cmdcfg.BaseDenom, Exponent: 0},
			{Denom: cmdcfg.DisplayDenom, Exponent: uint32(evmtypes.EighteenDecimals)},
		},
		Name:    cmdcfg.DisplayDenom,
		Symbol:  "MOCA",
		Display: cmdcfg.DisplayDenom,
	}
}

// mocaBankModuleBasic overrides the bank module's default genesis to register
// moca's native denom metadata. cosmos/evm's x/vm InitGenesis resolves the EVM
// coin info from bank denom metadata (keeper.LoadEvmCoinInfo) and panics if it
// is missing, so every genesis built via the basic module manager (mocad init,
// testnet, mainnet) must carry it.
type mocaBankModuleBasic struct {
	bank.AppModuleBasic
}

// DefaultGenesis returns the bank default genesis with moca's native denom
// metadata registered.
func (mocaBankModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	genState := banktypes.DefaultGenesisState()
	genState.DenomMetadata = []banktypes.Metadata{mocaDenomMetadata()}
	return cdc.MustMarshalJSON(genState)
}

// migrateToV2 performs the in-place state migration from moca's in-tree x/evm +
// x/feemarket (v1.3.0) to cosmos/evm's x/vm + x/feemarket (v2). It runs inside
// the v2.0.0 software-upgrade handler — there is NO genesis export/import.
//
// What it does and why:
//   - The old in-tree x/evm and x/feemarket Params are wire-INCOMPATIBLE with
//     cosmos/evm's (x/evm Params field 10 was a ChainConfig message, now a
//     uint64; feemarket Params field 6 BaseFee was math.Int, now LegacyDec), so
//     the old param bytes cannot be re-decoded — both are overwritten wholesale.
//   - cosmos/evm reads the EVM coin info from bank denom metadata, which the old
//     chain lacks, so we register it and initialize the coin info (InitGenesis,
//     which would normally do this, does not run on a software upgrade).
//   - cosmos/evm moved the per-contract code-hash index off the account (moca's
//     EthAccount.CodeHash) into a dedicated store prefix. That index is empty
//     for every pre-upgrade contract, so we backfill it from the EthAccounts;
//     without this, existing contracts' GetCode returns nil and they execute as
//     empty EOAs. Contract CODE and STORAGE themselves are byte-identical on
//     disk ("evm" store, same key prefixes) and need no rewriting.
func (app *Moca) migrateToV2(ctx sdk.Context) error {
	// 1. Register the native-token bank denom metadata (cosmos/evm coin info).
	app.BankKeeper.SetDenomMetaData(ctx, mocaDenomMetadata())

	// 2. Overwrite x/vm + x/feemarket params with fresh cosmos/evm-format params.
	// min_gas_price pinned to moca's 20 gwei floor (MainnetMinGasPrices), not the cosmos/evm default.
	network := upgradev2.NetworkForChainID(ctx.ChainID())

	evmParamsJSON, err := upgradev2.EVMParamsJSON(network)
	if err != nil {
		return fmt.Errorf("v2 migration: load evm params (%s): %w", network, err)
	}
	var evmParams evmtypes.Params
	if err := app.appCodec.UnmarshalJSON(evmParamsJSON, &evmParams); err != nil {
		return fmt.Errorf("v2 migration: unmarshal evm params (%s): %w", network, err)
	}
	if err := app.EvmKeeper.SetParams(ctx, evmParams); err != nil {
		return fmt.Errorf("v2 migration: set evm params: %w", err)
	}

	// 3. Initialize the EVM coin info from the bank metadata set above.
	if err := app.EvmKeeper.InitEvmCoinInfo(ctx); err != nil {
		return fmt.Errorf("v2 migration: init evm coin info: %w", err)
	}

	// 4. Overwrite feemarket params (wire-incompatible BaseFee type change).
	feeParamsJSON, err := upgradev2.FeeMarketParamsJSON(network)
	if err != nil {
		return fmt.Errorf("v2 migration: load feemarket params (%s): %w", network, err)
	}
	var feeParams feemarkettypes.Params
	if err := app.appCodec.UnmarshalJSON(feeParamsJSON, &feeParams); err != nil {
		return fmt.Errorf("v2 migration: unmarshal feemarket params (%s): %w", network, err)
	}
	if err := app.FeeMarketKeeper.SetParams(ctx, feeParams); err != nil {
		return fmt.Errorf("v2 migration: set feemarket params: %w", err)
	}

	// 5. Backfill the cosmos/evm code-hash index from the existing EthAccounts so
	//    contracts deployed under the old x/evm remain executable.
	app.AccountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) (stop bool) {
		ethAcct, ok := acc.(mocatypes.EthAccountI)
		if !ok {
			return false
		}
		codeHash := ethAcct.GetCodeHash()
		// Skip EOAs: both the Keccak256("") empty-code sentinel and a literal
		// all-zeros hash (an EthAccount whose CodeHash string was empty/unset
		// decodes to common.Hash{}) mean "no contract code" — backfilling either
		// would write a bogus 0x04 index entry pointing at non-existent code.
		if evmtypes.IsEmptyCodeHash(codeHash.Bytes()) || codeHash == (common.Hash{}) {
			return false
		}
		app.EvmKeeper.SetCodeHash(ctx, acc.GetAddress().Bytes(), codeHash.Bytes())
		return false
	})
	return nil
}

// v2StoreUpgrades returns the IAVL store add/delete/rename plan applied at the
// v2.0.0 upgrade height. It deletes the modules removed by the cosmos/evm
// migration. Crucially it does NOT touch the "evm" or "feemarket" stores: the
// in-tree x/evm and cosmos/evm x/vm share the same store key ("evm"), and the
// feemarket store key is unchanged, so contract code+storage survive in place.
// Adding/deleting/renaming either here would orphan that state — the v2 upgrade
// test asserts this never happens.
func v2StoreUpgrades() *storetypes.StoreUpgrades {
	return &storetypes.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{"epochs", "oracle", "bridge", "group", "crosschain", "transfer", "icahost", "ibc", "capability", "params", "crisis", "gashub", "erc20"},
	}
}

func (app *Moca) setupUpgradeHandlers() {
	// When a planned update height is reached, the old binary will panic
	// writing on disk the height and name of the update that triggered it
	// This will read that value, and execute the preparations for the upgrade.
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Errorf("failed to read upgrade info from disk: %w", err))
	}

	// Upgrade handlers
	app.UpgradeKeeper.SetUpgradeHandler("v1.1.0", func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// noop
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	app.UpgradeKeeper.SetUpgradeHandler("v1.2.0", func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// noop
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	// v1.3.0: no state-machine changes. The moca-iavl commit-time bug left authz
	// fastnode-vs-tree drift, but the only authz grants moca's handlers read are
	// create-time gates — validator self-del -> gov (MsgDelegate) in
	// MsgCreateValidator, and SP funding -> gov (MsgDeposit) in
	// MsgCreateStorageProvider. Nothing consumes them after creation (delegate,
	// withdraw, unjail, redelegate, edit, deposit top-ups, slashing all skip the
	// check), and a new creator re-grants right before creating, so the dropped
	// grants need no restoration. Restoring them is also consensus-unsafe from a
	// handler (a purge/regrant keyed off the store iterator, which reads the
	// fastnode not the tree, forks on a node whose fastnode is missing a
	// tree-backed key). The v1.3.0 binary carries cosmos/iavl#1009 (stops the
	// prove=true panic on the phantom keys); the residual fastnode drift is
	// cleared by an IAVL rebuild (state-sync / fastStorageVersionValue bump),
	// not from this handler.
	app.UpgradeKeeper.SetUpgradeHandler("v1.3.0", func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	app.UpgradeKeeper.SetUpgradeHandler("v2.0.0", func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		// In-place migration of the in-tree x/evm + x/feemarket state to
		// cosmos/evm's x/vm + x/feemarket. See migrateToV2 for details.
		if err := app.migrateToV2(sdkCtx); err != nil {
			return fromVM, err
		}
		// The in-tree modules ran at higher consensus versions (evm=5,
		// feemarket=4) than cosmos/evm's (both 1). RunMigrations would treat
		// that as a downgrade and error, and there are no cosmos/evm RegisterMigration
		// steps anyway — migrateToV2 above performs the state migration — so pin
		// both to the new modules' ConsensusVersion before running the rest.
		fromVM[evmtypes.ModuleName] = evmmodule.AppModule{}.ConsensusVersion()
		fromVM[feemarkettypes.ModuleName] = feemarketmodule.AppModule{}.ConsensusVersion()
		// Remove fee-grant expiration-queue entries orphaned by the pre-fix
		// x/feegrant revokeAllowance (swapped queue key). Requires the
		// moca-cosmos-sdk feegrant fix to also be vendored so no new orphans form.
		if _, err := upgrades.CleanupFeegrantQueueOrphans(ctx, app.FeeGrantKeeper, app.GetKey(feegrant.StoreKey)); err != nil {
			return fromVM, err
		}
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	// testnet only upgrade Handlers
	app.UpgradeKeeper.SetUpgradeHandler(
		"testnet-gov-param-fix",
		upgrades.TestnetGovParamFix(&app.GovKeeper, app.EvmKeeper, app.mm, app.configurator),
	)

	storeUpgrades := v2StoreUpgrades()

	if upgradeInfo.Name == "v2.0.0" && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, storeUpgrades))
	}
}
