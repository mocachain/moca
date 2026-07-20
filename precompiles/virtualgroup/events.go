package virtualgroup

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// CreateGlobalVirtualGroupEventName is the event emitted on a createGlobalVirtualGroup transaction.
	CreateGlobalVirtualGroupEventName = "CreateGlobalVirtualGroup"
	// DeleteGlobalVirtualGroupEventName is the event emitted on a deleteGlobalVirtualGroup transaction.
	DeleteGlobalVirtualGroupEventName = "DeleteGlobalVirtualGroup"
	// SwapOutEventName is the event emitted on a swapOut transaction.
	SwapOutEventName = "SwapOut"
	// CompleteSwapOutEventName is the event emitted on a completeSwapOut transaction.
	CompleteSwapOutEventName = "CompleteSwapOut"
	// SPExitEventName is the event emitted on a spExit transaction.
	SPExitEventName = "SPExit"
	// CompleteSPExitEventName is the event emitted on a completeSPExit transaction.
	CompleteSPExitEventName = "CompleteSPExit"
	// DepositEventName is the event emitted on a deposit transaction.
	DepositEventName = "Deposit"
	// ReserveSwapInEventName is the event emitted on a reserveSwapIn transaction.
	ReserveSwapInEventName = "ReserveSwapIn"
	// CompleteSwapInEventName is the event emitted on a completeSwapIn transaction.
	CompleteSwapInEventName = "CompleteSwapIn"
	// CancelSwapInEventName is the event emitted on a cancelSwapIn transaction.
	CancelSwapInEventName = "CancelSwapIn"
)

// EmitCreateGlobalVirtualGroupEvent emits the CreateGlobalVirtualGroup event with the
// caller as an indexed topic and the family id as data.
func (p Precompile) EmitCreateGlobalVirtualGroupEvent(evm *vm.EVM, caller common.Address, familyID *big.Int) error {
	return p.AddLog(evm, MustEvent(CreateGlobalVirtualGroupEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())}, familyID)
}

// EmitDeleteGlobalVirtualGroupEvent emits the DeleteGlobalVirtualGroup event with the
// caller as the sole indexed topic.
func (p Precompile) EmitDeleteGlobalVirtualGroupEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DeleteGlobalVirtualGroupEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitSwapOutEvent emits the SwapOut event with the caller as an indexed topic and the
// family id as data.
func (p Precompile) EmitSwapOutEvent(evm *vm.EVM, caller common.Address, familyID *big.Int) error {
	return p.AddLog(evm, MustEvent(SwapOutEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())}, familyID)
}

// EmitCompleteSwapOutEvent emits the CompleteSwapOut event with the caller as an indexed
// topic and the family id as data.
func (p Precompile) EmitCompleteSwapOutEvent(evm *vm.EVM, caller common.Address, familyID *big.Int) error {
	return p.AddLog(evm, MustEvent(CompleteSwapOutEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())}, familyID)
}

// EmitSPExitEvent emits the SPExit event with the caller as the sole indexed topic.
func (p Precompile) EmitSPExitEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(SPExitEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCompleteSPExitEvent emits the CompleteSPExit event. Per #365 the operator topic is
// the caller too (it is the GetSigners() signer), so both indexed topics are the caller.
func (p Precompile) EmitCompleteSPExitEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CompleteSPExitEventName),
		[]common.Hash{
			common.BytesToHash(caller.Bytes()),
			common.BytesToHash(caller.Bytes()),
		})
}

// EmitDepositEvent emits the Deposit event with the caller as the sole indexed topic.
func (p Precompile) EmitDepositEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(DepositEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitReserveSwapInEvent emits the ReserveSwapIn event with the caller as the sole indexed topic.
func (p Precompile) EmitReserveSwapInEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(ReserveSwapInEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCompleteSwapInEvent emits the CompleteSwapIn event with the caller as the sole indexed topic.
func (p Precompile) EmitCompleteSwapInEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CompleteSwapInEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitCancelSwapInEvent emits the CancelSwapIn event with the caller as the sole indexed topic.
func (p Precompile) EmitCancelSwapInEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(CancelSwapInEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// AddLog packs the given event and appends it to the StateDB logs at the precompile address.
func (p Precompile) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, packedTopics, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      packedTopics,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}
