package contracts

import (
	_ "embed" // embed the NFT contracts' runtime bytecode
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// The runtime bytecode of the ERC721NonTransferable contracts at the three NFT token
// addresses, exactly as moca's pre-v2 EVM module installed them at genesis and as every
// live network holds them. The object contract was built with solc 0.8.17 and the
// bucket and group contracts, which share one build, with solc 0.8.20.
var (
	//go:embed compiled_contracts/ERC721NonTransferable.object.runtime.hex
	objectNFTRuntimeHex string

	//go:embed compiled_contracts/ERC721NonTransferable.runtime.hex
	nftRuntimeHex string
)

// StorageSlot is one initial storage slot of a genesis contract.
type StorageSlot struct {
	Key   common.Hash
	Value common.Hash
}

// GenesisContract is a contract installed at a fixed address when a chain starts.
type GenesisContract struct {
	Address common.Address
	Code    []byte
	Storage []StorageSlot
}

// StorageNFTGenesisContracts returns the bucket, object and group NFT contracts the
// storage keeper mints and burns through, with their initial storage. Slots 0 to 3 of
// ERC721NonTransferable are the name, the symbol, the base URI and the control hub;
// strings of 32 bytes or more live at keccak256(slot), as Solidity lays them out.
func StorageNFTGenesisContracts() []GenesisContract {
	objectCode := mustDecodeRuntime(objectNFTRuntimeHex)
	nftCode := mustDecodeRuntime(nftRuntimeHex)
	return []GenesisContract{
		{
			Address: ObjectERC721TokenAddress,
			Code:    objectCode,
			Storage: []StorageSlot{
				slot("0x0", "0x4f626a656374204e465400000000000000000000000000000000000000000014"), // "Object NFT"
				slot("0x1", "0x4f424a454354000000000000000000000000000000000000000000000000000c"), // "OBJECT"
				slot("0x2", "0x000000000000000000000000000000000000000000000000000000000000004f"), // base URI, 39 bytes
				slot("0x3", controlHubSlot(ObjectControlHubAddress)),
				slot("0x405787fa12a823e0f2b7631cc41b3ba8828b3321ca811111fa75cd3aa3bb5ace",
					"0x68747470733a2f2f6465766e65742d6e66742e6d65636861696e2e746563682f"), // "https://devnet-nft.mechain.tech/"
				slot("0x405787fa12a823e0f2b7631cc41b3ba8828b3321ca811111fa75cd3aa3bb5acf",
					"0x6f626a6563742f00000000000000000000000000000000000000000000000000"), // "object/"
			},
		},
		{
			Address: GroupERC721TokenAddress,
			Code:    nftCode,
			Storage: []StorageSlot{
				slot("0x0", "0x47726f7570204e46540000000000000000000000000000000000000000000012"), // "Group NFT"
				slot("0x1", "0x47524f555000000000000000000000000000000000000000000000000000000a"), // "GROUP"
				slot("0x2", "0x67726f757000000000000000000000000000000000000000000000000000000a"), // "group"
				slot("0x3", controlHubSlot(GroupControlHubAddress)),
			},
		},
		{
			Address: BucketERC721TokenAddress,
			Code:    nftCode,
			Storage: []StorageSlot{
				slot("0x0", "0x4275636b6574204e465400000000000000000000000000000000000000000014"), // "Bucket NFT"
				slot("0x1", "0x4255434b4554000000000000000000000000000000000000000000000000000c"), // "BUCKET"
				slot("0x2", "0x6275636b6574000000000000000000000000000000000000000000000000000c"), // "bucket"
				slot("0x3", controlHubSlot(BucketControlHubAddress)),
			},
		},
	}
}

func slot(key, value string) StorageSlot {
	return StorageSlot{Key: common.HexToHash(key), Value: common.HexToHash(value)}
}

func controlHubSlot(hub common.Address) string {
	return common.BytesToHash(hub.Bytes()).Hex()
}

func mustDecodeRuntime(s string) []byte {
	code, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		panic(err)
	}
	return code
}
