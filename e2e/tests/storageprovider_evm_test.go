package tests

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/mocachain/moca/v2/precompiles/storageprovider"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// priceUpdaters is how many of the seven genesis SPs double their quotes: a
// majority, so the medians have to follow, while the first three alone are a
// minority that must leave them where they were.
const priceUpdaters = 4

// TestStorageProviderPriceEvmFlow drives x/sp's per-SP storage pricing
// through the storageprovider precompile's updateSPPrice method and checks
// the global price x/sp's EndBlocker aggregates from the quotes the way the
// retired suite's TestUpdateSpStoragePrice and CheckGlobalSpStorePrice did:
// the global store and read prices are the medians over every serving SP's
// quote, the secondary store price is SecondarySpStorePriceRatio times the
// store median, a fresh aggregate stamped after a quote change appears within
// UpdateGlobalPriceInterval of it (localup pins the interval to 1s), and the
// by-time query still returns the previous aggregate for timestamps before
// the change. Four of the seven genesis SPs double their quotes: after the
// third the medians must be unchanged, after the fourth they must have
// doubled.
//
// The retired test's last step, an update inside the UpdatePriceDisallowedDays
// window being refused, needs the x/sp params changed through governance
// mid-test; the harness has no governance path and localup pins the window
// to 0 days, so that check is not reproduced here.
func TestStorageProviderPriceEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	spClient := sptypes.NewQueryClient(conn)
	precompile := storageprovider.Precompile{}
	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)

	paramsResp, err := spClient.Params(ctx, &sptypes.QueryParamsRequest{})
	require.NoError(t, err)
	params := paramsResp.Params
	require.NotZero(t, params.UpdateGlobalPriceInterval, "this test needs interval-based global price updates (localup sets 1s)")

	spExport := loadSPExport(t)
	require.GreaterOrEqual(t, len(spExport), 2*priceUpdaters-1, "%d updaters must be a majority of the exported SPs", priceUpdaters)

	quote := func(operatorAddr string) sptypes.SpStoragePrice {
		resp, err := spClient.QuerySpStoragePrice(ctx, &sptypes.QuerySpStoragePriceRequest{SpAddr: operatorAddr})
		require.NoError(t, err)
		return resp.SpStoragePrice
	}
	globalPrice := func(timestamp int64) sptypes.GlobalSpStorePrice {
		resp, err := spClient.QueryGlobalSpStorePriceByTime(ctx, &sptypes.QueryGlobalSpStorePriceByTimeRequest{Timestamp: timestamp})
		require.NoError(t, err)
		return resp.GlobalSpStorePrice
	}
	// medians recomputes what the EndBlocker should have aggregated from the
	// serving SPs' current quotes.
	medians := func() (store, read sdkmath.LegacyDec) {
		spsResp, err := spClient.StorageProviders(ctx, &sptypes.QueryStorageProvidersRequest{
			Pagination: &query.PageRequest{Limit: math.MaxUint64},
		})
		require.NoError(t, err)
		var storePrices, readPrices []sdkmath.LegacyDec
		for _, sp := range spsResp.Sps {
			if sp.Status != sptypes.STATUS_IN_SERVICE && sp.Status != sptypes.STATUS_IN_MAINTENANCE {
				continue
			}
			q := quote(sp.OperatorAddress)
			storePrices = append(storePrices, q.StorePrice)
			readPrices = append(readPrices, q.ReadPrice)
		}
		return median(storePrices), median(readPrices)
	}
	// aggregateAfter waits for a global price stamped after the given quote
	// time; the budget is generous next to localup's 1s interval.
	aggregateAfter := func(quoteTime int64) sptypes.GlobalSpStorePrice {
		const budget = 30 * time.Second
		deadline := time.Now().Add(budget)
		for {
			agg := globalPrice(0)
			if agg.UpdateTimeSec > quoteTime {
				return agg
			}
			require.False(t, time.Now().After(deadline), "no global price aggregated within %s of the quote at %d", budget, quoteTime)
			time.Sleep(500 * time.Millisecond)
		}
	}

	globalBefore := globalPrice(0)
	storeMedianBefore, readMedianBefore := medians()
	require.True(t, globalBefore.PrimaryStorePrice.Equal(storeMedianBefore), "pre-test aggregate %s should already match the quotes' median %s", globalBefore.PrimaryStorePrice, storeMedianBefore)
	require.True(t, globalBefore.ReadPrice.Equal(readMedianBefore), "pre-test read aggregate %s should already match the quotes' median %s", globalBefore.ReadPrice, readMedianBefore)

	var lastQuoteTime int64
	for i := 0; i < priceUpdaters; i++ {
		name := fmt.Sprintf("sp%d", i)
		entry, ok := spExport[name]
		require.True(t, ok, "sp_export.json missing %s -- run localup.sh export_sps 1 7 first", name)
		operatorKey := mustHexKey(t, entry.OperatorPrivateKey)
		fundMoca(ctx, t, client, chainID, crypto.PubkeyToAddress(operatorKey.PublicKey), fundingAmountMOCA)

		before := quote(entry.OperatorAddress)
		// LegacyDec.BigInt() returns the value's raw 18-decimal-precision
		// internal representation, which is exactly what updateSPPrice expects
		// (it reconstructs via LegacyNewDecFromBigIntWithPrec(x, 18)) -- so
		// doubling that raw integer doubles the reconstructed Dec price.
		newReadPrice := new(big.Int).Mul(before.ReadPrice.BigInt(), big.NewInt(2))
		newStorePrice := new(big.Int).Mul(before.StorePrice.BigInt(), big.NewInt(2))
		newFreeReadQuota := before.FreeReadQuota + 1024
		packedArgs, err := method.Inputs.Pack(newReadPrice, newFreeReadQuota, newStorePrice)
		require.NoError(t, err)
		sendPrecompileTx(ctx, t, client, chainID, operatorKey, precompile.Address(), append(append([]byte{}, method.ID...), packedArgs...))

		after := quote(entry.OperatorAddress)
		require.Equal(t, newReadPrice, after.ReadPrice.BigInt())
		require.Equal(t, newStorePrice, after.StorePrice.BigInt())
		require.Equal(t, newFreeReadQuota, after.FreeReadQuota)
		require.Greater(t, after.UpdateTimeSec, before.UpdateTimeSec)
		lastQuoteTime = after.UpdateTimeSec

		if i == priceUpdaters-2 {
			// A minority of changed quotes must leave the medians where they
			// were, and the fresh aggregate must still agree with the quotes.
			agg := aggregateAfter(after.UpdateTimeSec)
			store, read := medians()
			require.True(t, store.Equal(storeMedianBefore), "store median moved to %s on a minority of changed quotes", store)
			require.True(t, read.Equal(readMedianBefore), "read median moved to %s on a minority of changed quotes", read)
			require.True(t, agg.PrimaryStorePrice.Equal(store), "global store price %s != quotes' median %s", agg.PrimaryStorePrice, store)
			require.True(t, agg.ReadPrice.Equal(read), "global read price %s != quotes' median %s", agg.ReadPrice, read)
		}
	}

	agg := aggregateAfter(lastQuoteTime)
	store, read := medians()
	require.True(t, store.Equal(storeMedianBefore.MulInt64(2)), "a majority of doubled quotes must double the store median, got %s", store)
	require.True(t, read.Equal(readMedianBefore.MulInt64(2)), "a majority of doubled quotes must double the read median, got %s", read)
	require.True(t, agg.PrimaryStorePrice.Equal(store), "global store price %s != quotes' median %s", agg.PrimaryStorePrice, store)
	require.True(t, agg.ReadPrice.Equal(read), "global read price %s != quotes' median %s", agg.ReadPrice, read)
	expectedSecondary := params.SecondarySpStorePriceRatio.Mul(store)
	require.True(t, agg.SecondaryStorePrice.Equal(expectedSecondary), "global secondary store price %s != ratio x median %s", agg.SecondaryStorePrice, expectedSecondary)

	// The previous aggregate is kept and still answers by-time lookups.
	old := globalPrice(globalBefore.UpdateTimeSec + 1)
	require.Equal(t, globalBefore.UpdateTimeSec, old.UpdateTimeSec)
	require.True(t, old.PrimaryStorePrice.Equal(globalBefore.PrimaryStorePrice), "by-time lookup returned %s for the pre-change aggregate %s", old.PrimaryStorePrice, globalBefore.PrimaryStorePrice)
	require.True(t, old.ReadPrice.Equal(globalBefore.ReadPrice))
}

// median mirrors x/sp's calculateMedian: the middle quote, or for an even
// count lo+(hi-lo)/2.
func median(prices []sdkmath.LegacyDec) sdkmath.LegacyDec {
	sorted := append([]sdkmath.LegacyDec(nil), prices...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LT(sorted[j]) })
	l := len(sorted)
	if l%2 == 0 {
		lo, hi := sorted[l/2-1], sorted[l/2]
		return lo.Add(hi.Sub(lo).QuoInt64(2))
	}
	return sorted[l/2]
}
