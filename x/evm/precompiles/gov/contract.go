package gov

import (
	"encoding/json"
	"fmt"

	accountkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/x/evm/precompiles/authz"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	govKeeper     govkeeper.Keeper
	accountKeeper accountkeeper.AccountKeeper
	queryServer   v1.QueryServer
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(govKeeper govkeeper.Keeper, accountKeeper accountkeeper.AccountKeeper) *Contract {
	return &Contract{
		govKeeper:     govKeeper,
		accountKeeper: accountKeeper,
		queryServer:   govkeeper.NewQueryServer(&govKeeper),
	}
}

func (c *Contract) Address() common.Address {
	return govAddress
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if err = types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Pull the live SDK context from the EVM StateDB (static precompiles don't bind ctx at construction).
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return types.PackRetError("gov precompile must run within the cosmos/evm StateDB")
	}
	cacheCtx, err := stateDB.GetCacheContext()
	if err != nil {
		return types.PackRetError(err.Error())
	}
	ctx, commit := cacheCtx.CacheContext()
	snapshot := evm.StateDB.Snapshot()

	method, err := GetMethodByID(contract.Input)
	if err == nil {
		switch method.Name {
		case LegacySubmitProposalMethodName:
			ret, err = c.LegacySubmitProposal(ctx, evm, contract, readonly)
		case SubmitProposalMethodName:
			ret, err = c.SubmitProposal(ctx, evm, contract, readonly)
		case VoteMethodName:
			ret, err = c.Vote(ctx, evm, contract, readonly)
		case VoteWeightedMethodName:
			ret, err = c.VoteWeighted(ctx, evm, contract, readonly)
		case DepositMethodName:
			ret, err = c.Deposit(ctx, evm, contract, readonly)
		case ProposalMethodName:
			ret, err = c.Proposal(ctx, evm, contract, readonly)
		case ProposalsMethodName:
			ret, err = c.Proposals(ctx, evm, contract, readonly)
		case VoteQueryMethodName:
			ret, err = c.VoteQuery(ctx, evm, contract, readonly)
		case VotesMethodName:
			ret, err = c.Votes(ctx, evm, contract, readonly)
		case DepositQueryMethodName:
			ret, err = c.DepositQuery(ctx, evm, contract, readonly)
		case DepositsMethodName:
			ret, err = c.Deposits(ctx, evm, contract, readonly)
		case TallyResultMethodName:
			ret, err = c.TallyResult(ctx, evm, contract, readonly)
		case ParamsMethodName:
			ret, err = c.Params(ctx, evm, contract, readonly)
		default:
			err = fmt.Errorf("method %s is not handled", method.Name)
		}
	}

	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		return types.PackRetError(err.Error())
	}

	commit()
	return ret, nil
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
