package staking

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	base.Precompile

	stakingKeeper *stakingkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(stakingKeeper *stakingkeeper.Keeper) *Contract {
	return &Contract{
		Precompile:    base.New(stakingAddress, stakingABI),
		stakingKeeper: stakingKeeper,
	}
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case EditValidatorMethodName:
		return EditValidatorGas
	case DelegateMethodName:
		return DelegateGas
	case UndelegateMethodName:
		return UndelegateGas
	case RedelegateMethodName:
		return RedelegateGas
	case CancelUnbondingDelegationMethodName:
		return CancelUnbondingDelegationGas
	case DelegationMethodName:
		return DelegationGas
	case UnbondingDelegationMethodName:
		return UnbondingDelegationGas
	case ValidatorMethodName:
		return ValidatorGas
	case ValidatorsMethodName:
		return ValidatorsGas
	case ValidatorDelegationsMethodName:
		return ValidatorDelegationsGas
	case ValidatorUnbondingDelegationsMethodName:
		return ValidatorUnbondingDelegationsGas
	case DelegatorDelegationsMethodName:
		return DelegatorDelegationsGas
	case DelegatorUnbondingDelegationsMethodName:
		return DelegatorUnbondingDelegationsGas
	case RedelegationsMethodName:
		return RedelegationsGas
	case DelegatorValidatorsMethodName:
		return DelegatorValidatorsGas
	case DelegatorValidatorMethodName:
		return DelegatorValidatorGas
	case HistoricalInfoMethodName:
		return HistoricalInfoGas
	case PoolMethodName:
		return PoolGas
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
	case EditValidatorMethodName:
		return c.EditValidator(ctx, evm, contract, readonly)
	case DelegateMethodName:
		return c.Delegate(ctx, evm, contract, readonly)
	case UndelegateMethodName:
		return c.Undelegate(ctx, evm, contract, readonly)
	case RedelegateMethodName:
		return c.Redelegate(ctx, evm, contract, readonly)
	case CancelUnbondingDelegationMethodName:
		return c.CancelUnbondingDelegation(ctx, evm, contract, readonly)
	case DelegationMethodName:
		return c.Delegation(ctx, evm, contract, readonly)
	case UnbondingDelegationMethodName:
		return c.UnbondingDelegation(ctx, evm, contract, readonly)
	case ValidatorMethodName:
		return c.Validator(ctx, evm, contract, readonly)
	case ValidatorsMethodName:
		return c.Validators(ctx, evm, contract, readonly)
	case ValidatorDelegationsMethodName:
		return c.ValidatorDelegations(ctx, evm, contract, readonly)
	case ValidatorUnbondingDelegationsMethodName:
		return c.ValidatorUnbondingDelegations(ctx, evm, contract, readonly)
	case DelegatorDelegationsMethodName:
		return c.DelegatorDelegations(ctx, evm, contract, readonly)
	case DelegatorUnbondingDelegationsMethodName:
		return c.DelegatorUnbondingDelegations(ctx, evm, contract, readonly)
	case RedelegationsMethodName:
		return c.Redelegations(ctx, evm, contract, readonly)
	case DelegatorValidatorsMethodName:
		return c.DelegatorValidators(ctx, evm, contract, readonly)
	case DelegatorValidatorMethodName:
		return c.DelegatorValidator(ctx, evm, contract, readonly)
	case HistoricalInfoMethodName:
		return c.HistoricalInfo(ctx, evm, contract, readonly)
	case PoolMethodName:
		return c.Pool(ctx, evm, contract, readonly)
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
