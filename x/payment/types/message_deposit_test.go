package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestNewMsgDeposit(t *testing.T) {
	creator := sample.RandAccAddressHex()
	to := sample.RandAccAddressHex()
	amount := sdkmath.NewInt(100)

	msg := NewMsgDeposit(creator, to, amount)
	require.Equal(t, creator, msg.Creator)
	require.Equal(t, to, msg.To)
	require.True(t, amount.Equal(msg.Amount))
}

func TestMsgDeposit_Route(t *testing.T) {
	msg := NewMsgDeposit(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, RouterKey, msg.Route())
}

func TestMsgDeposit_Type(t *testing.T) {
	msg := NewMsgDeposit(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, TypeMsgDeposit, msg.Type())
}

func TestMsgDeposit_GetSigners(t *testing.T) {
	creator := sample.RandAccAddress()
	msg := NewMsgDeposit(creator.String(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())

	badMsg := NewMsgDeposit(invalidAddress, sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Panics(t, func() { badMsg.GetSigners() })
}

func TestMsgDeposit_GetSignBytes(t *testing.T) {
	msg := NewMsgDeposit(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1))
	require.Contains(t, string(msg.GetSignBytes()), msg.Creator)
}

func TestMsgDeposit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeposit
		err  error
	}{
		{
			name: "valid",
			msg:  *NewMsgDeposit(sample.RandAccAddressHex(), sample.RandAccAddressHex(), sdkmath.NewInt(1)),
		},
		{
			name: "invalid creator",
			msg: MsgDeposit{
				Creator: invalidAddress,
				To:      sample.RandAccAddressHex(),
				Amount:  sdkmath.NewInt(1),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid to",
			msg: MsgDeposit{
				Creator: sample.RandAccAddressHex(),
				To:      invalidAddress,
				Amount:  sdkmath.NewInt(1),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "nil amount",
			msg: MsgDeposit{
				Creator: sample.RandAccAddressHex(),
				To:      sample.RandAccAddressHex(),
			},
			err: sdkerrors.ErrInvalidCoins,
		},
		{
			name: "zero amount",
			msg: MsgDeposit{
				Creator: sample.RandAccAddressHex(),
				To:      sample.RandAccAddressHex(),
				Amount:  sdkmath.ZeroInt(),
			},
			err: sdkerrors.ErrInvalidCoins,
		},
		{
			name: "negative amount",
			msg: MsgDeposit{
				Creator: sample.RandAccAddressHex(),
				To:      sample.RandAccAddressHex(),
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
