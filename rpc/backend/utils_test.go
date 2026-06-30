package backend

import (
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// logAddress and logData are shared fixtures for the log tests.
var (
	logAddress = common.HexToAddress("0x0200000000000000000000000000000000000001")
	logData    = []byte{0xde, 0xad, 0xbe, 0xef}
)

// buildLogTxResultData returns marshalled ExecTxResult.Data with one log and the expected decoded logs.
func buildLogTxResultData(height int64) (data []byte, expLogs []*ethtypes.Log) {
	resp := &evmtypes.MsgEthereumTxResponse{
		Hash: common.BytesToHash([]byte("eth_tx_hash")).Hex(),
		Logs: []*evmtypes.Log{
			{
				Address:     logAddress.String(),
				Topics:      []string{common.BytesToHash([]byte("topic0")).Hex()},
				Data:        logData,
				BlockNumber: uint64(height),
			},
		},
	}

	anyData := codectypes.UnsafePackAny(resp)
	bz, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyData}})
	if err != nil {
		panic(err)
	}

	expLog := resp.Logs[0].ToEthereum()
	expLog.TxHash = common.HexToHash(resp.Hash)
	expLog.BlockNumber = uint64(height)
	return bz, []*ethtypes.Log{expLog}
}

// TestGetLogsFromBlockResults verifies logs are decoded from tx response Data, not cosmos events.
func TestGetLogsFromBlockResults(t *testing.T) {
	height := int64(8)
	data, expLogs := buildLogTxResultData(height)

	blockRes := &tmrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: []*abci.ExecTxResult{{Code: 0, Data: data}},
	}

	blockLogs, err := GetLogsFromBlockResults(blockRes)
	require.NoError(t, err)
	require.Len(t, blockLogs, 1)    // one entry per tx
	require.Len(t, blockLogs[0], 1) // one log emitted by that tx
	require.Equal(t, expLogs, blockLogs[0])

	got := blockLogs[0][0]
	require.Equal(t, logAddress, got.Address)
	require.Equal(t, logData, got.Data)
	require.Equal(t, uint64(height), got.BlockNumber)
	require.NotEqual(t, common.Hash{}, got.TxHash)

	// A tx with empty Data yields empty logs without error (DecodeTxLogs(nil)).
	emptyRes := &tmrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: []*abci.ExecTxResult{{Code: 0}},
	}
	emptyLogs, err := GetLogsFromBlockResults(emptyRes)
	require.NoError(t, err)
	require.Len(t, emptyLogs, 1)
	require.Empty(t, emptyLogs[0])
}

// TestDecodeMsgLogsByMsgIndex proves that decoding logs by res.MsgIndex selects the
// correct EVM message's logs in a multi-message tx. In moca a MsgEthereumTx is only
// ever included in an all-EVM tx (the EVM ante decorators reject any non-EVM message,
// and the cosmos ante's RejectMessagesDecorator rejects MsgEthereumTx outside an EVM
// tx), so the Cosmos message index equals the position in the MsgEthereumTxResponse
// slice that DecodeMsgLogs indexes into. This guards the receipt (tx_info.go) and
// eth_getTransactionLogs (eth namespace) paths, which decode logs by res.MsgIndex.
func TestDecodeMsgLogsByMsgIndex(t *testing.T) {
	height := uint64(12)
	addr0 := common.HexToAddress("0x0200000000000000000000000000000000000001")
	addr1 := common.HexToAddress("0x0200000000000000000000000000000000000002")

	resp0 := &evmtypes.MsgEthereumTxResponse{
		Hash: common.BytesToHash([]byte("eth_tx_0")).Hex(),
		Logs: []*evmtypes.Log{{Address: addr0.String(), Data: []byte{0x00}, BlockNumber: height}},
	}
	resp1 := &evmtypes.MsgEthereumTxResponse{
		Hash: common.BytesToHash([]byte("eth_tx_1")).Hex(),
		Logs: []*evmtypes.Log{{Address: addr1.String(), Data: []byte{0x11}, BlockNumber: height}},
	}
	data, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{
		codectypes.UnsafePackAny(resp0),
		codectypes.UnsafePackAny(resp1),
	}})
	require.NoError(t, err)

	// MsgIndex 0 -> first EVM response's logs.
	logs0, err := evmtypes.DecodeMsgLogs(data, 0, height)
	require.NoError(t, err)
	require.Len(t, logs0, 1)
	require.Equal(t, addr0, logs0[0].Address)

	// MsgIndex 1 -> second EVM response's logs (not the first, and no error).
	logs1, err := evmtypes.DecodeMsgLogs(data, 1, height)
	require.NoError(t, err)
	require.Len(t, logs1, 1)
	require.Equal(t, addr1, logs1[0].Address)
}

func mookProofs(num int, withData bool) *crypto.ProofOps {
	var proofOps *crypto.ProofOps
	if num > 0 {
		proofOps = new(crypto.ProofOps)
		for i := 0; i < num; i++ {
			proof := crypto.ProofOp{}
			if withData {
				proof.Data = []byte("\n\031\n\003KEY\022\005VALUE\032\013\010\001\030\001 \001*\003\000\002\002")
			}
			proofOps.Ops = append(proofOps.Ops, proof)
		}
	}
	return proofOps
}

func (suite *BackendTestSuite) TestGetHexProofs() {
	defaultRes := []string{""}
	testCases := []struct {
		name  string
		proof *crypto.ProofOps
		exp   []string
	}{
		{
			"no proof provided",
			mookProofs(0, false),
			defaultRes,
		},
		{
			"no proof data provided",
			mookProofs(1, false),
			defaultRes,
		},
		{
			"valid proof provided",
			mookProofs(1, true),
			[]string{"0x0a190a034b4559120556414c55451a0b0801180120012a03000202"},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.Require().Equal(tc.exp, GetHexProofs(tc.proof))
		})
	}
}
