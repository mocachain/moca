package gov

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	accountkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/x/evm/precompiles/authz"
	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	base.Precompile

	govKeeper     govkeeper.Keeper
	accountKeeper accountkeeper.AccountKeeper
	queryServer   v1.QueryServer
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(govKeeper govkeeper.Keeper, accountKeeper accountkeeper.AccountKeeper) *Contract {
	return &Contract{
		Precompile:    base.New(govAddress, govABI),
		govKeeper:     govKeeper,
		accountKeeper: accountKeeper,
		queryServer:   govkeeper.NewQueryServer(&govKeeper),
	}
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case LegacySubmitProposalMethodName:
		return LegacySubmitProposalGas
	case SubmitProposalMethodName:
		return SubmitProposalBaseGas
	case VoteMethodName:
		return VoteGas
	case VoteWeightedMethodName:
		return VoteWeightedGas
	case DepositMethodName:
		return DepositGas
	case ProposalMethodName:
		return ProposalGas
	case ProposalsMethodName:
		return ProposalsGas
	case VoteQueryMethodName:
		return VoteQueryGas
	case VotesMethodName:
		return VotesGas
	case DepositQueryMethodName:
		return DepositQueryGas
	case DepositsMethodName:
		return DepositsGas
	case TallyResultMethodName:
		return TallyResultGas
	case ParamsMethodName:
		return ParamsGas
	default:
		return 0
	}
}

// Run is the precompile entrypoint. The base rejects native value, sets up the
// native cache context / snapshot / gas metering, and reverts on error; the
// per-method business logic runs in Execute.
func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return c.RunPrecompile(evm, contract, readonly, c.Execute)
}

// Execute dispatches the ABI method to its handler. Read-only write protection is
// enforced by the base Dispatch (SetupABI) using IsTransaction.
func (c *Contract) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, _, err := c.Dispatch(contract, readonly, c.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case LegacySubmitProposalMethodName:
		return c.LegacySubmitProposal(ctx, evm, contract, readonly)
	case SubmitProposalMethodName:
		return c.SubmitProposal(ctx, evm, contract, readonly)
	case VoteMethodName:
		return c.Vote(ctx, evm, contract, readonly)
	case VoteWeightedMethodName:
		return c.VoteWeighted(ctx, evm, contract, readonly)
	case DepositMethodName:
		return c.Deposit(ctx, evm, contract, readonly)
	case ProposalMethodName:
		return c.Proposal(ctx, evm, contract, readonly)
	case ProposalsMethodName:
		return c.Proposals(ctx, evm, contract, readonly)
	case VoteQueryMethodName:
		return c.VoteQuery(ctx, evm, contract, readonly)
	case VotesMethodName:
		return c.Votes(ctx, evm, contract, readonly)
	case DepositQueryMethodName:
		return c.DepositQuery(ctx, evm, contract, readonly)
	case DepositsMethodName:
		return c.Deposits(ctx, evm, contract, readonly)
	case TallyResultMethodName:
		return c.TallyResult(ctx, evm, contract, readonly)
	case ParamsMethodName:
		return c.Params(ctx, evm, contract, readonly)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
}

// IsTransaction reports whether a method mutates state (drives read-only write
// protection). A method is a transaction iff its ABI mutability is not view/pure.
func (Contract) IsTransaction(method *abi.Method) bool {
	return !method.IsConstant()
}

func (c *Contract) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, newTopic, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     c.Address(),
		Topics:      newTopic,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}

func (c *Contract) calculateSubmitProposalGas(input []byte) uint64 {
	if len(input) < 4 {
		return SubmitProposalBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return SubmitProposalBaseGas
	}

	var args SubmitProposalArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return SubmitProposalBaseGas
	}

	var messages []json.RawMessage
	err = json.Unmarshal([]byte(args.Messages), &messages)
	if err != nil {
		return SubmitProposalBaseGas
	}

	numMsgs := uint64(len(messages))
	if numMsgs > MaxSubmitProposalMsgs {
		numMsgs = MaxSubmitProposalMsgs
	}

	// Convert []json.RawMessage to []string to reuse authz.CalcPerMsgBytes
	msgStrings := make([]string, len(messages))
	for i, msg := range messages {
		msgStrings[i] = string(msg)
	}
	payloadSize := uint64(authz.CalcPerMsgBytes(msgStrings))
	if payloadSize > MaxSubmitProposalPayloadBytes {
		payloadSize = MaxSubmitProposalPayloadBytes
	}

	return SubmitProposalBaseGas + (numMsgs * SubmitProposalPerMsgGas) + (payloadSize * SubmitProposalPerByteGas)
}
