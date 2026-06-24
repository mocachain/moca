package utils_test

import (
	"math"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/simapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/eip712"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/app/ante"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/testutil"
	"github.com/mocachain/moca/v2/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

type AnteTestSuite struct {
	suite.Suite

	ctx             sdk.Context
	app             *app.Moca
	clientCtx       client.Context
	anteHandler     sdk.AnteHandler
	ethSigner       types.Signer
	enableFeemarket bool
	enableLondonHF  bool
	evmParamsOption func(*evmtypes.Params)
}

func (suite *AnteTestSuite) SetupTest() {
	checkTx := false

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
		if !suite.enableLondonHF {
			// cosmos/evm v0.6.0 migration: ChainConfig is no longer a field on
			// evmtypes.Params; it is a global value. Mutate the live global chain
			// config (returned pointer) to push the relevant forks out of range so
			// London (and later) stay disabled. GetEthChainConfig reads this global
			// lazily. Mirrors cosmos/evm tests/integration/ante/ante_test_suite.go.
			maxInt := sdkmath.NewInt(math.MaxInt64)
			chainConfig := evmtypes.GetChainConfig()
			chainConfig.LondonBlock = &maxInt
			chainConfig.ArrowGlacierBlock = &maxInt
			chainConfig.GrayGlacierBlock = &maxInt
			chainConfig.MergeNetsplitBlock = &maxInt
			chainConfig.ShanghaiTime = &maxInt
			chainConfig.CancunTime = &maxInt
		}
		if suite.evmParamsOption != nil {
			suite.evmParamsOption(&evmGenesis.Params)
		}
		genesis[evmtypes.ModuleName] = app.AppCodec().MustMarshalJSON(evmGenesis)
		return genesis
	})

	suite.ctx = suite.app.BaseApp.NewContext(checkTx)
	suite.ctx = suite.ctx.WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(evmtypes.DefaultEVMDenom, sdkmath.OneInt())))
	suite.ctx = suite.ctx.WithBlockGasMeter(storetypes.NewGasMeter(1000000000000000000))
	// cosmos/evm v0.6.0 migration: EvmKeeper.WithChainID was removed. The EVM
	// chain config (and its chain ID) is now a global value initialized during
	// app construction via evmtypes.SetChainConfig, so no per-context init is
	// needed here.

	// set staking denomination to Evmos denom
	params, err := suite.app.StakingKeeper.GetParams(suite.ctx)
	suite.Require().NoError(err)
	params.BondDenom = utils.BaseDenom
	err = suite.app.StakingKeeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	infCtx := suite.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	err = suite.app.AccountKeeper.Params.Set(infCtx, authtypes.DefaultParams())
	suite.Require().NoError(err)

	encodingConfig := encoding.MakeConfig()
	// We're using TestMsg amino encoding in some tests, so register it here.
	encodingConfig.Amino.RegisterConcrete(&testdata.TestMsg{}, "testdata.TestMsg", nil)
	eip712.AminoCodec = encodingConfig.Amino
	eip712.ProtoCodec = codec.NewProtoCodec(encodingConfig.InterfaceRegistry)

	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)

	suite.Require().NotNil(suite.app.AppCodec())

	anteHandler := ante.NewAnteHandler(ante.HandlerOptions{
		Cdc:                suite.app.AppCodec(),
		AccountKeeper:      suite.app.AccountKeeper,
		BankKeeper:         suite.app.BankKeeper,
		DistributionKeeper: suite.app.DistrKeeper,
		EvmKeeper:          suite.app.EvmKeeper,
		FeegrantKeeper:     suite.app.FeeGrantKeeper,
		FeeMarketKeeper:    suite.app.FeeMarketKeeper,
		SignModeHandler:    encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:     ante.SigVerificationGasConsumer,
	})

	suite.anteHandler = anteHandler
	// cosmos/evm v0.6.0 migration: EvmKeeper.ChainID() was removed; the EVM
	// chain ID now lives in the global chain config. Mirror production
	// (app.go / rpc/backend/chain_info.go), which reads it via
	// evmtypes.GetEthChainConfig().
	suite.ethSigner = types.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	suite.ctx, err = testutil.Commit(suite.ctx, suite.app, time.Second*0, nil)
	suite.Require().NoError(err)
}

func TestAnteTestSuite(t *testing.T) {
	suite.Run(t, &AnteTestSuite{
		enableLondonHF: true,
	})
}
