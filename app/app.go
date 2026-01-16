// Copyright 2022 Evmos Foundation
// This file is part of the Evmos Network packages.
//
// Evmos is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Evmos packages are distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Evmos packages. If not, see https://github.com/evmos/evmos/blob/main/LICENSE

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
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	"github.com/cosmos/cosmos-sdk/x/crosschain"
	crosschainkeeper "github.com/cosmos/cosmos-sdk/x/crosschain/keeper"
	crosschaintypes "github.com/cosmos/cosmos-sdk/x/crosschain/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/gashub"
	gashubkeeper "github.com/cosmos/cosmos-sdk/x/gashub/keeper"
	gashubtypes "github.com/cosmos/cosmos-sdk/x/gashub/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/group"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/oracle"
	oraclekeeper "github.com/cosmos/cosmos-sdk/x/oracle/keeper"
	oracletypes "github.com/cosmos/cosmos-sdk/x/oracle/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	// ibctestingtypes "github.com/cosmos/ibc-go/v10/testing/types"
	bridgemodule "github.com/evmos/evmos/v12/x/bridge"

	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v10/modules/core"

	// ibcclientclient "github.com/cosmos/ibc-go/v10/modules/core/02-client/client"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibcconnectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	ica "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts"
	icahost "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"

	ethante "github.com/evmos/evmos/v12/app/ante/evm"
	"github.com/evmos/evmos/v12/encoding"
	servercfg "github.com/evmos/evmos/v12/server/config"
	srvflags "github.com/evmos/evmos/v12/server/flags"
	evmostypes "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/x/evm"
	evmkeeper "github.com/evmos/evmos/v12/x/evm/keeper"
	precompilesauthz "github.com/evmos/evmos/v12/x/evm/precompiles/authz"
	precompilesbank "github.com/evmos/evmos/v12/x/evm/precompiles/bank"
	precompileserc20 "github.com/evmos/evmos/v12/x/evm/precompiles/erc20"
	precompilesgov "github.com/evmos/evmos/v12/x/evm/precompiles/gov"
	precompilespayment "github.com/evmos/evmos/v12/x/evm/precompiles/payment"
	precompilespermission "github.com/evmos/evmos/v12/x/evm/precompiles/permission"
	precompilesstorage "github.com/evmos/evmos/v12/x/evm/precompiles/storage"
	precompilessp "github.com/evmos/evmos/v12/x/evm/precompiles/storageprovider"
	precompilesvirtualgroup "github.com/evmos/evmos/v12/x/evm/precompiles/virtualgroup"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
	"github.com/evmos/evmos/v12/x/feemarket"
	feemarketkeeper "github.com/evmos/evmos/v12/x/feemarket/keeper"
	feemarkettypes "github.com/evmos/evmos/v12/x/feemarket/types"

	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"

	// unnamed import of statik for swagger UI support
	_ "github.com/evmos/evmos/v12/client/docs/statik"

	"github.com/evmos/evmos/v12/app/ante"
	"github.com/evmos/evmos/v12/x/epochs"
	epochskeeper "github.com/evmos/evmos/v12/x/epochs/keeper"
	epochstypes "github.com/evmos/evmos/v12/x/epochs/types"
	"github.com/evmos/evmos/v12/x/erc20"
	erc20keeper "github.com/evmos/evmos/v12/x/erc20/keeper"
	erc20types "github.com/evmos/evmos/v12/x/erc20/types"

	// NOTE: override ICS20 keeper to support IBC transfers of ERC20 tokens
	"github.com/evmos/evmos/v12/x/ibc/transfer"
	transferkeeper "github.com/evmos/evmos/v12/x/ibc/transfer/keeper"

	// Force-load the tracer engines to trigger registration due to Go-Ethereum v1.10.15 changes
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	bridgemodulekeeper "github.com/evmos/evmos/v12/x/bridge/keeper"
	bridgemoduletypes "github.com/evmos/evmos/v12/x/bridge/types"
	challengemodule "github.com/evmos/evmos/v12/x/challenge"
	challengemodulekeeper "github.com/evmos/evmos/v12/x/challenge/keeper"
	challengemoduletypes "github.com/evmos/evmos/v12/x/challenge/types"
	precompilesdistribution "github.com/evmos/evmos/v12/x/evm/precompiles/distribution"
	precompilesslashing "github.com/evmos/evmos/v12/x/evm/precompiles/slashing"
	precompilesstaking "github.com/evmos/evmos/v12/x/evm/precompiles/staking"
	"github.com/evmos/evmos/v12/x/gensp"
	gensptypes "github.com/evmos/evmos/v12/x/gensp/types"
	paymentmodule "github.com/evmos/evmos/v12/x/payment"
	paymentmodulekeeper "github.com/evmos/evmos/v12/x/payment/keeper"
	paymentmoduletypes "github.com/evmos/evmos/v12/x/payment/types"
	permissionmodule "github.com/evmos/evmos/v12/x/permission"
	permissionmodulekeeper "github.com/evmos/evmos/v12/x/permission/keeper"
	permissionmoduletypes "github.com/evmos/evmos/v12/x/permission/types"
	spmodule "github.com/evmos/evmos/v12/x/sp"
	spmodulekeeper "github.com/evmos/evmos/v12/x/sp/keeper"
	spmoduletypes "github.com/evmos/evmos/v12/x/sp/types"
	storagemodule "github.com/evmos/evmos/v12/x/storage"
	storagemodulekeeper "github.com/evmos/evmos/v12/x/storage/keeper"
	storagemoduletypes "github.com/evmos/evmos/v12/x/storage/types"
	virtualgroupmodule "github.com/evmos/evmos/v12/x/virtualgroup"
	virtualgroupmodulekeeper "github.com/evmos/evmos/v12/x/virtualgroup/keeper"
	virtualgroupmoduletypes "github.com/evmos/evmos/v12/x/virtualgroup/types"
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
		ibctransfertypes.ModuleName:        {authtypes.Minter, authtypes.Burner},
		icatypes.ModuleName:                nil,
		evmtypes.ModuleName:                {authtypes.Minter, authtypes.Burner}, // used for secure addition and subtraction of balance using module account
		erc20types.ModuleName:              {authtypes.Minter, authtypes.Burner},
		paymentmoduletypes.ModuleName:      {authtypes.Burner, authtypes.Staking},
		crosschaintypes.ModuleName:         {authtypes.Minter},
		permissionmoduletypes.ModuleName:   nil,
		bridgemoduletypes.ModuleName:       nil,
		spmoduletypes.ModuleName:           {authtypes.Staking},
		virtualgroupmoduletypes.ModuleName: nil,
	}
)

