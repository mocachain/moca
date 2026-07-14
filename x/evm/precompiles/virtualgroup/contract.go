package virtualgroup

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	virtualgroupkeeper "github.com/mocachain/moca/v2/x/virtualgroup/keeper"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	base.Precompile

	virtualGroupKeeper virtualgroupkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(virtualGroupKeeper virtualgroupkeeper.Keeper) *Contract {
	return &Contract{
		Precompile:         base.New(virtualGroupAddress, virtualGroupABI),
		virtualGroupKeeper: virtualGroupKeeper,
	}
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case CreateGlobalVirtualGroupMethodName:
		return CreateGlobalVirtualGroupGas
	case DeleteGlobalVirtualGroupMethodName:
		return DeleteGlobalVirtualGroupGas
	case SwapOutMethodName:
		return c.calculateSwapOutGas(input)
	case CompleteSwapOutMethodName:
		return c.calculateCompleteSwapOutGas(input)
	case SPExitMethodName:
		return SPExitGas
	case CompleteSPExitMethodName:
		return CompleteSPExitGas
	case DepositMethodName:
		return DepositGas
	case ReserveSwapInMethodName:
		return ReserveSwapInGas
	case CompleteSwapInMethodName:
		return CompleteSwapInGas
	case CancelSwapInMethodName:
		return CancelSwapInGas
	case GlobalVirtualGroupFamiliesMethodName:
		return GlobalVirtualGroupFamiliesGas
	case GlobalVirtualGroupFamilyMethodName:
		return GlobalVirtualGroupFamilyGas
	default:
		return 0
	}
}

// Run is the precompile entrypoint. The base rejects native value, sets up the
// native cache context / snapshot / gas metering, and reverts on error; the
// per-method business logic runs in Execute.
func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return c.RunPrecompile(evm, contract, readonly, c.Execute)
}

// Execute dispatches the ABI method to its handler. Read-only write protection is
// enforced by the base Dispatch (SetupABI) using IsTransaction.
func (c *Contract) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, _, err := c.Dispatch(contract, readonly, c.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case CreateGlobalVirtualGroupMethodName:
		return c.CreateGlobalVirtualGroup(ctx, evm, contract, readonly)
	case DeleteGlobalVirtualGroupMethodName:
		return c.DeleteGlobalVirtualGroup(ctx, evm, contract, readonly)
	case SwapOutMethodName:
		return c.SwapOut(ctx, evm, contract, readonly)
	case CompleteSwapOutMethodName:
		return c.CompleteSwapOut(ctx, evm, contract, readonly)
	case SPExitMethodName:
		return c.SPExit(ctx, evm, contract, readonly)
	case CompleteSPExitMethodName:
		return c.CompleteSPExit(ctx, evm, contract, readonly)
	case DepositMethodName:
		return c.Deposit(ctx, evm, contract, readonly)
	case ReserveSwapInMethodName:
		return c.ReserveSwapIn(ctx, evm, contract, readonly)
	case CompleteSwapInMethodName:
		return c.CompleteSwapIn(ctx, evm, contract, readonly)
	case CancelSwapInMethodName:
		return c.CancelSwapIn(ctx, evm, contract, readonly)
	case GlobalVirtualGroupFamiliesMethodName:
		return c.GlobalVirtualGroupFamilies(ctx, evm, contract, readonly)
	case GlobalVirtualGroupFamilyMethodName:
		return c.GlobalVirtualGroupFamily(ctx, evm, contract, readonly)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
}

// IsTransaction reports whether a method mutates state (drives read-only write
// protection). A method is a transaction iff its ABI mutability is not view/pure.
func (Contract) IsTransaction(method *abi.Method) bool {
	return !method.IsConstant()
}

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

func (c *Contract) calculateSwapOutGas(input []byte) uint64 {
	if len(input) < 4 {
		return SwapOutBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return SwapOutBaseGas
	}

	var args SwapOutArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return SwapOutBaseGas
	}

	numGvgIds := uint64(len(args.GvgIds))
	if numGvgIds > MaxSwapOutGvgIds {
		numGvgIds = MaxSwapOutGvgIds
	}

	return SwapOutBaseGas + (numGvgIds * SwapOutPerGvgIdGas)
}

func (c *Contract) calculateCompleteSwapOutGas(input []byte) uint64 {
	if len(input) < 4 {
		return CompleteSwapOutBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return CompleteSwapOutBaseGas
	}

	var args CompleteSwapOutArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return CompleteSwapOutBaseGas
	}

	numGvgIds := uint64(len(args.GvgIds))
	if numGvgIds > MaxCompleteSwapOutGvgIds {
		numGvgIds = MaxCompleteSwapOutGvgIds
	}

	return CompleteSwapOutBaseGas + (numGvgIds * CompleteSwapOutPerGvgIdGas)
}
