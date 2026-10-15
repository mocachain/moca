package gensp_test

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/gensp"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
)

// TestInitGenesis covers genesis.go's package-level InitGenesis, a thin
// wrapper that only calls DeliverGenTxs when GenspTxs is non-empty. It is a
// method on GenTxTestSuite (gentx_test.go) so it reuses that suite's
// encoding config, staking keeper mock, and context instead of re-deriving
// them; testify discovers it automatically via TestGenTxTestSuite.
func (suite *GenTxTestSuite) TestInitGenesis() {
	testCases := []struct {
		msg              string
		malleate         func() gensptypes.GenesisState
		setupStakingMock func()
		expPass          bool
		wantValidators   []abci.ValidatorUpdate
	}{
		{
			"empty GenspTxs is a no-op",
			func() gensptypes.GenesisState {
				return *gensptypes.DefaultGenesisState()
			},
			nil,
			true,
			nil,
		},
		{
			"non-empty GenspTxs delivers the gentx and returns the staking keeper's validator updates",
			func() gensptypes.GenesisState {
				return gensptypes.GenesisState{
					GenspTxs: []json.RawMessage{suite.genSignedSendTxJSON()},
				}
			},
			func() {
				suite.stakingKeeper.EXPECT().
					ApplyAndReturnValidatorSetUpdates(gomock.Any()).
					Return([]abci.ValidatorUpdate{{Power: 7}}, nil)
			},
			true,
			[]abci.ValidatorUpdate{{Power: 7}},
		},
		{
			"non-empty GenspTxs propagates a gentx decode failure",
			func() gensptypes.GenesisState {
				return gensptypes.GenesisState{
					GenspTxs: []json.RawMessage{[]byte("{not valid json")},
				}
			},
			nil,
			false,
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()

			genState := tc.malleate()
			if tc.setupStakingMock != nil {
				tc.setupStakingMock()
			}

			validators, err := gensp.InitGenesis(
				suite.ctx, suite.stakingKeeper, &mockTxHandler{}, genState, suite.encodingConfig.TxConfig,
			)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.wantValidators, validators)
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(validators)
			}
		})
	}
}
