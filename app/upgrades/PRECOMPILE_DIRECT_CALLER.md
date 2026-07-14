# Upgrade: precompile direct-caller

Upgrade name: `v2.1.0-precompile-direct-caller`
(constant `upgrades.PrecompileDirectCallerUpgradeName`)

## What changes

Before this upgrade, moca's transaction precompiles enforce **EOA-only**: a call
where the tx origin differs from the direct caller (i.e. a contract forwarding
the call) is rejected with `only allow EOA can call this method`.

From the upgrade height, the EOA-only guard is gone:

- **Contracts may call precompiles.** A Solidity contract can call
  `bank`, `gov`, `staking`, `storage`, … precompiles directly.
- **Identity is the direct caller.** The Cosmos msg sender / operator / voter /
  delegator is `contract.Caller()` — the immediate caller, which may be a
  contract. `tx.origin` is never the authority subject.

Everything else (native-value rejection, ABI, events, gas, revert semantics) is
unchanged.

## Why an upgrade (not a plain release)

This changes the **consensus execution result** of already-deployed contract
transactions: the same call that reverts before the upgrade succeeds after it.
It must activate at one coordinated height across all validators, gated so
pre-upgrade blocks keep EOA-only behavior. See
`upgrades.PrecompileDirectCaller` for the pending gating-mechanism decision.

## Migration notes for integrators

### Contract developers
- Contracts can now call moca precompiles. The precompile acts as **your
  contract**, not as the EOA that started the transaction. Design authorization
  around `msg.sender` (the calling contract), not `tx.origin`.
- Access-control patterns that relied on precompiles being EOA-only must be
  revisited: a contract can now perform bank sends, votes, delegations, storage
  writes, etc. as itself.

### SDK / CLI users
- No change for EOA-initiated transactions: a user calling a precompile directly
  behaves exactly as before.
- The `only allow EOA can call this method` error is no longer returned; tooling
  that special-cases that string can drop it.

### Node operators
- Run the `v2.1.0-precompile-direct-caller` binary at the governed upgrade
  height. Behavior before the height is unchanged.

## Delivery checklist (chain ops)

- [ ] Finalize the precompile-side gating mechanism (param flag vs height) and
      rework the EOA-only removal to be conditional on it.
- [ ] Choose the upgrade height; cut and tag the versioned binary.
- [ ] Submit the governance `MsgSoftwareUpgrade`.
- [ ] Testnet or local fork/replay proving pre- vs post-upgrade behavior for:
      EOA direct call, contract forwarding, internal keeper EVM calls, revert,
      balance sync.
- [ ] Publish these notes to SDK / CLI / contract integrators.
- [ ] Define stop/rollback handling if the upgrade fails.
