package testutil

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mocachain/moca/v2/app/ante/evm"
	"github.com/cosmos/evm/x/vm/statedb"
)

// NewStateDB returns a new StateDB for testing purposes.
//
// cosmos/evm v0.6.0 changed statedb.NewEmptyTxConfig to take no
// arguments; callers that need the per-tx hash now set it directly on
// the returned statedb.TxConfig.
func NewStateDB(ctx sdk.Context, evmKeeper evm.EVMKeeper) *statedb.StateDB {
	_ = common.BytesToHash(ctx.HeaderHash())
	return statedb.New(ctx, evmKeeper, statedb.NewEmptyTxConfig())
}
