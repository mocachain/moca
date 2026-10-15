package types_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestNewStreamRecord(t *testing.T) {
	addr := sample.RandAccAddress()
	crudTimestamp := int64(1717171717)

	record := types.NewStreamRecord(addr, crudTimestamp)

	require.Equal(t, addr.String(), record.Account)
	require.Equal(t, crudTimestamp, record.CrudTimestamp)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, record.Status)
	require.True(t, sdkmath.ZeroInt().Equal(record.StaticBalance))
	require.True(t, sdkmath.ZeroInt().Equal(record.BufferBalance))
	require.True(t, sdkmath.ZeroInt().Equal(record.NetflowRate))
	require.True(t, sdkmath.ZeroInt().Equal(record.LockBalance))
}