var (
	_ servertypes.Application = (*Evmos)(nil)
	_ ibctesting.TestingApp   = (*Evmos)(nil)
	_ runtime.AppI            = (*Evmos)(nil)
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, "."+ShortName)

	// manually update the power reduction by replacing micro (u) -> atto (a) evmos
	sdk.DefaultPowerReduction = evmostypes.PowerReduction
	// modify fee market parameter defaults through global
	feemarkettypes.DefaultMinGasPrice = MainnetMinGasPrices
	feemarkettypes.DefaultMinGasMultiplier = MainnetMinGasMultiplier
	// modify default min commission to 5%
	stakingtypes.DefaultMinCommissionRate = sdkmath.LegacyNewDecWithPrec(5, 2)
}

// Evmos implements an extended ABCI application. It is an application
// that may process transactions through Ethereum's EVM running atop of
// Tendermint consensus.
type Evmos struct {
	*baseapp.BaseApp

	// encoding
	cdc               *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry
	txConfig          client.TxConfig

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	AuthzKeeper           authzkeeper.Keeper
	BankKeeper            bankkeeper.Keeper
	CapabilityKeeper      *capabilitykeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	CrisisKeeper          crisiskeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	CrossChainKeeper      crosschainkeeper.Keeper
	OracleKeeper          oraclekeeper.Keeper
	GashubKeeper          gashubkeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	ICAHostKeeper         icahostkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	TransferKeeper        transferkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper

	BridgeKeeper       bridgemodulekeeper.Keeper
	SpKeeper           spmodulekeeper.Keeper
	PaymentKeeper      paymentmodulekeeper.Keeper
	ChallengeKeeper    challengemodulekeeper.Keeper
	PermissionKeeper   permissionmodulekeeper.Keeper
	VirtualgroupKeeper virtualgroupmodulekeeper.Keeper
	StorageKeeper      storagemodulekeeper.Keeper
	// make scoped keepers public for test purposes
	ScopedIBCKeeper      capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper capabilitykeeper.ScopedKeeper

	// Ethermint keepers
	EvmKeeper       *evmkeeper.Keeper
	FeeMarketKeeper feemarketkeeper.Keeper

	// Evmos keepers
	Erc20Keeper  erc20keeper.Keeper
	EpochsKeeper epochskeeper.Keeper

	// the module manager
	mm                 *module.Manager
	BasicModuleManager module.BasicManager

	// the configurator
	configurator module.Configurator

	// simulation manager
	sm *module.SimulationManager

	tpsCounter *tpsCounter
	// app config
	appConfig *servercfg.AppConfig
}

