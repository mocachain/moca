package distribution

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/utils"
)

const (
	// ValidatorDistributionInfoMethod is the ABI name for the ValidatorDistributionInfo query.
	ValidatorDistributionInfoMethod = "validatorDistributionInfo"
	// ValidatorOutstandingRewardsMethod is the ABI name for the ValidatorOutstandingRewards query.
	ValidatorOutstandingRewardsMethod = "validatorOutstandingRewards"
	// ValidatorCommissionMethod is the ABI name for the ValidatorCommission query.
	ValidatorCommissionMethod = "validatorCommission"
	// ValidatorSlashesMethod is the ABI name for the ValidatorSlashes query.
	ValidatorSlashesMethod = "validatorSlashes"
	// DelegationRewardsMethod is the ABI name for the DelegationRewards query.
	DelegationRewardsMethod = "delegationRewards"
	// DelegationTotalRewardsMethod is the ABI name for the DelegationTotalRewards query.
	DelegationTotalRewardsMethod = "delegationTotalRewards"
	// DelegatorValidatorsMethod is the ABI name for the DelegatorValidators query.
	DelegatorValidatorsMethod = "delegatorValidators"
	// DelegatorWithdrawAddressMethod is the ABI name for the DelegatorWithdrawAddress query.
	DelegatorWithdrawAddressMethod = "delegatorWithdrawAddress"
	// CommunityPoolMethod is the ABI name for the CommunityPool query.
	CommunityPoolMethod = "communityPool"
	// ParamsMethod is the ABI name for the Params query.
	ParamsMethod = "params"
)

// ValidatorDistributionInfo queries a validator's commission and self-delegation rewards.
func (p Precompile) ValidatorDistributionInfo(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	validatorAddr, err := hexAddressArg(args, "validator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorDistributionInfo(ctx, &distributiontypes.QueryValidatorDistributionInfoRequest{
		ValidatorAddress: validatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	operatorAddress := utils.AccAddressMustToHexAddress(res.OperatorAddress)

	return method.Outputs.Pack(operatorAddress, newDecCoins(res.SelfBondRewards), newDecCoins(res.Commission))
}

// ValidatorOutstandingRewards queries the outstanding rewards of a validator.
func (p Precompile) ValidatorOutstandingRewards(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	validatorAddr, err := hexAddressArg(args, "validator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorOutstandingRewards(ctx, &distributiontypes.QueryValidatorOutstandingRewardsRequest{
		ValidatorAddress: validatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(newDecCoins(res.Rewards.Rewards))
}

// ValidatorCommission queries the accumulated commission of a validator.
func (p Precompile) ValidatorCommission(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	validatorAddr, err := hexAddressArg(args, "validator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorCommission(ctx, &distributiontypes.QueryValidatorCommissionRequest{
		ValidatorAddress: validatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(newDecCoins(res.Commission.Commission))
}

// ValidatorSlashes queries the slash events of a validator.
func (p Precompile) ValidatorSlashes(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	req, err := NewValidatorSlashesRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorSlashes(ctx, req)
	if err != nil {
		return nil, err
	}

	slashEvents := make([]ValidatorSlashEvent, 0, len(res.Slashes))
	for _, slash := range res.Slashes {
		slashEvents = append(slashEvents, ValidatorSlashEvent{
			ValidatorPeriod: slash.ValidatorPeriod,
			Fraction:        slash.Fraction.BigInt(),
		})
	}

	pageResponse := PageResponse{
		NextKey: res.Pagination.NextKey,
		Total:   res.Pagination.Total,
	}

	return method.Outputs.Pack(slashEvents, pageResponse)
}

// DelegationRewards queries the rewards accrued by a delegation to a single validator.
func (p Precompile) DelegationRewards(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}
	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid delegator address: %v", args[0])
	}
	validatorAddr, ok := args[1].(common.Address)
	if !ok || validatorAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid validator address: %v", args[1])
	}

	res, err := p.distributionQuerier.DelegationRewards(ctx, &distributiontypes.QueryDelegationRewardsRequest{
		DelegatorAddress: delegatorAddr.String(),
		ValidatorAddress: validatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(newDecCoins(res.Rewards))
}

// DelegationTotalRewards queries the total rewards accrued across all of a delegator's validators.
func (p Precompile) DelegationTotalRewards(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	delegatorAddr, err := hexAddressArg(args, "delegator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegationTotalRewards(ctx, &distributiontypes.QueryDelegationTotalRewardsRequest{
		DelegatorAddress: delegatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	rewards := make([]DelegationDelegatorReward, 0, len(res.Rewards))
	for _, reward := range res.Rewards {
		rewards = append(rewards, DelegationDelegatorReward{
			ValidatorAddress: common.HexToAddress(reward.ValidatorAddress),
			Rewards:          newDecCoins(reward.Reward),
		})
	}

	return method.Outputs.Pack(rewards, newDecCoins(res.Total))
}

// DelegatorValidators queries the validators a delegator is bonded to.
func (p Precompile) DelegatorValidators(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	delegatorAddr, err := hexAddressArg(args, "delegator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegatorValidators(ctx, &distributiontypes.QueryDelegatorValidatorsRequest{
		DelegatorAddress: delegatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	validators := make([]common.Address, 0, len(res.Validators))
	for _, validator := range res.Validators {
		validators = append(validators, common.HexToAddress(validator))
	}

	return method.Outputs.Pack(validators)
}

// DelegatorWithdrawAddress queries a delegator's configured withdraw address.
func (p Precompile) DelegatorWithdrawAddress(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	delegatorAddr, err := hexAddressArg(args, "delegator address")
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegatorWithdrawAddress(ctx, &distributiontypes.QueryDelegatorWithdrawAddressRequest{
		DelegatorAddress: delegatorAddr.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(common.HexToAddress(res.WithdrawAddress))
}

// CommunityPool queries the community pool balance.
func (p Precompile) CommunityPool(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.distributionQuerier.CommunityPool(ctx, &distributiontypes.QueryCommunityPoolRequest{})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(newDecCoins(res.Pool))
}

// Params queries the distribution module parameters.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.distributionQuerier.Params(ctx, &distributiontypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	params := Params{
		CommunityTax:        res.Params.CommunityTax.BigInt(),
		BaseProposerReward:  res.Params.BaseProposerReward.BigInt(),
		BonusProposerReward: res.Params.BonusProposerReward.BigInt(),
		WithdrawAddrEnabled: res.Params.WithdrawAddrEnabled,
	}

	return method.Outputs.Pack(params)
}
