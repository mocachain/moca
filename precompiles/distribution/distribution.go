package distribution

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the distribution precompile. It follows the cosmos/evm precompile
// layout — Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch — so keeper
// coin moves stay reconciled with the EVM StateDB. The moca-specific surface is the
// hex (0x) address encoding, moca's method set, and the non-payable RejectValue guard.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	distributionMsgServer distributiontypes.MsgServer
	distributionQuerier   distributiontypes.QueryServer
}

// NewPrecompile creates a new distribution Precompile as a vm.PrecompiledContract.
// The msg server and querier are built from the distribution keeper at wiring time;
// the bank keeper drives the StateDB balance reconciliation.
func NewPrecompile(
	distributionMsgServer distributiontypes.MsgServer,
	distributionQuerier distributiontypes.QueryServer,
	bankKeeper bankkeeper.Keeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      distributionAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:                   distributionABI,
		distributionMsgServer: distributionMsgServer,
		distributionQuerier:   distributionQuerier,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return distributionAddress
}

// RequiredGas calculates the base gas via the cosmos/evm common flat+per-byte model.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoids panicking when trying to decode the method ID.
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		// This should never happen since this method is going to fail during Run.
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run dispatches the call through cosmos/evm's native-action protocol: the
// BalanceHandler translates bank coin_spent/coin_received events into StateDB
// SubBalance/AddBalance, the multistore is snapshotted for atomic revert, and store
// gas is metered against contract.Gas. moca precompiles are not payable, so any
// attached value is rejected up front.
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
	// Distribution transactions
	case SetWithdrawAddressMethod:
		bz, err = p.SetWithdrawAddress(ctx, evm, contract, method, args)
	case WithdrawDelegatorRewardMethod:
		bz, err = p.WithdrawDelegatorReward(ctx, evm, contract, method, args)
	case WithdrawDelegatorAllRewardsMethod:
		bz, err = p.WithdrawDelegatorAllRewards(ctx, evm, contract, method, args)
	case WithdrawValidatorCommissionMethod:
		bz, err = p.WithdrawValidatorCommission(ctx, evm, contract, method, args)
	case FundCommunityPoolMethod:
		bz, err = p.FundCommunityPool(ctx, evm, contract, method, args)
	// Distribution queries
	case ValidatorDistributionInfoMethod:
		bz, err = p.ValidatorDistributionInfo(ctx, method, args)
	case ValidatorOutstandingRewardsMethod:
		bz, err = p.ValidatorOutstandingRewards(ctx, method, args)
	case ValidatorCommissionMethod:
		bz, err = p.ValidatorCommission(ctx, method, args)
	case ValidatorSlashesMethod:
		bz, err = p.ValidatorSlashes(ctx, method, args)
	case DelegationRewardsMethod:
		bz, err = p.DelegationRewards(ctx, method, args)
	case DelegationTotalRewardsMethod:
		bz, err = p.DelegationTotalRewards(ctx, method, args)
	case DelegatorValidatorsMethod:
		bz, err = p.DelegatorValidators(ctx, method, args)
	case DelegatorWithdrawAddressMethod:
		bz, err = p.DelegatorWithdrawAddress(ctx, method, args)
	case CommunityPoolMethod:
		bz, err = p.CommunityPool(ctx, method, args)
	case ParamsMethod:
		bz, err = p.Params(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case SetWithdrawAddressMethod,
		WithdrawDelegatorRewardMethod,
		WithdrawDelegatorAllRewardsMethod,
		WithdrawValidatorCommissionMethod,
		FundCommunityPoolMethod:
		return true
	default:
		return false
	}
}