// SimulationManager implements runtime.AppI
func (app *Evmos) SimulationManager() *module.SimulationManager {
	return app.sm
}

// NewEvmos returns a reference to a new initialized Ethermint application.
func NewEvmos(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	customAppConfig *servercfg.AppConfig,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *Evmos {
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
		govtypes.StoreKey, paramstypes.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, capabilitytypes.StoreKey, consensusparamtypes.StoreKey,
		feegrant.StoreKey, crisistypes.StoreKey,
		group.StoreKey,
		crosschaintypes.StoreKey,
		oracletypes.StoreKey,
		bridgemoduletypes.StoreKey,
		gashubtypes.StoreKey,
		spmoduletypes.StoreKey,
		virtualgroupmoduletypes.StoreKey,
		paymentmoduletypes.StoreKey,
		permissionmoduletypes.StoreKey,
		storagemoduletypes.StoreKey,
		challengemoduletypes.StoreKey,
		reconStoreKey,
		// ibc keys
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		// ica keys
		icahosttypes.StoreKey,
		// ethermint keys
		evmtypes.StoreKey, feemarkettypes.StoreKey,
		// evmos keys
		erc20types.StoreKey,
		epochstypes.StoreKey,
	)

	// Add the EVM transient store key
	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey, evmtypes.TransientKey, feemarkettypes.TransientKey, challengemoduletypes.TStoreKey, storagemoduletypes.TStoreKey)
	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey, challengemoduletypes.MemStoreKey)

	app := &Evmos{
		BaseApp:           bApp,
		cdc:               cdc,
		appCodec:          appCodec,
		appConfig:         customAppConfig,
		interfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	// init params keeper and subspaces
	app.ParamsKeeper = initParamsKeeper(appCodec, cdc, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])

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

	// add capability keeper and ScopeToModule for ibc module
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(appCodec, keys[capabilitytypes.StoreKey], memKeys[capabilitytypes.MemStoreKey])

	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	scopedTransferKeeper := app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	app.CapabilityKeeper.Seal()

	// use custom Ethermint account for contracts
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec, runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		evmostypes.ProtoAccount, maccPerms,
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
	// optional: enable sign mode textual by overwriting the default tx config (after setting the bank keeper)
	enabledSignModes := append(authtx.DefaultSignModes, sigtypes.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
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
	)
	app.CrossChainKeeper = crosschainkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[crosschaintypes.StoreKey]),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.StakingKeeper,
		app.BankKeeper,
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
	app.OracleKeeper = oraclekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[crosschaintypes.StoreKey]),
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.CrossChainKeeper,
		app.BankKeeper,
		app.StakingKeeper,
	)
	app.CrisisKeeper = *crisiskeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[crisistypes.StoreKey]), invCheckPeriod, app.BankKeeper, authtypes.FeeCollectorName, authAddr,
	)
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keys[feegrant.StoreKey]), app.AccountKeeper)
	app.UpgradeKeeper = upgradekeeper.NewKeeper(skipUpgradeHeights, runtime.NewKVStoreService(keys[upgradetypes.StoreKey]), appCodec, homePath, app.BaseApp, authAddr)

	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	// Create Ethermint keepers
	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec, authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
		tkeys[feemarkettypes.TransientKey],
		app.GetSubspace(feemarkettypes.ModuleName),
	)

	app.EvmKeeper = evmkeeper.NewKeeper(
		appCodec, keys[evmtypes.StoreKey], tkeys[evmtypes.TransientKey], authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.FeeMarketKeeper,
		// FIX: Temporary solution to solve keeper interdependency while new precompile module
		// is being developed.
		tracer, app.GetSubspace(evmtypes.ModuleName),
	)

	// Create IBC Keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[ibcexported.StoreKey]), app.GetSubspace(ibcexported.ModuleName), app.UpgradeKeeper, authAddr,
	)

	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[govtypes.StoreKey]), app.AccountKeeper, app.BankKeeper,
		app.StakingKeeper, app.CrossChainKeeper, app.DistrKeeper, app.MsgServiceRouter(), govConfig, authAddr,
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

	app.Erc20Keeper = erc20keeper.NewKeeper(
		keys[erc20types.StoreKey], appCodec, authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper, app.BankKeeper, app.EvmKeeper, app.StakingKeeper,
	)

	epochsKeeper := epochskeeper.NewKeeper(appCodec, keys[epochstypes.StoreKey])
	app.EpochsKeeper = *epochsKeeper.SetHooks(
		epochskeeper.NewMultiEpochHooks(
		// insert epoch hooks receivers here
		),
	)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(),
	)

	app.EvmKeeper = app.EvmKeeper.SetHooks(
		evmkeeper.NewMultiEvmHooks(
			app.Erc20Keeper.Hooks(),
		),
	)

	app.TransferKeeper = transferkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[ibctransfertypes.StoreKey]), app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCKeeper.ChannelKeeper, // ICS4 Wrapper: claims IBC middleware
		app.IBCKeeper.ChannelKeeper, bApp.MsgServiceRouter(),
		app.AccountKeeper, app.BankKeeper,
		app.Erc20Keeper, // Add ERC20 Keeper for ERC20 transfers
		authAddr,
	)

	// Override the ICS20 app module
	transferModule := transfer.NewAppModule(app.TransferKeeper)

	// Create the app.ICAHostKeeper
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(app.keys[icahosttypes.StoreKey]),
		app.GetSubspace(icahosttypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		app.AccountKeeper,
		bApp.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		authAddr,
	)

	// create host IBC module
	icaHostIBCModule := icahost.NewIBCModule(app.ICAHostKeeper)

	// create IBC module from top to bottom of stack
	var transferStack porttypes.IBCModule

	transferStack = transfer.NewIBCModule(app.TransferKeeper)
	transferStack = erc20.NewIBCMiddleware(app.Erc20Keeper, transferStack)

	// Create static IBC router, add transfer route, then set and seal it
	ibcRouter := porttypes.NewRouter()
	ibcRouter.
		AddRoute(icahosttypes.SubModuleName, icaHostIBCModule).
		AddRoute(ibctransfertypes.ModuleName, transferStack)

	app.IBCKeeper.SetRouter(ibcRouter)
	storeProvider := app.IBCKeeper.ClientKeeper.GetStoreProvider()
	tmLightClientModule := ibctm.NewLightClientModule(appCodec, storeProvider)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper,
		app.SlashingKeeper,
		runtime.ProvideCometInfoService(),
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	// moca keeper
	app.BridgeKeeper = *bridgemodulekeeper.NewKeeper(
		appCodec,
		keys[bridgemoduletypes.StoreKey],
		app.BankKeeper,
		app.StakingKeeper,
		app.CrossChainKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	bridgeModule := bridgemodule.NewAppModule(appCodec, app.BridgeKeeper, app.AccountKeeper, app.BankKeeper)

	app.GashubKeeper = gashubkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[gashubtypes.StoreKey]),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	gashubModule := gashub.NewAppModule(app.GashubKeeper)

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
		app.CrossChainKeeper,
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

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(
		// SDK app modules
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper,
			app, app.txConfig,
		),
		gensp.NewAppModule(app.AccountKeeper, app.StakingKeeper, app, app.txConfig),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.PaymentKeeper, app.GetSubspace(banktypes.ModuleName)),
		capability.NewAppModule(appCodec, *app.CapabilityKeeper, false),
		crisis.NewAppModule(&app.CrisisKeeper, skipGenesisInvariants, app.GetSubspace(crisistypes.ModuleName)),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(govtypes.ModuleName)),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(slashingtypes.ModuleName), app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		crosschain.NewAppModule(app.CrossChainKeeper, app.BankKeeper, app.StakingKeeper),
		oracle.NewAppModule(app.OracleKeeper),
		upgrade.NewAppModule(app.UpgradeKeeper),
		evidence.NewAppModule(app.EvidenceKeeper),
		params.NewAppModule(app.ParamsKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		bridgeModule,
		gashubModule,
		spModule,
		virtualgroupModule,
		paymentModule,
		permissionModule,
		storageModule,
		challengeModule,

		// ibc modules
		ibc.NewAppModule(app.IBCKeeper),
		ica.NewAppModule(nil, &app.ICAHostKeeper),
		transferModule,
		ibctm.NewAppModule(tmLightClientModule),
		// Ethermint app modules
		evm.NewAppModule(app.EvmKeeper, app.AccountKeeper, app.GetSubspace(evmtypes.ModuleName)),
		feemarket.NewAppModule(app.FeeMarketKeeper, app.GetSubspace(feemarkettypes.ModuleName)),
		// Evmos app modules
		erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper,
			app.GetSubspace(erc20types.ModuleName)),
		epochs.NewAppModule(appCodec, app.EpochsKeeper),
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
			govtypes.ModuleName: gov.NewAppModuleBasic(
				[]govclient.ProposalHandler{
					paramsclient.ProposalHandler,
				},
			),
			ibctransfertypes.ModuleName: transfer.AppModuleBasic{AppModuleBasic: &ibctransfer.AppModuleBasic{}},
		},
	)
	app.BasicModuleManager.RegisterLegacyAminoCodec(cdc)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// NOTE: upgrade module is required to be prioritized
	app.mm.SetOrderPreBlockers(
		upgradetypes.ModuleName,
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0.
	// NOTE: capability module's beginblocker must come before any modules using capabilities (e.g. IBC)
	app.mm.SetOrderBeginBlockers(
		capabilitytypes.ModuleName,
		// Note: epochs' begin should be "real" start of epochs, we keep epochs beginblock at the beginning
		epochstypes.ModuleName,
		feemarkettypes.ModuleName,
		evmtypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibcexported.ModuleName,
		crisistypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		crosschaintypes.ModuleName,
		oracletypes.ModuleName,
		bridgemoduletypes.ModuleName,
		gashubtypes.ModuleName,
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
		crisistypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		crosschaintypes.ModuleName,
		oracletypes.ModuleName,
		// Evmos modules
		bridgemoduletypes.ModuleName,
		gashubtypes.ModuleName,
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
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	app.mm.SetOrderInitGenesis(
		// SDK modules
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		// NOTE: staking requires the claiming hook
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		gashubtypes.ModuleName,
		ibcexported.ModuleName,
		// Ethermint modules
		// evm module denomination is used by the revenue module, in AnteHandle
		evmtypes.ModuleName,
		// NOTE: feemarket module needs to be initialized before genutil module:
		// gentx transactions use MinGasPriceDecorator.AnteHandle
		feemarkettypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		upgradetypes.ModuleName,
		crosschaintypes.ModuleName,
		oracletypes.ModuleName,
		// Evmos modules
		erc20types.ModuleName,
		epochstypes.ModuleName,
		// NOTE: crisis module must go at the end to check for invariants on each module
		crisistypes.ModuleName,
		bridgemoduletypes.ModuleName,
		spmoduletypes.ModuleName,
		virtualgroupmoduletypes.ModuleName,
		paymentmoduletypes.ModuleName,
		permissionmoduletypes.ModuleName,
		storagemoduletypes.ModuleName,
		gensptypes.ModuleName,
		challengemoduletypes.ModuleName,
	)

	app.mm.RegisterInvariants(&app.CrisisKeeper)
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
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
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
	// add eth query router
	ethRouter := app.BaseApp.EthQueryRouter()
	ethRouter.RegisterConstHandler()
	ethRouter.RegisterEthQueryBalanceHandler(app.BankKeeper, bankkeeper.EthQueryBalanceHandlerGen)

	app.ScopedIBCKeeper = scopedIBCKeeper
	app.ScopedTransferKeeper = scopedTransferKeeper

	// Finally start the tpsCounter.
	app.tpsCounter = newTPSCounter(logger)
	go func() {
		// Unfortunately golangci-lint is so pedantic
		// so we have to ignore this error explicitly.
		_ = app.tpsCounter.start(context.Background())
	}()

	return app
}

func (app *Evmos) initModules(_ sdk.Context) {
	app.initCrossChain()
	app.initBridge()
	app.initStorage()
	app.initGov()
}

func (app *Evmos) initCrossChain() {
	app.CrossChainKeeper.SetSrcChainID(sdk.ChainID(app.appConfig.CrossChain.SrcChainId))
	app.CrossChainKeeper.SetDestBscChainID(sdk.ChainID(app.appConfig.CrossChain.DestBscChainId))
	app.CrossChainKeeper.SetDestPolygonChainID(sdk.ChainID(app.appConfig.CrossChain.DestPolygonChainId))
	app.CrossChainKeeper.SetDestScrollChainID(sdk.ChainID(app.appConfig.CrossChain.DestScrollChainId))
	app.CrossChainKeeper.SetDestLineaChainID(sdk.ChainID(app.appConfig.CrossChain.DestLineaChainId))
	app.CrossChainKeeper.SetDestMantleChainID(sdk.ChainID(app.appConfig.CrossChain.DestMantleChainId))
	app.CrossChainKeeper.SetDestArbitrumChainID(sdk.ChainID(app.appConfig.CrossChain.DestArbitrumChainId))
	app.CrossChainKeeper.SetDestOptimismChainID(sdk.ChainID(app.appConfig.CrossChain.DestOptimismChainId))
	app.CrossChainKeeper.SetDestBaseChainID(sdk.ChainID(app.appConfig.CrossChain.DestBaseChainId))
}

func (app *Evmos) initBridge() {
	bridgemodulekeeper.RegisterCrossApps(app.BridgeKeeper)
}

func (app *Evmos) initStorage() {
	storagemodulekeeper.RegisterCrossApps(app.StorageKeeper)
	storagemodulekeeper.InitPaymentCheck(app.StorageKeeper, app.appConfig.PaymentCheck.Enabled,
		app.appConfig.PaymentCheck.Interval)
}

func (app *Evmos) initGov() {
	err := app.GovKeeper.RegisterCrossChainSyncParamsApp()
	if err != nil {
		panic(err)
	}
}

// Name returns the name of the App
func (app *Evmos) Name() string { return app.BaseApp.Name() }

func (app *Evmos) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
	options := ante.HandlerOptions{
		Cdc:                    app.appCodec,
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: evmostypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EvmKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		GashubKeeper:           app.GashubKeeper,
		DistributionKeeper:     app.DistrKeeper,
		IBCKeeper:              app.IBCKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		TxFeeChecker:           ethante.NewDynamicFeeChecker(app.EvmKeeper),
	}

	if err := options.Validate(); err != nil {
		panic(err)
	}

	app.SetAnteHandler(ante.NewAnteHandler(options))
}

