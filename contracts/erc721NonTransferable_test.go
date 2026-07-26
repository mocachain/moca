package contracts_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/contracts"
)

// TestERC721NonTransferableContract pins the embedded compiled contract the
// storage keeper mints/burns bucket, object, and group NFTs through
// (x/storage/keeper CallEVM sites): the embed must parse and expose the exact
// ABI surface the keeper invokes, and the well-known token/hub addresses must
// stay fixed — they are protocol constants, not configuration.
func TestERC721NonTransferableContract(t *testing.T) {
	abi := contracts.ERC721NonTransferableContract.ABI

	mint, ok := abi.Methods["mint"]
	require.True(t, ok, "ABI must expose mint (used by the storage keeper)")
	require.Equal(t, "mint(address,uint256)", mint.Sig,
		"exact signature the keeper's CallEVM(..., \"mint\", owner, id) encodes against")

	burn, ok := abi.Methods["burn"]
	require.True(t, ok, "ABI must expose burn (used by the storage keeper)")
	require.Equal(t, "burn(uint256)", burn.Sig,
		"exact signature the keeper's CallEVM(..., \"burn\", id) encodes against")

	// The artifact intentionally ships no bytecode: the keeper only
	// ABI-encodes calldata against the fixed addresses below; nothing ever
	// deploys this contract.
	require.Empty(t, contracts.ERC721NonTransferableContract.Bin,
		"artifact is an ABI-only facade; bytecode appearing here would be unexplained")

	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000003000"), contracts.ObjectERC721TokenAddress)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000003001"), contracts.BucketERC721TokenAddress)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000003002"), contracts.GroupERC721TokenAddress)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000dead"), contracts.ObjectControlHubAddress)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000dead"), contracts.BucketControlHubAddress)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000dead"), contracts.GroupControlHubAddress)
}
