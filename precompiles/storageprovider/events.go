package storageprovider

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// UpdateSPPriceEventName is the event emitted on an UpdateSPPrice transaction.
	UpdateSPPriceEventName = "UpdateSPPrice"
)

// EmitUpdateSPPriceEvent emits the UpdateSPPrice event with the storage provider as the
// sole indexed topic and no data.
func (p Precompile) EmitUpdateSPPriceEvent(evm *vm.EVM, storageProvider common.Address) error {
	return p.AddLog(evm, MustEvent(UpdateSPPriceEventName),
		[]common.Hash{common.BytesToHash(storageProvider.Bytes())})
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
