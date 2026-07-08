package authz

import (
	"fmt"

	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	authzKeeper authzkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(authzKeeper authzkeeper.Keeper) *Contract {
	return &Contract{
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if err = types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Pull the live SDK context from the EVM StateDB (static precompiles don't bind ctx at construction).
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return types.PackRetError("authz precompile must run within the cosmos/evm StateDB")
	}
	cacheCtx, err := stateDB.GetCacheContext()
	if err != nil {
		return types.PackRetError(err.Error())
	}
	ctx, commit := cacheCtx.CacheContext()
	snapshot := evm.StateDB.Snapshot()

	method, err := GetMethodByID(contract.Input)
	if err == nil {
		switch method.Name {
		case GrantMethodName:
			ret, err = c.Grant(ctx, evm, contract, readonly)
		case RevokeMethodName:
			ret, err = c.Revoke(ctx, evm, contract, readonly)
		case ExecMethodName:
			ret, err = c.Exec(ctx, evm, contract, readonly)
		case GrantsMethodName:
			ret, err = c.Grants(ctx, evm, contract, readonly)
		case GranterGrantsMethodName:
			ret, err = c.GranterGrants(ctx, evm, contract, readonly)
		case GranteeGrantsMethodName:
			ret, err = c.GranteeGrants(ctx, evm, contract, readonly)
		default:
			err = fmt.Errorf("method %s is not handled", method.Name)
		}
	}

	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		return types.PackRetError(err.Error())
	}

	commit()
	return ret, nil
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
