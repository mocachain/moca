package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestPaymentAccount(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)

	owner1 := sample.RandAccAddress()
	addr1 := sample.RandAccAddress()
	paymentAccount1 := &types.PaymentAccount{
		Owner:      owner1.String(),
		Addr:       addr1.String(),
		Refundable: true,
	}

	owner2 := sample.RandAccAddress()
	addr2 := sample.RandAccAddress()
	paymentAccount2 := &types.PaymentAccount{
		Owner:      owner2.String(),
		Addr:       addr2.String(),
		Refundable: false,
	}

	// set
	keeper.SetPaymentAccount(ctx, paymentAccount1)
	keeper.SetPaymentAccount(ctx, paymentAccount2)

	// get
	resp1, _ := keeper.GetPaymentAccount(ctx, addr1)
	require.True(t, resp1.Owner == owner1.String())
	require.True(t, resp1.Addr == addr1.String())
	require.True(t, resp1.Refundable == paymentAccount1.Refundable)

	resp2, _ := keeper.GetPaymentAccount(ctx, addr2)
	require.True(t, resp2.Owner == owner2.String())
	require.True(t, resp2.Addr == addr2.String())
	require.True(t, resp2.Refundable == paymentAccount2.Refundable)

	_, found := keeper.GetPaymentAccount(ctx, sample.RandAccAddress())
	require.True(t, !found)

	// get all
	resp3 := keeper.GetAllPaymentAccount(ctx)
	require.True(t, len(resp3) == 2)
}

func TestIsPaymentAccountOwner(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)

	// identity: an address is always its own owner, payment account or not
	addr := sample.RandAccAddress()
	require.True(t, keeper.IsPaymentAccountOwner(ctx, addr, addr))

	// no payment account recorded for addr -> false
	other := sample.RandAccAddress()
	require.False(t, keeper.IsPaymentAccountOwner(ctx, addr, other))

	// payment account recorded, but owner is a different, valid address -> false
	realOwner := sample.RandAccAddress()
	keeper.SetPaymentAccount(ctx, &types.PaymentAccount{
		Addr:  addr.String(),
		Owner: realOwner.String(),
	})
	require.False(t, keeper.IsPaymentAccountOwner(ctx, addr, other))
	require.True(t, keeper.IsPaymentAccountOwner(ctx, addr, realOwner))
}
