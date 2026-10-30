package legacyevm_test

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	sdk "github.com/cosmos/cosmos-sdk/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/server/legacyevm"
)

// recordedTx is an Ethereum transaction a v1.4.3 test network recorded before its
// v2.0.0 upgrade: the CometBFT transaction bytes and the transaction's result data.
type recordedTx struct {
	EthHash    string `json:"eth_hash"`
	Height     int64  `json:"height"`
	Tx         []byte `json:"tx"`
	ResultData []byte `json:"result_data"`
}

func loadRecorded(t *testing.T) []recordedTx {
	t.Helper()
	bz, err := os.ReadFile("testdata/pre_v2_evm_txs.json")
	require.NoError(t, err)
	var txs []recordedTx
	require.NoError(t, json.Unmarshal(bz, &txs))
	require.NotEmpty(t, txs)
	return txs
}

func strictDecoder(t *testing.T) sdk.TxDecoder {
	t.Helper()
	encCfg := encoding.MakeConfig()
	evmtypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	return encCfg.TxConfig.TxDecoder()
}

func singleEthMsg(t *testing.T, tx sdk.Tx) *evmtypes.MsgEthereumTx {
	t.Helper()
	msgs := tx.GetMsgs()
	require.Len(t, msgs, 1)
	msg, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	require.True(t, ok, "got %T", msgs[0])
	return msg
}

func requireSigner(t *testing.T, msg *evmtypes.MsgEthereumTx) {
	t.Helper()
	ethTx := msg.AsTransaction()
	sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(ethTx.ChainId()), ethTx)
	require.NoError(t, err)
	require.Equal(t, sender.Bytes(), msg.From)
}

func TestTxDecoder_RecordedTransactions(t *testing.T) {
	strict := strictDecoder(t)
	decode := legacyevm.TxDecoder(strict)
	for _, rec := range loadRecorded(t) {
		t.Run(rec.EthHash, func(t *testing.T) {
			// The state machine's decoder still rejects the old format.
			_, err := strict(rec.Tx)
			require.ErrorContains(t, err, "ethermint.evm.v1.MsgEthereumTx")

			tx, err := decode(rec.Tx)
			require.NoError(t, err)
			msg := singleEthMsg(t, tx)
			require.Equal(t, rec.EthHash, msg.Hash().Hex())
			requireSigner(t, msg)
		})
	}
}

func TestConvertTxMsgData_RecordedResults(t *testing.T) {
	for _, rec := range loadRecorded(t) {
		t.Run(rec.EthHash, func(t *testing.T) {
			before, err := evmtypes.DecodeTxResponses(rec.ResultData)
			require.NoError(t, err)
			require.Empty(t, before, "cosmos/evm skips the old response type URL")

			after, err := evmtypes.DecodeTxResponses(legacyevm.ConvertTxMsgData(rec.ResultData))
			require.NoError(t, err)
			require.Len(t, after, 1)
			require.Equal(t, rec.EthHash, after[0].Hash)
		})
	}
}

func TestTxDecoder_AllTransactionTypes(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	chainID := big.NewInt(5151)
	to := common.HexToAddress("0x00000000000000000000000000000000000020ff")
	accessList := ethtypes.AccessList{{
		Address:     to,
		StorageKeys: []common.Hash{common.HexToHash("0x01"), common.HexToHash("0x02")},
	}}

	txs := map[string]ethtypes.TxData{
		"legacy contract creation": &ethtypes.LegacyTx{
			Nonce: 3, GasPrice: big.NewInt(1_000_000_000), Gas: 500_000, Value: new(big.Int), Data: []byte{0x60, 0x00},
		},
		"access list": &ethtypes.AccessListTx{
			ChainID: chainID, Nonce: 4, GasPrice: big.NewInt(2_000_000_000), Gas: 60_000, To: &to,
			Value: big.NewInt(7), Data: []byte{0xaa}, AccessList: accessList,
		},
		"dynamic fee": &ethtypes.DynamicFeeTx{
			ChainID: chainID, Nonce: 5, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(3_000_000_000), Gas: 21_000,
			To: &to, Value: big.NewInt(1_000_000_000_000_000_000), AccessList: accessList,
		},
	}
	decode := legacyevm.TxDecoder(strictDecoder(t))
	for name, data := range txs {
		t.Run(name, func(t *testing.T) {
			ethTx := signTx(t, key, chainID, data)

			for _, recordedFrom := range []string{from.Hex(), ""} {
				tx, err := decode(legacyTxRaw(legacyMsg(ethTx, ethTx.Hash().Hex(), recordedFrom)))
				require.NoError(t, err)
				msg := singleEthMsg(t, tx)
				require.Equal(t, ethTx.Hash(), msg.Hash())
				require.Equal(t, from.Bytes(), msg.From)
				requireSigner(t, msg)
			}
		})
	}
}

func TestTxDecoder_HashMismatchIsNotDecoded(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	ethTx := signTx(t, key, big.NewInt(5151), &ethtypes.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21_000, Value: new(big.Int)})

	txBytes := legacyTxRaw(legacyMsg(ethTx, common.Hash{0x01}.Hex(), ""))
	_, changed, err := legacyevm.ConvertTx(txBytes)
	require.ErrorContains(t, err, "recorded hash")
	require.False(t, changed)

	_, err = legacyevm.TxDecoder(strictDecoder(t))(txBytes)
	require.Error(t, err)
}

