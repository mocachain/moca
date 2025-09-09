package cosmos_test

import (
	"math"
	"testing"

	storetypes "cosmossdk.io/store/types"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/simapp"
	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/app/ante"
	evmante "github.com/evmos/evmos/v12/app/ante/evm"
	"github.com/evmos/evmos/v12/encoding"
	"github.com/evmos/evmos/v12/testutil"
	"github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/utils"
	"github.com/evmos/evmos/v12/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
	feemarkettypes "github.com/evmos/evmos/v12/x/feemarket/types"
)

type AnteTestSuite struct {
	suite.Suite

	ctx             sdk.Context
	app             *app.Evmos
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
	return statedb.New(suite.ctx, suite.app.EvmKeeper, statedb.NewEmptyTxConfig(common.BytesToHash(suite.ctx.HeaderHash())))
}

func (suite *AnteTestSuite) SetupTest() {
	checkTx := false
	priv, err := ethsecp256k1.GenPrivKey()
	suite.Require().NoError(err)
	suite.priv = priv

	suite.app = app.EthSetup(checkTx, func(app *app.Evmos, genesis simapp.GenesisState) simapp.GenesisState {
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
		evmGenesis.Params.AllowUnprotectedTxs = false
		if !suite.enableLondonHF {
			maxInt := sdkmath.NewInt(math.MaxInt64)
			evmGenesis.Params.ChainConfig.LondonBlock = &maxInt
			evmGenesis.Params.ChainConfig.ArrowGlacierBlock = &maxInt
			evmGenesis.Params.ChainConfig.GrayGlacierBlock = &maxInt
			evmGenesis.Params.ChainConfig.MergeNetsplitBlock = &maxInt
			evmGenesis.Params.ChainConfig.ShanghaiBlock = &maxInt
			evmGenesis.Params.ChainConfig.CancunBlock = &maxInt
		}
		if suite.evmParamsOption != nil {
			suite.evmParamsOption(&evmGenesis.Params)
		}
		genesis[evmtypes.ModuleName] = app.AppCodec().MustMarshalJSON(evmGenesis)
		return genesis
	})

	suite.ctx = suite.app.BaseApp.NewContext(checkTx)
	suite.ctx = suite.ctx.WithChainID(chainID)
	suite.ctx = suite.ctx.WithBlockHeight(1)
	suite.ctx = suite.ctx.WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(utils.BaseDenom, sdkmath.OneInt())))
	suite.ctx = suite.ctx.WithBlockGasMeter(storetypes.NewGasMeter(1000000000000000000))

	stakingParams, err := suite.app.StakingKeeper.GetParams(suite.ctx)
	suite.Require().NoError(err)
	stakingParams.BondDenom = utils.BaseDenom
	err = suite.app.StakingKeeper.SetParams(suite.ctx, stakingParams)
	suite.Require().NoError(err)

	suite.ctx = suite.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())

	encodingConfig := encoding.MakeConfig()
	// We're using TestMsg amino encoding in some tests, so register it here.
	encodingConfig.Amino.RegisterConcrete(&testdata.TestMsg{}, "testdata.TestMsg", nil)
	eip712.AminoCodec = encodingConfig.Amino
	eip712.ProtoCodec = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)

	// Register legacy amino codecs and interfaces for cosmos-sdk modules
	suite.app.BasicModuleManager.RegisterLegacyAminoCodec(encodingConfig.Amino)
	suite.app.BasicModuleManager.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)

	anteHandler := ante.NewAnteHandler(ante.HandlerOptions{
		AccountKeeper:          suite.app.AccountKeeper,
		BankKeeper:             suite.app.BankKeeper,
		EvmKeeper:              suite.app.EvmKeeper,
		FeegrantKeeper:         suite.app.FeeGrantKeeper,
		IBCKeeper:              suite.app.IBCKeeper,
		FeeMarketKeeper:        suite.app.FeeMarketKeeper,
		SignModeHandler:        encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		ExtensionOptionChecker: types.HasDynamicFeeExtensionOption,
		TxFeeChecker:           evmante.NewDynamicFeeChecker(suite.app.EvmKeeper),
		GashubKeeper:           suite.app.GashubKeeper,
	})

	suite.anteHandler = anteHandler

	// Initialize EVM chain ID for tests
	suite.app.EvmKeeper.WithChainID(suite.ctx)
	suite.ethSigner = ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID())

	// fund signer acc to pay for tx fees
	amt := sdkmath.NewInt(int64(math.Pow10(18) * 2))
	err = testutil.FundAccount(
		suite.ctx,
		suite.app.BankKeeper,
		suite.priv.PubKey().Address().Bytes(),
		sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, amt)),
	)
	suite.Require().NoError(err)
}

func TestAnteTestSuite(t *testing.T) {
	suite.Run(t, &AnteTestSuite{
		enableLondonHF: true,
	})
}
