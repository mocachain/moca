package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestNewMsgWithdraw(t *testing.T) {
	creator := sample.RandAccAddressHex()
	from := sample.RandAccAddressHex()
	amount := sdkmath.NewInt(100)

	msg := NewMsgWithdraw(creator, from, amount)
	require.Equal(t, creator, msg.Creator)
	require.Equal(t, from, msg.From)
	require.True(t, amount.Equal(msg.Amount))
}

func TestMsgWithdraw_Route(t *testing.T) {
	msg := NewMsgWithdraw(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, RouterKey, msg.Route())
}

func TestMsgWithdraw_Type(t *testing.T) {
	msg := NewMsgWithdraw(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, TypeMsgWithdraw, msg.Type())
}

func TestMsgWithdraw_GetSigners(t *testing.T) {
	creator := sample.RandAccAddress()
	msg := NewMsgWithdraw(creator.String(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())

	badMsg := NewMsgWithdraw("invalid_address", sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Panics(t, func() { badMsg.GetSigners() })
}

func TestMsgWithdraw_GetSignBytes(t *testing.T) {
	msg := NewMsgWithdraw(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Contains(t, string(msg.GetSignBytes()), msg.Creator)
}

func TestMsgWithdraw_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgWithdraw
		err  error
	}{
		{
			name: "valid with from",
			msg:  *NewMsgWithdraw(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1)),
		},
		{
			// From is optional: an empty value skips the from-address check
			// entirely (see message_withdraw.go's ValidateBasic).
			name: "valid with empty from",
			msg:  *NewMsgWithdraw(sample.RandAccAddressHex(), "", sdkmath.NewInt(1)),
		},
		{
			name: "invalid creator",
			msg: MsgWithdraw{
				Creator: "invalid_address",
				From:    sample.RandAccAddressHex(),
				Amount:  sdkmath.NewInt(1),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid from",
			msg: MsgWithdraw{
				Creator: sample.RandAccAddressHex(),
				From:    "invalid_address",
				Amount:  sdkmath.NewInt(1),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "nil amount",
			msg: MsgWithdraw{
				Creator: sample.RandAccAddressHex(),
				From:    sample.RandAccAddressHex(),
			},
			err: sdkerrors.ErrInvalidCoins,
		},
		{
			name: "negative amount",
			msg: MsgWithdraw{
				Creator: sample.RandAccAddressHex(),
				From:    sample.RandAccAddressHex(),
				Amount:  sdkmath.NewInt(-1),
			},
			err: sdkerrors.ErrInvalidCoins,
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
