package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestVerifyPaymentAccount(t *testing.T) {
	k, ctx := makeKeeper(t)
	owner := sample.RandAccAddress()
	payer := sample.RandAccAddress()

	got, err := k.VerifyPaymentAccount(ctx, "", owner)
	require.NoError(t, err)
	require.True(t, got.Equals(owner), "an empty payment address falls back to the owner")

	got, err = k.VerifyPaymentAccount(ctx, payer.String(), owner)
	require.NoError(t, err)
	require.True(t, got.Equals(payer))

	_, err = k.VerifyPaymentAccount(ctx, "0xnothex", owner)
	require.Error(t, err)

	for _, addr := range []string{paymenttypes.GovernanceAddress.String(), sdk.AccAddress(paymenttypes.GovernanceAddress).String()} {
		_, err = k.VerifyPaymentAccount(ctx, addr, owner)
		require.ErrorIs(t, err, types.ErrInvalidPaymentAddress, "the payment governance account must be rejected as a payment address")
	}
}
