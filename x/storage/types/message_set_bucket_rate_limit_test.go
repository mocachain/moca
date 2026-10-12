package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	gnfderrors "github.com/mocachain/moca/v2/types/errors"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgSetBucketFlowRateLimit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSetBucketFlowRateLimit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgSetBucketFlowRateLimit{
				Operator:   "invalid_address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid address",
			msg: MsgSetBucketFlowRateLimit{
				Operator:       sample.RandAccAddressHex(),
				PaymentAddress: "invalid address",
				BucketName:     testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid address",
			msg: MsgSetBucketFlowRateLimit{
				Operator:       sample.RandAccAddressHex(),
				PaymentAddress: sample.RandAccAddressHex(),
				BucketOwner:    "invalid address",
				BucketName:     testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgSetBucketFlowRateLimit{
				Operator:       sample.RandAccAddressHex(),
				PaymentAddress: sample.RandAccAddressHex(),
				BucketOwner:    sample.RandAccAddressHex(),
				BucketName:     string(testInvalidBucketNameWithLongLength[:]),
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid flow rate limit",
			msg: MsgSetBucketFlowRateLimit{
				Operator:       sample.RandAccAddressHex(),
				PaymentAddress: sample.RandAccAddressHex(),
				BucketOwner:    sample.RandAccAddressHex(),
				BucketName:     testBucketName,
				FlowRateLimit:  sdkmath.NewInt(-1),
			},
			err: sdkerrors.ErrInvalidRequest,
		}, {
			name: "valid case",
			msg: MsgSetBucketFlowRateLimit{
				Operator:       sample.RandAccAddressHex(),
				PaymentAddress: sample.RandAccAddressHex(),
				BucketOwner:    sample.RandAccAddressHex(),
				BucketName:     testBucketName,
				FlowRateLimit:  sdkmath.NewInt(1),
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

func TestNewMsgSetBucketFlowRateLimit(t *testing.T) {
	operator := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	rateLimit := sdkmath.NewInt(500)
	msg := NewMsgSetBucketFlowRateLimit(operator, bucketOwner, paymentAccount, testBucketName, rateLimit)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, bucketOwner.String(), msg.BucketOwner)
	require.Equal(t, paymentAccount.String(), msg.PaymentAddress)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, rateLimit, msg.FlowRateLimit)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSetBucketFlowRateLimit, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid_address"
	require.Panics(t, func() { bad.GetSigners() })
}
