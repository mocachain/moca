package authz

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// GrantEventName is the event emitted on a Grant transaction.
	GrantEventName = "Grant"
	// RevokeEventName is the event emitted on a Revoke transaction.
	RevokeEventName = "Revoke"
	// ExecEventName is the event emitted on an Exec transaction.
	ExecEventName = "Exec"
)

// EmitGrantEvent emits the Grant event with the granter and grantee as topics and the
// authorization type as data.
func (p Precompile) EmitGrantEvent(evm *vm.EVM, granter, grantee common.Address, authzType string) error {
	return p.AddLog(evm, MustEvent(GrantEventName),
		[]common.Hash{common.BytesToHash(granter.Bytes()), common.BytesToHash(grantee.Bytes())}, authzType)
}

// EmitRevokeEvent emits the Revoke event with the granter and grantee as topics and the
// message type url as data.
func (p Precompile) EmitRevokeEvent(evm *vm.EVM, granter, grantee common.Address, msgTypeURL string) error {
	return p.AddLog(evm, MustEvent(RevokeEventName),
		[]common.Hash{common.BytesToHash(granter.Bytes()), common.BytesToHash(grantee.Bytes())}, msgTypeURL)
}

// EmitExecEvent emits the Exec event with the grantee as topic.
func (p Precompile) EmitExecEvent(evm *vm.EVM, grantee common.Address) error {
	return p.AddLog(evm, MustEvent(ExecEventName),
		[]common.Hash{common.BytesToHash(grantee.Bytes())})
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
