# Moca precompiles

Moca exposes its Cosmos modules to the EVM as **static precompiles** registered
with the `cosmos/evm` VM keeper. There are 11 of them:

`bank`, `authz`, `gov`, `payment`, `permission`, `staking`, `distribution`,
`slashing`, `storage`, `storageprovider`, `virtualgroup`.

Their hex addresses are the sorted list returned by
`app.MocaActiveStaticPrecompiles()` and are enabled via
`x/vm Params.ActiveStaticPrecompiles`.

## Native-action runtime

Every precompile embeds `cosmos/evm`'s `cmn.Precompile` (from
`github.com/cosmos/evm/precompiles/common`) and routes dispatch through
`RunNativeAction` — the official cosmos/evm v0.6.0 native-action protocol:

```go
type Contract struct {
    cmn.Precompile
    // module keeper(s) + bankKeeper (for the balance handler)
}

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
    if err := types.RejectValue(contract); err != nil {   // precompiles are not payable
        return types.PackRetError(err.Error())
    }
    if len(contract.Input) < 4 {
        return types.PackRetError("invalid input")
    }
    return c.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
        return c.execute(ctx, evm, contract, readonly)
    })
}
```

`RunNativeAction` gives every precompile, for free:

- **Balance reconciliation** — a `BalanceHandlerFactory(bankKeeper)` translates the
  bank `coin_spent` / `coin_received` events emitted during the call into
  `StateDB.SubBalance` / `AddBalance`, keeping the EVM stateObject balances in sync
  with the keeper coin moves. **All 11 precompiles wire this** (see below).
- **Atomic revert** — the multistore is snapshotted (`AddPrecompileFn`) so an outer
  EVM-frame revert rolls back the keeper writes; there are no partial writes.
- **Gas metering** — the SDK gas meter is re-capped to `contract.Gas`, so store
  work (including iteration) is metered. Note: precompile calls therefore consume
  **real store gas** on top of the flat `RequiredGas`; callers must supply
  adequate gas limits.
- **Native revert semantics** — a handler error surfaces as `vm.ErrExecutionReverted`
  with the reason ABI-encoded in the return data (decode with
  `abi.UnpackRevert(res.Ret)`), not as a raw string in `VmError`.

## Why every precompile wires the balance handler

A keeper coin move updates the bank store, but the EVM `StateDB` also caches an
account's balance in its stateObject. If the two are not kept in sync during a
precompile call, `StateDB.Commit` can reconcile them against a stale value. The
`BalanceHandler` translates the bank events emitted during the call into `StateDB`
balance updates so the two stay consistent. Because bank / staking / distribution /
gov / payment / storage / storageprovider / virtualgroup precompiles all move
coins, **all 11** wire the balance handler, not just `bank`.

Regression guards assert that a precompile transfer leaves total bank supply
unchanged: `TestBankSend_TotalSupplyInvariant`, `TestDelegate_TotalSupplyInvariant`,
`TestDeposit_TotalSupplyInvariant`.

## Why native value is still rejected

`Run` calls `types.RejectValue(contract)` first, so **no precompile accepts native
value**. Moca is not on the ERC-20 / WERC20 path; sending native `value` to a
precompile while the handler also moves Cosmos funds is a double-write hazard.

## Caller semantics (EOA-only)

Transaction methods enforce **EOA-only**: a call where `evm.Origin != contract.Caller()`
(a contract forwarding the call) is rejected with `only allow EOA can call this method`.
Business identity is the direct caller.

Removing EOA-only to let contracts call precompiles (direct-caller model) is a
separate **consensus behavior change** that must ship behind a versioned chain
upgrade — it is intentionally **not** part of the native-action migration.

## Tests

Regression / characterization coverage layered on top of the migration:

- `bank` / `staking` / `payment`: **total-supply-invariant** guards, plus bank dispatch
  success and native revert on failure.
- `storage`: `createGroup` dispatch success, EOA-only rejection, failure-does-not-mutate.
- `storageprovider`: `updateSPPrice` decode + EVM-apply dispatch.

Follow-ups: total-supply-invariant guards for the remaining coin-moving precompiles
(distribution / gov / storageprovider / virtualgroup), and an end-to-end variant
across a full transaction.
