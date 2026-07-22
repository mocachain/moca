package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageSetTagEvmFlow drives x/storage's resource-tagging through the
// storage precompile's setTag method, exercising the same functional path as
// the retired suite's TestCreateBucketAndSetTag: the owner tags an existing
// bucket and the tag is persisted onto its BucketInfo. The old test bundled
// creation and tagging as two Cosmos messages in one tx to assert atomicity;
// an EOA-signed EVM tx can only make a single top-level call, so here they're
// two separate precompile txs -- still real coverage of setTag itself, which
// no other test in this package exercises.
func TestStorageSetTagEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(ownerKey.PublicKey), fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	before, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Nil(t, before.BucketInfo.Tags, "freshly created bucket should have no tags yet")

	setTagMethod := storage.GetAbiMethod(storage.SetTagMethodName)
	setTagArgs, err := setTagMethod.Inputs.Pack(
		mocatypes.NewBucketGRN(bucketName).String(),
		[]storage.Tag{{Key: "key1", Value: "value1"}},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, setTagMethod.ID...), setTagArgs...))

	after, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, []storagetypes.ResourceTags_Tag{{Key: "key1", Value: "value1"}}, after.BucketInfo.Tags.Tags)
}
