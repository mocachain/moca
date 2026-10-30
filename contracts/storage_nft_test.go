package contracts_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/contracts"
)

// TestStorageNFTGenesisContracts pins the NFT contracts installed at genesis to what
// mainnet, testnet and devnet hold at 0x3000-0x3002 (code hash and initial slots), so
// fresh chains and upgraded chains run the same contracts.
func TestStorageNFTGenesisContracts(t *testing.T) {
	const (
		objectCodeHash = "0xecd6bf3007d73828911249dc5ec7c7d1c2daf142729b0a455680d327cf76a7d5"
		nftCodeHash    = "0x62d035107881fc41c013a7ba38a2774f29a8e272f85826baf84f3a61663cbadd"
	)
	want := map[common.Address]struct {
		codeHash, name, symbol, baseURI string
	}{
		contracts.ObjectERC721TokenAddress: {objectCodeHash, "Object NFT", "OBJECT", "https://devnet-nft.mechain.tech/object/"},
		contracts.BucketERC721TokenAddress: {nftCodeHash, "Bucket NFT", "BUCKET", "bucket"},
		contracts.GroupERC721TokenAddress:  {nftCodeHash, "Group NFT", "GROUP", "group"},
	}

	got := contracts.StorageNFTGenesisContracts()
	require.Len(t, got, len(want))
	for _, c := range got {
		w, ok := want[c.Address]
		require.True(t, ok, "unexpected genesis contract at %s", c.Address)
		require.Equal(t, w.codeHash, crypto.Keccak256Hash(c.Code).Hex(), "code at %s", c.Address)

		slots := make(map[common.Hash]common.Hash, len(c.Storage))
		for _, s := range c.Storage {
			slots[s.Key] = s.Value
		}
		require.Equal(t, w.name, solidityString(slots, 0), "name at %s", c.Address)
		require.Equal(t, w.symbol, solidityString(slots, 1), "symbol at %s", c.Address)
		require.Equal(t, w.baseURI, solidityString(slots, 2), "base URI at %s", c.Address)
		require.Equal(t, common.BytesToHash(common.HexToAddress("0x000000000000000000000000000000000000dead").Bytes()),
			slots[common.BigToHash(big.NewInt(3))], "control hub at %s", c.Address)
	}
}

// solidityString decodes the string Solidity stores at slot n: inline (length*2 in the
// last byte) when shorter than 32 bytes, otherwise length*2+1 in the slot and the bytes
// from keccak256(n) on.
func solidityString(slots map[common.Hash]common.Hash, n int64) string {
	head := slots[common.BigToHash(big.NewInt(n))]
	if head[31]&1 == 0 {
		return string(head[:head[31]/2])
	}
	length := int(head.Big().Int64()-1) / 2
	base := crypto.Keccak256Hash(common.BigToHash(big.NewInt(n)).Bytes()).Big()
	var b strings.Builder
	for i := 0; b.Len() < length; i++ {
		word := slots[common.BigToHash(new(big.Int).Add(base, big.NewInt(int64(i))))]
		b.Write(word[:])
	}
	return b.String()[:length]
}
