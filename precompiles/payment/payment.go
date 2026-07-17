package payment

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the payment precompile. It follows the cosmos/evm precompile layout —
// Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The moca-specific
// surface is the payment method set and the non-payable RejectValue guard. The
// payment msg server executes the transactions and the payment keeper serves the
// read queries.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	paymentMsgServer paymenttypes.MsgServer
	paymentKeeper    paymentkeeper.Keeper
}

// NewPrecompile creates a new payment Precompile as a vm.PrecompiledContract. The msg
// server is built from the payment keeper at wiring time; the payment keeper serves
// queries and the bank keeper reconciles coin moves with the EVM StateDB.
func NewPrecompile(
	paymentMsgServer paymenttypes.MsgServer,
	paymentKeeper paymentkeeper.Keeper,
	bankKeeper bankkeeper.Keeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      paymentAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:              paymentABI,
		paymentMsgServer: paymentMsgServer,
		paymentKeeper:    paymentKeeper,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return paymentAddress
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

// Run dispatches the call through cosmos/evm's native-action protocol so keeper coin
// moves stay reconciled with the EVM StateDB: FlushToCacheCtx + the BalanceHandler
// translate the bank coin_spent/coin_received events into StateDB
// SubBalance/AddBalance, the multistore is snapshotted for atomic revert
// (AddPrecompileFn), and store gas is metered against contract.Gas. moca precompiles
// are not payable, so any attached value is rejected up front.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
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
	// Payment transactions
	case CreatePaymentAccountMethodName:
		bz, err = p.CreatePaymentAccount(ctx, evm, contract, method, args)
	case DepositMethodName:
		bz, err = p.Deposit(ctx, evm, contract, method, args)
	case DisableRefundMethodName:
		bz, err = p.DisableRefund(ctx, evm, contract, method, args)
	case WithdrawMethodName:
		bz, err = p.Withdraw(ctx, evm, contract, method, args)
	// Payment queries
	case PaymentAccountsByOwnerMethodName:
		bz, err = p.PaymentAccountsByOwner(ctx, method, args)
	case PaymentAccountMethodName:
		bz, err = p.PaymentAccount(ctx, method, args)
	case ParamsMethodName:
		bz, err = p.Params(ctx, method, args)
	case ParamsByTimestampMethodName:
		bz, err = p.ParamsByTimestamp(ctx, method, args)
	case OutFlowsMethodName:
		bz, err = p.OutFlows(ctx, method, args)
	case StreamRecordMethodName:
		bz, err = p.StreamRecord(ctx, method, args)
	case StreamRecordsMethodName:
		bz, err = p.StreamRecords(ctx, method, args)
	case PaymentAccountCountMethodName:
		bz, err = p.PaymentAccountCount(ctx, method, args)
	case PaymentAccountCountsMethodName:
		bz, err = p.PaymentAccountCounts(ctx, method, args)
	case PaymentAccountsMethodName:
		bz, err = p.PaymentAccounts(ctx, method, args)
	case DynamicBalanceMethodName:
		bz, err = p.DynamicBalance(ctx, method, args)
	case AutoSettleRecordsMethodName:
		bz, err = p.AutoSettleRecords(ctx, method, args)
	case DelayedWithdrawalMethodName:
		bz, err = p.DelayedWithdrawal(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case CreatePaymentAccountMethodName,
		DepositMethodName,
		DisableRefundMethodName,
		WithdrawMethodName:
		return true
	default:
		return false
	}
}
