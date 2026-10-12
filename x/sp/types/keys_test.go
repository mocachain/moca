package types_test

import (
	"encoding/binary"
	stdmath "math"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestGetDepositLockKey(t *testing.T) {
	key := types.GetDepositLockKey(7)
	require.Equal(t, types.DepositLockKeyPrefix, key[:len(types.DepositLockKeyPrefix)])
	require.Equal(t, uint32(7), binary.BigEndian.Uint32(key[len(types.DepositLockKeyPrefix):]))
}

func TestGetStorageProviderKey(t *testing.T) {
	id := []byte{0, 0, 0, 5}
	key := types.GetStorageProviderKey(id)
	require.Equal(t, types.StorageProviderKey, key[:len(types.StorageProviderKey)])
	require.Equal(t, id, key[len(types.StorageProviderKey):])
}

func TestGetStorageProviderByAddrKeys(t *testing.T) {
	addr := sample.RandAccAddress()

	tests := []struct {
		name   string
		prefix []byte
		key    []byte
	}{
		{"operator", types.StorageProviderByOperatorAddrKey, types.GetStorageProviderByOperatorAddrKey(addr)},
		{"funding", types.StorageProviderByFundingAddrKey, types.GetStorageProviderByFundingAddrKey(addr)},
		{"seal", types.StorageProviderBySealAddrKey, types.GetStorageProviderBySealAddrKey(addr)},
		{"approval", types.StorageProviderByApprovalAddrKey, types.GetStorageProviderByApprovalAddrKey(addr)},
		{"gc", types.StorageProviderByGcAddrKey, types.GetStorageProviderByGcAddrKey(addr)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.prefix, tc.key[:len(tc.prefix)])
			require.Equal(t, addr.Bytes(), tc.key[len(tc.prefix):])
		})
	}
}

func TestGetStorageProviderByBlsKeyKey(t *testing.T) {
	blsPk := sample.RandBlsPubKey()
	key := types.GetStorageProviderByBlsKeyKey(blsPk)
	require.Equal(t, types.StorageProviderByBlsPubKeyKey, key[:len(types.StorageProviderByBlsPubKeyKey)])
	require.Equal(t, blsPk, key[len(types.StorageProviderByBlsPubKeyKey):])
}

func TestGetStorageProviderMaintenanceRecordsKey(t *testing.T) {
	addr := sample.RandAccAddress()
	key := types.GetStorageProviderMaintenanceRecordsKey(addr)
	require.Equal(t, types.StorageProviderMaintenanceRecordPrefix, key[:len(types.StorageProviderMaintenanceRecordPrefix)])
	require.Equal(t, addr.Bytes(), key[len(types.StorageProviderMaintenanceRecordPrefix):])
}

func TestMarshalUnmarshalStorageProvider(t *testing.T) {
	cdc := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{}).Codec
	operator := sample.RandAccAddress()
	sp := types.StorageProvider{Id: 1, OperatorAddress: operator.String(), TotalDeposit: math.NewInt(1)}

	bz := types.MustMarshalStorageProvider(cdc, &sp)
	require.NotEmpty(t, bz)

	got, err := types.UnmarshalStorageProvider(cdc, bz)
	require.NoError(t, err)
	require.Equal(t, sp.OperatorAddress, got.OperatorAddress)
	require.Equal(t, sp.Id, got.Id)
	require.True(t, sp.TotalDeposit.Equal(got.TotalDeposit))

	gotMust := types.MustUnmarshalStorageProvider(cdc, bz)
	require.Equal(t, got, gotMust)

	_, err = types.UnmarshalStorageProvider(cdc, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	require.Error(t, err)

	require.Panics(t, func() {
		types.MustUnmarshalStorageProvider(cdc, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	})
}

func TestSpStoragePriceKeyRoundTrip(t *testing.T) {
	for _, id := range []uint32{0, 1, 42, stdmath.MaxUint32} {
		key := types.SpStoragePriceKey(id)
		require.Equal(t, id, types.ParseSpStoragePriceKey(key))
	}
}

func TestGlobalSpStorePriceKeyRoundTrip(t *testing.T) {
	for _, ts := range []int64{0, 1700000000, stdmath.MaxInt64} {
		key := types.GlobalSpStorePriceKey(ts)
		require.Equal(t, ts, types.ParseGlobalSpStorePriceKey(key))
	}
}
