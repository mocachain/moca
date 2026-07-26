package staking

import (
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	EditValidatorMethodName             = "editValidator"
	DelegateMethodName                  = "delegate"
	UndelegateMethodName                = "undelegate"
	RedelegateMethodName                = "redelegate"
	CancelUnbondingDelegationMethodName = "cancelUnbondingDelegation"
)

// EditValidator edits an existing validator's description and moca PoA/BLS fields.
func (p Precompile) EditValidator(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input EditValidatorArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &stakingtypes.MsgEditValidator{
		Description:       stakingtypes.Description(input.Description),
		ValidatorAddress:  contract.Caller().String(),
		CommissionRate:    input.GetCommissionRate(),
		MinSelfDelegation: input.GetMinSelfDelegation(),
		RelayerAddress:    input.GetRelayerAddress(),
		ChallengerAddress: input.GetChallengerAddress(),
		BlsKey:            input.BlsKey,
		BlsProof:          input.BlsProof,
	}

	if _, err := p.stakingMsgServer.EditValidator(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitEditValidatorEvent(evm, contract.Caller(), input.CommissionRate, input.MinSelfDelegation); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Delegate delegates coins from the caller to a validator.
func (p Precompile) Delegate(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegateArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	params, err := p.stakingQuerier.Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	msg := &stakingtypes.MsgDelegate{
		DelegatorAddress: sdk.AccAddress(contract.Caller().Bytes()).String(),
		ValidatorAddress: input.GetValidator().String(),
		Amount: sdk.Coin{
			Denom:  params.Params.BondDenom,
			Amount: math.NewIntFromBigInt(input.Amount),
		},
	}

	if _, err := p.stakingMsgServer.Delegate(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDelegateEvent(evm, contract.Caller(), input.GetValidator(), input.Amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Undelegate undelegates coins from a validator back to the caller.
func (p Precompile) Undelegate(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input UndelegateArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	params, err := p.stakingQuerier.Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	msg := &stakingtypes.MsgUndelegate{
		DelegatorAddress: sdk.AccAddress(contract.Caller().Bytes()).String(),
		ValidatorAddress: input.GetValidator().String(),
		Amount: sdk.Coin{
			Denom:  params.Params.BondDenom,
			Amount: math.NewIntFromBigInt(input.Amount),
		},
	}

	res, err := p.stakingMsgServer.Undelegate(ctx, msg)
	if err != nil {
		return nil, err
	}
	completionTime := big.NewInt(res.CompletionTime.Unix())

	if err := p.EmitUndelegateEvent(evm, contract.Caller(), input.GetValidator(), input.Amount, completionTime); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(completionTime)
}

// Redelegate moves a delegation from one validator to another.
func (p Precompile) Redelegate(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input RedelegateArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	params, err := p.stakingQuerier.Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	msg := &stakingtypes.MsgBeginRedelegate{
		DelegatorAddress:    sdk.AccAddress(contract.Caller().Bytes()).String(),
		ValidatorSrcAddress: input.GetSrcValidator().String(),
		ValidatorDstAddress: input.GetDstValidator().String(),
		Amount: sdk.Coin{
			Denom:  params.Params.BondDenom,
			Amount: math.NewIntFromBigInt(input.Amount),
		},
	}

	res, err := p.stakingMsgServer.BeginRedelegate(ctx, msg)
	if err != nil {
		return nil, err
	}
	completionTime := big.NewInt(res.CompletionTime.Unix())

	if err := p.EmitRedelegateEvent(evm, contract.Caller(), input.GetSrcValidator(), input.GetDstValidator(), input.Amount, completionTime); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(completionTime)
}

// CancelUnbondingDelegation cancels an unbonding delegation and re-delegates the coins.
func (p Precompile) CancelUnbondingDelegation(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CancelUnbondingDelegationArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	params, err := p.stakingQuerier.Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	msg := &stakingtypes.MsgCancelUnbondingDelegation{
		DelegatorAddress: sdk.AccAddress(contract.Caller().Bytes()).String(),
		ValidatorAddress: input.GetValidator().String(),
		Amount: sdk.Coin{
			Denom:  params.Params.BondDenom,
			Amount: math.NewIntFromBigInt(input.Amount),
		},
		CreationHeight: input.GetCreationHeight(),
	}

	if _, err := p.stakingMsgServer.CancelUnbondingDelegation(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelUnbondingDelegationEvent(evm, contract.Caller(), input.GetValidator(), input.Amount, input.CreationHeight); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
