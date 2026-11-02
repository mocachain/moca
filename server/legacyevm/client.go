// Package legacyevm wraps the client context of the node's JSON-RPC server and EVM
// indexer so they read the Ethereum transactions recorded before v2.0.0: each
// /ethermint.evm.v1.MsgEthereumTx decodes as a /cosmos.evm.vm.v1.MsgEthereumTx, and
// each old MsgEthereumTxResponse in a transaction result gets the cosmos/evm type URL.
// basefee.go answers the base fee of pre-v2 heights.
package legacyevm

import (
	"bytes"
	"context"
	"slices"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	legacyevmtypes "github.com/mocachain/moca/v2/x/legacy/evm/types"
)

const legacyResponseURL = "/ethermint.evm.v1.MsgEthereumTxResponse"

var (
	responseURL              = "/" + proto.MessageName(&evmtypes.MsgEthereumTxResponse{})
	extensionOptionURL       = "/" + proto.MessageName(&evmtypes.ExtensionOptionsEthereumTx{})
	legacyExtensionOptionURL = "/" + proto.MessageName(&legacyevmtypes.ExtensionOptionsEthereumTx{})
)

// WrapClientContext returns clientCtx for the JSON-RPC server and the EVM indexer. Its
// transaction decoder returns pre-v2 Ethereum transactions in the cosmos/evm format and,
// when its client is a CometBFT RPC client, the transaction results it returns carry the
// cosmos/evm response type URL. clientCtx's registry must know the legacy types.
func WrapClientContext(clientCtx client.Context) client.Context {
	if clientCtx.TxConfig != nil {
		if _, wrapped := clientCtx.TxConfig.(txConfig); !wrapped {
			clientCtx = clientCtx.WithTxConfig(txConfig{TxConfig: clientCtx.TxConfig})
		}
	}
	if c, ok := clientCtx.Client.(rpcclient.Client); ok {
		if _, wrapped := c.(cometClient); !wrapped {
			clientCtx = clientCtx.WithClient(cometClient{Client: c})
		}
	}
	return clientCtx
}

type txConfig struct {
	client.TxConfig
}

func (c txConfig) TxDecoder() sdk.TxDecoder {
	return TxDecoder(c.TxConfig)
}

// TxDecoder returns txConfig's decoder with each pre-v2 Ethereum message of a
// transaction replaced by its cosmos/evm form, and the old extension option by the
// cosmos/evm one. It fails on an old message it cannot convert.
func TxDecoder(txConfig client.TxConfig) sdk.TxDecoder {
	decode := txConfig.TxDecoder()
	return func(txBytes []byte) (sdk.Tx, error) {
		tx, err := decode(txBytes)
		if err != nil {
			return nil, err
		}
		msgs := tx.GetMsgs()
		var converted []sdk.Msg
		for i, msg := range msgs {
			legacy, ok := msg.(*legacyevmtypes.MsgEthereumTx)
			if !ok {
				continue
			}
			if converted == nil {
				converted = slices.Clone(msgs)
			}
			if converted[i], err = legacy.ToMsgEthereumTx(); err != nil {
				return nil, err
			}
		}
		if converted == nil {
			return tx, nil
		}

		builder, err := txConfig.WrapTxBuilder(tx)
		if err != nil {
			return nil, err
		}
		if err := builder.SetMsgs(converted...); err != nil {
			return nil, err
		}
		if extTx, ok := tx.(interface{ GetExtensionOptions() []*codectypes.Any }); ok {
			if extBuilder, ok := builder.(client.ExtendedTxBuilder); ok {
				extBuilder.SetExtensionOptions(convertExtensionOptions(extTx.GetExtensionOptions())...)
			}
		}
		return builder.GetTx(), nil
	}
}

func convertExtensionOptions(opts []*codectypes.Any) []*codectypes.Any {
	out := slices.Clone(opts)
	for i, opt := range out {
		if opt.TypeUrl == legacyExtensionOptionURL {
			out[i] = &codectypes.Any{TypeUrl: extensionOptionURL}
		}
	}
	return out
}

// cometClient renames the pre-v2 Ethereum responses in the transaction results it
// returns.
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

// convertResult gives the pre-v2 Ethereum responses in r's data the cosmos/evm type URL.
func convertResult(r *abci.ExecTxResult) *abci.ExecTxResult {
	if r == nil || !bytes.Contains(r.Data, []byte(legacyResponseURL)) {
		return r
	}
	var data sdk.TxMsgData
	if err := proto.Unmarshal(r.Data, &data); err != nil {
		return r
	}
	for _, res := range data.MsgResponses {
		if res.TypeUrl == legacyResponseURL {
			res.TypeUrl = responseURL
		}
	}
	bz, err := proto.Marshal(&data)
	if err != nil {
		return r
	}
	out := *r
	out.Data = bz
	return &out
}
