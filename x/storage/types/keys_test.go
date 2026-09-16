package types_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/internal/sequence"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/storage/types"
)

// concatBytes recomputes an expected key from its parts using a fresh backing
// array, so it never aliases the package-level prefix vars under test.
func concatBytes(parts ...[]byte) []byte {
	out := make([]byte, 0)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// encodeUintSeq mirrors the math.Uint encoding keys.go itself delegates to,
// so tests recompute expectations from the same primitive the production
// code uses rather than re-implementing it.
func encodeUintSeq(id math.Uint) []byte {
	var seq sequence.Sequence[math.Uint]
	return seq.EncodeSequence(id)
}

func bigEndian8(v int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(v))
	return bz
}

func TestGetBucketKey(t *testing.T) {
	got := types.GetBucketKey("mybucket")
	want := concatBytes(types.BucketInfoPrefix, crypto.Keccak256([]byte("mybucket")))
	require.Equal(t, want, got)
	require.NotEqual(t, got, types.GetBucketKey("other-bucket"), "different bucket names must not collide")
}

func TestGetObjectKey_And_OnlyBucketPrefix(t *testing.T) {
	full := types.GetObjectKey("mybucket", "myobject")
	want := concatBytes(types.ObjectInfoPrefix, crypto.Keccak256([]byte("mybucket")), crypto.Keccak256([]byte("myobject")))
	require.Equal(t, want, full)

	bucketPrefix := types.GetObjectKeyOnlyBucketPrefix("mybucket")
	require.Equal(t, concatBytes(types.ObjectInfoPrefix, crypto.Keccak256([]byte("mybucket"))), bucketPrefix)
	require.True(t, bytes.HasPrefix(full, bucketPrefix),
		"an object key must be enumerable under its bucket's prefix")
	require.False(t, bytes.HasPrefix(full, types.GetObjectKeyOnlyBucketPrefix("other-bucket")),
		"a different bucket's prefix must not match")
}

func TestGetShadowObjectKey(t *testing.T) {
	got := types.GetShadowObjectKey("mybucket", "myobject")
	want := concatBytes(types.ShadowObjectInfoPrefix, crypto.Keccak256([]byte("mybucket")), crypto.Keccak256([]byte("myobject")))
	require.Equal(t, want, got)
	require.NotEqual(t, got, types.GetObjectKey("mybucket", "myobject"),
		"shadow and regular object entries must live under different prefixes")
}

func TestGetGroupKey_And_OnlyOwnerPrefix(t *testing.T) {
	owner := sample.RandAccAddress()
	full := types.GetGroupKey(owner, "mygroup")
	want := concatBytes(types.GroupInfoPrefix, owner.Bytes(), crypto.Keccak256([]byte("mygroup")))
	require.Equal(t, want, full)

	ownerPrefix := types.GetGroupKeyOnlyOwnerPrefix(owner)
	require.Equal(t, concatBytes(types.GroupInfoPrefix, owner.Bytes()), ownerPrefix)
	require.True(t, bytes.HasPrefix(full, ownerPrefix),
		"a group key must be enumerable under its owner's prefix")

	other := sample.RandAccAddress()
	require.False(t, bytes.HasPrefix(full, types.GetGroupKeyOnlyOwnerPrefix(other)),
		"a different owner's prefix must not match")
}

