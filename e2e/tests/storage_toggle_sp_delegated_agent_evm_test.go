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

// TestStorageToggleSPDelegatedAgentEvmFlow drives x/storage's delegated-agent
// opt-out through the storage precompile's toggleSPAsDelegatedAgent method:
// delegateCreateObject defaults to allowed (see
// TestStorageDelegateCreateObjectEvmFlow), but the bucket owner can disable
// it, after which the same primary-SP delegated call that used to succeed is
// rejected.
func TestStorageToggleSPDelegatedAgentEvmFlow(t *testing.T) {
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

	toggleMethod := storage.GetAbiMethod(storage.ToggleSPAsDelegatedAgentMethodName)
	toggleArgs, err := toggleMethod.Inputs.Pack(bucketName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompileAddr,
		append(append([]byte{}, toggleMethod.ID...), toggleArgs...))

	headResp, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.True(t, headResp.BucketInfo.SpAsDelegatedAgentDisabled, "toggling once from the default must disable delegation")

	delegateMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	_, b64Checksums := threeChecksums()
	delegateArgs, err := delegateMethod.Inputs.Pack(
		ownerAddr.String(), bucketName, storageutils.GenRandomObjectName(),
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	_, callErr := client.CallContract(ctx, ethereum.CallMsg{
		From: sp.OperatorAddr, To: &precompileAddr, Data: append(append([]byte{}, delegateMethod.ID...), delegateArgs...),
	}, nil)
	require.Error(t, callErr, "the primary SP's delegated create must now be rejected")
	require.Contains(t, callErr.Error(), "disabled by the bucket owner previously")
}
