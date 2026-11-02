package legacyevm_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/server/legacyevm"
	legacyevmtypes "github.com/mocachain/moca/v2/x/legacy/evm/types"
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

// txConfigs returns the tx config of the app, which does not know the legacy types,
// and the tx config of the CLI and the query services, which does.
func txConfigs() (appTxConfig, queryTxConfig client.TxConfig) {
	appCfg := encoding.MakeConfig()
	evmtypes.RegisterInterfaces(appCfg.InterfaceRegistry)
	queryCfg := encoding.MakeConfig()
	evmtypes.RegisterInterfaces(queryCfg.InterfaceRegistry)
	legacyevmtypes.RegisterInterfaces(queryCfg.InterfaceRegistry)
	return appCfg.TxConfig, queryCfg.TxConfig
}

func singleMsg[T sdk.Msg](t *testing.T, tx sdk.Tx) T {
	t.Helper()
	msgs := tx.GetMsgs()
	require.Len(t, msgs, 1)
	msg, ok := msgs[0].(T)
	require.True(t, ok, "got %T", msgs[0])
	return msg
}

func requireEthMsg(t *testing.T, tx sdk.Tx, hash common.Hash) {
	t.Helper()
	msg := singleMsg[*evmtypes.MsgEthereumTx](t, tx)
	require.Equal(t, hash, msg.Hash())
	ethTx := msg.AsTransaction()
	sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(ethTx.ChainId()), ethTx)
	require.NoError(t, err)
	require.Equal(t, sender.Bytes(), msg.From)

	extTx, ok := tx.(interface{ GetExtensionOptions() []*codectypes.Any })
	require.True(t, ok)
	require.Len(t, extTx.GetExtensionOptions(), 1)
	require.Equal(t, "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx", extTx.GetExtensionOptions()[0].TypeUrl)
}

func TestRecordedTransactions(t *testing.T) {
	appTxConfig, queryTxConfig := txConfigs()
	ethDecode := legacyevm.TxDecoder(queryTxConfig)
	for _, rec := range loadRecorded(t) {
		t.Run(rec.EthHash, func(t *testing.T) {
			// The state machine still rejects the old format.
			_, err := appTxConfig.TxDecoder()(rec.Tx)
			require.ErrorIs(t, err, sdkerrors.ErrTxDecode)
			require.ErrorContains(t, err, "/ethermint.evm.v1.MsgEthereumTx")

			// The query services decode it as recorded.
			tx, err := queryTxConfig.TxDecoder()(rec.Tx)
			require.NoError(t, err)
			require.Equal(t, rec.EthHash, singleMsg[*legacyevmtypes.MsgEthereumTx](t, tx).Hash)
			txJSON, err := queryTxConfig.TxJSONEncoder()(tx)
			require.NoError(t, err)
			require.Contains(t, string(txJSON), `"@type":"/ethermint.evm.v1.MsgEthereumTx"`)

			// The JSON-RPC server gets the cosmos/evm message.
			tx, err = ethDecode(rec.Tx)
			require.NoError(t, err)
			requireEthMsg(t, tx, common.HexToHash(rec.EthHash))
		})
	}
}

// resultsClient serves one block's transaction results.
type resultsClient struct {
	rpcclient.Client
	results []*abci.ExecTxResult
}

func (c resultsClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	return &coretypes.ResultBlockResults{Height: *height, TxsResults: c.results}, nil
}

func TestRecordedResults(t *testing.T) {
	recs := loadRecorded(t)
	results := make([]*abci.ExecTxResult, len(recs))
	for i, rec := range recs {
		results[i] = &abci.ExecTxResult{Data: rec.ResultData}
	}
	clientCtx := legacyevm.WrapClientContext(client.Context{}.WithClient(resultsClient{results: results}))
	height := int64(300)
	res, err := clientCtx.Client.(rpcclient.Client).BlockResults(context.Background(), &height)
	require.NoError(t, err)

	for i, rec := range recs {
		before, err := evmtypes.DecodeTxResponses(rec.ResultData)
		require.NoError(t, err)
		require.Empty(t, before, "cosmos/evm skips the old response type URL")

		after, err := evmtypes.DecodeTxResponses(res.TxsResults[i].Data)
		require.NoError(t, err)
		require.Len(t, after, 1)
		require.Equal(t, rec.EthHash, after[0].Hash)
	}
}

