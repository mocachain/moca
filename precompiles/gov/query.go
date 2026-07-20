package gov

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/utils"
)

const (
	// ProposalMethod is the ABI name for the Proposal query.
	ProposalMethod = "proposal"
	// ProposalsMethod is the ABI name for the Proposals query.
	ProposalsMethod = "proposals"
	// VoteQueryMethod is the ABI name for the Vote query.
	VoteQueryMethod = "vote0"
	// VotesMethod is the ABI name for the Votes query.
	VotesMethod = "votes"
	// DepositQueryMethod is the ABI name for the Deposit query.
	DepositQueryMethod = "deposit"
	// DepositsMethod is the ABI name for the Deposits query.
	DepositsMethod = "deposits"
	// TallyResultMethod is the ABI name for the TallyResult query.
	TallyResultMethod = "tallyResult"
	// ParamsMethod is the ABI name for the Params query.
	ParamsMethod = "params"
	// ConstitutionMethod is the ABI name for the Constitution query.
	ConstitutionMethod = "constitution"
)

// Proposal queries a proposal by id.
func (p Precompile) Proposal(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ProposalArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Proposal(ctx, &govv1.QueryProposalRequest{ProposalId: input.ProposalID})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(p.outputsProposal(*res.Proposal))
}

// Proposals queries all proposals filtered by status, voter and depositor.
func (p Precompile) Proposals(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ProposalsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	voter := ""
	if input.Voter != (common.Address{}) {
		voter = sdk.AccAddress(input.Voter.Bytes()).String()
	}
	depositor := ""
	if input.Depositor != (common.Address{}) {
		depositor = sdk.AccAddress(input.Depositor.Bytes()).String()
	}

	res, err := p.govQuerier.Proposals(ctx, &govv1.QueryProposalsRequest{
		ProposalStatus: govv1.ProposalStatus(input.Status),
		Voter:          voter,
		Depositor:      depositor,
		Pagination:     pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	proposals := make([]Proposal, 0, len(res.Proposals))
	for _, proposal := range res.Proposals {
		proposals = append(proposals, p.outputsProposal(*proposal))
	}

	return method.Outputs.Pack(proposals, pageResponse(res.Pagination))
}

// VoteQuery queries a single vote by proposal id and voter.
func (p Precompile) VoteQuery(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input VoteQueryArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Vote(ctx, &govv1.QueryVoteRequest{
		ProposalId: input.ProposalID,
		Voter:      sdk.AccAddress(input.Voter.Bytes()).String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(OutputsVote(*res.Vote))
}

// Votes queries all votes of a proposal.
func (p Precompile) Votes(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input VotesArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Votes(ctx, &govv1.QueryVotesRequest{
		ProposalId: input.ProposalID,
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	votes := make([]VoteData, 0, len(res.Votes))
	for _, vote := range res.Votes {
		votes = append(votes, OutputsVote(*vote))
	}

	return method.Outputs.Pack(votes, pageResponse(res.Pagination))
}

// DepositQuery queries a single deposit by proposal id and depositor.
func (p Precompile) DepositQuery(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DepositQueryArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposit(ctx, &govv1.QueryDepositRequest{
		ProposalId: input.ProposalID,
		Depositor:  sdk.AccAddress(input.Depositor.Bytes()).String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(OutputsDeposit(*res.Deposit))
}

// Deposits queries all deposits of a proposal.
func (p Precompile) Deposits(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DepositsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposits(ctx, &govv1.QueryDepositsRequest{
		ProposalId: input.ProposalID,
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	deposits := make([]DepositData, 0, len(res.Deposits))
	for _, deposit := range res.Deposits {
		deposits = append(deposits, OutputsDeposit(*deposit))
	}

	return method.Outputs.Pack(deposits, pageResponse(res.Pagination))
}

// TallyResult queries the tally of a proposal.
func (p Precompile) TallyResult(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ProposalArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.govQuerier.TallyResult(ctx, &govv1.QueryTallyResultRequest{ProposalId: input.ProposalID})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(TallyResult(*res.Tally))
}

// Params queries the gov deposit, voting and tally parameters.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	depositRes, err := p.govQuerier.Params(ctx, &govv1.QueryParamsRequest{ParamsType: govv1.ParamDeposit})
	if err != nil {
		return nil, err
	}
	votingRes, err := p.govQuerier.Params(ctx, &govv1.QueryParamsRequest{ParamsType: govv1.ParamVoting})
	if err != nil {
		return nil, err
	}
	tallyRes, err := p.govQuerier.Params(ctx, &govv1.QueryParamsRequest{ParamsType: govv1.ParamTallying})
	if err != nil {
		return nil, err
	}

	params := Params{
		MinDeposit: []Coin{
			{
				Denom:  depositRes.Params.MinDeposit[0].Denom,
				Amount: depositRes.Params.MinDeposit[0].Amount.BigInt(),
			},
		},
		MaxDepositPeriod:           int64(depositRes.Params.MaxDepositPeriod.Seconds()),
		VotingPeriod:               int64(votingRes.Params.VotingPeriod.Seconds()),
		Quorum:                     tallyRes.Params.Quorum,
		Threshold:                  tallyRes.Params.Threshold,
		VetoThreshold:              tallyRes.Params.VetoThreshold,
		MinInitialDepositRatio:     tallyRes.Params.MinInitialDepositRatio,
		BurnProposalDepositPrevote: tallyRes.Params.BurnProposalDepositPrevote,
		BurnVoteQuorum:             tallyRes.Params.BurnVoteQuorum,
		BurnVoteVeto:               tallyRes.Params.BurnVoteVeto,
	}

	return method.Outputs.Pack(params)
}

// Constitution queries the chain constitution.
func (p Precompile) Constitution(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.govQuerier.Constitution(ctx, &govv1.QueryConstitutionRequest{})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.Constitution)
}

// outputsProposal maps a gov v1 Proposal into the ABI tuple, encoding its messages
// through the application codec.
func (p Precompile) outputsProposal(proposal govv1.Proposal) Proposal {
	msgs, err := proposal.GetMsgs()
	if err != nil {
		return Proposal{}
	}

	var messages []string
	for _, msg := range msgs {
		bz, err := p.cdc.MarshalInterfaceJSON(msg)
		if err != nil {
			messages = append(messages, msg.String())
			continue
		}
		messages = append(messages, string(bz))
	}

	var totalDeposit []Coin
	for _, coin := range proposal.TotalDeposit {
		totalDeposit = append(totalDeposit, Coin{Denom: coin.Denom, Amount: coin.Amount.BigInt()})
	}

	var votingStartTime int64
	if proposal.VotingStartTime != nil {
		votingStartTime = proposal.VotingStartTime.Unix()
	}
	var votingEndTime int64
	if proposal.VotingEndTime != nil {
		votingEndTime = proposal.VotingEndTime.Unix()
	}

	return Proposal{
		Id:               proposal.Id,
		Messages:         messages,
		Status:           uint8(proposal.Status),
		FinalTallyResult: TallyResult(*proposal.FinalTallyResult),
		SubmitTime:       proposal.SubmitTime.Unix(),
		DepositEndTime:   proposal.DepositEndTime.Unix(),
		TotalDeposit:     totalDeposit,
		VotingStartTime:  votingStartTime,
		VotingEndTime:    votingEndTime,
		Metadata:         proposal.Metadata,
		Proposer:         common.HexToAddress(proposal.Proposer),
		FailedReason:     proposal.FailedReason,
	}
}

// OutputsVote maps a gov v1 Vote into the ABI tuple with a hex voter address.
func OutputsVote(vote govv1.Vote) VoteData {
	options := make([]WeightedVoteOption, 0, len(vote.Options))
	for _, option := range vote.Options {
		options = append(options, WeightedVoteOption{
			Option: uint8(option.Option),
			Weight: option.Weight,
		})
	}

	return VoteData{
		ProposalId: vote.ProposalId,
		Voter:      utils.AccAddressMustToHexAddress(vote.Voter),
		Options:    options,
		Metadata:   vote.Metadata,
	}
}

// OutputsDeposit maps a gov v1 Deposit into the ABI tuple with a hex depositor address.
func OutputsDeposit(deposit govv1.Deposit) DepositData {
	amount := make([]Coin, 0, len(deposit.Amount))
	for _, coin := range deposit.Amount {
		amount = append(amount, Coin{Denom: coin.Denom, Amount: coin.Amount.BigInt()})
	}

	return DepositData{
		ProposalId: deposit.ProposalId,
		Depositor:  utils.AccAddressMustToHexAddress(deposit.Depositor),
		Amount:     amount,
	}
}
