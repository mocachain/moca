package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestNewMsgCreatePaymentAccount(t *testing.T) {
	creator := sample.RandAccAddressHex()
	msg := NewMsgCreatePaymentAccount(creator)
	require.Equal(t, creator, msg.Creator)
}

func TestMsgCreatePaymentAccount_Route(t *testing.T) {
	msg := NewMsgCreatePaymentAccount(sample.RandAccAddressHex())
	require.Equal(t, RouterKey, msg.Route())
}

func TestMsgCreatePaymentAccount_Type(t *testing.T) {
	msg := NewMsgCreatePaymentAccount(sample.RandAccAddressHex())
	require.Equal(t, TypeMsgCreatePaymentAccount, msg.Type())
}

func TestMsgCreatePaymentAccount_GetSigners(t *testing.T) {
	creator := sample.RandAccAddress()
	msg := NewMsgCreatePaymentAccount(creator.String())
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())

	badMsg := NewMsgCreatePaymentAccount(invalidAddress)
	require.Panics(t, func() { badMsg.GetSigners() })
}

func TestMsgCreatePaymentAccount_GetSignBytes(t *testing.T) {
	msg := NewMsgCreatePaymentAccount(sample.RandAccAddressHex())
	require.Contains(t, string(msg.GetSignBytes()), msg.Creator)
}

func TestMsgCreatePaymentAccount_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCreatePaymentAccount
		err  error
	}{
		{
			name: "valid address",
			msg:  *NewMsgCreatePaymentAccount(sample.RandAccAddressHex()),
		},
		{
			name: "invalid address",
			msg:  MsgCreatePaymentAccount{Creator: invalidAddress},
			err:  sdkerrors.ErrInvalidAddress,
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
