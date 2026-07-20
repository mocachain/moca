package app

import (
	"encoding/json"
	"math"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	precompilesstorage "github.com/mocachain/moca/v2/precompiles/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	storagemoduletypes "github.com/mocachain/moca/v2/x/storage/types"
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
	initStorageParamsForUpgrade(t, mocaApp, ctx)

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

	// feemarket min_gas_price must match the 20 gwei floor (MainnetMinGasPrices), not the cosmos/evm default.
	require.Equal(t,
		MainnetMinGasPrices,
		mocaApp.FeeMarketKeeper.GetParams(ctx).MinGasPrice,
		"post-upgrade feemarket MinGasPrice must equal MainnetMinGasPrices (20 gwei)")
}

// TestMigrateToV2_MultipleContractsAndExecution strengthens the survival proof:
// several pre-upgrade contracts (one carrying multiple storage slots) must all
// be re-indexed by the backfill, and — beyond keeper getters — a surviving
// contract must actually EXECUTE through the EVM after the upgrade, returning
// its stored value. This catches regressions that leave code resolvable via
// getters but unrunnable through the VM (e.g. wrong index key/value).
func TestMigrateToV2_MultipleContractsAndExecution(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	initStorageParamsForUpgrade(t, mocaApp, ctx)

	// Contract A: SLOAD slot 0, then RETURN it (32 bytes). It reads storage at
	// execution time, so it only returns the right value if BOTH code and
	// storage survive the in-place migration.
	//   PUSH1 0x00; SLOAD; PUSH1 0x00; MSTORE; PUSH1 0x20; PUSH1 0x00; RETURN
	codeA := common.FromHex("60005460005260206000f3")
	hashA := crypto.Keccak256Hash(codeA)
	addrA := common.HexToAddress("0x00000000000000000000000000000000000000aa")

	// Contract A carries several storage slots; slot 0 holds the returned value.
	slot0 := common.HexToHash("0x00")
	wantA := common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000007b") // 123
	extraSlots := map[common.Hash]common.Hash{
		common.HexToHash("0x01"): common.HexToHash("0x11"),
		common.HexToHash("0x02"): common.HexToHash("0x22"),
		common.HexToHash("0x03"): common.HexToHash("0x33"),
	}

	// Contract B: a second, distinct contract (constant return 0x2a) — proves the
	// backfill handles MULTIPLE accounts, not just one.
	codeB := common.FromHex("602a60005260206000f3")
	hashB := crypto.Keccak256Hash(codeB)
	addrB := common.HexToAddress("0x00000000000000000000000000000000000000bb")

	// Seed v1.3.0-style state for both contracts: EthAccount carries the code
	// hash, code + storage live in the "evm" store, NO cosmos/evm index.
	for _, c := range []struct {
		addr common.Address
		hash common.Hash
		code []byte
	}{{addrA, hashA, codeA}, {addrB, hashB, codeB}} {
		accI := mocaApp.AccountKeeper.NewAccountWithAddress(ctx, sdk.AccAddress(c.addr.Bytes()))
		ethAcc := accI.(*mocatypes.EthAccount)
		require.NoError(t, ethAcc.SetCodeHash(c.hash))
		mocaApp.AccountKeeper.SetAccount(ctx, ethAcc)
		mocaApp.EvmKeeper.SetCode(ctx, c.hash.Bytes(), c.code)
	}
	mocaApp.EvmKeeper.SetState(ctx, addrA, slot0, wantA.Bytes())
	for s, v := range extraSlots {
		mocaApp.EvmKeeper.SetState(ctx, addrA, s, v.Bytes())
	}

	require.True(t, evmtypes.IsEmptyCodeHash(mocaApp.EvmKeeper.GetCodeHash(ctx, addrA).Bytes()))
	require.True(t, evmtypes.IsEmptyCodeHash(mocaApp.EvmKeeper.GetCodeHash(ctx, addrB).Bytes()))

	// Run the migration.
	require.NoError(t, mocaApp.migrateToV2(ctx))

	// Both contracts re-indexed.
	require.Equal(t, hashA, mocaApp.EvmKeeper.GetCodeHash(ctx, addrA), "contract A re-indexed")
	require.Equal(t, hashB, mocaApp.EvmKeeper.GetCodeHash(ctx, addrB), "contract B re-indexed")
	require.Equal(t, codeA, mocaApp.EvmKeeper.GetCode(ctx, hashA))
	require.Equal(t, codeB, mocaApp.EvmKeeper.GetCode(ctx, hashB))

	// All of contract A's storage slots survived in place.
	require.Equal(t, wantA.Bytes(), mocaApp.EvmKeeper.GetState(ctx, addrA, slot0).Bytes())
	for s, v := range extraSlots {
		require.Equal(t, v.Bytes(), mocaApp.EvmKeeper.GetState(ctx, addrA, s).Bytes(),
			"storage slot %s preserved", s.Hex())
	}

	// Execution-level proof: actually CALL contract A through the EVM. It SLOADs
	// slot 0 and returns it — so a correct (non-zero, code+storage intact)
	// result can only come from real VM execution against the migrated state.
	proposer := firstValidatorConsAddr(t, mocaApp, ctx)
	from := common.HexToAddress("0x00000000000000000000000000000000000000ff")

	callArgs := evmtypes.TransactionArgs{From: &from, To: &addrA}
	argsJSON, err := json.Marshal(callArgs)
	require.NoError(t, err)

	resp, err := mocaApp.EvmKeeper.EthCall(ctx, &evmtypes.EthCallRequest{
		Args:            argsJSON,
		GasCap:          25_000_000,
		ProposerAddress: proposer,
	})
	require.NoError(t, err, "EVM call against surviving contract must succeed")
	require.False(t, resp.Failed(), "VM execution must not revert: %s", resp.VmError)
	require.Equal(t, wantA.Bytes(), resp.Ret,
		"contract returns its surviving storage value via real EVM execution")

	// And contract B executes too (constant return).
	callArgsB := evmtypes.TransactionArgs{From: &from, To: &addrB}
	argsJSONB, err := json.Marshal(callArgsB)
	require.NoError(t, err)
	respB, err := mocaApp.EvmKeeper.EthCall(ctx, &evmtypes.EthCallRequest{
		Args:            argsJSONB,
		GasCap:          25_000_000,
		ProposerAddress: proposer,
	})
	require.NoError(t, err)
	require.False(t, respB.Failed(), "contract B VM execution must not revert: %s", respB.VmError)
	require.Equal(t,
		common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000002a").Bytes(),
		respB.Ret)
}

