package gov

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	accountkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the gov precompile. It follows the cosmos/evm precompile layout —
// Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The moca-specific
// surface is the hex (0x) address encoding, the legacy v1beta1 path, moca's method
// set, and the non-payable RejectValue guard. Proposal messages are decoded and
// encoded through the injected application codec.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	govMsgServer  govv1.MsgServer
	govQuerier    govv1.QueryServer
	accountKeeper accountkeeper.AccountKeeper
	cdc           codec.Codec
}

// NewPrecompile creates a new gov Precompile as a vm.PrecompiledContract. The msg
// server and querier are built from the gov keeper at wiring time; the account
// keeper supplies the gov module authority for the legacy proposal path; the codec
// is the application codec used to (un)marshal proposal messages.
func NewPrecompile(
	govMsgServer govv1.MsgServer,
	govQuerier govv1.QueryServer,
	accountKeeper accountkeeper.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
	cdc codec.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      govAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:           govABI,
		govMsgServer:  govMsgServer,
		govQuerier:    govQuerier,
		accountKeeper: accountKeeper,
		cdc:           cdc,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return govAddress
}

// RequiredGas calculates the base gas via the cosmos/evm common flat+per-byte model.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run dispatches the call through cosmos/evm's native-action protocol so keeper
// coin moves stay reconciled with the EVM StateDB. moca precompiles are not
// payable, so any attached value is rejected up front.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}

	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm, contract, readonly)
	})
}

// Execute parses the calldata against the ABI and routes to the matching handler.
func (p Precompile) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte
	switch method.Name {
	// Gov transactions
	case LegacySubmitProposalMethod:
		bz, err = p.LegacySubmitProposal(ctx, evm, contract, method, args)
	case SubmitProposalMethod:
		bz, err = p.SubmitProposal(ctx, evm, contract, method, args)
	case VoteMethod:
		bz, err = p.Vote(ctx, evm, contract, method, args)
	case VoteWeightedMethod:
		bz, err = p.VoteWeighted(ctx, evm, contract, method, args)
	case DepositMethod:
		bz, err = p.Deposit(ctx, evm, contract, method, args)
	case CancelProposalMethod:
		bz, err = p.CancelProposal(ctx, evm, contract, method, args)
	// Gov queries
	case ProposalMethod:
		bz, err = p.Proposal(ctx, method, args)
	case ProposalsMethod:
		bz, err = p.Proposals(ctx, method, args)
	case VoteQueryMethod:
		bz, err = p.VoteQuery(ctx, method, args)
	case VotesMethod:
		bz, err = p.Votes(ctx, method, args)
	case DepositQueryMethod:
		bz, err = p.DepositQuery(ctx, method, args)
	case DepositsMethod:
		bz, err = p.Deposits(ctx, method, args)
	case TallyResultMethod:
		bz, err = p.TallyResult(ctx, method, args)
	case ParamsMethod:
		bz, err = p.Params(ctx, method, args)
	case ConstitutionMethod:
		bz, err = p.Constitution(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case LegacySubmitProposalMethod,
		SubmitProposalMethod,
		VoteMethod,
		VoteWeightedMethod,
		DepositMethod,
		CancelProposalMethod:
		return true
	default:
		return false
	}
}
