package v2_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	v2 "github.com/mocachain/moca/v2/x/storage/keeper/v2"
	"github.com/mocachain/moca/v2/x/storage/types"
)

// TestMigrateStore seeds the pre-migration (buggy) layout -- a
// BucketFlowRateLimitStatus entry written under the shared
// BucketRateLimitPrefix, the same prefix a genuine BucketFlowRateLimit entry
// uses -- and asserts the migration relocates only the status entry to the
// dedicated BucketRateLimitStatusPrefix, leaving the rate-limit entry alone.
func TestMigrateStore(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx
	store := ctx.KVStore(storeKey)

	const bucketName = "mybucket"
	bucketNameHash := crypto.Keccak256([]byte(bucketName))

	// Pre-migration (buggy) status key: BucketRateLimitPrefix + hash, exactly
	// what setBucketFlowRateLimitStatus wrote before this fix -- and what
	// live networks have on disk today.
	oldStatusKey := append(append([]byte{}, types.BucketRateLimitPrefix...), bucketNameHash...)
	status := &types.BucketFlowRateLimitStatus{
		IsBucketLimited: true,
		PaymentAddress:  sample.RandAccAddress().String(),
	}
	store.Set(oldStatusKey, cdc.MustMarshal(status))

	// A genuine rate-limit entry sharing the same prefix; it must survive
	// untouched since its key shape (73 bytes) differs from the status
	// shape (33 bytes).
	paymentAccount := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	rateLimitKey := types.GetBucketFlowRateLimitKey(paymentAccount, bucketOwner, bucketName)
	rateLimit := &types.BucketFlowRateLimit{FlowRateLimit: sdkmath.NewInt(100)}
	store.Set(rateLimitKey, cdc.MustMarshal(rateLimit))

	require.NoError(t, v2.MigrateStore(ctx, storeKey))

	// The status entry now lives under the dedicated prefix and is readable
	// via the (now-fixed) key builder.
	newStatusKey := types.GetBucketFlowRateLimitStatusKey(bucketName)
	bz := store.Get(newStatusKey)
	require.NotNil(t, bz, "status entry must be readable at the dedicated status prefix")
	var gotStatus types.BucketFlowRateLimitStatus
	cdc.MustUnmarshal(bz, &gotStatus)
	require.Equal(t, *status, gotStatus)

	// The old (buggy) key is gone.
	require.Nil(t, store.Get(oldStatusKey), "the old key under BucketRateLimitPrefix must be deleted")

	// The rate-limit entry is untouched.
	bz = store.Get(rateLimitKey)
	require.NotNil(t, bz, "the unrelated rate-limit entry must survive the migration")
	var gotRateLimit types.BucketFlowRateLimit
	cdc.MustUnmarshal(bz, &gotRateLimit)
	require.True(t, rateLimit.FlowRateLimit.Equal(gotRateLimit.FlowRateLimit))
}
