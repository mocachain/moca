package virtualgroup

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	virtualgroupkeeper "github.com/mocachain/moca/v2/x/virtualgroup/keeper"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/x/evm/types"
)

type Contract struct {
	virtualGroupKeeper virtualgroupkeeper.Keeper
}

// NewPrecompiledContract builds a context-free static precompile instance.
// cosmos/evm v0.6.0 registers precompiles once (WithStaticPrecompiles) rather
// than rebuilding them per-tx, so the sdk.Context is no longer bound at
// construction; Run pulls the live context from the EVM StateDB instead.
func NewPrecompiledContract(virtualGroupKeeper virtualgroupkeeper.Keeper) *Contract {
	return &Contract{
		virtualGroupKeeper: virtualGroupKeeper,
	}
}

func (c *Contract) Address() common.Address {
	return virtualGroupAddress
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// cosmos/evm static precompiles are built once, so the live SDK context is
	// sourced from the EVM StateDB's cache context per call (not bound at
	// construction). We branch a writable cache off it and only commit on
	// success; the StateDB flushes that cache when the EVM tx commits.
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return types.PackRetError("virtualgroup precompile must run within the cosmos/evm StateDB")
	}
	cacheCtx, err := stateDB.GetCacheContext()
	if err != nil {
		return types.PackRetError(err.Error())
	}
	ctx, commit := cacheCtx.CacheContext()
	snapshot := evm.StateDB.Snapshot()

	method, err := GetMethodByID(contract.Input)
	if err == nil {
		switch method.Name {
		case CreateGlobalVirtualGroupMethodName:
			ret, err = c.CreateGlobalVirtualGroup(ctx, evm, contract, readonly)
		case DeleteGlobalVirtualGroupMethodName:
			ret, err = c.DeleteGlobalVirtualGroup(ctx, evm, contract, readonly)
		case SwapOutMethodName:
			ret, err = c.SwapOut(ctx, evm, contract, readonly)
		case CompleteSwapOutMethodName:
			ret, err = c.CompleteSwapOut(ctx, evm, contract, readonly)
		case SPExitMethodName:
			ret, err = c.SPExit(ctx, evm, contract, readonly)
		case CompleteSPExitMethodName:
			ret, err = c.CompleteSPExit(ctx, evm, contract, readonly)
		case DepositMethodName:
			ret, err = c.Deposit(ctx, evm, contract, readonly)
		case ReserveSwapInMethodName:
			ret, err = c.ReserveSwapIn(ctx, evm, contract, readonly)
		case CompleteSwapInMethodName:
			ret, err = c.CompleteSwapIn(ctx, evm, contract, readonly)
		case CancelSwapInMethodName:
			ret, err = c.CancelSwapIn(ctx, evm, contract, readonly)
		case GlobalVirtualGroupFamiliesMethodName:
			ret, err = c.GlobalVirtualGroupFamilies(ctx, evm, contract, readonly)
		case GlobalVirtualGroupFamilyMethodName:
			ret, err = c.GlobalVirtualGroupFamily(ctx, evm, contract, readonly)
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

// calculateSwapOutGas calculates gas cost based on number of GVG IDs
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

	// Calculate dynamic gas: base + per_gvg * num_gvgs
	numGvgIds := uint64(len(args.GvgIds))
	if numGvgIds > MaxSwapOutGvgIds {
		numGvgIds = MaxSwapOutGvgIds
	}

	return SwapOutBaseGas + (numGvgIds * SwapOutPerGvgIdGas)
}

// calculateCompleteSwapOutGas calculates gas cost based on number of GVG IDs
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

	// Calculate dynamic gas: base + per_gvg * num_gvgs
	numGvgIds := uint64(len(args.GvgIds))
	if numGvgIds > MaxCompleteSwapOutGvgIds {
		numGvgIds = MaxCompleteSwapOutGvgIds
	}

	return CompleteSwapOutBaseGas + (numGvgIds * CompleteSwapOutPerGvgIdGas)
}
