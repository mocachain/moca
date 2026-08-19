package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageDiscontinueBucketEvmFlow drives x/storage's discontinue-bucket
// flow through the storage precompile's discontinueBucket method, exercising
// the same functional path as the retired suite's TestDiscontinueBucket: only
// the bucket's primary SP (identified by its GC address) may discontinue it,
// discontinuing flips BucketStatus to DISCONTINUED immediately (the actual
// deletion is a 7-day-later EndBlocker sweep, out of scope for a live test),
// and a bucket already discontinued can't be discontinued again.
func TestStorageDiscontinueBucketEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}
	precompileAddr := precompile.Address()

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)
	sp0Export, ok := loadSPExport(t)["sp0"]
	require.True(t, ok)
	gcKey := mustHexKey(t, sp0Export.GcPrivateKey)
	gcAddr := crypto.PubkeyToAddress(gcKey.PublicKey)
	fundMoca(t, ctx, client, chainID, gcAddr, fundingAmountMOCA)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	discontinueMethod := storage.GetAbiMethod(storage.DiscontinueBucketMethodName)
	discontinueArgs, err := discontinueMethod.Inputs.Pack(bucketName, "policy violation")
	require.NoError(t, err)
	calldata := append(append([]byte{}, discontinueMethod.ID...), discontinueArgs...)

	// The bucket's owner is not a registered storage provider at all, so it
	// can't discontinue its own bucket.
	_, callErr := client.CallContract(ctx, ethereum.CallMsg{
		From: ownerAddr, To: &precompileAddr, Data: calldata,
	}, nil)
	require.Error(t, callErr, "a non-SP account must not be able to discontinue a bucket")
	require.Contains(t, callErr.Error(), "No such storage provider")

	// SP's GC address discontinues the bucket for real.
	sendPrecompileTx(t, ctx, client, chainID, gcKey, precompileAddr, calldata)

	after, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.BUCKET_STATUS_DISCONTINUED, after.BucketInfo.BucketStatus)

	// Discontinuing an already-discontinued bucket must fail.
	_, callErr = client.CallContract(ctx, ethereum.CallMsg{
		From: gcAddr, To: &precompileAddr, Data: calldata,
	}, nil)
	require.Error(t, callErr, "discontinuing an already-discontinued bucket must be rejected")
	require.Contains(t, callErr.Error(), "invalid bucket status")
}
