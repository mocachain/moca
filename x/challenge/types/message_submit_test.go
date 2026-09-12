package types

import (
	"testing"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

func TestNewMsgSubmit(t *testing.T) {
	challenger := sample.RandAccAddress()
	spOperatorAddress := sample.RandAccAddress()

	msg := NewMsgSubmit(challenger, spOperatorAddress, "bucket", "object", true, 5)

	require.Equal(t, challenger.String(), msg.Challenger)
	require.Equal(t, spOperatorAddress.String(), msg.SpOperatorAddress)
	require.Equal(t, "bucket", msg.BucketName)
	require.Equal(t, "object", msg.ObjectName)
	require.True(t, msg.RandomIndex)
	require.Equal(t, uint32(5), msg.SegmentIndex)
}

func TestMsgSubmit_RouteAndType(t *testing.T) {
	msg := MsgSubmit{}
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSubmit, msg.Type())
}

func TestMsgSubmit_GetSigners(t *testing.T) {
	challenger := sample.RandAccAddress()
	msg := MsgSubmit{Challenger: challenger.String()}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, challenger, signers[0])

	invalid := MsgSubmit{Challenger: "invalid_address"}
	require.Panics(t, func() { invalid.GetSigners() })
}

func TestMsgSubmit_GetSignBytes(t *testing.T) {
	msg := MsgSubmit{
		Challenger:        sample.RandAccAddressHex(),
		SpOperatorAddress: sample.RandAccAddressHex(),
		BucketName:        "bucket",
		ObjectName:        "object",
	}

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)

	var decoded MsgSubmit
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, &decoded))
	require.Equal(t, msg.Challenger, decoded.Challenger)
	require.Equal(t, msg.BucketName, decoded.BucketName)
	require.Equal(t, msg.ObjectName, decoded.ObjectName)
}

func TestMsgSubmit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSubmit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgSubmit{
				Challenger: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid sp operator address",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        "1",
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        "bucket",
				ObjectName:        "",
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "valid message with random index",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        "bucket",
				ObjectName:        "object",
				RandomIndex:       true,
				SegmentIndex:      10,
			},
		}, {
			name: "valid message with specific index",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        "bucket",
				ObjectName:        "object",
				RandomIndex:       false,
				SegmentIndex:      2,
			},
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
