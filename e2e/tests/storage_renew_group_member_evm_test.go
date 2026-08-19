package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageRenewGroupMemberEvmFlow drives x/storage's group-member
// expiration renewal through the storage precompile's renewGroupMember
// method: the group owner extends an existing member's expiration without
// removing and re-adding them. Plain KV-store update, no ERC721
// interaction -- doesn't hit the transient-storage bug that blocks
// deleteGroup.
func TestStorageRenewGroupMemberEvmFlow(t *testing.T) {
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

	groupName := "evm-renew-group-test"
	createGroupMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createGroupArgs, err := createGroupMethod.Inputs.Pack(groupName, "")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, createGroupMethod.ID...), createGroupArgs...))

	shortExpiry := time.Now().Add(1 * time.Hour).Unix()
	updateGroupMethod := storage.GetAbiMethod(storage.UpdateGroupMethodName)
	updateGroupArgs, err := updateGroupMethod.Inputs.Pack(
		ownerAddr, groupName, []common.Address{memberAddr}, []int64{shortExpiry}, []common.Address{},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, updateGroupMethod.ID...), updateGroupArgs...))

	before, err := storageClient.HeadGroupMember(ctx, &storagetypes.QueryHeadGroupMemberRequest{
		Member: memberAddr.String(), GroupOwner: ownerAddr.String(), GroupName: groupName,
	})
	require.NoError(t, err)
	require.Equal(t, shortExpiry, before.GroupMember.ExpirationTime.Unix())

	longExpiry := time.Now().Add(48 * time.Hour).Unix()
	renewMethod := storage.GetAbiMethod(storage.RenewGroupMemberMethodName)
	renewArgs, err := renewMethod.Inputs.Pack(
		ownerAddr, groupName, []common.Address{memberAddr}, []int64{longExpiry},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, renewMethod.ID...), renewArgs...))

	after, err := storageClient.HeadGroupMember(ctx, &storagetypes.QueryHeadGroupMemberRequest{
		Member: memberAddr.String(), GroupOwner: ownerAddr.String(), GroupName: groupName,
	})
	require.NoError(t, err)
	require.Equal(t, longExpiry, after.GroupMember.ExpirationTime.Unix(), "renewGroupMember must update the existing member's expiration in place")
}
