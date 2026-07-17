package payment

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

const (
	// CreatePaymentAccountMethodName is the ABI name for the CreatePaymentAccount transaction.
	CreatePaymentAccountMethodName = "createPaymentAccount"
	// DepositMethodName is the ABI name for the Deposit transaction.
	DepositMethodName = "deposit"
	// DisableRefundMethodName is the ABI name for the DisableRefund transaction.
	DisableRefundMethodName = "disableRefund"
	// WithdrawMethodName is the ABI name for the Withdraw transaction.
	WithdrawMethodName = "withdraw"
)

// CreatePaymentAccount creates a new payment account owned by the caller.
func (p Precompile) CreatePaymentAccount(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, _ []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	msg := &paymenttypes.MsgCreatePaymentAccount{
		Creator: contract.Caller().String(),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.paymentMsgServer.CreatePaymentAccount(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCreatePaymentAccountEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Deposit deposits coins from the caller into a payment stream.
func (p Precompile) Deposit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DepositArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &paymenttypes.MsgDeposit{
		Creator: contract.Caller().String(),
		To:      input.To,
		Amount:  math.NewIntFromBigInt(input.Amount),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.paymentMsgServer.Deposit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDepositEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DisableRefund disables refunds on a payment account owned by the caller.
func (p Precompile) DisableRefund(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DisableRefundArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &paymenttypes.MsgDisableRefund{
		Owner: contract.Caller().String(),
		Addr:  input.Addr,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.paymentMsgServer.DisableRefund(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDisableRefundEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Withdraw withdraws coins from a payment stream back to the caller.
func (p Precompile) Withdraw(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input WithdrawArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &paymenttypes.MsgWithdraw{
		Creator: contract.Caller().String(),
		From:    input.From,
		Amount:  math.NewIntFromBigInt(input.Amount),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.paymentMsgServer.Withdraw(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitWithdrawEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