// TestGetSequenceIDKeys covers the family of key builders that append a
// math.Uint sequence encoding to a fixed prefix: same shape, distinct prefix
// bytes per resource. It does not assert cross-magnitude ordering, since
// sequence.Sequence[math.Uint].EncodeSequence is a minimal (unpadded)
// big-endian encoding, not fixed width -- see the PR description.
func TestGetSequenceIDKeys(t *testing.T) {
	id1 := math.NewUint(1)
	id2 := math.NewUint(2)

	tests := []struct {
		name   string
		prefix []byte
		fn     func(math.Uint) []byte
	}{
		{"bucket by id", types.BucketByIDPrefix, types.GetBucketByIDKey},
		{"object by id", types.ObjectByIDPrefix, types.GetObjectByIDKey},
		{"group by id", types.GroupByIDPrefix, types.GetGroupByIDKey},
		{"discontinue object status", types.DiscontinueObjectStatusPrefix, types.GetDiscontinueObjectStatusKey},
		{"migration bucket", types.MigrateBucketPrefix, types.GetMigrationBucketKey},
		{"quota", types.QuotaPrefix, types.GetQuotaKey},
		{"internal bucket info", types.InternalBucketInfoPrefix, types.GetInternalBucketInfoKey},
		{"locked object count", types.LockedObjectCountPrefix, types.GetLockedObjectCountKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1 := tt.fn(id1)
			require.Equal(t, concatBytes(tt.prefix, encodeUintSeq(id1)), got1)

			got2 := tt.fn(id2)
			require.NotEqual(t, got1, got2, "different ids must not collide")
		})
	}
}

// TestGetTimestampKeys covers the family of key builders that append a fixed
// 8-byte big-endian encoding of an int64 to a prefix. Unlike the sequence
// encoding above, this IS fixed width, so ascending timestamps/heights must
// sort into ascending keys -- callers rely on that for range iteration.
func TestGetTimestampKeys(t *testing.T) {
	tests := []struct {
		name   string
		prefix []byte
		fn     func(int64) []byte
	}{
		{"discontinue object ids", types.DiscontinueObjectIDsPrefix, types.GetDiscontinueObjectIdsKey},
		{"discontinue bucket ids", types.DiscontinueBucketIDsPrefix, types.GetDiscontinueBucketIDsKey},
		{"params with timestamp", types.ParamsKey, types.GetParamsKeyWithTimestamp},
		{"delete stale policies", types.DeleteStalePoliciesPrefix, types.GetDeleteStalePoliciesKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			early := tt.fn(1000)
			late := tt.fn(2000)
			require.Equal(t, concatBytes(tt.prefix, bigEndian8(1000)), early)
			require.Equal(t, concatBytes(tt.prefix, bigEndian8(2000)), late)
			require.Less(t, bytes.Compare(early, late), 0,
				"a later timestamp/height must sort after an earlier one")
		})
	}
}

func TestGetBucketFlowRateLimitKey(t *testing.T) {
	paymentAccount := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	got := types.GetBucketFlowRateLimitKey(paymentAccount, bucketOwner, "mybucket")
	want := concatBytes(types.BucketRateLimitPrefix, paymentAccount.Bytes(), bucketOwner.Bytes(), crypto.Keccak256([]byte("mybucket")))
	require.Equal(t, want, got)

	other := sample.RandAccAddress()
	require.NotEqual(t, got, types.GetBucketFlowRateLimitKey(other, bucketOwner, "mybucket"),
		"different payment accounts must not collide")
	require.NotEqual(t, got, types.GetBucketFlowRateLimitKey(paymentAccount, other, "mybucket"),
		"different bucket owners must not collide")
}

func TestGetBucketCountByOwnerKey(t *testing.T) {
	owner := sample.RandAccAddress()
	got := types.GetBucketCountByOwnerKey(owner)
	require.Equal(t, concatBytes(types.BucketCountByOwnerPrefix, owner.Bytes()), got)

	other := sample.RandAccAddress()
	require.NotEqual(t, got, types.GetBucketCountByOwnerKey(other), "different owners must not collide")
}

// NOTE: GetBucketFlowRateLimitStatusKey is deliberately not covered here --
// see "Findings" in the PR description. It builds its key from
// BucketRateLimitPrefix (the same prefix GetBucketFlowRateLimitKey uses)
// instead of the dedicated BucketRateLimitStatusPrefix the file declares, so
// asserting today's byte layout would enshrine what looks like a copy-paste
// prefix mistake rather than a deliberate contract.
