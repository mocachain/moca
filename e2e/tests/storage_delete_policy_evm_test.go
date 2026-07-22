package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageDeletePolicyEvmFlow drives x/storage's policy revocation
// through the storage precompile's deletePolicy method: a grant made via
// putPolicy can be fully revoked, returning the grantee to the default-deny
// baseline, exercising the same functional path as the retired suite's
// policy-revocation scenarios.
func TestStorageDeletePolicyEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	granteeKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	granteeAddr := crypto.PubkeyToAddress(granteeKey.PublicKey)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)
	resource := mocatypes.NewBucketGRN(bucketName).String()
	precompile := storage.Precompile{}

	verify := func() permtypes.Effect {
		resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
			Operator:   granteeAddr.String(),
			BucketName: bucketName,
			ActionType: permtypes.ACTION_DELETE_BUCKET,
		})
		require.NoError(t, err)
		return resp.Effect
	}
	require.Equal(t, permtypes.EFFECT_DENY, verify(), "baseline: no grant yet")

	principal := storage.Principal{PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT), Value: granteeAddr.String()}

	putPolicyMethod := storage.GetAbiMethod(storage.PutPolicyMethodName)
	putPolicyArgs, err := putPolicyMethod.Inputs.Pack(
		principal, resource,
		[]storage.Statement{{Effect: int32(permtypes.EFFECT_ALLOW), Actions: []int32{int32(permtypes.ACTION_DELETE_BUCKET)}, Resources: nil, ExpirationTime: 0, LimitSize: 0}},
		int64(0),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, putPolicyMethod.ID...), putPolicyArgs...))
	require.Equal(t, permtypes.EFFECT_ALLOW, verify(), "grant should take effect")

	deletePolicyMethod := storage.GetAbiMethod(storage.DeletePolicyMethodName)
	deletePolicyArgs, err := deletePolicyMethod.Inputs.Pack(principal, resource)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, deletePolicyMethod.ID...), deletePolicyArgs...))
	require.Equal(t, permtypes.EFFECT_DENY, verify(), "deletePolicy must fully revoke the grant")
}
