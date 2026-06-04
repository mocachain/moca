package keeper_test

import (
	"github.com/ethereum/go-ethereum/common"

	erc20types "github.com/mocachain/moca/v2/x/erc20/types"
)

func (suite *KeeperTestSuite) TestPrecompilesIncludesDynamicERC20TokenPair() {
	tokenAddr := common.HexToAddress("0x0000000000000000000000000000000000002100")
	pair := erc20types.NewTokenPair(tokenAddr, suite.denom, erc20types.OWNER_MODULE)

	params := suite.app.Erc20Keeper.GetParams(suite.ctx)
	params.DynamicPrecompiles = []string{tokenAddr.Hex()}
	suite.Require().NoError(suite.app.Erc20Keeper.SetParams(suite.ctx, params))
	suite.app.Erc20Keeper.SetToken(suite.ctx, pair)

	activeAddrs, activePrecompiles := suite.app.EvmKeeper.Precompiles(suite.ctx)

	suite.Require().Contains(activePrecompiles, tokenAddr)
	suite.Require().Equal(tokenAddr, activePrecompiles[tokenAddr].Address())
	suite.Require().Contains(activeAddrs, tokenAddr)
}