// TestMigrateToV2_ZeroCodeHashSkipped guards the de-risk fix: an EthAccount
// whose code hash is the literal all-zeros hash (an empty/unset CodeHash string
// decodes to common.Hash{}) is an EOA, not a contract, and must NOT receive a
// bogus 0x04 code-hash index entry during the backfill.
func TestMigrateToV2_ZeroCodeHashSkipped(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	initStorageParamsForUpgrade(t, mocaApp, ctx)

	addr := common.HexToAddress("0x00000000000000000000000000000000000000ee")
	accI := mocaApp.AccountKeeper.NewAccountWithAddress(ctx, sdk.AccAddress(addr.Bytes()))
	ethAcc := accI.(*mocatypes.EthAccount)
	// Set the zero hash explicitly (this is what an empty CodeHash string yields).
	require.NoError(t, ethAcc.SetCodeHash(common.Hash{}))
	mocaApp.AccountKeeper.SetAccount(ctx, ethAcc)

	require.NoError(t, mocaApp.migrateToV2(ctx))

	// The account must still read as an empty-code EOA: no index entry written.
	require.True(t, evmtypes.IsEmptyCodeHash(mocaApp.EvmKeeper.GetCodeHash(ctx, addr).Bytes()),
		"zero code-hash account must not be backfilled into the contract index")
}

