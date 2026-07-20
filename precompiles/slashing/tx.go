package slashing

import (
	"github.com/mocachain/moca/v2/precompiles/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	UnjailGas = 60_000

	UnjailMethodName = "unjail"

	UnjailEventName = "Unjail"
)

func (c *Contract) Unjail(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(UnjailMethodName)

	msg := &slashingtypes.MsgUnjail{
		ValidatorAddr: sdk.ValAddress(caller.Bytes()).String(),
	}

	server := slashingkeeper.NewMsgServerImpl(c.slashingkeeper)

	_, err := server.Unjail(ctx, msg)
	if err != nil {
		return nil, err
	}

	// add undelegate log
	if err := c.AddLog(
		evm,
		MustEvent(UnjailEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
