package upgrades_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegranttestutil "cosmossdk.io/x/feegrant/testutil"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/mocachain/moca/v2/app/upgrades"
)

// TestCleanupFeegrantQueueOrphans injects orphaned expiration-queue entries
// (simulating the pre-fix revoke bug) alongside one legitimate entry, then
// verifies the upgrade migration removes exactly the orphans, keeps the live
// entry, leaves the live allowance intact, and is idempotent.
func TestCleanupFeegrantQueueOrphans(t *testing.T) {
	addrs := simtestutil.CreateIncrementalAccounts(3)
	granter, grantee, other := addrs[0], addrs[1], addrs[2]

	storeKey := storetypes.NewKVStoreKey(feegrant.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))

	registry := codectypes.NewInterfaceRegistry()
	feegrant.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	ctrl := gomock.NewController(t)
	ak := feegranttestutil.NewMockAccountKeeper(ctrl)
	ak.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(authtypes.NewBaseAccountWithAddress(grantee)).AnyTimes()
	bk := feegranttestutil.NewMockBankKeeper(ctrl)
	bk.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()

	ss := runtime.NewKVStoreService(storeKey)
	k := feegrantkeeper.NewKeeper(cdc, ss, ak).SetBankKeeper(bk)
	ctx := testCtx.Ctx
	store := ctx.KVStore(storeKey)

	coins := sdk.NewCoins(sdk.NewInt64Coin("atom", 100))
	liveExp := ctx.BlockTime().AddDate(1, 0, 0)

	// 1. a legitimate live expiring grant -> writes one valid queue entry
	require.NoError(t, k.GrantAllowance(ctx, granter, grantee,
		&feegrant.BasicAllowance{SpendLimit: coins, Expiration: &liveExp}))
	validKey := feegrant.FeeAllowancePrefixQueue(&liveExp, feegrant.FeeAllowanceKey(granter, grantee)[1:])

	// 2. inject orphans simulating the pre-fix state:
	//    (a) distinct-expiry orphans for the SAME live pair (the flood vector)
	for i := 1; i <= 5; i++ {
		ei := liveExp.Add(time.Duration(i) * time.Hour)
		store.Set(feegrant.FeeAllowancePrefixQueue(&ei, feegrant.FeeAllowanceKey(granter, grantee)[1:]), []byte{})
	}
	//    (b) an orphan for a pair with NO live allowance (revoked, not re-granted)
	otherExp := liveExp.Add(48 * time.Hour)
	store.Set(feegrant.FeeAllowancePrefixQueue(&otherExp, feegrant.FeeAllowanceKey(granter, other)[1:]), []byte{})

	countQueue := func() int {
		it := storetypes.KVStorePrefixIterator(store, feegrant.FeeAllowanceQueueKeyPrefix)
		defer it.Close()
		n := 0
		for ; it.Valid(); it.Next() {
			n++
		}
		return n
	}
	require.Equal(t, 7, countQueue(), "setup: 1 valid + 6 orphans")

	// run the migration
	removed, err := upgrades.CleanupFeegrantQueueOrphans(ctx, k, storeKey)
	require.NoError(t, err)
	require.Equal(t, 6, removed, "must remove exactly the 6 orphans")
	require.Equal(t, 1, countQueue(), "only the valid entry remains")

	// the surviving entry is the valid one
	it := storetypes.KVStorePrefixIterator(store, feegrant.FeeAllowanceQueueKeyPrefix)
	require.True(t, it.Valid())
	require.Equal(t, validKey, it.Key())
	require.NoError(t, it.Close())

	// the live allowance is untouched
	_, err = k.GetAllowance(ctx, granter, grantee)
	require.NoError(t, err, "live allowance must survive the cleanup")

	// idempotent: a second run removes nothing
	removed2, err := upgrades.CleanupFeegrantQueueOrphans(ctx, k, storeKey)
	require.NoError(t, err)
	require.Equal(t, 0, removed2)
}
