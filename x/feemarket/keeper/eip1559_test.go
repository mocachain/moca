package keeper_test

import (
	"fmt"
	"math/big"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

func (suite *KeeperTestSuite) TestCalculateBaseFee() {
	// Captured per-case after SetupTest so expFee tracks the current BaseFee.
	var initialBaseFee math.Int

	testCases := []struct {
		name                 string
		NoBaseFee            bool
		blockHeight          int64
		parentBlockGasWanted uint64
		minGasPrice          math.LegacyDec
		expFee               func() *big.Int
	}{
		{
			"without BaseFee",
			true, 0, 0, math.LegacyZeroDec(),
			nil,
		},
		{
			"with BaseFee - initial EIP-1559 block",
			false, 0, 0, math.LegacyZeroDec(),
			func() *big.Int { return initialBaseFee.BigInt() },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target (ElasticityMultiplier = 2)",
			false, 1, 50, math.LegacyZeroDec(),
			func() *big.Int { return initialBaseFee.BigInt() },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target, with higher min gas price (ElasticityMultiplier = 2)",
			false, 1, 50, math.LegacyNewDec(1500000000),
			func() *big.Int { return initialBaseFee.BigInt() },
		},
		{
			"with BaseFee - parent block wanted more gas than its target (ElasticityMultiplier = 2)",
			false, 1, 100, math.LegacyZeroDec(),
			// delta = parent * (gasUsed - target) / target / denom = parent * 50 / 50 / 8 = parent/8
			func() *big.Int { return initialBaseFee.Add(initialBaseFee.QuoRaw(8)).BigInt() },
		},
		{
			"with BaseFee - parent block wanted more gas than its target, with higher min gas price (ElasticityMultiplier = 2)",
			false, 1, 100, math.LegacyNewDec(1500000000),
			func() *big.Int { return initialBaseFee.Add(initialBaseFee.QuoRaw(8)).BigInt() },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target (ElasticityMultiplier = 2)",
			false, 1, 25, math.LegacyZeroDec(),
			// delta = parent * (target - gasUsed) / target / denom = parent * 25 / 50 / 8 = parent/16
			func() *big.Int { return initialBaseFee.Sub(initialBaseFee.QuoRaw(16)).BigInt() },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target, with higher min gas price (ElasticityMultiplier = 2)",
			false, 1, 25, math.LegacyNewDec(1500000000),
			// Clamped to minGasPrice.
			func() *big.Int { return big.NewInt(1500000000) },
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset

			params := suite.app.FeeMarketKeeper.GetParams(suite.ctx)
			params.NoBaseFee = tc.NoBaseFee
			params.MinGasPrice = tc.minGasPrice
			err := suite.app.FeeMarketKeeper.SetParams(suite.ctx, params)
			suite.Require().NoError(err)

			initialBaseFee = params.BaseFee

			suite.ctx = suite.ctx.WithBlockHeight(tc.blockHeight)
			suite.app.FeeMarketKeeper.SetBlockGasWanted(suite.ctx, tc.parentBlockGasWanted)

			blockParams := tmproto.BlockParams{
				MaxGas:   100,
				MaxBytes: 10,
			}
			consParams := tmproto.ConsensusParams{Block: &blockParams}
			suite.ctx = suite.ctx.WithConsensusParams(consParams)

			fee := suite.app.FeeMarketKeeper.CalculateBaseFee(suite.ctx)
			if tc.NoBaseFee {
				suite.Require().Nil(fee, tc.name)
			} else {
				suite.Require().Equal(tc.expFee(), fee, tc.name)
			}
		})
	}
}
