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
  with the keeper coin moves. **All 11 precompiles wire this** (see the inflation
  note below).
- **Atomic revert** — the multistore is snapshotted (`AddPrecompileFn`) so an outer
  EVM-frame revert rolls back the keeper writes; there are no partial writes.
- **Gas metering** — the SDK gas meter is re-capped to `contract.Gas`, so store
  work (including iteration) is metered. Note: precompile calls therefore consume
  **real store gas** on top of the flat `RequiredGas`; callers must supply
  adequate gas limits.
- **Native revert semantics** — a handler error surfaces as `vm.ErrExecutionReverted`
  with the reason ABI-encoded in the return data (decode with
  `abi.UnpackRevert(res.Ret)`), not as a raw string in `VmError`.

## Why every precompile wires the balance handler (native-token inflation)

Before the native-action migration, precompiles used the legacy Greenfield
`GetCacheContext → keeper write → commit` pattern: a keeper coin move updated the
**bank store** but not the EVM **StateDB stateObject** balance. With EIP-7702
active from genesis, an attacker could self-delegate an EOA to a contract that
calls `bank.send`, leaving the EOA a dirty stateObject with a stale balance;
`StateDB.Commit` then reconciled it by **minting the debited amount back** — net
total-supply inflation, repeatable per block.

The `BalanceHandler` keeps the stateObject reconciled so `Commit` has zero delta
to mint. Because staking / distribution / gov / payment / storage precompiles also
move coins, **all 11** wire the balance handler, not just `bank`.

Regression guard: `TestBankSend_NoSupplyInflation` asserts a precompile `bank.send`
leaves total supply flat.

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

- `bank`: dispatch success, **no-supply-inflation invariant**, native revert on failure.
- `storage`: `createGroup` dispatch success, EOA-only rejection, failure-does-not-mutate.
- `storageprovider`: `updateSPPrice` decode + EVM-apply dispatch.

Follow-ups: total-supply-invariant guards for the other coin-moving precompiles
(staking / distribution / gov / payment), and a type-4 (7702) end-to-end inflation
reproduction.
