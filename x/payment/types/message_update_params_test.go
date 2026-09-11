package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	wrongParams := DefaultParams()
	wrongParams.ForcedSettleTime = 0

	tests := []struct {
		name string
		msg  MsgUpdateParams
		err  error
	}{
		{
			name: "invalid authority",
			msg: MsgUpdateParams{
				Authority: "invalid_address",
				Params:    DefaultParams(),
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid params",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    wrongParams,
			},
			err: ErrInvalidParams,
		}, {
			name: "valid authority and params",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    DefaultParams(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgUpdateParams_GetSignBytes(t *testing.T) {
	msg := MsgUpdateParams{Authority: sample.RandAccAddressHex(), Params: DefaultParams()}
	require.Contains(t, string(msg.GetSignBytes()), msg.Authority)
}

func TestMsgUpdateParams_GetSigners(t *testing.T) {
	authority := sample.RandAccAddress()
	msg := MsgUpdateParams{Authority: authority.String(), Params: DefaultParams()}
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())

	// Unlike the other payment messages, GetSigners swallows the address
	// parse error instead of panicking on an invalid authority -- it still
	// returns a single (zero-value) signer rather than an empty list.
	badMsg := MsgUpdateParams{Authority: "invalid_address", Params: DefaultParams()}
	require.NotPanics(t, func() { badMsg.GetSigners() })
	require.Len(t, badMsg.GetSigners(), 1)
}
