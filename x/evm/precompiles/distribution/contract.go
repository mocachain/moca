package distribution

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	base.Precompile

	distributionKeeper distributionkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(distributionKeeper distributionkeeper.Keeper) *Contract {
	return &Contract{
		Precompile:         base.New(distributionAddress, distributionABI),
		distributionKeeper: distributionKeeper,
	}
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case SetWithdrawAddressMethodName:
		return SetWithdrawAddressGas
	case WithdrawDelegatorRewardMethodName:
		return WithdrawDelegatorRewardGas
	case WithdrawDelegatorAllRewardsMethodName:
		return WithdrawDelegatorAllRewardsGas
	case WithdrawValidatorCommissionMethodName:
		return WithdrawValidatorCommissionGas
	case FundCommunityPoolMethodName:
		return FundCommunityPoolGas
	case ValidatorDistributionInfoMethodName:
		return ValidatorDistributionInfoGas
	case ValidatorOutstandingRewardsMethodName:
		return ValidatorOutstandingRewardsGas
	case ValidatorCommissionMethodName:
		return ValidatorCommissionGas
	case DelegationRewardsMethodName:
		return DelegationRewardsGas
	case DelegationTotalRewardsMethodName:
		return DelegationTotalRewardsGas
	case CommunityPoolMethodName:
		return CommunityPoolGas
	case ParamsMethodName:
		return ParamsGas
	case ValidatorSlashesMethodName:
		return ValidatorSlashesGas
	case DelegatorValidatorsMethodName:
		return DelegatorValidatorsGas
	case delegatorWithdrawAddressMethodName:
		return delegatorWithdrawAddressGas
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
	case SetWithdrawAddressMethodName:
		return c.SetWithdrawAddress(ctx, evm, contract, readonly)
	case WithdrawDelegatorRewardMethodName:
		return c.WithdrawDelegatorReward(ctx, evm, contract, readonly)
	case WithdrawDelegatorAllRewardsMethodName:
		return c.WithdrawDelegatorAllRewards(ctx, evm, contract, readonly)
	case WithdrawValidatorCommissionMethodName:
		return c.WithdrawValidatorCommission(ctx, evm, contract, readonly)
	case FundCommunityPoolMethodName:
		return c.FundCommunityPool(ctx, evm, contract, readonly)
	case ValidatorDistributionInfoMethodName:
		return c.ValidatorDistributionInfo(ctx, evm, contract, readonly)
	case ValidatorOutstandingRewardsMethodName:
		return c.ValidatorOutstandingRewards(ctx, evm, contract, readonly)
	case ValidatorCommissionMethodName:
		return c.ValidatorCommission(ctx, evm, contract, readonly)
	case DelegationRewardsMethodName:
		return c.DelegationRewards(ctx, evm, contract, readonly)
	case DelegationTotalRewardsMethodName:
		return c.DelegationTotalRewards(ctx, evm, contract, readonly)
	case CommunityPoolMethodName:
		return c.CommunityPool(ctx, evm, contract, readonly)
	case ParamsMethodName:
		return c.Params(ctx, evm, contract, readonly)
	case ValidatorSlashesMethodName:
		return c.ValidatorSlashes(ctx, evm, contract, readonly)
	case DelegatorValidatorsMethodName:
		return c.DelegatorValidators(ctx, evm, contract, readonly)
	case delegatorWithdrawAddressMethodName:
		return c.DelegatorWithdrawAddress(ctx, evm, contract, readonly)
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
