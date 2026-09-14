package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestNewMsgDisableRefund(t *testing.T) {
	owner := sample.RandAccAddressHex()
	addr := sample.RandAccAddressHex()

	msg := NewMsgDisableRefund(owner, addr)
	require.Equal(t, owner, msg.Owner)
	require.Equal(t, addr, msg.Addr)
}

func TestMsgDisableRefund_Route(t *testing.T) {
	msg := NewMsgDisableRefund(sample.RandAccAddressHex(), sample.RandAccAddressHex())
	require.Equal(t, RouterKey, msg.Route())
}

func TestMsgDisableRefund_Type(t *testing.T) {
	msg := NewMsgDisableRefund(sample.RandAccAddressHex(), sample.RandAccAddressHex())
	require.Equal(t, TypeMsgDisableRefund, msg.Type())
}

func TestMsgDisableRefund_GetSigners(t *testing.T) {
	owner := sample.RandAccAddress()
	msg := NewMsgDisableRefund(owner.String(), sample.RandAccAddressHex())
	require.Equal(t, []sdk.AccAddress{owner}, msg.GetSigners())

	badMsg := NewMsgDisableRefund(invalidAddress, sample.RandAccAddressHex())
	require.Panics(t, func() { badMsg.GetSigners() })
}

func TestMsgDisableRefund_GetSignBytes(t *testing.T) {
	msg := NewMsgDisableRefund(sample.RandAccAddressHex(), sample.RandAccAddressHex())
	require.Contains(t, string(msg.GetSignBytes()), msg.Owner)
}

func TestMsgDisableRefund_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDisableRefund
		err  error
	}{
		{
			name: "valid",
			msg:  *NewMsgDisableRefund(sample.RandAccAddressHex(), sample.RandAccAddressHex()),
		},
		{
			name: "invalid owner",
			msg: MsgDisableRefund{
				Owner: invalidAddress,
				Addr:  sample.RandAccAddressHex(),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid addr",
			msg: MsgDisableRefund{
				Owner: sample.RandAccAddressHex(),
				Addr:  invalidAddress,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
