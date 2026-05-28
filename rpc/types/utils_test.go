package types

import (
	"math/big"
	"testing"
	"time"

	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestRPCMarshalHeaderUsesCanonicalBlockHash(t *testing.T) {
	header := tmtypes.Header{
		Version: tmversion.Consensus{
			Block: 11,
			App:   22,
		},
		ChainID:            "moca_222888-1",
		Height:             7,
		Time:               time.Unix(1710000000, 0).UTC(),
		LastBlockID:        tmtypes.BlockID{Hash: cmtbytes.HexBytes(common.Hex2Bytes("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))},
		LastCommitHash:     cmtbytes.HexBytes(common.Hex2Bytes("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")),
		DataHash:           cmtbytes.HexBytes(common.Hex2Bytes("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")),
		ValidatorsHash:     cmtbytes.HexBytes(common.Hex2Bytes("dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")),
		NextValidatorsHash: cmtbytes.HexBytes(common.Hex2Bytes("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")),
		ConsensusHash:      cmtbytes.HexBytes(common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")),
		AppHash:            cmtbytes.HexBytes(common.Hex2Bytes("1111111111111111111111111111111111111111111111111111111111111111")),
		LastResultsHash:    cmtbytes.HexBytes(common.Hex2Bytes("2222222222222222222222222222222222222222222222222222222222222222")),
		EvidenceHash:       cmtbytes.HexBytes(common.Hex2Bytes("3333333333333333333333333333333333333333333333333333333333333333")),
		ProposerAddress:    cmtbytes.HexBytes(common.Hex2Bytes("4444444444444444444444444444444444444444")),
	}

	baseFee := big.NewInt(12345)
	ethHeader := EthHeaderFromTendermint(header, ethtypes.Bloom{}, baseFee)
	canonicalHash := common.BytesToHash(header.Hash())

	require.NotEqual(t, canonicalHash, ethHeader.Hash())

	rpcHeader := RPCMarshalHeader(ethHeader, canonicalHash)

	require.Equal(t, canonicalHash, rpcHeader["hash"])
	require.Equal(t, common.BytesToHash(header.LastBlockID.Hash), rpcHeader["parentHash"])
	require.Equal(t, header.Height, rpcHeader["number"].(*hexutil.Big).ToInt().Int64())
	require.Equal(t, (*hexutil.Big)(baseFee), rpcHeader["baseFeePerGas"])
}

func TestBlockHeaderFromProtoCanonicalHash(t *testing.T) {
	protoHeader := tmproto.Header{
		Version:         tmversion.Consensus{Block: 11, App: 22},
		ChainID:         "moca_222888-1",
		Height:          9,
		Time:            time.Unix(1710000100, 0).UTC(),
		LastBlockId:     tmproto.BlockID{Hash: common.Hex2Bytes("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")},
		AppHash:         common.Hex2Bytes("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		DataHash:        common.Hex2Bytes("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
		ProposerAddress: common.Hex2Bytes("dddddddddddddddddddddddddddddddddddddddd"),
	}

	baseFee := big.NewInt(42)
	rpcHeader, err := BlockHeaderFromProto(&protoHeader, ethtypes.Bloom{}, baseFee)
	require.NoError(t, err)

	tmHeader, err := tmtypes.HeaderFromProto(&protoHeader)
	require.NoError(t, err)

	require.Equal(t, common.BytesToHash(tmHeader.Hash()), rpcHeader["hash"])
	require.Equal(t, common.BytesToHash(tmHeader.LastBlockID.Hash), rpcHeader["parentHash"])
	require.Equal(t, (*hexutil.Big)(baseFee), rpcHeader["baseFeePerGas"])
}
