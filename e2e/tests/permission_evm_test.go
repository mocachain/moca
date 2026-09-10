package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestPermissionStaleGroupDenialEvmFlow drives x/permission's group-principal
// grant through the storage precompile's createGroup, updateGroup, and
// putPolicy and deleteGroup methods, the full flow of the retired suite's
// TestVerifyStaleGroupPermission (flagged in the coverage audit as the most
// subtle test in that file): a grant held through a group must stop applying
// once the group itself is deleted.
func TestPermissionStaleGroupDenialEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	storagePrecompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(ctx, t, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(ctx, t, client, chainID, ownerAddr, fundingAmountMOCA)

	memberKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	memberAddr := crypto.PubkeyToAddress(memberKey.PublicKey)
	fundMoca(ctx, t, client, chainID, memberAddr, fundingAmountMOCA)

	bucketName := createTestBucket(ctx, t, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	// 1) Baseline: the member has no grant at all, so deleting the bucket
	// must already be denied.
	verify := func(operator common.Address) permtypes.Effect {
		resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
			Operator:   operator.String(),
			BucketName: bucketName,
			ActionType: permtypes.ACTION_DELETE_BUCKET,
		})
		require.NoError(t, err)
		return resp.Effect
	}
	require.Equal(t, permtypes.EFFECT_DENY, verify(memberAddr), "member should not have delete rights before any grant")

	// 2) Owner creates a group and adds the member.
	groupName := "evm-test-group"
	createGroupMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createGroupArgs, err := createGroupMethod.Inputs.Pack(groupName, "")
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, createGroupMethod.ID...), createGroupArgs...))

	updateGroupMethod := storage.GetAbiMethod(storage.UpdateGroupMethodName)
	updateGroupArgs, err := updateGroupMethod.Inputs.Pack(
		ownerAddr,
		groupName,
		[]common.Address{memberAddr},
		[]int64{0}, // 0 -> no expiration
		[]common.Address{},
	)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, updateGroupMethod.ID...), updateGroupArgs...))

	headGroupResp, err := storageClient.HeadGroup(ctx, &storagetypes.QueryHeadGroupRequest{
		GroupOwner: ownerAddr.String(),
		GroupName:  groupName,
	})
	require.NoError(t, err)
	groupID := headGroupResp.GroupInfo.Id

	// 3) Grant the group ACTION_DELETE_BUCKET on the bucket.
	putPolicyMethod := storage.GetAbiMethod(storage.PutPolicyMethodName)
	putPolicyArgs, err := putPolicyMethod.Inputs.Pack(
		storage.Principal{
			PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_GROUP),
			Value:         groupID.String(),
		},
		mocatypes.NewBucketGRN(bucketName).String(),
		[]storage.Statement{{
			Effect:         int32(permtypes.EFFECT_ALLOW),
			Actions:        []int32{int32(permtypes.ACTION_DELETE_BUCKET)},
			Resources:      nil,
			ExpirationTime: 0,
			LimitSize:      0,
		}},
		int64(0),
	)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, putPolicyMethod.ID...), putPolicyArgs...))

	require.Equal(t, permtypes.EFFECT_ALLOW, verify(memberAddr), "member should have delete rights once the group is granted")

	// 4) Owner deletes the group. This is the path that faulted before #408
	// (the delete-side policy sweep lived in a transient store the precompile
	// multi-store doesn't carry), so it doubles as that fix's regression test.
	deleteGroupMethod := storage.GetAbiMethod(storage.DeleteGroupMethodName)
	deleteGroupArgs, err := deleteGroupMethod.Inputs.Pack(groupName)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, deleteGroupMethod.ID...), deleteGroupArgs...))

	_, err = storageClient.HeadGroup(ctx, &storagetypes.QueryHeadGroupRequest{
		GroupOwner: ownerAddr.String(),
		GroupName:  groupName,
	})
	require.Error(t, err, "group should be gone after deleteGroup")

	// 5) The stale group grant no longer applies: the member is denied again.
	require.Equal(t, permtypes.EFFECT_DENY, verify(memberAddr), "member must lose delete rights once the granting group is deleted")
}
