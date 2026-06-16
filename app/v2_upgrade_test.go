package app

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	mocatypes "github.com/mocachain/moca/v2/types"
	precompilesstorage "github.com/mocachain/moca/v2/x/evm/precompiles/storage"
)

// TestMigrateToV2_ContractSurvivesInPlace proves the v1.3.0 -> v2 in-place
// software upgrade: a contract deployed under the in-tree x/evm (its code hash
// carried on the EthAccount, code+storage in the "evm" store, and NO cosmos/evm
// code-hash index) must remain executable after migrateToV2 — without any
// genesis export/import. The EVM loads contract code via GetCodeHash(addr) ->
// GetCode(hash); if the backfill is missing, GetCodeHash returns empty and the
// contract is seen as an empty EOA.
func TestMigrateToV2_ContractSurvivesInPlace(t *testing.T) {
	// EthSetup builds a fully-initialized app whose genesis does not re-seal the
	// process-global EVM config (see EthSetup/NewTestGenesisState), so this test
	// coexists with other full-app tests in the package. migrateToV2 sets the
	// EVM params/coin info itself, which is exactly what we're exercising.
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)

	// Minimal runtime bytecode: returns the 32-byte word 0x2a (42).
	//   PUSH1 0x2a; PUSH1 0x00; MSTORE; PUSH1 0x20; PUSH1 0x00; RETURN
	code := common.FromHex("602a60005260206000f3")
	codeHash := crypto.Keccak256Hash(code)
	addr := common.HexToAddress("0x000000000000000000000000000000000000c0fe")
	slot := common.HexToHash("0x01")
	val := common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000002a")

	// Simulate v1.3.0 on-disk state for the contract:
	//   - EthAccount carrying the code hash (where the old x/evm kept it)
	//   - code under {0x01}+codeHash, storage under {0x02}+addr+slot
	//   - deliberately NO SetCodeHash (the cosmos/evm {0x04}+addr index is absent)
	accI := mocaApp.AccountKeeper.NewAccountWithAddress(ctx, sdk.AccAddress(addr.Bytes()))
	ethAcc := accI.(*mocatypes.EthAccount)
	require.NoError(t, ethAcc.SetCodeHash(codeHash))
	mocaApp.AccountKeeper.SetAccount(ctx, ethAcc)
	mocaApp.EvmKeeper.SetCode(ctx, codeHash.Bytes(), code)
	mocaApp.EvmKeeper.SetState(ctx, addr, slot, val.Bytes())

	// Pre-upgrade: cosmos/evm cannot resolve the code (index empty) -> EOA.
	require.True(t, evmtypes.IsEmptyCodeHash(mocaApp.EvmKeeper.GetCodeHash(ctx, addr).Bytes()),
		"precondition: code-hash index must be empty before migration")

	// Run the in-place v2 migration.
	require.NoError(t, mocaApp.migrateToV2(ctx))

	// Post-upgrade: the contract is executable again.
	require.Equal(t, codeHash, mocaApp.EvmKeeper.GetCodeHash(ctx, addr), "code-hash index backfilled")
	require.Equal(t, code, mocaApp.EvmKeeper.GetCode(ctx, codeHash), "code resolvable via the new index")
	require.Equal(t, val.Bytes(), mocaApp.EvmKeeper.GetState(ctx, addr, slot).Bytes(), "storage preserved in place")

	// EVM params + coin info migrated to cosmos/evm format.
	params := mocaApp.EvmKeeper.GetParams(ctx)
	require.Equal(t, cmdcfg.BaseDenom, params.EvmDenom, "evm denom set to moca base denom")
	require.Contains(t, params.ActiveStaticPrecompiles, precompilesstorage.GetAddress().Hex(),
		"moca precompiles activated")
	coinInfo := mocaApp.EvmKeeper.GetEvmCoinInfo(ctx)
	require.Equal(t, cmdcfg.BaseDenom, coinInfo.Denom)
	require.Equal(t, uint32(evmtypes.EighteenDecimals), coinInfo.Decimals)

	// feemarket params migrated (moca's configured min gas price).
	require.False(t, mocaApp.FeeMarketKeeper.GetParams(ctx).MinGasPrice.IsNegative())
}
