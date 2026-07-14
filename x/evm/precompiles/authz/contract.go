package authz

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	base.Precompile

	authzKeeper authzkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(authzKeeper authzkeeper.Keeper) *Contract {
	return &Contract{
		Precompile:  base.New(authzAddress, authzABI),
		authzKeeper: authzKeeper,
	}
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
