package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestDelayedWithdrawalRecord(t *testing.T) {
	keeper, ctx, _ := makePaymentKeeper(t)

	addr := sample.RandAccAddress()
	from := sample.RandAccAddress()
	record := &types.DelayedWithdrawalRecord{
		Addr:            addr.String(),
		Amount:          sdkmath.NewInt(100),
		From:            from.String(),
		UnlockTimestamp: ctx.BlockTime().Unix() + 100,
	}

	// not found before it is ever set
	_, found := keeper.GetDelayedWithdrawalRecord(ctx, addr)
	require.False(t, found)

	// set then get
	keeper.SetDelayedWithdrawalRecord(ctx, record)
	got, found := keeper.GetDelayedWithdrawalRecord(ctx, addr)
	require.True(t, found)
	require.Equal(t, addr.String(), got.Addr)
	require.Equal(t, from.String(), got.From)
	require.True(t, record.Amount.Equal(got.Amount))
	require.Equal(t, record.UnlockTimestamp, got.UnlockTimestamp)

	// remove then get -> not found again
	keeper.RemoveDelayedWithdrawalRecord(ctx, addr)
	_, found = keeper.GetDelayedWithdrawalRecord(ctx, addr)
	require.False(t, found)
}
