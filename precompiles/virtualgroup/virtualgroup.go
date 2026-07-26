package virtualgroup

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
	virtualgroupkeeper "github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the virtualgroup precompile. It follows the cosmos/evm precompile
// layout — Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch — so keeper
// coin moves stay reconciled with the EVM StateDB. The moca-specific surface is the
// hex (0x) address encoding, moca's virtualgroup method set, and the non-payable
// RejectValue guard. The virtualgroup msg server executes the transactions and the
// virtualgroup keeper serves the read queries.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	virtualGroupMsgServer virtualgrouptypes.MsgServer
	virtualGroupKeeper    virtualgroupkeeper.Keeper
}

// NewPrecompile creates a new virtualgroup Precompile as a vm.PrecompiledContract. The
// msg server is built from the virtualgroup keeper at wiring time; the virtualgroup
// keeper serves queries and the bank keeper reconciles coin moves with the EVM StateDB.
func NewPrecompile(
	virtualGroupMsgServer virtualgrouptypes.MsgServer,
	virtualGroupKeeper virtualgroupkeeper.Keeper,
	bankKeeper bankkeeper.Keeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      virtualGroupAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:                   virtualGroupABI,
		virtualGroupMsgServer: virtualGroupMsgServer,
		virtualGroupKeeper:    virtualGroupKeeper,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return virtualGroupAddress
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
// (AddPrecompileFn), and store gas is metered against contract.Gas. Without this,
// StateDB.Commit's balance reconciliation would mint a debited amount back to a
// 7702-dirtied caller (native-token inflation). moca precompiles are not payable, so
// any attached value is rejected up front.
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
	// Virtualgroup transactions
	case CreateGlobalVirtualGroupMethodName:
		bz, err = p.CreateGlobalVirtualGroup(ctx, evm, contract, method, args)
	case DeleteGlobalVirtualGroupMethodName:
		bz, err = p.DeleteGlobalVirtualGroup(ctx, evm, contract, method, args)
	case SwapOutMethodName:
		bz, err = p.SwapOut(ctx, evm, contract, method, args)
	case CompleteSwapOutMethodName:
		bz, err = p.CompleteSwapOut(ctx, evm, contract, method, args)
	case SPExitMethodName:
		bz, err = p.SPExit(ctx, evm, contract, method, args)
	case CompleteSPExitMethodName:
		bz, err = p.CompleteSPExit(ctx, evm, contract, method, args)
	case DepositMethodName:
		bz, err = p.Deposit(ctx, evm, contract, method, args)
	case ReserveSwapInMethodName:
		bz, err = p.ReserveSwapIn(ctx, evm, contract, method, args)
	case CompleteSwapInMethodName:
		bz, err = p.CompleteSwapIn(ctx, evm, contract, method, args)
	case CancelSwapInMethodName:
		bz, err = p.CancelSwapIn(ctx, evm, contract, method, args)
	// Virtualgroup queries
	case GlobalVirtualGroupFamiliesMethodName:
		bz, err = p.GlobalVirtualGroupFamilies(ctx, method, args)
	case GlobalVirtualGroupFamilyMethodName:
		bz, err = p.GlobalVirtualGroupFamily(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case CreateGlobalVirtualGroupMethodName,
		DeleteGlobalVirtualGroupMethodName,
		SwapOutMethodName,
		CompleteSwapOutMethodName,
		SPExitMethodName,
		CompleteSPExitMethodName,
		DepositMethodName,
		ReserveSwapInMethodName,
		CompleteSwapInMethodName,
		CancelSwapInMethodName:
		return true
	default:
		return false
	}
}
