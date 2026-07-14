package authz

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	cmn.Precompile
	authzKeeper authzkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(authzKeeper authzkeeper.Keeper, bankKeeper bankkeeper.Keeper) *Contract {
	return &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      authzAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		authzKeeper: authzKeeper,
	}
}

func (c *Contract) Address() common.Address {
	return authzAddress
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case GrantMethodName:
		return GrantGas
	case RevokeMethodName:
		return RevokeGas
	case ExecMethodName:
		return c.calculateExecGas(input)
	case GrantsMethodName:
		return GrantsGas
	case GranterGrantsMethodName:
		return GranterGrantsGas
	case GranteeGrantsMethodName:
		return GranteeGrantsGas
	default:
		return 0
	}
}

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Route dispatch through cosmos/evm's native-action protocol so keeper coin
	// moves stay reconciled with the EVM StateDB: FlushToCacheCtx + the
	// BalanceHandler translate the bank coin_spent/coin_received events into
	// StateDB SubBalance/AddBalance, the multistore is snapshotted for atomic
	// revert (AddPrecompileFn), and store gas is metered against contract.Gas.
	// Without this, StateDB.Commit's balance reconciliation would mint a debited
	// amount back to a 7702-dirtied caller (native-token inflation).
	return c.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return c.execute(ctx, evm, contract, readonly)
	})
}

func (c *Contract) execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, err := GetMethodByID(contract.Input)
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case GrantMethodName:
		return c.Grant(ctx, evm, contract, readonly)
	case RevokeMethodName:
		return c.Revoke(ctx, evm, contract, readonly)
	case ExecMethodName:
		return c.Exec(ctx, evm, contract, readonly)
	case GrantsMethodName:
		return c.Grants(ctx, evm, contract, readonly)
	case GranterGrantsMethodName:
		return c.GranterGrants(ctx, evm, contract, readonly)
	case GranteeGrantsMethodName:
		return c.GranteeGrants(ctx, evm, contract, readonly)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
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

func (c *Contract) calculateExecGas(input []byte) uint64 {
	if len(input) < 4 {
		return ExecBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return ExecBaseGas
	}

	var args ExecArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return ExecBaseGas
	}

	numMsgs := uint64(len(args.Msgs))
	if numMsgs > MaxExecMsgs {
		numMsgs = MaxExecMsgs
	}

	payloadSize := uint64(CalcPerMsgBytes(args.Msgs))
	if payloadSize > MaxExecPayloadBytes {
		payloadSize = MaxExecPayloadBytes
	}

	return ExecBaseGas + (numMsgs * ExecPerMsgGas) + (payloadSize * ExecPerByteGas)
}
