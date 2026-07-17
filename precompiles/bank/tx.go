package bank

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// SendMethodName is the ABI name for the Send transaction.
	SendMethodName = "send"
	// MultiSendMethodName is the ABI name for the MultiSend transaction.
	MultiSendMethodName = "multiSend"
)

// Send sends coins from the caller to a single recipient.
func (p Precompile) Send(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SendArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var amount sdk.Coins
	for _, coin := range input.Amount {
		amount = amount.Add(sdk.Coin{Denom: coin.Denom, Amount: math.NewIntFromBigInt(coin.Amount)})
	}

	msg := &banktypes.MsgSend{
		FromAddress: contract.Caller().String(),
		ToAddress:   input.ToAddress.String(),
		Amount:      amount,
	}

	if _, err := p.bankMsgServer.Send(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSendEvent(evm, contract.Caller(), input.ToAddress, amount.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// MultiSend sends coins from the caller to several recipients in a single transaction.
func (p Precompile) MultiSend(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input MultiSendArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var totalCoins sdk.Coins
	var outputs []banktypes.Output
	for _, output := range input.Outputs {
		var coins sdk.Coins
		for _, coin := range output.Amount {
			coins = coins.Add(sdk.Coin{Denom: coin.Denom, Amount: math.NewIntFromBigInt(coin.Amount)})
		}

		outputs = append(outputs, banktypes.Output{
			Address: output.ToAddress.String(),
			Coins:   coins,
		})

		totalCoins = totalCoins.Add(coins.Sort()...)
	}

	msg := &banktypes.MsgMultiSend{
		Inputs: []banktypes.Input{{
			Address: contract.Caller().String(),
			Coins:   totalCoins,
		}},
		Outputs: outputs,
	}

	if _, err := p.bankMsgServer.MultiSend(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitMultiSendEvent(evm, contract.Caller(), totalCoins.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
