package evm_test

import (
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmante "github.com/mocachain/moca/v2/app/ante/evm"
	"github.com/mocachain/moca/v2/testutil"
	testutiltx "github.com/mocachain/moca/v2/testutil/tx"
	evmtypes "github.com/mocachain/moca/v2/x/evm/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

// stubFeeMarketKeeperNilMinGasPrice satisfies evm.FeeMarketKeeper while always
// returning a feemarket Params whose MinGasPrice has a nil internal *big.Int
// (i.e. sdkmath.LegacyDec{}). This exercises the IsNil() short-circuit added
// to EthMinGasPriceDecorator: production-style genesis would normalise this to
// a zero LegacyDec, but a number of test setups (and any code path that
// constructs Params via the zero value) leave it unset, in which case
// LegacyDec.IsZero -> (*big.Int).Sign panics on the nil pointer without the
// guard.
type stubFeeMarketKeeperNilMinGasPrice struct{}

func (stubFeeMarketKeeperNilMinGasPrice) GetParams(_ sdk.Context) feemarkettypes.Params {
	return feemarkettypes.Params{}
}

func (stubFeeMarketKeeperNilMinGasPrice) AddTransientGasWanted(_ sdk.Context, gasWanted uint64) (uint64, error) {
	return gasWanted, nil
}

func (stubFeeMarketKeeperNilMinGasPrice) GetBaseFeeEnabled(_ sdk.Context) bool {
	return false
}

var execTypes = []struct {
	name      string
	isCheckTx bool
	simulate  bool
}{
	{"deliverTx", false, false},
	{"deliverTxSimulate", false, true},
}

func (suite *AnteTestSuite) TestEthMinGasPriceDecorator() {
	denom := evmtypes.DefaultEVMDenom
	from, privKey := testutiltx.NewAddrKey()
	to := testutiltx.GenerateAddress()
	emptyAccessList := ethtypes.AccessList{}

	testCases := []struct {
		name     string
		malleate func() sdk.Tx
		expPass  bool
		errMsg   string
	}{
		{
			"invalid tx type",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				testMsg := banktypes.MsgSend{
					FromAddress: "mc1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0pfxrg8d",
					ToAddress:   "mc1dx67l23hz9l0k9hcher8xz04uj7wf3yug727ml",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: denom}},
				}
				txBuilder := suite.CreateTestCosmosTxBuilder(sdkmath.NewInt(0), denom, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			"invalid message type",
		},
		{
			"wrong tx type",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)
				testMsg := banktypes.MsgSend{
					FromAddress: "mc1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0pfxrg8d",
					ToAddress:   "mc1dx67l23hz9l0k9hcher8xz04uj7wf3yug727ml",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: denom}},
				}
				txBuilder := suite.CreateTestCosmosTxBuilder(sdkmath.NewInt(0), denom, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			"invalid message type",
		},
		{
			"valid: invalid tx type with MinGasPrices = 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyZeroDec()
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)
				testMsg := banktypes.MsgSend{
					FromAddress: "mc1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0pfxrg8d",
					ToAddress:   "mc1dx67l23hz9l0k9hcher8xz04uj7wf3yug727ml",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: denom}},
				}
				txBuilder := suite.CreateTestCosmosTxBuilder(sdkmath.NewInt(0), denom, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
		},
		{
			"valid legacy tx with MinGasPrices = 0, gasPrice = 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyZeroDec()
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(0), nil, nil, nil)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"valid legacy tx with MinGasPrices = 0, gasPrice > 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyZeroDec()
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(10), nil, nil, nil)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"valid legacy tx with MinGasPrices = 10, gasPrice = 10",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(10), nil, nil, nil)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"invalid legacy tx with MinGasPrices = 10, gasPrice = 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(0), nil, nil, nil)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			false,
			"provided fee < minimum global fee",
		},
		{
			"valid dynamic tx with MinGasPrices = 0, EffectivePrice = 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyZeroDec()
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(0), big.NewInt(0), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"valid dynamic tx with MinGasPrices = 0, EffectivePrice > 0",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyZeroDec()
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(100), big.NewInt(50), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"valid dynamic tx with MinGasPrices < EffectivePrice",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(100), big.NewInt(100), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"invalid dynamic tx with MinGasPrices > EffectivePrice",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(10)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(0), big.NewInt(0), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			false,
			"provided fee < minimum global fee",
		},
		{
			"invalid dynamic tx with MinGasPrices > BaseFee, MinGasPrices > EffectivePrice",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(100)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				feemarketParams := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				feemarketParams.BaseFee = sdkmath.NewInt(10)
				err = suite.app.FeeMarketKeeper.SetParams(suite.ctx, feemarketParams)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(1000), big.NewInt(0), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			false,
			"provided fee < minimum global fee",
		},
		{
			"valid dynamic tx with MinGasPrices > BaseFee, MinGasPrices < EffectivePrice (big GasTipCap)",
			func() sdk.Tx {
				params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				params.MinGasPrice = sdkmath.LegacyNewDec(100)
				err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
				suite.Require().NoError(err)

				feemarketParams := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
				feemarketParams.BaseFee = sdkmath.NewInt(10)
				err = suite.app.FeeMarketKeeper.SetParams(suite.ctx, feemarketParams)
				suite.Require().NoError(err)

				msg := suite.BuildTestEthTx(from, to, nil, make([]byte, 0), nil, big.NewInt(1000), big.NewInt(101), &emptyAccessList)
				return suite.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
	}

	for _, et := range execTypes {
		for _, tc := range testCases {
			suite.Run(et.name+"_"+tc.name, func() {
				// s.SetupTest(et.isCheckTx)
				suite.SetupTest()
				dec := evmante.NewEthMinGasPriceDecorator(suite.app.FeeMarketKeeper, suite.app.EvmKeeper)
				_, err := dec.AnteHandle(suite.ctx, tc.malleate(), et.simulate, testutil.NextFn)

				if tc.expPass {
					suite.Require().NoError(err, tc.name)
				} else {
					suite.Require().Error(err, tc.name)
					suite.Require().Contains(err.Error(), tc.errMsg, tc.name)
				}
			})
		}
	}
}

