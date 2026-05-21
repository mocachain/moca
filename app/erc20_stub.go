package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// noopErc20Keeper satisfies the cosmos/evm x/vm types.Erc20Keeper interface,
// which the x/vm keeper requires at construction.
//
// moca's x/erc20 module was removed (PRs #220/#221). cosmos/evm's x/vm keeper
// uses an Erc20Keeper only to resolve dynamic, per-token ERC20 precompile
// instances — a feature moca does not use. This no-op implementation always
// reports that no dynamic ERC20 precompile exists for a given address.
type noopErc20Keeper struct{}

func (noopErc20Keeper) GetERC20PrecompileInstance(
	_ sdk.Context,
	_ common.Address,
) (contract vm.PrecompiledContract, found bool, err error) {
	return nil, false, nil
}