func (app *Evmos) setPostHandler() {
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
func (app *Evmos) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	// Perform any scheduled forks before executing the modules logic
	app.ScheduleForkUpgrade(ctx)
	return app.mm.BeginBlock(ctx)
}

// EndBlocker updates every end block
func (app *Evmos) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
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
func (app *Evmos) FinalizeBlock(req *abci.RequestFinalizeBlock) (res *abci.ResponseFinalizeBlock, err error) {
	defer func() {
		// TODO: Record the count along with the code and or reason so as to display
		// in the transactions per second live dashboards.
		if res != nil && res.TxResults != nil {
			for _, txRes := range res.TxResults {
				if txRes.IsErr() {
					app.tpsCounter.incrementFailure()
				} else {
					app.tpsCounter.incrementSuccess()
				}
			}
		}
	}()
	res, err = app.BaseApp.FinalizeBlock(req)
	return
}

// InitChainer updates at chain initialization
func (app *Evmos) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState evmostypes.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap()); err != nil {
		panic(err)
	}

	chainIDs := []uint32{
		app.appConfig.CrossChain.DestBscChainId,
		app.appConfig.CrossChain.DestPolygonChainId,
		app.appConfig.CrossChain.DestScrollChainId,
		app.appConfig.CrossChain.DestLineaChainId,
		app.appConfig.CrossChain.DestMantleChainId,
		app.appConfig.CrossChain.DestArbitrumChainId,
		app.appConfig.CrossChain.DestOptimismChainId,
		app.appConfig.CrossChain.DestBaseChainId,
	}

	channels := []sdk.ChannelID{
		bridgemoduletypes.TransferOutChannelID,
		bridgemoduletypes.TransferInChannelID,
		bridgemoduletypes.SyncParamsChannelID,
		storagemoduletypes.BucketChannelId,
		storagemoduletypes.ObjectChannelId,
		storagemoduletypes.GroupChannelId,
		storagemoduletypes.MocaSBTChannelId,
		storagemoduletypes.MocaVCChannelId,
	}

	// init cross chain channel permissions
	for _, chainID := range chainIDs {
		for _, ch := range channels {
			app.CrossChainKeeper.SetChannelSendPermission(
				ctx,
				sdk.ChainID(chainID),
				ch,
				sdk.ChannelAllow,
			)
		}
	}

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

