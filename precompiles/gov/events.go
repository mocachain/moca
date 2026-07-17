package gov

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// EventTypeLegacySubmitProposal is the event emitted on a LegacySubmitProposal transaction.
	EventTypeLegacySubmitProposal = "LegacySubmitProposal"
	// EventTypeSubmitProposal is the event emitted on a SubmitProposal transaction.
	EventTypeSubmitProposal = "SubmitProposal"
	// EventTypeVote is the event emitted on a Vote transaction.
	EventTypeVote = "Vote"
	// EventTypeVoteWeighted is the event emitted on a VoteWeighted transaction.
	EventTypeVoteWeighted = "VoteWeighted"
	// EventTypeDeposit is the event emitted on a Deposit transaction.
	EventTypeDeposit = "Deposit"
	// EventTypeCancelProposal is the event emitted on a CancelProposal transaction.
	EventTypeCancelProposal = "CancelProposal"
)

// EmitLegacySubmitProposalEvent emits the LegacySubmitProposal event.
func (p Precompile) EmitLegacySubmitProposalEvent(evm *vm.EVM, proposer common.Address, proposalID uint64) error {
	return p.AddLog(evm, MustEvent(EventTypeLegacySubmitProposal),
		[]common.Hash{common.BytesToHash(proposer.Bytes())}, proposalID)
}

// EmitSubmitProposalEvent emits the SubmitProposal event.
func (p Precompile) EmitSubmitProposalEvent(evm *vm.EVM, proposer common.Address, proposalID uint64) error {
	return p.AddLog(evm, MustEvent(EventTypeSubmitProposal),
		[]common.Hash{common.BytesToHash(proposer.Bytes())}, proposalID)
}

// EmitVoteEvent emits the Vote event with the proposal id and option as data.
func (p Precompile) EmitVoteEvent(evm *vm.EVM, voter common.Address, proposalID uint64, option uint8) error {
	return p.AddLog(evm, MustEvent(EventTypeVote),
		[]common.Hash{common.BytesToHash(voter.Bytes())}, proposalID, option)
}

// EmitVoteWeightedEvent emits the VoteWeighted event.
func (p Precompile) EmitVoteWeightedEvent(evm *vm.EVM, voter common.Address, proposalID uint64) error {
	return p.AddLog(evm, MustEvent(EventTypeVoteWeighted),
		[]common.Hash{common.BytesToHash(voter.Bytes())}, proposalID)
}

// EmitDepositEvent emits the Deposit event.
func (p Precompile) EmitDepositEvent(evm *vm.EVM, depositor common.Address, proposalID uint64) error {
	return p.AddLog(evm, MustEvent(EventTypeDeposit),
		[]common.Hash{common.BytesToHash(depositor.Bytes())}, proposalID)
}

// EmitCancelProposalEvent emits the CancelProposal event.
func (p Precompile) EmitCancelProposalEvent(evm *vm.EVM, proposer common.Address, proposalID uint64) error {
	return p.AddLog(evm, MustEvent(EventTypeCancelProposal),
		[]common.Hash{common.BytesToHash(proposer.Bytes())}, proposalID)
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
