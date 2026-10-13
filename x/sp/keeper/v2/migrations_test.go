package v2_test

import (
	"encoding/hex"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	v2 "github.com/mocachain/moca/v2/x/sp/keeper/v2"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// v1ParamsHex is the x/sp params record (key 0x01 of the sp store) as mainnet stores it
// before consensus version 2: fields 1-7 set, update_price_disallowed_days 0, no max_sp_price.
const v1ParamsHex = "0a05616d6f6361121731303030303030303030303030303030303030303030301a123132303030303030303030303030303030302080de3428e0a80130e0a8013801"

func newStore(t *testing.T) (*codec.ProtoCodec, storetypes.KVStore, func() error) {
	t.Helper()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	return cdc, ctx.KVStore(storeKey), func() error { return v2.MigrateStore(ctx, storeKey, cdc) }
}

// TestMigrateStore checks the migration sets max_sp_price to its default and keeps the
// stored v1 fields byte for byte.
func TestMigrateStore(t *testing.T) {
	cdc, store, migrate := newStore(t)
	v1, err := hex.DecodeString(v1ParamsHex)
	require.NoError(t, err)
	store.Set(types.ParamsKey, v1)

	var before types.Params
	require.NoError(t, cdc.Unmarshal(v1, &before))
	require.True(t, before.MaxSpPrice.IsNil())

	require.NoError(t, migrate())

	migrated := store.Get(types.ParamsKey)
	require.Equal(t, v1, migrated[:len(v1)])
	var after types.Params
	require.NoError(t, cdc.Unmarshal(migrated, &after))
	require.True(t, types.DefaultMaxSpPrice.Equal(after.MaxSpPrice))
	require.NoError(t, after.Validate())
	after.MaxSpPrice = before.MaxSpPrice
	require.Equal(t, before, after)
}

func TestMigrateStore_KeepsSetValue(t *testing.T) {
	cdc, store, migrate := newStore(t)
	params := types.DefaultParams()
	params.MaxSpPrice = math.LegacyNewDec(1000)
	store.Set(types.ParamsKey, cdc.MustMarshal(&params))

	require.NoError(t, migrate())

	var after types.Params
	require.NoError(t, cdc.Unmarshal(store.Get(types.ParamsKey), &after))
	require.Equal(t, params, after)
}

func TestMigrateStore_MissingParams(t *testing.T) {
	_, _, migrate := newStore(t)
	require.ErrorContains(t, migrate(), "sp params not found")
}
