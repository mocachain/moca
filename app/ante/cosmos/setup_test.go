package cosmos_test

import (
	"math"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/simapp"
	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	evmantetypes "github.com/cosmos/evm/ante/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/app/ante"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/testutil"
	"github.com/mocachain/moca/v2/utils"
)

type AnteTestSuite struct {
	suite.Suite

	ctx             sdk.Context
	app             *app.Moca
	clientCtx       client.Context
	anteHandler     sdk.AnteHandler
	ethSigner       ethtypes.Signer
	priv            cryptotypes.PrivKey
	enableFeemarket bool
	enableLondonHF  bool
	evmParamsOption func(*evmtypes.Params)
}

const TestGasLimit uint64 = 100000

var chainID = utils.TestnetChainID + "-1"

func (suite *AnteTestSuite) StateDB() *statedb.StateDB {
	return statedb.New(suite.ctx, suite.app.EvmKeeper, statedb.NewEmptyTxConfig())
}

// sharedApp is built (and InitChain'd) exactly once for the whole package.
//
// cosmos/evm v0.6.0 seals a PROCESS-GLOBAL EVM coin info during x/vm's
// InitGenesis (types.setEVMCoinInfo -> "EVM coin info already set"); the
// non-test build provides no reset. Building a second fully-InitChain'd app in
// the same process therefore panics. This suite (and the standalone
// TestAuthzLimiterDecorator) build many "apps" via SetupTest, so we construct
// one app, seal the global config once, and reuse it. Each SetupTest still
// derives a fresh per-test context, regenerates the signer key, and re-funds /
// re-sets params so tests stay isolated at the account level. Tests that need a
// clean balance for a fixed address regenerate that address per sub-test.
var sharedApp *app.Moca

func (suite *AnteTestSuite) SetupTest() {
	checkTx := false
	priv, err := ethsecp256k1.GenPrivKey()
	suite.Require().NoError(err)
	suite.priv = priv

	if sharedApp != nil {
		suite.app = sharedApp
		// On a reused app the previous SetupTest already committed a block, which
		// nils baseapp.finalizeBlockState. NewContext(false) dereferences that
		// state, so open a fresh block (mirrors what InitChain does on the first
		// run) before deriving the per-test context. The trailing testutil.Commit
		// in setupPerTestState reuses this same finalizeBlockState and commits it.
		_, err = suite.app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Height: suite.app.LastBlockHeight() + 1,
		})
		suite.Require().NoError(err)
		suite.setupPerTestState(checkTx)
		return
	}

	suite.app = app.EthSetup(checkTx, func(app *app.Moca, genesis simapp.GenesisState) simapp.GenesisState {
		if suite.enableFeemarket {
			// setup feemarketGenesis params
			feemarketGenesis := feemarkettypes.DefaultGenesisState()
			feemarketGenesis.Params.EnableHeight = 1
			feemarketGenesis.Params.NoBaseFee = false
			// Verify feeMarket genesis
			err := feemarketGenesis.Validate()
			suite.Require().NoError(err)
			genesis[feemarkettypes.ModuleName] = app.AppCodec().MustMarshalJSON(feemarketGenesis)
		}
		evmGenesis := evmtypes.DefaultGenesisState()
		// cosmos/evm v0.6.0 migration: evmtypes.Params no longer has
		// AllowUnprotectedTxs, and ChainConfig is now a global value
		// (evmtypes.GetChainConfig/SetChainConfig) rather than a Params field.
		// The previous code only disabled London-and-later hard forks when
		// enableLondonHF was false; every test in this package runs with
		// enableLondonHF: true (London active), so there is nothing to mutate
		// here under the new global config.
		//
		// cosmos/evm v0.6.0 also resolves the EVM coin info from bank denom
		// metadata at InitGenesis (keeper.LoadEvmCoinInfo). The cosmos/evm
		// default EvmDenom is "aatom", for which the test genesis has no bank
		// metadata, so InitGenesis panics ("denom metadata aatom could not be
		// found"). Mirror production app.DefaultGenesis: use moca's 18-decimal
		// base denom (amoca) as the EVM denom and register its bank metadata.
		evmGenesis.Params.EvmDenom = utils.BaseDenom
		evmGenesis.Params.ExtendedDenomOptions = nil // 18-decimal native chain: extended == base
		if suite.evmParamsOption != nil {
			suite.evmParamsOption(&evmGenesis.Params)
		}
		genesis[evmtypes.ModuleName] = app.AppCodec().MustMarshalJSON(evmGenesis)

		// Register the EVM denom's bank metadata (mirrors production
		// app.mocaDenomMetadata) so cosmos/evm's LoadEvmCoinInfo can resolve
		// decimals/display at InitGenesis.
		var bankGen banktypes.GenesisState
		app.AppCodec().MustUnmarshalJSON(genesis[banktypes.ModuleName], &bankGen)
		bankGen.DenomMetadata = append(bankGen.DenomMetadata, banktypes.Metadata{
			Description: "The native staking and EVM token of the Moca chain",
			Base:        utils.BaseDenom,
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: utils.BaseDenom, Exponent: 0},
				{Denom: "moca", Exponent: uint32(evmtypes.EighteenDecimals)},
			},
			Name:    "moca",
			Symbol:  "MOCA",
			Display: "moca",
		})
		genesis[banktypes.ModuleName] = app.AppCodec().MustMarshalJSON(&bankGen)
		return genesis
	})
	sharedApp = suite.app

	suite.setupPerTestState(checkTx)
}

