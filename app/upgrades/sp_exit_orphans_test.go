package upgrades_test

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/app/upgrades"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	spkeeper "github.com/mocachain/moca/v2/x/sp/keeper"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// TestPruneExitedStorageProviderEntries seeds one live storage provider with a price
// and a maintenance record next to the entries of providers that no longer exist,
// and verifies the upgrade step removes exactly the orphaned entries and is
// idempotent.
func TestPruneExitedStorageProviderEntries(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	storeKey := storetypes.NewKVStoreKey(sptypes.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx
	store := ctx.KVStore(storeKey)

	ctrl := gomock.NewController(t)
	k := spkeeper.NewKeeper(
		encCfg.Codec,
		storeKey,
		sptypes.NewMockAccountKeeper(ctrl),
		sptypes.NewMockBankKeeper(ctrl),
		sptypes.NewMockAuthzKeeper(ctrl),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	price := func(spID uint32) sptypes.SpStoragePrice {
		return sptypes.SpStoragePrice{
			SpId:          spID,
			UpdateTimeSec: 1,
			ReadPrice:     math.LegacyNewDec(100),
			StorePrice:    math.LegacyNewDec(100),
		}
	}
	records := encCfg.Codec.MustMarshal(&sptypes.SpMaintenanceStats{
		Records: []*sptypes.MaintenanceRecord{{Height: 1, RequestDuration: 10, RequestAt: 1, ActualDuration: 10}},
	})

	// A live provider keeps its price and its maintenance records.
	liveOperator := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	live := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: liveOperator.String(),
		Status:          sptypes.STATUS_IN_SERVICE,
	}
	k.SetStorageProvider(ctx, live)
	k.SetStorageProviderByOperatorAddr(ctx, live)
	k.SetSpStoragePrice(ctx, price(live.Id))
	liveRecordsKey := sptypes.GetStorageProviderMaintenanceRecordsKey(liveOperator)
	store.Set(liveRecordsKey, records)

	// Entries left behind by providers that exited before Exit cleaned them up.
	k.SetSpStoragePrice(ctx, price(2))
	k.SetSpStoragePrice(ctx, price(3))
	orphanRecordsKey := sptypes.GetStorageProviderMaintenanceRecordsKey(sdk.MustAccAddressFromHex(sample.RandAccAddressHex()))
	store.Set(orphanRecordsKey, records)
	require.Len(t, k.GetAllSpStoragePrice(ctx), 3)

	prices, recs, err := upgrades.PruneExitedStorageProviderEntries(ctx, *k, storeKey)
	require.NoError(t, err)
	require.Equal(t, 2, prices)
	require.Equal(t, 1, recs)

	_, found := k.GetSpStoragePrice(ctx, live.Id)
	require.True(t, found, "the live provider's price must survive")
	require.NotNil(t, store.Get(liveRecordsKey), "the live provider's maintenance records must survive")
	for _, spID := range []uint32{2, 3} {
		_, found = k.GetSpStoragePrice(ctx, spID)
		require.False(t, found, "the price of exited provider %d must be removed", spID)
	}
	require.Nil(t, store.Get(orphanRecordsKey), "the exited provider's maintenance records must be removed")

	// A second run finds nothing to remove and leaves the live entries alone.
	prices, recs, err = upgrades.PruneExitedStorageProviderEntries(ctx, *k, storeKey)
	require.NoError(t, err)
	require.Zero(t, prices)
	require.Zero(t, recs)
	require.Len(t, k.GetAllSpStoragePrice(ctx), 1)
	require.NotNil(t, store.Get(liveRecordsKey))
}
