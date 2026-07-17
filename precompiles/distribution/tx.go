package distribution

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
)

const (
	// SetWithdrawAddressMethod is the ABI name for the SetWithdrawAddress transaction.
	SetWithdrawAddressMethod = "setWithdrawAddress"
	// WithdrawDelegatorRewardMethod is the ABI name for the WithdrawDelegatorReward transaction.
	WithdrawDelegatorRewardMethod = "withdrawDelegatorReward"
	// WithdrawDelegatorAllRewardsMethod is the ABI name for the moca-specific
	// WithdrawDelegatorAllRewards transaction.
	WithdrawDelegatorAllRewardsMethod = "withdrawDelegatorAllRewards"
	// WithdrawValidatorCommissionMethod is the ABI name for the WithdrawValidatorCommission transaction.
	WithdrawValidatorCommissionMethod = "withdrawValidatorCommission"
	// FundCommunityPoolMethod is the ABI name for the FundCommunityPool transaction.
	FundCommunityPoolMethod = "fundCommunityPool"
)

// errOnlyEOA is the moca-specific guard: state-changing methods may only be
// invoked directly by an externally owned account (evm.Origin == caller).
var errOnlyEOA = errors.New("only allow EOA can call this method")

// SetWithdrawAddress sets the withdraw address for the caller's delegation rewards.
func (p Precompile) SetWithdrawAddress(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, errOnlyEOA
	}

	msg, withdrawAddr, err := NewMsgSetWithdrawAddress(args, contract.Caller())
	if err != nil {
		return nil, err
	}

	if _, err = p.distributionMsgServer.SetWithdrawAddress(ctx, msg); err != nil {
		return nil, err
	}

	if err = p.EmitSetWithdrawAddressEvent(evm, contract.Caller(), withdrawAddr); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// WithdrawDelegatorReward withdraws the caller's rewards from a single validator.
func (p Precompile) WithdrawDelegatorReward(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, errOnlyEOA
	}

	msg, _, err := NewMsgWithdrawDelegatorReward(args, contract.Caller())
	if err != nil {
		return nil, err
	}

	res, err := p.distributionMsgServer.WithdrawDelegatorReward(ctx, msg)
	if err != nil {
		return nil, err
	}

	// The reward event topic is the delegator's withdraw address, not the validator.
	withdrawAddr, err := p.delegatorWithdrawAddress(ctx, msg.DelegatorAddress)
	if err != nil {
		return nil, err
	}

	if err = p.EmitWithdrawDelegatorRewardEvent(evm, contract.Caller(), withdrawAddr, res.Amount.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// WithdrawDelegatorAllRewards withdraws the caller's rewards from every validator it
// is delegated to. This is a moca-specific convenience method with no cosmos/evm analog.
func (p Precompile) WithdrawDelegatorAllRewards(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, _ []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, errOnlyEOA
	}

	delegator := contract.Caller().String()

	resQuery, err := p.distributionQuerier.DelegatorValidators(ctx, &distributiontypes.QueryDelegatorValidatorsRequest{
		DelegatorAddress: delegator,
	})
	if err != nil {
		return nil, err
	}

	withdrawAddr, err := p.delegatorWithdrawAddress(ctx, delegator)
	if err != nil {
		return nil, err
	}

	var total sdk.Coins
	for _, validator := range resQuery.Validators {
		res, err := p.distributionMsgServer.WithdrawDelegatorReward(ctx, &distributiontypes.MsgWithdrawDelegatorReward{
			DelegatorAddress: delegator,
			ValidatorAddress: validator,
		})
		if err != nil {
			return nil, err
		}
		if err = p.EmitWithdrawDelegatorRewardEvent(evm, contract.Caller(), withdrawAddr, res.Amount.String()); err != nil {
			return nil, err
		}
		total = total.Add(res.Amount...)
	}

	if err = p.EmitWithdrawDelegatorAllRewardsEvent(evm, contract.Caller(), total.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(total))
}

// WithdrawValidatorCommission withdraws the caller's accumulated validator commission.
func (p Precompile) WithdrawValidatorCommission(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, _ []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, errOnlyEOA
	}

	msg := NewMsgWithdrawValidatorCommission(contract.Caller())

	res, err := p.distributionMsgServer.WithdrawValidatorCommission(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitWithdrawValidatorCommissionEvent(evm, contract.Caller(), res.Amount.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// FundCommunityPool funds the community pool from the caller's balance.
func (p Precompile) FundCommunityPool(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, errOnlyEOA
	}

	msg, err := NewMsgFundCommunityPool(args, contract.Caller())
	if err != nil {
		return nil, err
	}

	if _, err = p.distributionMsgServer.FundCommunityPool(ctx, msg); err != nil {
		return nil, err
	}

	if err = p.EmitFundCommunityPoolEvent(evm, contract.Caller(), msg.Amount.String()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// delegatorWithdrawAddress resolves a delegator's configured withdraw address as a hex address.
func (p Precompile) delegatorWithdrawAddress(ctx sdk.Context, delegator string) (common.Address, error) {
	res, err := p.distributionQuerier.DelegatorWithdrawAddress(ctx, &distributiontypes.QueryDelegatorWithdrawAddressRequest{
		DelegatorAddress: delegator,
	})
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(res.WithdrawAddress), nil
}