// TestV2StoreUpgradesPreserveEvmFeemarket guards against a future edit silently
// orphaning migrated contract state: the v2.0.0 store-upgrade plan must never
// add, delete, or rename the "evm" or "feemarket" stores. The in-tree x/evm and
// cosmos/evm x/vm share the "evm" store key (same code+storage prefixes), and
// the feemarket key is unchanged, so touching either store here would discard
// the very state migrateToV2 carefully preserves in place.
func TestV2StoreUpgradesPreserveEvmFeemarket(t *testing.T) {
	su := v2StoreUpgrades()

	for _, name := range []string{evmtypes.StoreKey, feemarkettypes.StoreKey} {
		require.NotContains(t, su.Added, name, "v2 store-upgrade must not Add %q", name)
		require.NotContains(t, su.Deleted, name, "v2 store-upgrade must not Delete %q", name)
		for _, r := range su.Renamed {
			require.NotEqual(t, name, r.OldKey, "v2 store-upgrade must not Rename from %q", name)
			require.NotEqual(t, name, r.NewKey, "v2 store-upgrade must not Rename to %q", name)
		}
	}
}

// firstValidatorConsAddr returns the consensus address of the first genesis
// validator, used as the block proposer so the EVM's coinbase lookup resolves
// during EthCall (NewContext leaves the block header's proposer empty).
func firstValidatorConsAddr(t *testing.T, app *Moca, ctx sdk.Context) sdk.ConsAddress {
	t.Helper()
	vals, err := app.StakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals, "genesis must seed at least one validator")
	consBz, err := vals[0].GetConsAddr()
	require.NoError(t, err)
	return sdk.ConsAddress(consBz)
}

// initStorageParamsForUpgrade seeds the storage module's params in the test app.
// The EVM upgrade harness (NewTestGenesisState) deliberately omits the storage
// module from genesis, so StorageKeeper.GetParams returns a zero-value struct.
// migrateToV2 caps the discontinue queue (MOCA-743) by re-validating and
// re-saving the full storage params, which requires a realistically-initialized
// set — exactly what any real v1.3.0 chain already has.
func initStorageParamsForUpgrade(t *testing.T, app *Moca, ctx sdk.Context) {
	t.Helper()
	require.NoError(t, app.StorageKeeper.SetParams(ctx, storagemoduletypes.DefaultParams()))
}

// TestMigrateToV2_CapsDiscontinueQueue asserts the v2 upgrade replaces the
// uncapped MaxUint64 discontinue quotas with the finite defaults (MOCA-743).
func TestMigrateToV2_CapsDiscontinueQueue(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)

	// Pre-upgrade: a realistically-initialized chain whose discontinue queue is
	// still uncapped at MaxUint64 (the state MOCA-743 fixes). The EVM harness
	// omits storage from genesis, so start from DefaultParams, not the zero-value
	// GetParams.
	params := storagemoduletypes.DefaultParams()
	params.DiscontinueObjectMax = math.MaxUint64
	params.DiscontinueBucketMax = math.MaxUint64
	require.NoError(t, mocaApp.StorageKeeper.SetParams(ctx, params))

	require.NoError(t, mocaApp.migrateToV2(ctx))

	got := mocaApp.StorageKeeper.GetParams(ctx)
	require.Equal(t, storagemoduletypes.DefaultDiscontinueObjectMax, got.DiscontinueObjectMax)
	require.Equal(t, storagemoduletypes.DefaultDiscontinueBucketMax, got.DiscontinueBucketMax)
	require.Less(t, got.DiscontinueObjectMax, uint64(math.MaxUint64), "object discontinue cap must be finite")
	require.Less(t, got.DiscontinueBucketMax, uint64(math.MaxUint64), "bucket discontinue cap must be finite")
}
