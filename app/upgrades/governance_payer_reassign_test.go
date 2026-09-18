package upgrades_test

import (
	"testing"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"

	"github.com/mocachain/moca/v2/app/upgrades"
	"github.com/mocachain/moca/v2/x/payment"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// setupReassignTest pairs a real storage keeper with a real payment keeper on one store,
// mocking only SP price, GVG family and SP lookups.
func setupReassignTest(t *testing.T) (
	*storagekeeper.Keeper, *paymentkeeper.Keeper, sdk.Context,
	*storagetypes.MockVirtualGroupKeeper, *storagetypes.MockSpKeeper,
) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{}, storage.AppModuleBasic{})

	storageKey := storetypes.NewKVStoreKey(storagetypes.StoreKey)
	paymentKey := storetypes.NewKVStoreKey(paymenttypes.StoreKey)
	tKey := storetypes.NewTransientStoreKey("transient_test")

	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	cms.MountStoreWithDB(storageKey, storetypes.StoreTypeIAVL, db)
	cms.MountStoreWithDB(paymentKey, storetypes.StoreTypeIAVL, db)
	cms.MountStoreWithDB(tKey, storetypes.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())

	ctx := sdk.NewContext(cms, cmtproto.Header{ChainID: "moca_test-1", Time: time.Now()}, false, log.NewNopLogger())

	ctrl := gomock.NewController(t)

	payBankKeeper := paymenttypes.NewMockBankKeeper(ctrl)
	payAccountKeeper := paymenttypes.NewMockAccountKeeper(ctrl)
	// An insolvent owner's balance goes negative before it is force-settled;
	// no test address here has a bank account to auto-cover it from.
	payAccountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	pk := paymentkeeper.NewKeeper(encCfg.Codec, paymentKey, payBankKeeper, payAccountKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String())
	require.NoError(t, pk.SetParams(ctx, paymenttypes.DefaultParams()))

	// GetVersionedParamsWithTs reads entries strictly before the queried time,
	// so move block time past the SetParams write.
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Hour))

	accountKeeper := storagetypes.NewMockAccountKeeper(ctrl)
	spKeeper := storagetypes.NewMockSpKeeper(ctrl)
	permKeeper := storagetypes.NewMockPermissionKeeper(ctrl)
	virtualGroupKeeper := storagetypes.NewMockVirtualGroupKeeper(ctrl)
	evmKeeper := storagetypes.NewMockEVMKeeper(ctrl)

	sk := storagekeeper.NewKeeper(
		encCfg.Codec,
		storageKey,
		accountKeeper,
		spKeeper,
		*pk,
		permKeeper,
		virtualGroupKeeper,
		evmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	require.NoError(t, sk.SetParams(ctx, storagetypes.DefaultParams()))

	// Storage's versioned params use the same strictly-before lookup; move past
	// this SetParams too so the lock-fee math can read them.
	ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Hour))

	return sk, pk, ctx, virtualGroupKeeper, spKeeper
}

