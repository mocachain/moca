package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageRateLimitEvmFlow drives x/storage's bucket flow-rate limit
// through the storage precompile's setBucketFlowRateLimit method, exercising
// the same functional path as the retired suite's
// TestSetBucketRateLimitToZero: setting a bucket's own rate limit to exactly
// 0 is a distinct, legitimately-set state (not "unlimited"), and object
// creation is then rejected.
func TestStorageRateLimitEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	// Non-zero quota: the bucket needs an actual outgoing flow rate for a
	// limit of 0 to bite -- the keeper only flips the "limited" status when
	// the new limit falls below the bucket's current rate.
	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 10_000_000)

	// Set the bucket's own flow rate limit to exactly 0 (owner acting as its
	// own payment address).
	setLimitMethod := storage.GetAbiMethod(storage.SetBucketFlowRateLimitMethodName)
	setLimitArgs, err := setLimitMethod.Inputs.Pack(bucketName, ownerAddr.String(), ownerAddr.String(), big.NewInt(0))
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, setLimitMethod.ID...), setLimitArgs...))

	// (QueryPaymentAccountBucketFlowRateLimit is the more direct query for
	// this, but its response type currently panics on unmarshal client-side
	// -- a separate, narrow protobuf-reflection issue unrelated to this
	// test. HeadBucket's own ExtraInfo carries the same information and is
	// used elsewhere in this suite without issue.)
	headResp, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.NotNil(t, headResp.ExtraInfo)
	require.True(t, headResp.ExtraInfo.IsRateLimited, "0 must be recorded as a legitimately-set limit, not treated as unset")
	require.True(t, headResp.ExtraInfo.FlowRateLimit.IsZero())

	// Creating an object against the zero-limited bucket must now fail.
	checksums := make([]string, 3) // 1 primary + 2 secondary, matching this genesis's EC scheme
	for i := range checksums {
		sum := sha256.Sum256([]byte{byte(i)})
		checksums[i] = base64.StdEncoding.EncodeToString(sum[:])
	}
	createObjMethod := storage.GetAbiMethod(storage.CreateObjectMethodName)
	createObjArgs, err := createObjMethod.Inputs.Pack(
		bucketName,
		storageutils.GenRandomObjectName(),
		uint64(1024),
		uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		"application/octet-stream",
		storage.Approval{ExpiredHeight: 0, GlobalVirtualGroupFamilyId: 0, Sig: []byte{}},
		checksums,
		uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	createObjCalldata := append(append([]byte{}, createObjMethod.ID...), createObjArgs...)

	precompileAddr := precompile.Address()
	_, callErr := client.CallContract(ctx, ethereum.CallMsg{
		From: ownerAddr, To: &precompileAddr, Data: createObjCalldata,
	}, nil)
	require.Error(t, callErr, "creating an object against a zero-rate-limited bucket must be rejected")
}
