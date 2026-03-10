package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestInitGenesis() {
	// check calculated epochMintProvision at genesis
	epochMintProvision := suite.app.InflationKeeper.GetEpochMintProvision(suite.ctx)
	expMintProvision, _ := sdk.NewDecFromStr("847602739726027397260274.000000000000000000")
	suite.Require().Equal(expMintProvision, epochMintProvision)
}
