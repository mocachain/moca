package gov

import (
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	accountkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/authz"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	cmn.Precompile
	govKeeper     govkeeper.Keeper
	accountKeeper accountkeeper.AccountKeeper
	queryServer   v1.QueryServer
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(govKeeper govkeeper.Keeper, accountKeeper accountkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper) *Contract {
	return &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      govAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Route dispatch through cosmos/evm's native-action protocol so keeper coin
	// moves stay reconciled with the EVM StateDB: FlushToCacheCtx + the
	// BalanceHandler translate the bank coin_spent/coin_received events into
	// StateDB SubBalance/AddBalance, the multistore is snapshotted for atomic
	// revert (AddPrecompileFn), and store gas is metered against contract.Gas.
	// Without this, StateDB.Commit's balance reconciliation would mint a debited
	// amount back to a 7702-dirtied caller (native-token inflation).
	return c.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return c.execute(ctx, evm, contract, readonly)
	})
}

func (c *Contract) execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, err := GetMethodByID(contract.Input)
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
