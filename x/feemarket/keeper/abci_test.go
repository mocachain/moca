package keeper_test

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/evmos/evmos/v12/x/feemarket/types"
)

func (suite *KeeperTestSuite) TestEndBlock() {
	testCases := []struct {
		name         string
		NoBaseFee    bool
		malleate     func()
		expGasWanted uint64
	}{
		{
			"baseFee nil",
			true,
			func() {},
			uint64(0),
		},
		{
			"pass",
			false,
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				suite.ctx = suite.ctx.WithBlockGasMeter(meter)
				suite.app.FeeMarketKeeper.SetTransientBlockGasWanted(suite.ctx, 5000000)
			},
			uint64(2500000),
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset
			params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
			params.NoBaseFee = tc.NoBaseFee
			err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
			suite.Require().NoError(err)

			tc.malleate()
			suite.app.FeeMarketKeeper.EndBlock(suite.ctx)
			gasWanted := suite.app.FeeMarketKeeper.GetBlockGasWanted(suite.ctx)
			suite.Require().Equal(tc.expGasWanted, gasWanted, tc.name)
		})
	}
}

// TestEndBlock_NilMinGasMultiplier exercises the abci.go IsNil guard: when
// the params row is absent, GetParams returns a zero-value Params whose
// MinGasMultiplier wraps a nil *big.Int and Mul would panic. EndBlock must
// treat that as zero and fall back to gasUsed.
func (suite *KeeperTestSuite) TestEndBlock_NilMinGasMultiplier() {
	suite.SetupTest()

	storeKey := suite.app.GetKey(types.StoreKey)
	suite.Require().NotNil(storeKey)
	suite.ctx.KVStore(storeKey).Delete(types.ParamsKey)
	suite.Require().True(suite.app.FeeMarketKeeper.GetParams(suite.ctx).MinGasMultiplier.IsNil())

	meter := storetypes.NewGasMeter(uint64(1_000_000_000))
	suite.ctx = suite.ctx.WithBlockGasMeter(meter)
	suite.app.FeeMarketKeeper.SetTransientBlockGasWanted(suite.ctx, 5_000_000)

	suite.Require().NotPanics(func() {
		suite.Require().NoError(suite.app.FeeMarketKeeper.EndBlock(suite.ctx))
	})
	// With nil MinGasMultiplier the limit collapses to gasUsed (0 here, no tx).
	suite.Require().Equal(uint64(0), suite.app.FeeMarketKeeper.GetBlockGasWanted(suite.ctx))
}
