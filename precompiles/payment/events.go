package payment

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// EventTypeCreatePaymentAccount is the event emitted on a CreatePaymentAccount transaction.
	EventTypeCreatePaymentAccount = "CreatePaymentAccount"
	// EventTypeDeposit is the event emitted on a Deposit transaction.
	EventTypeDeposit = "Deposit"
	// EventTypeDisableRefund is the event emitted on a DisableRefund transaction.
	EventTypeDisableRefund = "DisableRefund"
	// EventTypeWithdraw is the event emitted on a Withdraw transaction.
	EventTypeWithdraw = "Withdraw"
)

// EmitCreatePaymentAccountEvent emits the CreatePaymentAccount event with the caller as the sole topic.
func (p Precompile) EmitCreatePaymentAccountEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeCreatePaymentAccount),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDepositEvent emits the Deposit event with the caller as the sole topic.
func (p Precompile) EmitDepositEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeDeposit),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitDisableRefundEvent emits the DisableRefund event with the caller as the sole topic.
func (p Precompile) EmitDisableRefundEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeDisableRefund),
		[]common.Hash{common.BytesToHash(caller.Bytes())})
}

// EmitWithdrawEvent emits the Withdraw event with the caller as the sole topic.
func (p Precompile) EmitWithdrawEvent(evm *vm.EVM, caller common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeWithdraw),
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
