package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgUpdateParams_GetSignBytes(t *testing.T) {
	msg := MsgUpdateParams{
		Authority: sample.RandAccAddressHex(),
		Params:    DefaultParams(),
	}
	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)

	// The sign bytes must decode back to the exact same message content.
	var decoded MsgUpdateParams
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, &decoded))
	require.Equal(t, msg.Authority, decoded.Authority)
	require.Equal(t, msg.Params, decoded.Params)

	// Deterministic: identical content must sign identically every time.
	require.Equal(t, bz, msg.GetSignBytes())
}

func TestMsgUpdateParams_GetSigners(t *testing.T) {
	addr := sample.RandAccAddress()
	msg := MsgUpdateParams{Authority: addr.String()}
	require.Equal(t, []sdk.AccAddress{addr}, msg.GetSigners())

	invalid := MsgUpdateParams{Authority: "not-hex"}
	signers := invalid.GetSigners()
	require.Len(t, signers, 1)
	require.True(t, signers[0].Empty())
}

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	validAuthority := sample.RandAccAddressHex()

	tests := []struct {
		name    string
		msg     MsgUpdateParams
		wantErr bool
	}{
		{
			name: "valid",
			msg:  MsgUpdateParams{Authority: validAuthority, Params: DefaultParams()},
		},
		{
			name:    "invalid authority",
			msg:     MsgUpdateParams{Authority: "not-hex", Params: DefaultParams()},
			wantErr: true,
		},
		{
			name:    "invalid params",
			msg:     MsgUpdateParams{Authority: validAuthority, Params: NewParams(0, 0, 0)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
