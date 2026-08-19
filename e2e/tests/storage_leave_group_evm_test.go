package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageLeaveGroupEvmFlow drives x/storage's voluntary group departure
// through the storage precompile's leaveGroup method: a member added via
// updateGroup can remove themself without needing the group owner's
// cooperation or any UpdateGroupMember permission grant, unlike being kicked
// by the owner. This is a plain KV-store member-list removal (no ERC721
// interaction), unlike deleteGroup's ERC721 burn.
func TestStorageLeaveGroupEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	memberKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	memberAddr := crypto.PubkeyToAddress(memberKey.PublicKey)
	fundMoca(t, ctx, client, chainID, memberAddr, fundingAmountMOCA)

	groupName := "evm-leave-group-test"
	createGroupMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createGroupArgs, err := createGroupMethod.Inputs.Pack(groupName, "")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, createGroupMethod.ID...), createGroupArgs...))

	updateGroupMethod := storage.GetAbiMethod(storage.UpdateGroupMethodName)
	updateGroupArgs, err := updateGroupMethod.Inputs.Pack(
		ownerAddr, groupName, []common.Address{memberAddr}, []int64{0}, []common.Address{},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, updateGroupMethod.ID...), updateGroupArgs...))

	_, err = storageClient.HeadGroupMember(ctx, &storagetypes.QueryHeadGroupMemberRequest{
		Member: memberAddr.String(), GroupOwner: ownerAddr.String(), GroupName: groupName,
	})
	require.NoError(t, err, "member should be present right after being added")

	leaveGroupMethod := storage.GetAbiMethod(storage.LeaveGroupMethodName)
	leaveGroupArgs, err := leaveGroupMethod.Inputs.Pack(ownerAddr, groupName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, memberKey, precompile.Address(),
		append(append([]byte{}, leaveGroupMethod.ID...), leaveGroupArgs...))

	_, err = storageClient.HeadGroupMember(ctx, &storagetypes.QueryHeadGroupMemberRequest{
		Member: memberAddr.String(), GroupOwner: ownerAddr.String(), GroupName: groupName,
	})
	require.Error(t, err, "member should be gone after leaving")
	require.Contains(t, err.Error(), "No such group member")
}
