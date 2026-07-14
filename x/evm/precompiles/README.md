# Moca precompiles

Moca exposes its Cosmos modules to the EVM as **static precompiles** registered
with the `cosmos/evm` VM keeper. There are 11 of them:

`bank`, `authz`, `gov`, `payment`, `permission`, `staking`, `distribution`,
`slashing`, `storage`, `storageprovider`, `virtualgroup`.

Their hex addresses are the sorted list returned by
`app.MocaActiveStaticPrecompiles()` and are enabled via
`x/vm Params.ActiveStaticPrecompiles`.

## Native runtime base

All precompiles share one runtime skeleton in [`base`](./base), a thin wrapper
over `cosmos/evm`'s `precompiles/common` (the official native execution model).
`base.Precompile` embeds `common.Precompile` and the precompile ABI.

A precompile embeds `base.Precompile` and implements the standard shape:

```go
func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
    return c.RunPrecompile(evm, contract, readonly, c.Execute)
}

func (c *Contract) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
    method, _, err := c.Dispatch(contract, readonly, c.IsTransaction)
    if err != nil {
        return nil, err
    }
    switch method.Name { /* ... route to handlers ... */ }
}

func (Contract) IsTransaction(m *abi.Method) bool { return !m.IsConstant() }
```

The base provides:

- **`RunPrecompile`** — rejects native value up front (see below), then delegates
  to `cosmos/evm`'s `RunNativeAction`, which manages the cache context, multistore
  snapshot/revert, gas metering, and the optional balance handler, and converts
  any handler error into an EVM revert.
- **`Dispatch`** — pass-through to `SetupABI`: method routing by ID, read-only
  write protection, and argument unpacking. `IsTransaction` (derived from ABI
  mutability, `!method.IsConstant()`) tells `SetupABI` which methods mutate state.
- **`WithBalanceHandler(bankKeeper)`** — opt-in `BalanceHandlerFactory` that
  reconciles Cosmos bank events back into the EVM `StateDB`. Only `bank` wires it
  today (it is the canonical coin mover); the other precompiles move no native
  balances through the precompile boundary and leave it unset.

Before this base existed, every precompile hand-rolled its own
`GetCacheContext / CacheContext / Snapshot / RevertToSnapshot / commit` template.
That duplication is gone; the snapshot, gas, and revert semantics now come from
one shared, upstream-aligned implementation.

### Revert semantics

Under the native runtime a failing precompile call surfaces as a **proper EVM
revert** — `vm.ErrExecutionReverted` with the reason ABI-encoded in the return
data — rather than the raw error string in `VmError`. Callers (and tests) decode
the reason with `abi.UnpackRevert(res.Ret)`.

## Why native value is still rejected

`base.RunPrecompile` calls `types.RejectValue(contract)` before anything else, so
**no precompile accepts native value**. This is deliberate and must stay:

- Moca is not on the ERC-20 / WERC20 path; a precompile is not a payable token.
- Sending native `value` to a precompile while the handler also moves Cosmos funds
  is a double-write hazard. The `2026-06-29` fix
  (`fix(evm)!: reject native value to precompiles`) closed a mint bug of exactly
  this shape. Until every balance-changing path is provably reconciled to the
  `StateDB`, rejecting value is the safe invariant.

## Caller semantics (current)

Transaction methods still enforce **EOA-only**: they reject calls where
`evm.Origin != contract.Caller()` (a contract forwarding the call). Business
identity is the direct caller today, and contracts cannot yet call precompiles.

Removing EOA-only and moving to a direct-caller model is a **consensus behavior
change** delivered separately as a versioned chain upgrade — it changes the
execution result of already-deployed contract transactions and must not ship as a
silent refactor.

## Internal keeper EVM calls

`x/storage/keeper/evm.go` (`CallEVM` / `CallEVMWithData`) invokes the EVM keeper
directly to mint/burn resource NFTs (e.g. the group ERC721 at `0x3002`). This path
is unchanged by the native-mode migration: precompile constructor signatures are
identical, so app wiring and `expected_keepers` are untouched, and the
`storage.createGroup` success path exercises the full chain
(native precompile → `keeper.CreateGroup` → internal `CallEVM` mint) end-to-end.

## Migration status

| Stage | Scope | Done |
|---|---|---|
| Baselines | characterization tests for bank / storageprovider / storage | ✅ |
| Runtime base | `base` package | ✅ |
| Runtime migration | all 11 precompiles on the base | ✅ |
| Direct-caller | remove EOA-only, contract calls (chain upgrade) | pending (HITL) |

The runtime migration preserves external behavior except for the native revert
semantics noted above; EOA-only is intact everywhere.
