package erc20

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	erc20keeper "github.com/evmos/evmos/v12/x/erc20/keeper"
	"github.com/evmos/evmos/v12/x/evm/types"
)

// Contract is the precompiled contract implementation for erc20 related methods
type Contract struct {
	ctx         sdk.Context
	erc20Keeper erc20keeper.Keeper
}

// NewPrecompiledContract creates a new instance of the erc20 precompiled contract
func NewPrecompiledContract(ctx sdk.Context, erc20Keeper erc20keeper.Keeper) *Contract {
	return &Contract{
		ctx:         ctx,
		erc20Keeper: erc20Keeper,
	}
}

// Address returns the address of the erc20 precompiled contract
func (c *Contract) Address() common.Address {
	return erc20Address
}

// RequiredGas calculates the contract gas use
func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case ConvertCoinMethodName:
		return ConvertCoinGas
	case ConvertERC20MethodName:
		return ConvertERC20Gas
	case TokenPairsMethodName:
		return TokenPairsGas
	case TokenPairMethodName:
		return TokenPairGas
	case ParamsMethodName:
		return ParamsGas
	default:
		return 0
	}
}

// Run executes the precompiled contract methods
func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	ctx, commit := c.ctx.CacheContext()
	snapshot := evm.StateDB.Snapshot()

	method, err := GetMethodByID(contract.Input)
	if err == nil {
		// parse input
		switch method.Name {
		case ConvertCoinMethodName:
			ret, err = c.ConvertCoin(ctx, evm, contract, readonly)
		case ConvertERC20MethodName:
			ret, err = c.ConvertERC20(ctx, evm, contract, readonly)
		case TokenPairsMethodName:
			ret, err = c.TokenPairs(ctx, evm, contract, readonly)
		case TokenPairMethodName:
			ret, err = c.TokenPair(ctx, evm, contract, readonly)
		case ParamsMethodName:
			ret, err = c.Params(ctx, evm, contract, readonly)
		default:
			err = fmt.Errorf("method %s is not handled", method.Name)
		}
	}

	if err != nil {
		// revert evm state
		evm.StateDB.RevertToSnapshot(snapshot)
		return types.PackRetError(err.Error())
	}

	// commit and append events
	commit()
	return ret, nil
}

// AddLog adds a new log to the state
func (c *Contract) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, newTopic, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     c.Address(),
		Topics:      newTopic,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}