// setupPerTestState (re)initializes everything that is safe to run repeatedly
// against the shared app: per-test context, signer funding, params, ante
// handler and signer. It deliberately does NOT rebuild the app (which would
// re-seal the process-global EVM coin info and panic).
func (suite *AnteTestSuite) setupPerTestState(checkTx bool) {
	suite.ctx = suite.app.BaseApp.NewContext(checkTx).WithChainID(chainID)
	suite.ctx = suite.ctx.WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(utils.BaseDenom, sdkmath.OneInt())))
	suite.ctx = suite.ctx.WithBlockGasMeter(storetypes.NewGasMeter(1000000000000000000))

	stakingParams, err := suite.app.StakingKeeper.GetParams(suite.ctx)
	suite.Require().NoError(err)
	stakingParams.BondDenom = utils.BaseDenom
	err = suite.app.StakingKeeper.SetParams(suite.ctx, stakingParams)
	suite.Require().NoError(err)

	encodingConfig := encoding.MakeConfig()
	// We're using TestMsg amino encoding in some tests, so register it here.
	encodingConfig.Amino.RegisterConcrete(&testdata.TestMsg{}, "testdata.TestMsg", nil)
	eip712.AminoCodec = encodingConfig.Amino
	eip712.ProtoCodec = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)

	suite.clientCtx = client.Context{}.
		WithTxConfig(encodingConfig.TxConfig).
		WithChainID(chainID)

	// cosmos/evm v0.6.0 migration: evmante.NewDynamicFeeChecker was removed.
	// Mirror production (app.go setAnteHandler): leave TxFeeChecker nil so
	// moca's NewDeductFeeDecorator falls back to
	// checkTxFeeWithValidatorMinGasPrices. Cdc and DistributionKeeper are now
	// required by the cosmos-tx decorator chain.
	anteHandler := ante.NewAnteHandler(ante.HandlerOptions{
		Cdc:                    suite.app.AppCodec(),
		AccountKeeper:          suite.app.AccountKeeper,
		BankKeeper:             suite.app.BankKeeper,
		DistributionKeeper:     suite.app.DistrKeeper,
		EvmKeeper:              suite.app.EvmKeeper,
		FeegrantKeeper:         suite.app.FeeGrantKeeper,
		FeeMarketKeeper:        suite.app.FeeMarketKeeper,
		SignModeHandler:        encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		ExtensionOptionChecker: evmantetypes.HasDynamicFeeExtensionOption,
	})

	suite.anteHandler = anteHandler
	// cosmos/evm v0.6.0 migration: EVM keeper no longer exposes ChainID();
	// the EVM chain id is read from the global config.
	suite.ethSigner = ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	// fund signer acc to pay for tx fees
	amt := sdkmath.NewInt(int64(math.Pow10(18) * 2))
	err = testutil.FundAccount(
		suite.ctx,
		suite.app.BankKeeper,
		suite.priv.PubKey().Address().Bytes(),
		sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, amt)),
	)
	suite.Require().NoError(err)

	header := suite.ctx.BlockHeader()
	suite.ctx = suite.ctx.WithBlockHeight(header.Height - 1)
	suite.ctx, err = testutil.Commit(suite.ctx, suite.app, time.Second*0, nil)
	suite.Require().NoError(err)
	suite.ctx = suite.ctx.WithChainID(chainID)
}

func TestAnteTestSuite(t *testing.T) {
	suite.Run(t, &AnteTestSuite{
		enableLondonHF: true,
	})
}
