package gov

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
	"github.com/mocachain/moca/v2/utils"
)

const (
	// LegacySubmitProposalMethod is the ABI name for the v1beta1 LegacySubmitProposal transaction.
	LegacySubmitProposalMethod = "legacySubmitProposal"
	// SubmitProposalMethod is the ABI name for the SubmitProposal transaction.
	SubmitProposalMethod = "submitProposal"
	// VoteMethod is the ABI name for the Vote transaction.
	VoteMethod = "vote"
	// VoteWeightedMethod is the ABI name for the VoteWeighted transaction.
	VoteWeightedMethod = "voteWeighted"
	// DepositMethod is the ABI name for the Deposit transaction.
	DepositMethod = "deposit0"
	// CancelProposalMethod is the ABI name for the CancelProposal transaction.
	CancelProposalMethod = "cancelProposal"
)

// LegacySubmitProposal submits a v1beta1 text proposal from the caller.
func (p Precompile) LegacySubmitProposal(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input LegacySubmitProposalArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var amount sdk.Coins
	for _, deposit := range input.InitialDeposit {
		if deposit.Amount.Sign() > 0 {
			amount = amount.Add(sdk.Coin{Denom: deposit.Denom, Amount: math.NewIntFromBigInt(deposit.Amount)})
		}
	}

	content, _ := govv1beta1.ContentFromProposalType(input.Title, input.Description, govv1beta1.ProposalTypeText)
	msg, err := govv1beta1.NewMsgSubmitProposal(content, amount, contract.Caller().Bytes())
	if err != nil {
		return nil, fmt.Errorf("invalid message: %w", err)
	}

	authority := p.accountKeeper.GetModuleAddress(govtypes.ModuleName).String()
	server := govkeeper.NewLegacyMsgServerImpl(authority, p.govMsgServer)
	res, err := server.SubmitProposal(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitLegacySubmitProposalEvent(evm, contract.Caller(), res.ProposalId); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.ProposalId)
}

// SubmitProposal submits a v1 proposal from the caller. Proposal messages are
// decoded through the application codec, so any registered message type is accepted.
func (p Precompile) SubmitProposal(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SubmitProposalArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var messages []json.RawMessage
	if err := json.Unmarshal([]byte(input.Messages), &messages); err != nil {
		return nil, err
	}

	msgs := make([]sdk.Msg, len(messages))
	for i, message := range messages {
		var msg sdk.Msg
		if err := p.cdc.UnmarshalInterfaceJSON(message, &msg); err != nil {
			return nil, err
		}
		msgs[i] = msg
	}

	var amount sdk.Coins
	for _, deposit := range input.InitialDeposit {
		if deposit.Amount.Sign() > 0 {
			amount = amount.Add(sdk.Coin{Denom: deposit.Denom, Amount: math.NewIntFromBigInt(deposit.Amount)})
		}
	}

	msg, err := govv1.NewMsgSubmitProposal(msgs, amount, sdk.AccAddress(contract.Caller().Bytes()).String(), input.Metadata, input.Title, input.Summary, input.Expedited)
	if err != nil {
		return nil, fmt.Errorf("invalid message: %w", err)
	}

	res, err := p.govMsgServer.SubmitProposal(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitSubmitProposalEvent(evm, contract.Caller(), res.ProposalId); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.ProposalId)
}

// Vote casts the caller's vote on a proposal.
func (p Precompile) Vote(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input VoteArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &govv1.MsgVote{
		ProposalId: input.ProposalId,
		Voter:      sdk.AccAddress(contract.Caller().Bytes()).String(),
		Option:     govv1.VoteOption(input.Option),
		Metadata:   input.Metadata,
	}

	if _, err := p.govMsgServer.Vote(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitVoteEvent(evm, contract.Caller(), input.ProposalId, input.Option); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// VoteWeighted casts the caller's weighted vote on a proposal.
func (p Precompile) VoteWeighted(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input VoteWeightedArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	options := make([]*govv1.WeightedVoteOption, 0, len(input.Options))
	for _, option := range input.Options {
		options = append(options, &govv1.WeightedVoteOption{
			Option: govv1.VoteOption(option.Option),
			Weight: option.Weight,
		})
	}

	msg := &govv1.MsgVoteWeighted{
		ProposalId: input.ProposalId,
		Voter:      sdk.AccAddress(contract.Caller().Bytes()).String(),
		Options:    options,
		Metadata:   input.Metadata,
	}

	if _, err := p.govMsgServer.VoteWeighted(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitVoteWeightedEvent(evm, contract.Caller(), input.ProposalId); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Deposit adds a deposit from the caller to a proposal.
func (p Precompile) Deposit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DepositArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var amount sdk.Coins
	amount = amount.Add(sdk.Coin{Denom: utils.BaseDenom, Amount: math.NewIntFromBigInt(input.Amount)})

	msg := &govv1.MsgDeposit{
		ProposalId: input.ProposalId,
		Depositor:  sdk.AccAddress(contract.Caller().Bytes()).String(),
		Amount:     amount,
	}

	if _, err := p.govMsgServer.Deposit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDepositEvent(evm, contract.Caller(), input.ProposalId); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CancelProposal cancels a proposal submitted by the caller.
func (p Precompile) CancelProposal(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}
	proposalID, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf("invalid proposal id: %v", args[0])
	}

	msg := &govv1.MsgCancelProposal{
		ProposalId: proposalID,
		Proposer:   sdk.AccAddress(contract.Caller().Bytes()).String(),
	}

	if _, err := p.govMsgServer.CancelProposal(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelProposalEvent(evm, contract.Caller(), proposalID); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
