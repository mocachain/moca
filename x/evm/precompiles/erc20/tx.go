package erc20

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	erc20types "github.com/evmos/evmos/v12/x/erc20/types"
	"github.com/evmos/evmos/v12/x/evm/types"
)

const (
	ConvertCoinGas  = 60_000
	ConvertERC20Gas = 80_000

	ConvertCoinMethodName  = "convertCoin"
	ConvertERC20MethodName = "convertERC20"

	ConvertCoinEventName  = "ConvertCoin"
	ConvertERC20EventName = "ConvertERC20"
)

func (c *Contract) ConvertCoin(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, types.ErrReadOnly
	}

	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	method := MustMethod(ConvertCoinMethodName)

	var args ConvertCoinArgs
	err := types.ParseMethodArgs(method, &args, contract.Input[4:])
	if err != nil {
		return nil, err
	}

	if err := args.Validate(); err != nil {
		return nil, err
	}

	coin := sdk.Coin{
		Denom:  args.Coin.Denom,
		Amount: math.NewIntFromBigInt(args.Coin.Amount),
	}

	msg := &erc20types.MsgConvertCoin{
		Coin:     coin,
		Receiver: args.Receiver.String(),
		Sender:   sdk.AccAddress(contract.Caller().Bytes()).String(),
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	_, err = c.erc20Keeper.ConvertCoin(ctx, msg)
	if err != nil {
		return nil, err
	}

	// add convert coin log
	if err := c.AddLog(
		evm,
		MustEvent(ConvertCoinEventName),
		[]common.Hash{
			common.BytesToHash(args.Receiver.Bytes()),
			common.BytesToHash(contract.Caller().Bytes()),
		},
		coin.Denom,
		coin.Amount.BigInt(),
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (c *Contract) ConvertERC20(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, types.ErrReadOnly
	}

	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	method := MustMethod(ConvertERC20MethodName)

	var args ConvertERC20Args
	err := types.ParseMethodArgs(method, &args, contract.Input[4:])
	if err != nil {
		return nil, err
	}

	if err := args.Validate(); err != nil {
		return nil, err
	}

	msg := &erc20types.MsgConvertERC20{
		ContractAddress: args.ContractAddress.String(),
		Amount:          math.NewIntFromBigInt(args.Amount),
		Receiver:        sdk.AccAddress(args.Receiver.Bytes()).String(),
		Sender:          contract.Caller().String(),
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	_, err = c.erc20Keeper.ConvertERC20(ctx, msg)
	if err != nil {
		return nil, err
	}

	// add convert erc20 log
	if err := c.AddLog(
		evm,
		MustEvent(ConvertERC20EventName),
		[]common.Hash{
			common.BytesToHash(args.ContractAddress.Bytes()),
			common.BytesToHash(args.Receiver.Bytes()),
			common.BytesToHash(contract.Caller().Bytes()),
		},
		args.Amount,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
