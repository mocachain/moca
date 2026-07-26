package staking

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	EditValidatorEventName             = "EditValidator"
	DelegateEventName                  = "Delegate"
	UndelegateEventName                = "Undelegate"
	RedelegateEventName                = "Redelegate"
	CancelUnbondingDelegationEventName = "CancelUnbondingDelegation"
)

// EmitEditValidatorEvent emits the EditValidator event with the caller as an indexed
// topic and the commission rate and min self delegation as data.
func (p Precompile) EmitEditValidatorEvent(evm *vm.EVM, caller common.Address, commissionRate, minSelfDelegation *big.Int) error {
	return p.AddLog(evm, MustEvent(EditValidatorEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		commissionRate, minSelfDelegation)
}

// EmitDelegateEvent emits the Delegate event with the caller and validator as indexed
// topics and the amount as data.
func (p Precompile) EmitDelegateEvent(evm *vm.EVM, caller common.Address, validator sdk.ValAddress, amount *big.Int) error {
	return p.AddLog(evm, MustEvent(DelegateEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(validator.Bytes())},
		amount)
}

// EmitUndelegateEvent emits the Undelegate event with the caller and validator as
// indexed topics and the amount and completion time as data.
func (p Precompile) EmitUndelegateEvent(evm *vm.EVM, caller common.Address, validator sdk.ValAddress, amount, completionTime *big.Int) error {
	return p.AddLog(evm, MustEvent(UndelegateEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(validator.Bytes())},
		amount, completionTime)
}

// EmitRedelegateEvent emits the Redelegate event with the caller, src validator and
// dst validator as indexed topics and the amount and completion time as data.
func (p Precompile) EmitRedelegateEvent(evm *vm.EVM, caller common.Address, srcValidator, dstValidator sdk.ValAddress, amount, completionTime *big.Int) error {
	return p.AddLog(evm, MustEvent(RedelegateEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(srcValidator.Bytes()), common.BytesToHash(dstValidator.Bytes())},
		amount, completionTime)
}

// EmitCancelUnbondingDelegationEvent emits the CancelUnbondingDelegation event with the
// caller and validator as indexed topics and the amount and creation height as data.
func (p Precompile) EmitCancelUnbondingDelegationEvent(evm *vm.EVM, caller common.Address, validator sdk.ValAddress, amount, creationHeight *big.Int) error {
	return p.AddLog(evm, MustEvent(CancelUnbondingDelegationEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(validator.Bytes())},
		amount, creationHeight)
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
