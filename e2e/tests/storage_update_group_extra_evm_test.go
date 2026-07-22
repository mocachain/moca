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

// TestStorageUpdateGroupExtraEvmFlow drives x/storage's group-extra metadata
// update through the storage precompile's updateGroupExtra method: the
// owner can attach/replace a free-form extra string on an existing group.
func TestStorageUpdateGroupExtraEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	groupName := "evm-update-group-extra-test"
	createGroupMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createGroupArgs, err := createGroupMethod.Inputs.Pack(groupName, "original extra")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, createGroupMethod.ID...), createGroupArgs...))

	updateExtraMethod := storage.GetAbiMethod(storage.UpdateGroupExtraMethodName)
	updateExtraArgs, err := updateExtraMethod.Inputs.Pack(ownerAddr, groupName, "updated extra")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, updateExtraMethod.ID...), updateExtraArgs...))

	headResp, err := storageClient.HeadGroup(ctx, &storagetypes.QueryHeadGroupRequest{GroupOwner: ownerAddr.String(), GroupName: groupName})
	require.NoError(t, err)
	require.Equal(t, "updated extra", headResp.GroupInfo.Extra)
}