func (app *Evmos) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.mm.PreBlock(ctx)
}

// LoadHeight loads state at a particular height
func (app *Evmos) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *Evmos) ModuleAccountAddrs() map[string]bool {
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
func (app *Evmos) BlockedAccountAddrs() map[string]bool {
	blockedAddrs := app.ModuleAccountAddrs()

	blockedPrecompilesHex := []string{
		evmostypes.BankAddress,
		evmostypes.AuthAddress,
		evmostypes.GovAddress,
		evmostypes.StakingAddress,
		evmostypes.DistributionAddress,
		evmostypes.SlashingAddress,
		evmostypes.EvidenceAddress,
		evmostypes.EpochsAddress,
		evmostypes.AuthzAddress,
		evmostypes.FeemarketAddress,
		evmostypes.PaymentAddress,
		evmostypes.PermissionAddress,
		evmostypes.Erc20Address,
		evmostypes.VirtualGroupAddress,
		evmostypes.StorageAddress,
		evmostypes.SpAddress,
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
func (app *Evmos) LegacyAmino() *codec.LegacyAmino {
	return app.cdc
}

// AppCodec returns Evmos's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *Evmos) AppCodec() codec.Codec {
	return app.appCodec
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (app *Evmos) DefaultGenesis() evmostypes.GenesisState {
	return app.BasicModuleManager.DefaultGenesis(app.appCodec)
}

// InterfaceRegistry returns Evmos's InterfaceRegistry
func (app *Evmos) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *Evmos) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// GetTKey returns the TransientStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *Evmos) GetTKey(storeKey string) *storetypes.TransientStoreKey {
	return app.tkeys[storeKey]
}

// GetMemKey returns the MemStoreKey for the provided mem key.
//
// NOTE: This is solely used for testing purposes.
func (app *Evmos) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	return app.memKeys[storeKey]
}

// GetSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *Evmos) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *Evmos) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
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

func (app *Evmos) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *Evmos) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

// RegisterNodeService registers the node gRPC service on the provided
// application gRPC query router.
func (app *Evmos) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	node.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// IBC Go TestingApp functions

// GetBaseApp implements the TestingApp interface.
func (app *Evmos) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetStakingKeeper implements the TestingApp interface.
// func (app *Evmos) GetStakingKeeper() ibctestingtypes.StakingKeeper {
// 	return app.StakingKeeper
// }

// GetStakingKeeperSDK implements the TestingApp interface.
func (app *Evmos) GetStakingKeeperSDK() stakingkeeper.Keeper {
	return *app.StakingKeeper
}

// GetIBCKeeper implements the TestingApp interface.
func (app *Evmos) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

// GetScopedIBCKeeper implements the TestingApp interface.
func (app *Evmos) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedIBCKeeper
}

// GetTxConfig implements the TestingApp interface.
func (app *Evmos) GetTxConfig() client.TxConfig {
	return app.txConfig
}

// AutoCliOpts returns the autocli options for the app.
func (app *Evmos) AutoCliOpts() autocli.AppOptions {
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

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(
	appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey,
) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	// SDK subspaces
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govv1.ParamKeyTable()) //nolint: staticcheck
	paramsKeeper.Subspace(crisistypes.ModuleName)
	keyTable := ibcclienttypes.ParamKeyTable()
	keyTable.RegisterParamSet(&ibcconnectiontypes.Params{})
	paramsKeeper.Subspace(ibcexported.ModuleName).WithKeyTable(keyTable)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName).WithKeyTable(ibctransfertypes.ParamKeyTable())
	paramsKeeper.Subspace(icahosttypes.SubModuleName).WithKeyTable(icahosttypes.ParamKeyTable())
	// ethermint subspaces
	paramsKeeper.Subspace(evmtypes.ModuleName).WithKeyTable(evmtypes.ParamKeyTable()) //nolint: staticcheck
	paramsKeeper.Subspace(feemarkettypes.ModuleName).WithKeyTable(feemarkettypes.ParamKeyTable())
	// evmos subspaces
	paramsKeeper.Subspace(erc20types.ModuleName)
	return paramsKeeper
}