func TestAllTransactionTypes(t *testing.T) {
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
	_, queryTxConfig := txConfigs()
	ethDecode := legacyevm.TxDecoder(queryTxConfig)
	for name, data := range txs {
		t.Run(name, func(t *testing.T) {
			ethTx := signTx(t, key, chainID, data)
			for _, recordedFrom := range []string{from.Hex(), ""} {
				tx, err := ethDecode(encodeLegacyTx(t, queryTxConfig, ethTx, ethTx.Hash().Hex(), recordedFrom))
				require.NoError(t, err)
				requireEthMsg(t, tx, ethTx.Hash())
			}
		})
	}
}

func TestHashMismatch(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	ethTx := signTx(t, key, big.NewInt(5151), &ethtypes.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21_000, Value: new(big.Int)})
	_, queryTxConfig := txConfigs()
	txBytes := encodeLegacyTx(t, queryTxConfig, ethTx, common.Hash{0x01}.Hex(), "")

	_, err = queryTxConfig.TxDecoder()(txBytes)
	require.NoError(t, err, "the query services return the transaction as recorded")
	_, err = legacyevm.TxDecoder(queryTxConfig)(txBytes)
	require.ErrorContains(t, err, "rebuilt transaction hashes to")
}

func TestWrapClientContext_IsIdempotent(t *testing.T) {
	clientCtx := legacyevm.WrapClientContext(client.Context{}.WithTxConfig(encoding.MakeConfig().TxConfig))
	require.Equal(t, clientCtx.TxConfig, legacyevm.WrapClientContext(clientCtx).TxConfig)
}

func signTx(t *testing.T, key *ecdsa.PrivateKey, chainID *big.Int, data ethtypes.TxData) *ethtypes.Transaction {
	t.Helper()
	tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), data)
	require.NoError(t, err)
	return tx
}

// encodeLegacyTx encodes tx the way the pre-v2 ethermint module recorded it.
func encodeLegacyTx(t *testing.T, txConfig client.TxConfig, tx *ethtypes.Transaction, hash, from string) []byte {
	t.Helper()
	v, r, s := tx.RawSignatureValues()
	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}
	accesses := make([]legacyevmtypes.AccessTuple, 0, len(tx.AccessList()))
	for _, tuple := range tx.AccessList() {
		keys := make([]string, len(tuple.StorageKeys))
		for i, k := range tuple.StorageKeys {
			keys[i] = k.Hex()
		}
		accesses = append(accesses, legacyevmtypes.AccessTuple{Address: tuple.Address.Hex(), StorageKeys: keys})
	}
	var data legacyevmtypes.TxData
	switch tx.Type() {
	case ethtypes.LegacyTxType:
		data = &legacyevmtypes.LegacyTx{
			Nonce: tx.Nonce(), GasPrice: tx.GasPrice().String(), Gas: tx.Gas(), To: to, Value: tx.Value().String(),
			Data: tx.Data(), V: v.Bytes(), R: r.Bytes(), S: s.Bytes(),
		}
	case ethtypes.AccessListTxType:
		data = &legacyevmtypes.AccessListTx{
			ChainId: tx.ChainId().String(), Nonce: tx.Nonce(), GasPrice: tx.GasPrice().String(), Gas: tx.Gas(), To: to,
			Value: tx.Value().String(), Data: tx.Data(), Accesses: accesses, V: v.Bytes(), R: r.Bytes(), S: s.Bytes(),
		}
	case ethtypes.DynamicFeeTxType:
		data = &legacyevmtypes.DynamicFeeTx{
			ChainId: tx.ChainId().String(), Nonce: tx.Nonce(), GasTipCap: tx.GasTipCap().String(),
			GasFeeCap: tx.GasFeeCap().String(), Gas: tx.Gas(), To: to, Value: tx.Value().String(), Data: tx.Data(),
			Accesses: accesses, V: v.Bytes(), R: r.Bytes(), S: s.Bytes(),
		}
	}
	dataAny, err := codectypes.NewAnyWithValue(data)
	require.NoError(t, err)
	extOpt, err := codectypes.NewAnyWithValue(&legacyevmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)

	builder := txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&legacyevmtypes.MsgEthereumTx{Data: dataAny, Hash: hash, From: from}))
	builder.(client.ExtendedTxBuilder).SetExtensionOptions(extOpt)
	builder.SetGasLimit(tx.Gas())
	bz, err := txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return bz
}
