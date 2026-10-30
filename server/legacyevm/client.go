package legacyevm

import (
	"bytes"
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// WrapClientContext returns clientCtx with a transaction decoder that reads pre-v2
// Ethereum transactions and, when its client is a CometBFT RPC client, with block
// results whose pre-v2 Ethereum responses carry the cosmos/evm type URL. It is for the
// node's query services only; the state machine keeps its own decoder.
func WrapClientContext(clientCtx client.Context) client.Context {
	if clientCtx.TxConfig != nil {
		clientCtx = clientCtx.WithTxConfig(WrapTxConfig(clientCtx.TxConfig))
	}
	if c, ok := clientCtx.Client.(rpcclient.Client); ok {
		if _, wrapped := c.(cometClient); !wrapped {
			clientCtx = clientCtx.WithClient(cometClient{Client: c})
		}
	}
	return clientCtx
}

// WrapTxConfig returns txConfig with a decoder that converts pre-v2 Ethereum
// transactions before decoding them. Everything else, encoding included, is
// txConfig's own.
func WrapTxConfig(txConfig client.TxConfig) client.TxConfig {
	if _, wrapped := txConfig.(txConfigWrapper); wrapped {
		return txConfig
	}
	return txConfigWrapper{TxConfig: txConfig}
}

type txConfigWrapper struct {
	client.TxConfig
}

func (c txConfigWrapper) TxDecoder() sdk.TxDecoder {
	return TxDecoder(c.TxConfig.TxDecoder())
}

// TxDecoder wraps next so that pre-v2 Ethereum transactions are converted first.
// Bytes without a legacy type URL go to next unchanged.
func TxDecoder(next sdk.TxDecoder) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		converted, changed, err := ConvertTx(txBytes)
		if err != nil || !changed {
			return next(txBytes)
		}
		return next(converted)
	}
}

// cometClient rewrites the transaction results it returns with ConvertTxMsgData.
type cometClient struct {
	rpcclient.Client
}

func (c cometClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	res, err := c.Client.BlockResults(ctx, height)
	if err != nil || res == nil {
		return res, err
	}
	out := *res
	out.TxsResults = make([]*abci.ExecTxResult, len(res.TxsResults))
	for i, r := range res.TxsResults {
		out.TxsResults[i] = convertResult(r)
	}
	return &out, nil
}

func (c cometClient) Tx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	res, err := c.Client.Tx(ctx, hash, prove)
	if err != nil || res == nil {
		return res, err
	}
	out := *res
	out.TxResult = *convertResult(&res.TxResult)
	return &out, nil
}

func (c cometClient) TxSearch(
	ctx context.Context, query string, prove bool, page, perPage *int, orderBy string,
) (*coretypes.ResultTxSearch, error) {
	res, err := c.Client.TxSearch(ctx, query, prove, page, perPage, orderBy)
	if err != nil || res == nil {
		return res, err
	}
	out := *res
	out.Txs = make([]*coretypes.ResultTx, len(res.Txs))
	for i, tx := range res.Txs {
		if tx == nil {
			continue
		}
		t := *tx
		t.TxResult = *convertResult(&tx.TxResult)
		out.Txs[i] = &t
	}
	return &out, nil
}

func convertResult(r *abci.ExecTxResult) *abci.ExecTxResult {
	if r == nil {
		return nil
	}
	data := ConvertTxMsgData(r.Data)
	if bytes.Equal(data, r.Data) {
		return r
	}
	out := *r
	out.Data = data
	return &out
}
