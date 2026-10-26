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

	coins := make(sdk.Coins, 0, len(input.Amount))
	for _, coin := range input.Amount {
		if coin.Amount.Sign() <= 0 {
			return nil, fmt.Errorf("send %s amount is %s, need to greater than 0", coin.Denom, coin.Amount.String())
		}
		coins = append(coins, sdk.Coin{Denom: coin.Denom, Amount: math.NewIntFromBigInt(coin.Amount)})
	}
	amount := sdk.Coins{}.Add(coins.Sort()...)

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

	var allCoins sdk.Coins
	outputs := make([]banktypes.Output, 0, len(input.Outputs))
	for _, output := range input.Outputs {
		coins := make(sdk.Coins, 0, len(output.Amount))
		for _, coin := range output.Amount {
			if coin.Amount.Sign() <= 0 {
				return nil, fmt.Errorf("multiSend %s amount is %s, need to greater than 0", coin.Denom, coin.Amount.String())
			}
			coins = append(coins, sdk.Coin{Denom: coin.Denom, Amount: math.NewIntFromBigInt(coin.Amount)})
		}
		coins = sdk.Coins{}.Add(coins.Sort()...)

		outputs = append(outputs, banktypes.Output{
			Address: output.ToAddress.String(),
			Coins:   coins,
		})

		allCoins = append(allCoins, coins...)
	}
	totalCoins := sdk.Coins{}.Add(allCoins.Sort()...)

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