func TestConvert_LeavesOtherBytesAlone(t *testing.T) {
	other := []byte("not a legacy transaction")
	converted, changed, err := legacyevm.ConvertTx(other)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, other, converted)
	require.Equal(t, other, legacyevm.ConvertTxMsgData(other))
}

func TestWrapTxConfig_IsIdempotent(t *testing.T) {
	txConfig := encoding.MakeConfig().TxConfig
	wrapped := legacyevm.WrapTxConfig(txConfig)
	require.Equal(t, wrapped, legacyevm.WrapTxConfig(wrapped))
}

func signTx(t *testing.T, key *ecdsa.PrivateKey, chainID *big.Int, data ethtypes.TxData) *ethtypes.Transaction {
	t.Helper()
	tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), data)
	require.NoError(t, err)
	return tx
}

// legacyMsg encodes tx the way the pre-v2 ethermint module recorded it.
func legacyMsg(tx *ethtypes.Transaction, hash, from string) []byte {
	v, r, s := tx.RawSignatureValues()
	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}
	var (
		url  string
		data []byte
	)
	switch tx.Type() {
	case ethtypes.LegacyTxType:
		url = "/ethermint.evm.v1.LegacyTx"
		data = appendUint(data, 1, tx.Nonce())
		data = appendStr(data, 2, tx.GasPrice().String())
		data = appendUint(data, 3, tx.Gas())
		data = appendStr(data, 4, to)
		data = appendStr(data, 5, tx.Value().String())
		data = appendBz(data, 6, tx.Data())
		data = appendBz(data, 7, v.Bytes())
		data = appendBz(data, 8, r.Bytes())
		data = appendBz(data, 9, s.Bytes())
	case ethtypes.AccessListTxType:
		url = "/ethermint.evm.v1.AccessListTx"
		data = appendStr(data, 1, tx.ChainId().String())
		data = appendUint(data, 2, tx.Nonce())
		data = appendStr(data, 3, tx.GasPrice().String())
		data = appendUint(data, 4, tx.Gas())
		data = appendStr(data, 5, to)
		data = appendStr(data, 6, tx.Value().String())
		data = appendBz(data, 7, tx.Data())
		data = appendAccessList(data, 8, tx.AccessList())
		data = appendBz(data, 9, v.Bytes())
		data = appendBz(data, 10, r.Bytes())
		data = appendBz(data, 11, s.Bytes())
	case ethtypes.DynamicFeeTxType:
		url = "/ethermint.evm.v1.DynamicFeeTx"
		data = appendStr(data, 1, tx.ChainId().String())
		data = appendUint(data, 2, tx.Nonce())
		data = appendStr(data, 3, tx.GasTipCap().String())
		data = appendStr(data, 4, tx.GasFeeCap().String())
		data = appendUint(data, 5, tx.Gas())
		data = appendStr(data, 6, to)
		data = appendStr(data, 7, tx.Value().String())
		data = appendBz(data, 8, tx.Data())
		data = appendAccessList(data, 9, tx.AccessList())
		data = appendBz(data, 10, v.Bytes())
		data = appendBz(data, 11, r.Bytes())
		data = appendBz(data, 12, s.Bytes())
	}
	msg := appendBz(nil, 1, anyBytes(url, data))
	msg = appendStr(msg, 3, hash)
	return appendStr(msg, 4, from)
}

// legacyTxRaw wraps one legacy message in a TxRaw the way ethermint's clients did.
func legacyTxRaw(msg []byte) []byte {
	body := appendBz(nil, 1, anyBytes("/ethermint.evm.v1.MsgEthereumTx", msg))
	body = appendBz(body, 1023, anyBytes("/ethermint.evm.v1.ExtensionOptionsEthereumTx", nil))
	fee := appendUint(nil, 2, 21_000)
	authInfo := appendBz(nil, 2, fee)
	raw := appendBz(nil, 1, body)
	return appendBz(raw, 2, authInfo)
}

func anyBytes(url string, value []byte) []byte {
	return appendBz(appendStr(nil, 1, url), 2, value)
}

func appendAccessList(b []byte, num protowire.Number, list ethtypes.AccessList) []byte {
	for _, tuple := range list {
		t := appendStr(nil, 1, tuple.Address.Hex())
		for _, key := range tuple.StorageKeys {
			t = appendStr(t, 2, key.Hex())
		}
		b = appendBz(b, num, t)
	}
	return b
}

func appendUint(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	return protowire.AppendVarint(protowire.AppendTag(b, num, protowire.VarintType), v)
}

func appendStr(b []byte, num protowire.Number, s string) []byte {
	if s == "" {
		return b
	}
	return protowire.AppendString(protowire.AppendTag(b, num, protowire.BytesType), s)
}

func appendBz(b []byte, num protowire.Number, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	return protowire.AppendBytes(protowire.AppendTag(b, num, protowire.BytesType), v)
}
