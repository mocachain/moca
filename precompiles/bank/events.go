package bank

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// SendEventName is the event emitted on a Send transaction.
	SendEventName = "Send"
	// MultiSendEventName is the event emitted on a MultiSend transaction.
	MultiSendEventName = "MultiSend"
)

// EmitSendEvent emits the Send event with the sender and recipient as topics and the
// amount as data.
func (p Precompile) EmitSendEvent(evm *vm.EVM, from, to common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(SendEventName),
		[]common.Hash{common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())}, amount)
}

// EmitMultiSendEvent emits the MultiSend event with the sender as topic and the total
// amount as data.
func (p Precompile) EmitMultiSendEvent(evm *vm.EVM, from common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(MultiSendEventName),
		[]common.Hash{common.BytesToHash(from.Bytes())}, amount)
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
