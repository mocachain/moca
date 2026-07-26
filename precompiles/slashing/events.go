package slashing

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// EventTypeUnjail is the event emitted on an Unjail transaction.
	EventTypeUnjail = "Unjail"
)

// EmitUnjailEvent emits the Unjail event with the caller (validator) as an indexed topic.
func (p Precompile) EmitUnjailEvent(evm *vm.EVM, validator common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeUnjail), []common.Hash{
		common.BytesToHash(validator.Bytes()),
	})
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
