# Direct Precompile Caller Semantics Design

## Context

Moca's transaction precompiles currently reject calls when `evm.Origin` differs
from `contract.Caller()`. This prevents smart contracts from composing with
native modules even though `RunNativeAction` now provides the snapshot and
rollback boundary required for safe nested EVM calls.

The transaction methods also repeatedly read `contract.Caller()` while building
Cosmos messages and EVM logs. The resulting behavior is already mostly
direct-caller based, but the simultaneous `evm.Origin` check obscures which
identity is authoritative.

## Decision

Transaction precompiles use the immediate EVM caller as their sole acting
identity. Each method reads `contract.Caller()` once and uses that value for the
Cosmos message signer or actor field and for the corresponding EVM event topic.
The `evm.Origin == contract.Caller()` checks are removed.

Method arguments that identify a target resource or counterparty remain
unchanged. Examples include validator addresses, bucket owners, group owners,
storage providers, recipients, and authz grantees. Existing Cosmos module
authorization remains the final authority for whether the direct caller may
perform an action.

## Upgrade Boundary

This is a consensus behavior change. It is intended for a coordinated software
upgrade: the old binary enforces EOA-only behavior before the upgrade, and the
new binary activates direct-caller behavior when validators switch binaries at
the upgrade height. No in-process height toggle is added because the existing
`x/upgrade` binary transition is the activation boundary.

## Testing

- Change the storage `createGroup` forwarding characterization into a positive
  direct-caller test and assert the created group belongs to the contract
  caller, not the transaction origin.
- Add a bank forwarding test that asserts funds are debited from the contract
  caller and credited to the recipient while the origin balance is unchanged.
- Keep the existing native-action failure and supply-invariant tests to cover
  rollback and balance reconciliation.
- Run all precompile tests, formatting checks, lint, and a full build.

## Documentation

Update `precompiles/README.md` to define direct-caller behavior and record the
consensus upgrade requirement. Add an Unreleased changelog entry describing
contract-to-precompile composability.
