package bank

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	// SendMethodName is the ABI name for the Send transaction.
	SendMethodName = "send"
	// MultiSendMethodName is the ABI name for the MultiSend transaction.
	MultiSendMethodName = "multiSend"
)

// Send sends coins from the caller to a single recipient.
func (p Precompile) Send(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SendArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var amount sdk.Coins
	for _, coin := range input.Amount {
		if coin.Amount.Sign() <= 0 {
			return nil, fmt.Errorf("send %s amount is %s, need to greater than 0", coin.Denom, coin.Amount.String())
		}
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
	var input MultiSendArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	if len(input.Outputs) < 1 {
		return nil, fmt.Errorf("the number of outputs is %v, need to greater than 1", len(input.Outputs))
	}

	var totalCoins sdk.Coins
	var outputs []banktypes.Output
	for _, output := range input.Outputs {
		var coins sdk.Coins
		for _, coin := range output.Amount {
			if coin.Amount.Sign() <= 0 {
				return nil, fmt.Errorf("multiSend %s amount is %s, need to greater than 0", coin.Denom, coin.Amount.String())
			}
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