// EvmPrecompiled  set evm precompiled contracts
func (app *Evmos) EvmPrecompiled() {
	precompiled := evmkeeper.BerlinPrecompiled()

	// bank precompile
	precompiled[precompilesbank.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesbank.NewPrecompiledContract(ctx, app.BankKeeper, app.PaymentKeeper)
	}

	// authz precompile
	precompiled[precompilesauthz.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesauthz.NewPrecompiledContract(ctx, app.AuthzKeeper)
	}

	// gov precompile
	precompiled[precompilesgov.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesgov.NewPrecompiledContract(ctx, app.GovKeeper, app.AccountKeeper)
	}

	// payment precompile
	precompiled[precompilespayment.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilespayment.NewPrecompiledContract(ctx, app.PaymentKeeper)
	}

	// permission precompile
	precompiled[precompilespermission.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilespermission.NewPrecompiledContract(ctx, app.PermissionKeeper)
	}

	// staking precompile
	precompiled[precompilesstaking.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesstaking.NewPrecompiledContract(ctx, app.StakingKeeper)
	}

	// distribution precompile
	precompiled[precompilesdistribution.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesdistribution.NewPrecompiledContract(ctx, app.DistrKeeper)
	}

	// storage precompile
	precompiled[precompilesstorage.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesstorage.NewPrecompiledContract(ctx, app.StorageKeeper)
	}

	// virtualgroup precompile
	precompiled[precompilesvirtualgroup.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesvirtualgroup.NewPrecompiledContract(ctx, app.VirtualgroupKeeper)
	}

	// storageprovider precompile
	precompiled[precompilessp.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilessp.NewPrecompiledContract(ctx, app.SpKeeper)
	}

	// slashing precompile
	precompiled[precompilesslashing.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompilesslashing.NewPrecompiledContract(ctx, app.SlashingKeeper)
	}

	// erc20 precompile
	precompiled[precompileserc20.GetAddress()] = func(ctx sdk.Context) vm.PrecompiledContract {
		return precompileserc20.NewPrecompiledContract(ctx, app.Erc20Keeper)
	}

	// set precompiled contracts
	app.EvmKeeper.WithPrecompiled(precompiled)
}

func (app *Evmos) setupUpgradeHandlers() {
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

	var storeUpgrades *storetypes.StoreUpgrades

	if storeUpgrades != nil {
		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, storeUpgrades))
	}
}
