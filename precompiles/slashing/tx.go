package slashing

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	// UnjailMethod is the ABI name for the Unjail transaction.
	UnjailMethod = "unjail"
)

// Unjail releases the caller's validator from jail. The validator is the caller,
// so there are no arguments.
func (p Precompile) Unjail(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, _ []interface{}) ([]byte, error) {
	msg := &slashingtypes.MsgUnjail{
		ValidatorAddr: sdk.ValAddress(contract.Caller().Bytes()).String(),
	}

	if _, err := p.slashingMsgServer.Unjail(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUnjailEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
