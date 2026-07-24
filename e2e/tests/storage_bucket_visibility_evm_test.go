package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageBucketVisibilityEvmFlow drives x/storage's public-read
// visibility shortcut in VerifyBucketPermission, exercising the same
// functional path as the retired suite's bucket-visibility matrix tests: a
// PUBLIC_READ bucket grants a random, ungranted third party read-only
// actions (e.g. listing objects) but not admin actions, while a PRIVATE
// bucket denies that same third party everything. This is pure query-side
// behavior -- no precompile call beyond the two bucket creations is needed.
func TestStorageBucketVisibilityEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(ownerKey.PublicKey), fundingAmountMOCA)

	strangerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	strangerAddr := crypto.PubkeyToAddress(strangerKey.PublicKey)

	publicBucket := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 0)
	privateBucket := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	verify := func(bucketName string, action permtypes.ActionType) permtypes.Effect {
		resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
			Operator:   strangerAddr.String(),
			BucketName: bucketName,
			ActionType: action,
		})
		require.NoError(t, err)
		return resp.Effect
	}

	require.Equal(t, permtypes.EFFECT_ALLOW, verify(publicBucket, permtypes.ACTION_LIST_OBJECT),
		"a public-read bucket must allow an ungranted third party to list its objects")
	require.Equal(t, permtypes.EFFECT_DENY, verify(publicBucket, permtypes.ACTION_UPDATE_BUCKET_INFO),
		"public-read only covers read-only actions, not admin actions")
	require.Equal(t, permtypes.EFFECT_DENY, verify(privateBucket, permtypes.ACTION_LIST_OBJECT),
		"a private bucket must deny an ungranted third party even read-only actions")
}
