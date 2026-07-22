package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageEvmFlow drives moca's storage/virtualgroup modules entirely
// through their EVM precompiles (createGlobalVirtualGroup, createBucket,
// updateBucketInfo), exercising the same functional path as the retired
// suite's eip712_test.go (create a bucket, then flip its visibility) but
// over real signed EVM transactions against a live node.
func TestStorageEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(t, ctx, client, chainID, userAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, userKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 0)

	// updateBucketInfo via the storage precompile: flip to private.
	storagePrecompile := storage.Precompile{}
	updateMethod := storage.GetAbiMethod(storage.UpdateBucketInfoMethodName)
	updateArgs, err := updateMethod.Inputs.Pack(
		bucketName,
		uint8(storagetypes.VISIBILITY_TYPE_PRIVATE),
		userAddr,
		big.NewInt(-1), // chargedReadQuota: -1 sentinel means "leave unchanged"
	)
	require.NoError(t, err)
	updateCalldata := append(append([]byte{}, updateMethod.ID...), updateArgs...)
	sendPrecompileTx(t, ctx, client, chainID, userKey, storagePrecompile.Address(), updateCalldata)

	// Verify via the same gRPC query the legacy suite used.
	headResp, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.VISIBILITY_TYPE_PRIVATE, headResp.BucketInfo.Visibility)
}
