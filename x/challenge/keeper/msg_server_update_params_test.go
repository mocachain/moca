package keeper_test

import (
	"testing"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge/types"
)

func (s *TestSuite) TestUpdateParams() {
	params := types.DefaultParams()
	params.HeartbeatInterval = 10

	tests := []struct {
		name    string
		msg     types.MsgUpdateParams
		errIs   error
		errText string
	}{
		{
			name: "invalid authority",
			msg: types.MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    types.DefaultParams(),
			},
			errIs: govtypes.ErrInvalidSigner,
		}, {
			name: "invalid params",
			msg: types.MsgUpdateParams{
				Authority: s.challengeKeeper.GetAuthority(),
				Params:    types.Params{},
			},
			errText: "keep alive period cannot be zero",
		}, {
			name: "valid authority accepts the params invalid authority was rejected with",
			msg: types.MsgUpdateParams{
				Authority: s.challengeKeeper.GetAuthority(),
				Params:    types.DefaultParams(),
			},
		}, {
			name: "success",
			msg: types.MsgUpdateParams{
				Authority: s.challengeKeeper.GetAuthority(),
				Params:    params,
			},
		},
	}
	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			msg := tt.msg
			_, err := s.msgServer.UpdateParams(s.ctx, &msg)
			switch {
			case tt.errIs != nil:
				require.ErrorIs(t, err, tt.errIs)
			case tt.errText != "":
				require.ErrorContains(t, err, tt.errText)
			default:
				require.NoError(t, err)
			}
		})
	}

	// verify storage
	s.Require().Equal(params, s.challengeKeeper.GetParams(s.ctx))
}
