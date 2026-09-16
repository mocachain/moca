package types

import (
	"encoding/binary"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestAutoSettleRecordKey(t *testing.T) {
	addr := sample.RandAccAddress()
	timestamp := int64(1234567890)

	key := AutoSettleRecordKey(timestamp, addr)
	res := ParseAutoSettleRecordKey(key)

	require.Equal(t, timestamp, res.Timestamp)
	require.Equal(t, addr.String(), res.Addr)
}

func TestAutoResumeRecordKey(t *testing.T) {
	addr := sample.RandAccAddress()
	timestamp := int64(987654321)

	key := AutoResumeRecordKey(timestamp, addr)
	res := ParseAutoResumeRecordKey(key)

	require.Equal(t, timestamp, res.Timestamp)
	require.Equal(t, addr.String(), res.Addr)
}

func TestPaymentAccountKey(t *testing.T) {
	addr := sample.RandAccAddress()
	require.Equal(t, []byte(addr), PaymentAccountKey(addr))
}

func TestPaymentAccountCountKey(t *testing.T) {
	owner := sample.RandAccAddress()
	require.Equal(t, []byte(owner), PaymentAccountCountKey(owner))
}

func TestStreamRecordKey(t *testing.T) {
	account := sample.RandAccAddress()
	require.Equal(t, []byte(account), StreamRecordKey(account))
}

func TestDelayedWithdrawalKey(t *testing.T) {
	account := sample.RandAccAddress()
	require.Equal(t, []byte(account), DelayedWithdrawalKey(account))
}

func TestVersionedParamsKey(t *testing.T) {
	key := VersionedParamsKey(42)

	require.Equal(t, ParamsKey, key[:len(ParamsKey)])
	require.Equal(t, uint64(42), binary.BigEndian.Uint64(key[len(ParamsKey):]))
}

func TestOutFlowKey(t *testing.T) {
	addr := sample.RandAccAddress()
	toAddr := sample.RandAccAddress()

	tests := []struct {
		name   string
		status OutFlowStatus
		toAddr sdk.AccAddress
	}{
		{name: "active, no to-address", status: OUT_FLOW_STATUS_ACTIVE, toAddr: nil},
		{name: "active, with to-address", status: OUT_FLOW_STATUS_ACTIVE, toAddr: toAddr},
		{name: "frozen, no to-address", status: OUT_FLOW_STATUS_FROZEN, toAddr: nil},
		{name: "frozen, with to-address", status: OUT_FLOW_STATUS_FROZEN, toAddr: toAddr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := OutFlowKey(addr, tt.status, tt.toAddr)
			gotAddr, res := ParseOutFlowKey(key)

			require.Equal(t, addr, gotAddr)
			require.Equal(t, tt.status, res.Status)
			require.Equal(t, sdk.AccAddress(tt.toAddr.Bytes()).String(), res.ToAddress)
		})
	}
}

func TestParseOutFlowValue(t *testing.T) {
	rate := sdkmath.NewInt(12345)
	bz, err := rate.Marshal()
	require.NoError(t, err)

	got := ParseOutFlowValue(bz)
	require.True(t, rate.Equal(got))

	require.Panics(t, func() { ParseOutFlowValue([]byte("not-a-number")) })
}