// TestReassignGovernancePayerBuckets seeds two governance-payer buckets, one self-paying bucket and one
// governance-payer bucket with an unsealed object, then checks the reassignment and cleanup.
func TestReassignGovernancePayerBuckets(t *testing.T) {
	sk, pk, ctx, virtualGroupKeeper, spKeeper := setupReassignTest(t)

	// ReadPrice * quota(1) = 50 with a 1% tax truncating to zero: one recipient flow per bucket;
	// the small PrimaryStorePrice only feeds the locked-object case's lock amount.
	spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(sptypes.GlobalSpStorePrice{
			ReadPrice:           sdkmath.LegacyNewDec(50),
			PrimaryStorePrice:   sdkmath.LegacyNewDecWithPrec(1, 6),
			SecondaryStorePrice: sdkmath.LegacyZeroDec(),
		}, nil).AnyTimes()

	family1 := sample.RandAccAddress()
	family2 := sample.RandAccAddress()
	virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(1)).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: 1, VirtualPaymentAddress: family1.String()}, true).AnyTimes()
	virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(2)).
		Return(&virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 2, VirtualPaymentAddress: family2.String()}, true).AnyTimes()
	// Resolved by CancelCreateObject's primary-SP lookup for bucket4, which
	// shares family 1.
	spKeeper.EXPECT().GetStorageProvider(gomock.Any(), uint32(1)).
		Return(&sptypes.StorageProvider{Id: 1}, true).AnyTimes()

	owner1 := sample.RandAccAddress() // funded: affords the reassigned rate
	owner2 := sample.RandAccAddress() // unfunded: never funded a payment account
	owner3 := sample.RandAccAddress() // already pays through itself
	owner4 := sample.RandAccAddress() // bucket has an unsealed object

	rate := sdkmath.NewInt(50)

	bucket1 := &storagetypes.BucketInfo{
		Owner: owner1.String(), BucketName: "gov-payer-funded-owner", Id: sdkmath.NewUint(1),
		PaymentAddress: paymenttypes.GovernanceAddress.String(), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 1,
	}
	bucket2 := &storagetypes.BucketInfo{
		Owner: owner2.String(), BucketName: "gov-payer-unfunded-owner", Id: sdkmath.NewUint(2),
		PaymentAddress: paymenttypes.GovernanceAddress.String(), GlobalVirtualGroupFamilyId: 2, ChargedReadQuota: 1,
	}
	bucket3 := &storagetypes.BucketInfo{
		Owner: owner3.String(), BucketName: "owner-is-already-payer", Id: sdkmath.NewUint(3),
		PaymentAddress: owner3.String(), GlobalVirtualGroupFamilyId: 3,
	}
	bucket4 := &storagetypes.BucketInfo{
		Owner: owner4.String(), BucketName: "gov-payer-unsealed-object", Id: sdkmath.NewUint(4),
		PaymentAddress: paymenttypes.GovernanceAddress.String(), GlobalVirtualGroupFamilyId: 1,
	}

	for _, b := range []*storagetypes.BucketInfo{bucket1, bucket2, bucket3, bucket4} {
		sk.StoreBucketInfo(ctx, b)
		sk.SetInternalBucketInfo(ctx, b.Id, &storagetypes.InternalBucketInfo{PriceTime: ctx.BlockTime().Unix()})
	}

	// bucket4's still-unsealed object: its fee is locked against the current
	// payer (governance), not charged as an ongoing rate.
	const objectPayloadSize = uint64(100)
	lockedAmount, _, err := sk.GetObjectLockFee(ctx, ctx.BlockTime().Unix(), objectPayloadSize)
	require.NoError(t, err)
	require.True(t, lockedAmount.IsPositive(), "setup sanity: the seeded price must lock a non-zero amount")
	unsealedObject := &storagetypes.ObjectInfo{
		Owner:        owner4.String(),
		BucketName:   bucket4.BucketName,
		ObjectName:   "still-uploading",
		Id:           sdkmath.NewUint(400),
		PayloadSize:  objectPayloadSize,
		CreateAt:     ctx.BlockTime().Unix(),
		ObjectStatus: storagetypes.OBJECT_STATUS_CREATED,
	}
	sk.StoreObjectInfo(ctx, unsealedObject)
	sk.IncreaseLockedObjectCount(ctx, bucket4.Id)

	// Pre-existing state: the governance account carries bucket1's and bucket2's out-flow rates plus
	// bucket4's locked fee, with a static balance far above the forced-settle threshold.
	govRecord := paymenttypes.NewStreamRecord(paymenttypes.GovernanceAddress, ctx.BlockTime().Unix())
	govRecord.NetflowRate = rate.MulRaw(2).Neg()
	govRecord.StaticBalance = sdkmath.NewInt(1_000_000_000_000)
	govRecord.LockBalance = lockedAmount
	govRecord.OutFlowCount = 2
	pk.SetStreamRecord(ctx, govRecord)
	pk.SetOutFlow(ctx, paymenttypes.GovernanceAddress,
		&paymenttypes.OutFlow{ToAddress: family1.String(), Rate: rate, Status: paymenttypes.OUT_FLOW_STATUS_ACTIVE})
	pk.SetOutFlow(ctx, paymenttypes.GovernanceAddress,
		&paymenttypes.OutFlow{ToAddress: family2.String(), Rate: rate, Status: paymenttypes.OUT_FLOW_STATUS_ACTIVE})

	// Each recipient's matching pre-existing inflow, so swapping the payer
	// leaves its own rate unchanged instead of momentarily going negative.
	for _, addr := range []sdk.AccAddress{family1, family2} {
		rec := paymenttypes.NewStreamRecord(addr, ctx.BlockTime().Unix())
		rec.NetflowRate = rate
		pk.SetStreamRecord(ctx, rec)
	}

	// owner1 can comfortably afford the reassigned rate.
	funded := paymenttypes.NewStreamRecord(owner1, ctx.BlockTime().Unix())
	funded.StaticBalance = sdkmath.NewInt(1_000_000_000_000)
	pk.SetStreamRecord(ctx, funded)
	// owner2 has no stream record at all: an owner who never funded one.

	reassigned, canceled, err := upgrades.ReassignGovernancePayerBuckets(ctx, *sk)
	require.NoError(t, err)
	require.Equal(t, 3, reassigned)
	require.Equal(t, 1, canceled)

	got1, found := sk.GetBucketInfoById(ctx, bucket1.Id)
	require.True(t, found)
	require.Equal(t, owner1.String(), got1.PaymentAddress, "bucket1 must now pay through its owner")

	got2, found := sk.GetBucketInfoById(ctx, bucket2.Id)
	require.True(t, found)
	require.Equal(t, owner2.String(), got2.PaymentAddress, "bucket2 must now pay through its owner")

	got3, found := sk.GetBucketInfoById(ctx, bucket3.Id)
	require.True(t, found)
	require.Equal(t, owner3.String(), got3.PaymentAddress, "a bucket that already pays its owner must be untouched")

	got4, found := sk.GetBucketInfoById(ctx, bucket4.Id)
	require.True(t, found)
	require.Equal(t, owner4.String(), got4.PaymentAddress, "bucket4 must now pay through its owner too")
	require.Zero(t, sk.GetLockedObjectCount(ctx, bucket4.Id), "bucket4's unsealed object must be canceled, not left locked")
	_, found = sk.GetObjectInfo(ctx, bucket4.BucketName, unsealedObject.ObjectName)
	require.False(t, found, "the unsealed object must be gone")

	govAfter, found := pk.GetStreamRecord(ctx, paymenttypes.GovernanceAddress)
	require.True(t, found)
	require.True(t, govAfter.NetflowRate.IsZero(), "the governance account must have no out-flow rate left")
	require.True(t, govAfter.LockBalance.IsZero(), "the governance account must have no locked balance left")
	require.Empty(t, pk.GetOutFlows(ctx, paymenttypes.GovernanceAddress))

	owner1After, found := pk.GetStreamRecord(ctx, owner1)
	require.True(t, found)
	require.Equal(t, rate.Neg(), owner1After.NetflowRate)
	require.Equal(t, paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE, owner1After.Status, "a funded owner stays active")

	owner2After, found := pk.GetStreamRecord(ctx, owner2)
	require.True(t, found)
	require.Equal(t, paymenttypes.STREAM_ACCOUNT_STATUS_FROZEN, owner2After.Status,
		"an owner who cannot afford the reassigned rate is force-settled instead of failing the upgrade")
	// Freezing moves the rate out of NetflowRate (active obligations) and into
	// FrozenNetflowRate, the same accounting any other forced settlement uses.
	require.True(t, owner2After.NetflowRate.IsZero())
	require.Equal(t, rate.Neg(), owner2After.FrozenNetflowRate)

	// A second pass has nothing left to reassign or cancel.
	reassigned2, canceled2, err := upgrades.ReassignGovernancePayerBuckets(ctx, *sk)
	require.NoError(t, err)
	require.Equal(t, 0, reassigned2)
	require.Equal(t, 0, canceled2)
}
