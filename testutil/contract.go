package testutil

import (
	"fmt"
	"math/big"

	"github.com/cosmos/gogoproto/proto"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/testutil/tx"
	evm "github.com/cosmos/evm/x/vm/types"
)

// DeployContract deploys a contract with the provided private key,
// compiled contract data and constructor arguments
func DeployContract(
	ctx sdk.Context,
	mocaApp *app.Moca,
	priv cryptotypes.PrivKey,
	queryClientEvm evm.QueryClient,
	contract evm.CompiledContract,
	constructorArgs ...interface{},
) (common.Address, error) {
	// TODO(cosmos-evm migration): chainID + GetBaseFee shape changed.
	// See testutil/tx/eth.go for the same shim notes.
	var chainID *big.Int
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	nonce := mocaApp.EvmKeeper.GetNonce(ctx, from)

	ctorArgs, err := contract.ABI.Pack("", constructorArgs...)
	if err != nil {
		return common.Address{}, err
	}

	data := append(contract.Bin, ctorArgs...) //nolint:gocritic
	gas, err := tx.GasLimit(ctx, from, data, queryClientEvm)
	if err != nil {
		return common.Address{}, err
	}

	baseFee := mocaApp.FeeMarketKeeper.GetBaseFee(ctx)
	msgEthereumTx := evm.NewTx(&evm.EvmTxArgs{
		ChainID:   chainID,
		Nonce:     nonce,
		GasLimit:  gas,
		GasFeeCap: baseFee.TruncateInt().BigInt(),
		GasTipCap: big.NewInt(1),
		Input:     data,
		Accesses:  &ethtypes.AccessList{},
	})
	// cosmos/evm v0.6.0: MsgEthereumTx.From is now []byte.
	msgEthereumTx.From = from.Bytes()

	res, err := DeliverEthTx(ctx, mocaApp, priv, msgEthereumTx)
	if err != nil {
		return common.Address{}, err
	}

	if _, err := CheckEthTxResponse(res, mocaApp.AppCodec()); err != nil {
		return common.Address{}, err
	}

	return crypto.CreateAddress(from, nonce), nil
}

// DeployContractWithFactory deploys a contract using a contract factory
// with the provided factoryAddress
func DeployContractWithFactory(
	ctx sdk.Context,
	mocaApp *app.Moca,
	priv cryptotypes.PrivKey,
	factoryAddress common.Address,
) (common.Address, abci.ExecTxResult, error) {
	// TODO(cosmos-evm migration): chainID source removed; see above.
	var chainID *big.Int
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	factoryNonce := mocaApp.EvmKeeper.GetNonce(ctx, factoryAddress)
	nonce := mocaApp.EvmKeeper.GetNonce(ctx, from)

	msgEthereumTx := evm.NewTx(&evm.EvmTxArgs{
		ChainID:  chainID,
		Nonce:    nonce,
		To:       &factoryAddress,
		GasLimit: uint64(100000),
		GasPrice: big.NewInt(1000000000),
	})
	// cosmos/evm v0.6.0: MsgEthereumTx.From is now []byte.
	msgEthereumTx.From = from.Bytes()

	res, err := DeliverEthTx(ctx, mocaApp, priv, msgEthereumTx)
	if err != nil {
		return common.Address{}, abci.ExecTxResult{}, err
	}

	if _, err := CheckEthTxResponse(res, mocaApp.AppCodec()); err != nil {
		return common.Address{}, abci.ExecTxResult{}, err
	}

	return crypto.CreateAddress(factoryAddress, factoryNonce), res, err
}

// CheckEthTxResponse checks that the transaction was executed successfully
func CheckEthTxResponse(r abci.ExecTxResult, cdc codec.Codec) (*evm.MsgEthereumTxResponse, error) {
	if !r.IsOK() {
		return nil, fmt.Errorf("tx failed. Code: %d, Logs: %s", r.Code, r.Log)
	}
	var txData sdk.TxMsgData
	if err := cdc.Unmarshal(r.Data, &txData); err != nil {
		return nil, err
	}

	var res evm.MsgEthereumTxResponse
	if err := proto.Unmarshal(txData.MsgResponses[0].Value, &res); err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, fmt.Errorf("tx failed. VmError: %s", res.VmError)
	}

	return &res, nil
}