// TestEthMinGasPriceDecorator_NilMinGasPrice is a regression test for the
// IsNil() short-circuit added to EthMinGasPriceDecorator.AnteHandle. It feeds
// the decorator a stub FeeMarketKeeper that returns a zero-value Params (whose
// MinGasPrice wraps a nil *big.Int) and asserts that the decorator forwards to
// next() instead of dereferencing the nil pointer inside LegacyDec.IsZero.
//
// Without the guard, this test panics with:
//
//	runtime error: invalid memory address or nil pointer dereference
//	math/big.(*Int).Sign(...)
//	cosmossdk.io/math.LegacyDec.IsZero(...)
//	app/ante/evm.EthMinGasPriceDecorator.AnteHandle(...)
func (suite *AnteTestSuite) TestEthMinGasPriceDecorator_NilMinGasPrice() {
	suite.SetupTest()

	// Sanity-check that the stub really produces a nil-internal LegacyDec; if
	// upstream ever changes the zero value of feemarkettypes.Params we want
	// this regression test to fail loudly rather than silently passing.
	suite.Require().True(
		stubFeeMarketKeeperNilMinGasPrice{}.GetParams(suite.ctx).MinGasPrice.IsNil(),
		"stub must produce a LegacyDec whose internal *big.Int is nil to exercise the guard",
	)

	dec := evmante.NewEthMinGasPriceDecorator(stubFeeMarketKeeperNilMinGasPrice{}, suite.app.EvmKeeper)

	// The tx body is irrelevant: the IsNil short-circuit fires before any
	// per-message logic runs. Use InvalidTx (which has no messages) to make
	// that contract obvious - if the guard regresses, the panic will surface
	// here instead of inside the for-range loop.
	require := suite.Require()
	require.NotPanics(func() {
		_, err := dec.AnteHandle(suite.ctx, &testutiltx.InvalidTx{}, false, testutil.NextFn)
		require.NoError(err, "decorator must short-circuit on nil MinGasPrice without invoking downstream validation")
	})
}

func (suite *AnteTestSuite) TestEthMempoolFeeDecorator() {
	// TODO: add test
}
