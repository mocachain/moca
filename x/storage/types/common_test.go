package types_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestNewSecondarySpSealObjectSignDoc(t *testing.T) {
	objectID := math.NewUint(7)
	doc := types.NewSecondarySpSealObjectSignDoc("moca_1-1", 3, objectID, []byte("checksum"))
	require.Equal(t, "moca_1-1", doc.ChainId)
	require.Equal(t, uint32(3), doc.GlobalVirtualGroupId)
	require.True(t, objectID.Equal(doc.ObjectId))
	require.Equal(t, []byte("checksum"), doc.Checksum)
}

func TestSecondarySpSealObjectSignDoc_SignBytesAndHash(t *testing.T) {
	doc := types.NewSecondarySpSealObjectSignDoc("moca_1-1", 3, math.NewUint(7), []byte("checksum"))

	bz1 := doc.GetSignBytes()
	bz2 := doc.GetSignBytes()
	require.NotEmpty(t, bz1)
	require.Equal(t, bz1, bz2, "sign bytes must be deterministic across calls")
	require.Contains(t, string(bz1), "moca_1-1", "the chain id must round-trip into the signed payload")

	require.Equal(t, [32]byte(crypto.Keccak256Hash(bz1)), doc.GetBlsSignHash())

	other := types.NewSecondarySpSealObjectSignDoc("moca_1-1", 3, math.NewUint(7), []byte("different-checksum"))
	require.NotEqual(t, doc.GetBlsSignHash(), other.GetBlsSignHash(),
		"the signed hash must change when the signed content changes")
}

func TestNewSecondarySpMigrationBucketSignDoc(t *testing.T) {
	bucketID := math.NewUint(42)
	doc := types.NewSecondarySpMigrationBucketSignDoc("moca_1-1", bucketID, 1, 2, 3)
	require.Equal(t, "moca_1-1", doc.ChainId)
	require.True(t, bucketID.Equal(doc.BucketId))
	require.Equal(t, uint32(1), doc.DstPrimarySpId)
	require.Equal(t, uint32(2), doc.SrcGlobalVirtualGroupId)
	require.Equal(t, uint32(3), doc.DstGlobalVirtualGroupId)
}

func TestSecondarySpMigrationBucketSignDoc_SignBytesAndHash(t *testing.T) {
	doc := types.NewSecondarySpMigrationBucketSignDoc("moca_1-1", math.NewUint(42), 1, 2, 3)

	bz1 := doc.GetSignBytes()
	bz2 := doc.GetSignBytes()
	require.NotEmpty(t, bz1)
	require.Equal(t, bz1, bz2, "sign bytes must be deterministic across calls")

	require.Equal(t, [32]byte(crypto.Keccak256Hash(bz1)), doc.GetBlsSignHash())

	other := types.NewSecondarySpMigrationBucketSignDoc("moca_1-1", math.NewUint(42), 1, 2, 4)
	require.NotEqual(t, doc.GetBlsSignHash(), other.GetBlsSignHash(),
		"the signed hash must change when the destination gvg changes")
}

func TestGenerateHash(t *testing.T) {
	a, b := []byte("checksum-a"), []byte("checksum-b")

	want := sha256.Sum256(bytes.Join([][]byte{a, b}, []byte("")))
	require.Equal(t, want[:], types.GenerateHash([][]byte{a, b}))

	require.NotEqual(t, types.GenerateHash([][]byte{a, b}), types.GenerateHash([][]byte{b, a}),
		"the hash must depend on checksum order")

	emptyWant := sha256.Sum256(nil)
	require.Equal(t, emptyWant[:], types.GenerateHash(nil))
}
