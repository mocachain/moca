package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// CallEVM performs a smart-contract method call (the ERC-721 mint/burn that
// mirrors buckets/objects/groups as non-transferable NFTs) by packing the
// calldata and delegating to CallEVMWithData. It is a thin wrapper over
// cosmos/evm v0.6.0's keeper EVM execution.
func (k Keeper) CallEVM(
	ctx sdk.Context,
	contractABI abi.ABI,
	from, contract common.Address,
	commit bool,
	method string,
	args ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, errorsmod.Wrap(
			evmtypes.ErrABIPack,
			errorsmod.Wrap(err, "failed to create transaction data").Error(),
		)
	}

	return k.CallEVMWithData(ctx, from, &contract, data, commit)
}

// CallEVMWithData routes a raw-calldata EVM call through cosmos/evm's keeper.
//
// cosmos/evm v0.6.0 requires the caller to supply an explicit *statedb.StateDB
// (the geth-v1.15-era keeper no longer builds the core.Message via
// ethtypes.NewMessage). Storage's calls originate from msg handlers / keeper
// logic — not from inside a running precompile — so a fresh StateDB is created
// per call and callFromPrecompile is false. gasCap is nil because cosmos/evm's
// CallEVMWithData hardcodes the gas limit to its DefaultGasCap. When commit is
// true (mint/burn) the resulting state is flushed to the underlying store.
func (k Keeper) CallEVMWithData(
	ctx sdk.Context,
	from common.Address,
	contract *common.Address,
	data []byte,
	commit bool,
) (*evmtypes.MsgEthereumTxResponse, error) {
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())
	return k.evmKeeper.CallEVMWithData(ctx, stateDB, from, contract, data, commit, false, nil)
}
