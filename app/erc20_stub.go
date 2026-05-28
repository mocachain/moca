package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// erc20StubKeeper is a no-op implementation of cosmos/evm's
// vmtypes.Erc20Keeper. moca removed its in-tree x/erc20 module; cosmos/evm's
// EVM keeper only calls GetERC20PrecompileInstance to resolve dynamically
// registered ERC-20 precompiles, which moca does not register. Returning
// (nil, false, nil) means "no dynamic ERC-20 precompile at this address",
// which is the correct behavior for a chain without x/erc20.
//
// TODO(cosmos-evm migration): if moca later adopts cosmos/evm's x/erc20
// module (for native-token <-> ERC-20 mirroring), replace this stub with the
// real erc20 keeper.
type erc20StubKeeper struct{}

var _ evmtypes.Erc20Keeper = erc20StubKeeper{}

func (erc20StubKeeper) GetERC20PrecompileInstance(_ sdk.Context, _ common.Address) (vm.PrecompiledContract, bool, error) {
	return nil, false, nil
}
