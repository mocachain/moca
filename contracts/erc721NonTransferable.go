// Package contracts holds the compiled-contract artifacts moca keepers encode
// EVM calls against (mirroring cosmos/evm's top-level contracts/ package).
//
// The JSON artifacts here are ABI-only and have no in-tree Solidity source: the
// embedded JSON is the source of truth (greenfield heritage) and keepers only
// encode calldata from it. The NFT contracts' runtime bytecode is kept apart, in
// compiled_contracts/*.runtime.hex, exactly as live networks hold it; the app
// installs it at the fixed token addresses when a chain starts
// (StorageNFTGenesisContracts). There is deliberately no compile/regenerate
// target; to change an artifact, edit it and update the guard tests that pin its
// call signatures, addresses and code hashes.
//
// Adding a new contract: author the Solidity under solidity/ (the maintained
// hardhat project), compile there, commit the artifact JSON here with a Go
// embed loader, and pin it with a guard test like the one in this package.
package contracts

import (
	_ "embed" // embed compiled smart contract
	"encoding/json"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

// ERC-721 contract for bucket/object/group NFT mirroring; transfers disabled at the contract level.
var (
	//go:embed compiled_contracts/ERC721NonTransferable.json
	ERC721NonTransferableJSON []byte //nolint: golint

	// ERC721NonTransferableContract is the compiled erc721 contract
	ERC721NonTransferableContract evmtypes.CompiledContract

	// ObjectERC721TokenAddress is the object erc721 module address
	ObjectERC721TokenAddress common.Address

	// BucketERC721TokenAddress is the bucket erc721 module address
	BucketERC721TokenAddress common.Address

	// GroupERC721TokenAddress is the group erc721 module address
	GroupERC721TokenAddress common.Address

	// ObjectControlHubAddress is the object nft control hub address
	ObjectControlHubAddress common.Address

	// BucketControlHubAddress is the bucket nft control hub address
	BucketControlHubAddress common.Address

	// GroupControlHubAddress is the group nft control hub address
	GroupControlHubAddress common.Address
)

func init() {
	ObjectERC721TokenAddress = common.HexToAddress("0x0000000000000000000000000000000000003000")
	BucketERC721TokenAddress = common.HexToAddress("0x0000000000000000000000000000000000003001")
	GroupERC721TokenAddress = common.HexToAddress("0x0000000000000000000000000000000000003002")
	ObjectControlHubAddress = common.HexToAddress("0x000000000000000000000000000000000000dead")
	GroupControlHubAddress = common.HexToAddress("0x000000000000000000000000000000000000dead")
	BucketControlHubAddress = common.HexToAddress("0x000000000000000000000000000000000000dead")

	err := json.Unmarshal(ERC721NonTransferableJSON, &ERC721NonTransferableContract)
	if err != nil {
		panic(err)
	}
}
