package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageDelegateCreateObjectEvmFlow drives x/storage's delegated object
// creation through the storage precompile's delegateCreateObject method,
// exercising the same functional path as the retired suite's delegated-agent
// scenarios: a bucket's primary SP may create an object "as" the bucket
// owner without any per-call approval signature (delegation defaults to
// allowed until the owner opts out), while any other account -- even one
// with its own valid SP registration -- is rejected.
func TestStorageDelegateCreateObjectEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}
	precompileAddr := precompile.Address()

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	objectName := storageutils.GenRandomObjectName()
	_, b64Checksums := threeChecksums()
	delegateMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	delegateArgs, err := delegateMethod.Inputs.Pack(
		ownerAddr.String(),
		bucketName,
		objectName,
		uint64(0), // empty object: sealed immediately, no SP-side upload needed
		"application/octet-stream",
		uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums,
		uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	calldata := append(append([]byte{}, delegateMethod.ID...), delegateArgs...)

	// A random funded stranger (not the bucket's primary SP) must be denied.
	strangerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	strangerAddr := crypto.PubkeyToAddress(strangerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, strangerAddr, fundingAmountMOCA)
	_, callErr := client.CallContract(ctx, ethereum.CallMsg{
		From: strangerAddr, To: &precompileAddr, Data: calldata,
	}, nil)
	require.Error(t, callErr, "only the bucket's primary SP may delegate-create an object")
	require.Contains(t, callErr.Error(), "only the primary SP is allowed")

	// The bucket's primary SP creates the object on the owner's behalf.
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompileAddr, calldata)

	headResp, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.OBJECT_STATUS_SEALED, headResp.ObjectInfo.ObjectStatus, "an empty-payload object is sealed immediately, no upload round-trip needed")
	// ObjectInfo.Creator is left blank when the creator is the bucket owner
	// themself -- it's only populated for a genuinely different delegator.
	require.Empty(t, headResp.ObjectInfo.Creator)
	require.Equal(t, ownerAddr.String(), headResp.ObjectInfo.Owner)
}
