package keeper

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// CallEVM performs a smart contract method call using the given ABI / args.
//
// TODO(cosmos-evm migration): the previous implementation invoked the EVM
// keeper's ApplyMessage helper directly with a manually-built core.Message
// and a no-op tracer. cosmos/evm v0.6.0 changed the keeper surface
// significantly:
//
//   - core.Message is no longer constructed by ethtypes.NewMessage (geth
//     v1.15 dropped the constructor in favor of a struct-literal Message
//     value).
//   - Keeper.ApplyMessage now takes (ctx, *statedb.StateDB, core.Message,
//     *tracing.Hooks, commit, callFromPrecompile, internal bool).
//   - The high-level path is now Keeper.CallEVM(ctx, stateDB, abi, from,
//     contract, commit, callFromPrecompile, gasCap, method, args...).
//
// Storage's cross-chain mirror flow uses this helper to invoke ERC-721
// burns and similar; rewiring it requires routing through cosmos/evm's
// CallEVM (with a fresh StateDB) plus updating the EVMKeeper expected-
// interface accordingly. Until that lands the call returns a clear
// not-implemented error so callers fail fast instead of executing
// against a half-wired stub.
func (k Keeper) CallEVM(
	_ sdk.Context,
	_ abi.ABI,
	_, _ common.Address,
	_ bool,
	method string,
	_ ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	return nil, errors.New("storage CallEVM is temporarily disabled during cosmos/evm v0.6.0 migration; method=" + method)
}

// CallEVMWithData mirrors CallEVM but takes pre-packed call data instead of
// (abi, method, args). It carries the same migration TODO.
func (k Keeper) CallEVMWithData(
	_ sdk.Context,
	_ common.Address,
	_ *common.Address,
	_ []byte,
	_ bool,
) (*evmtypes.MsgEthereumTxResponse, error) {
	return nil, errors.New("storage CallEVMWithData is temporarily disabled during cosmos/evm v0.6.0 migration")
}
