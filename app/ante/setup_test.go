package ante_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint
	. "github.com/onsi/gomega"    //nolint

	simapp "cosmossdk.io/simapp"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/utils"
	"github.com/stretchr/testify/suite"
)

var s *AnteTestSuite

type AnteTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	clientCtx client.Context
	app       *app.Moca
	denom     string
}

// sharedApp is built (and InitChain'd) exactly once for the whole package.
//
// cosmos/evm v0.6.0 seals a PROCESS-GLOBAL EVM coin info. The x/vm module does
// so under a per-app sync.Once in BOTH InitGenesis and PreBlock
// (SetGlobalConfigVariables -> "EVM coin info already set"); the non-test build
// provides no reset hook. Every spec here calls testutil.Commit, which runs
// FinalizeBlock -> the EVM module's PreBlock -> seals the global config. The
// first app seals fine, but a SECOND fully-initialised *app.Moca built in the
// same process panics on its first PreBlock. Ginkgo runs many specs (each
// BeforeEach calls s.SetupTest), so we build ONE app, seal the global config
// once, and reuse it across all specs. Each SetupTest still derives a fresh
// per-test context. Specs keep account-level isolation by generating their own
// fresh keys/addresses, so shared-app state accumulation is harmless.
var sharedApp *app.Moca

func (suite *AnteTestSuite) SetupTest() {
	isCheckTx := false
	chainID := utils.TestnetChainID + "-1"

	if sharedApp == nil {
		// Use EthSetup (not Setup): its NewTestGenesisState does not run the EVM
		// module's InitGenesis with the cosmos/evm default denom; we register the
		// feemarket genesis explicitly. The EVM global config is sealed exactly
		// once here (and again, harmlessly, by the first testutil.Commit's
		// PreBlock under the same sync.Once).
		sharedApp = app.EthSetup(isCheckTx, func(a *app.Moca, genesis simapp.GenesisState) simapp.GenesisState {
			genesis[feemarkettypes.ModuleName] = a.AppCodec().MustMarshalJSON(feemarkettypes.DefaultGenesisState())
			return genesis
		})
		suite.Require().NotNil(sharedApp.AppCodec())
	}

	suite.app = sharedApp

	// On a reused app the previous spec already committed its block, which nils
	// baseapp.finalizeBlockState. NewContext(false) dereferences that state, so
	// open a fresh block before deriving the per-test context. The trailing
	// testutil.Commit run by each spec's BeforeEach reuses this same
	// finalizeBlockState and commits it. On the very first run InitChain has
	// already established finalizeBlockState, so skip the extra FinalizeBlock.
	if suite.app.LastBlockHeight() > 0 {
		_, err := suite.app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Height: suite.app.LastBlockHeight() + 1,
		})
		suite.Require().NoError(err)
	}

	suite.ctx = suite.app.BaseApp.NewContext(isCheckTx)
	suite.ctx = suite.ctx.
		WithBlockHeight(suite.app.LastBlockHeight() + 1).
		WithBlockGasMeter(storetypes.NewInfiniteGasMeter()).
		WithChainID(chainID)

	suite.denom = utils.BaseDenom
	evmParams := suite.app.EvmKeeper.GetParams(suite.ctx)
	evmParams.EvmDenom = suite.denom
	_ = suite.app.EvmKeeper.SetParams(suite.ctx, evmParams)

	encodingConfig := encoding.MakeConfig()
	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)
}

func TestAnteTestSuite(t *testing.T) {
	s = new(AnteTestSuite)
	suite.Run(t, s)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Run AnteHandler Integration Tests")
}
