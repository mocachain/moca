package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
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

	badMsg := NewMsgWithdraw(invalidAddress, sample.RandAccAddressHex(), sdkmath.NewInt(1))
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
				Creator: invalidAddress,
				From:    sample.RandAccAddressHex(),
				Amount:  sdkmath.NewInt(1),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid from",
			msg: MsgWithdraw{
				Creator: sample.RandAccAddressHex(),
				From:    invalidAddress,
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
				Authority: invalidAddress,
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
	badMsg := MsgUpdateParams{Authority: invalidAddress, Params: DefaultParams()}
	require.NotPanics(t, func() { badMsg.GetSigners() })
	require.Len(t, badMsg.GetSigners(), 1)
}
