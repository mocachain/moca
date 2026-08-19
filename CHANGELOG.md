<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:

* (<tag>) \#<issue-number> message

The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState given same genesisState and txList.

Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

## Unreleased

### Features

- (proto) [#67](https://github.com/mocachain/moca/pull/67) Publish protos to BSR under moca org
- (cli) [#243](https://github.com/mocachain/moca/pull/243) Add `mocad snapshots` command tree (list/delete/dump/export/load/restore) for managing local state-sync snapshots
- (upgrade) [#266](https://github.com/mocachain/moca/pull/266) Add `v1.3.0` upgrade handler (noop `RunMigrations`) and bump `moca-iavl` to `v1.2.0-rc3`. The moca-iavl commit-time bug (`SaveVersion` early-return) can leave stale **fastnode phantoms** — live-state index entries for grants already removed from the IAVL tree. The validator→gov `StakeAuthorization` and SP funding→gov `DepositAuthorization` grants that can appear "missing" are **consumed and auto-deleted by normal authz flow** at validator/SP creation (`CheckStakeAuthorization`/`CheckDepositAuthorization` call `Accept` then `DeleteGrant` once the scoped limit is exhausted) — they are not dropped by the bug, so nothing needs restoring. `moca-iavl v1.2.0-rc3` bumps `fastStorageVersionValue` to `1.1.1`, which forces a one-time fastnode rebuild from the canonical tree on every node at the `v1.3.0` restart, clearing any residual phantoms automatically and in-binary (node-local, non-consensus, no state-sync or operator action). The `prove=true` panic remains fixed by `cosmos/iavl#1009`

### Improvements

- (docs) [#66](https://github.com/mocachain/moca/pull/66) Update RELEASE_GUIDE.md security notes for GITHUB_TOKEN

### Bug Fixes
- (payment) [#433](https://github.com/mocachain/moca/pull/433) Backport to `release/1.4.x`: close the auto-resume frozen-flow iterator per account instead of deferring it, so the iterators do not accumulate until `AutoResume` returns (MOCA-415).

- (challenge) [#447](https://github.com/mocachain/moca/pull/447) Backport to `release/1.4.x`: bind the event type into the attestation signature payload so the chain verifies the aggregate against the same preimage the vote pool signs (MOCA-836)
- (virtualgroup) [#388](https://github.com/mocachain/moca/pull/388) Backport to `release/1.4.x`: settle the global virtual groups named in `MsgSettle` in a deterministic (request-id) order instead of Go map iteration order (MOCA-957)
- (x/payment) [#287](https://github.com/mocachain/moca/pull/287) Backport to `release/1.4.x`: compute the settle-timestamp math in `sdkmath.Int` and bound it to the int64 range in `UpdateStreamRecord`/`TryResumeStreamRecord` (MOCA-385)
- (virtualgroup, app) [#281](https://github.com/mocachain/moca/pull/281) Backport to `release/1.4.x`: give `x/virtualgroup` its own transient store key (`transient_virtualgroup`) and register it, so it no longer shares the `transient_storage` key with `x/storage` (MOCA-959)
- (app) [#398](https://github.com/mocachain/moca/pull/398) Backport to `release/1.4.x`: normalize the precompile blocklist keys to the checksummed `AccAddress.String()` form the bank keeper looks up (MOCA-961, MOCA-967)
- (x/sp) [#397](https://github.com/mocachain/moca/pull/397) Backport to `release/1.4.x`: delete the correct BLS-key index entry when a storage provider exits (MOCA-972)
- (x/storage, x/sp) [#298](https://github.com/mocachain/moca/pull/298) [#301](https://github.com/mocachain/moca/pull/301) [#299](https://github.com/mocachain/moca/pull/299) [#302](https://github.com/mocachain/moca/pull/302) Backport to `release/1.4.x`: close store iterators in the storage `IsNonEmptyBucket`/payment-check and sp `GetAllStorageProviders`/force-update-maintenance paths
- (x/permission) [#406](https://github.com/mocachain/moca/pull/406) Backport to `release/1.4.x`: match policy statement resources as anchored, escaped wildcard patterns and skip an uncompilable pattern instead of panicking (MOCA-807, MOCA-936)
- (x/permission) [#411](https://github.com/mocachain/moca/pull/411) Backport to `release/1.4.x`: treat an empty `Resources` slice as bucket-scoped, so a bucket-wide Deny still applies to sub-resources (MOCA-808)
- (app) [#391](https://github.com/mocachain/moca/pull/391) Backport to `release/1.4.x`: schedule configured hardforks in `PreBlocker` (so a plan applies in the block its height names) and let a configured hardfork supersede a pending/stale upgrade plan instead of panicking (MOCA-962)
- (x/payment) [#380](https://github.com/mocachain/moca/pull/380) Backport to `release/1.4.x`: freeze all of an account's active out-flows in a single block during force settle, including across cascades and cycles (MOCA-806)
- (x/payment) [#407](https://github.com/mocachain/moca/pull/407) Backport to `release/1.4.x`: settle a resuming account once per batch and charge only the real active window (MOCA-1073)
- (virtualgroup) [#372](https://github.com/mocachain/moca/pull/372) Backport to `release/1.4.x`: restrict `MsgDeleteGlobalVirtualGroup` to the GVG's own primary SP
- (virtualgroup) [#399](https://github.com/mocachain/moca/pull/399) Backport to `release/1.4.x`: reject a new global virtual group once the family is at its configured limit (MOCA-974)
- (virtualgroup) [#405](https://github.com/mocachain/moca/pull/405) Backport to `release/1.4.x`: check GVG family ownership in `CreateGlobalVirtualGroup`/`SwapIn` and reject a self-listed secondary (MOCA-1207)
- (virtualgroup) [#396](https://github.com/mocachain/moca/pull/396) Backport to `release/1.4.x`: compare the full secondary-SP list when detecting a duplicate global virtual group
- (x/storage, virtualgroup) [#306](https://github.com/mocachain/moca/pull/306) Backport to `release/1.4.x`: require a residual GVG family to be swapped out before its primary provider can exit, and let the end-of-block delete sweep complete for a resource whose provider is already gone, attributing the delete event to the module (MOCA-1347)
- (x/challenge, x/sp, virtualgroup) [#409](https://github.com/mocachain/moca/pull/409) Backport to `release/1.4.x`: bind a submitted challenge to the provider it names and resolve the attestation from that record instead of re-deriving it from current state, and hold the provider's deposit until its pending challenges expire so a completed exit no longer refunds ahead of an attestation (MOCA-1355)
- (x/storage, virtualgroup, app) [#408](https://github.com/mocachain/moca/pull/408) Backport to `release/1.4.x`: keep the per-block delete bookkeeping in the regular KV store (cleared on every path out of EndBlocker, so it never reaches committed state) and drop the two modules' now-unused transient store keys; metered gas for deletes carrying a policy rises by ~3.6k plus 30 per byte, which is consensus-visible (MOCA-959, MOCA-1353)
- (virtualgroup) [#368](https://github.com/mocachain/moca/pull/368) Backport to `release/1.4.x`: pay every secondary SP an equal share in `SettleAndDistributeGVG`, and drain the GVG's virtual payment account on delete — settle first, then sweep the indivisible remainder to the payment governance address (MOCA-742, MOCA-1343)
- (x/storage) [#366](https://github.com/mocachain/moca/pull/366) Backport to `release/1.4.x`: verify in `CompleteMigrateBucket` that the destination GVG family belongs to the destination storage provider (MOCA-741, MOCA-1344)
- (x/storage) [#406](https://github.com/mocachain/moca/pull/406) Backport to `release/1.4.x`: invoke `MsgPutPolicy.ValidateRuntime` in the storage message server's `PutPolicy`, covering the EVM storage precompile too since it shares this server (MOCA-1375, MOCA-1376, MOCA-1377)
- (deps) [#239](https://github.com/mocachain/moca/pull/239) Bump `moca-iavl` to `v1.2.0-rc1` (adds the reformatted-root `GetNode` fallback, cosmos/iavl #1009) to fix silently-truncated IAVL snapshot exports that produced corrupt mainnet state-sync snapshots
- (rpc) [#232](https://github.com/mocachain/moca/pull/232) Align `newHeads` subscription block hashes with the canonical hashes returned by block-by-number / block-by-hash APIs
- (ci) [#65](https://github.com/mocachain/moca/pull/65) Resolve goreleaser CI failures for arm64 docker builds
- (audit) [#63](https://github.com/mocachain/moca/pull/63) Apply audit fixes

## [v1.1.2] - 2026-01-19

### Bug Fixes

- (config) [`07bcc46`](https://github.com/mocachain/moca/commit/07bcc46e) Fix missing EIP-155 configs during chain config load

## [v1.1.1] - 2026-01-19

### Features

- (app) [`cb12c58`](https://github.com/mocachain/moca/commit/cb12c589) Configurable hardfork activation support
- (app) [`cb487bd`](https://github.com/mocachain/moca/commit/cb487bd1) Add `testnet_gov_param_fix` upgrade handler

## [v1.1.0] - 2026-01-14

This release includes the Cosmos SDK v0.50.13 migration, comprehensive security audit fixes, cosmovisor support, and numerous module improvements.

### State Machine Breaking

- (deps) [`5e8e39c`](https://github.com/mocachain/moca/commit/5e8e39cc) Migrate to Cosmos SDK v0.50.13 and CometBFT v0.38+
- (deps) [`76f9cb0`](https://github.com/mocachain/moca/commit/76f9cb0d) Migrate to IBC-Go v10.0.0
- (deps) [`34e5416`](https://github.com/mocachain/moca/commit/34e54169) Migrate module imports to `cosmossdk.io/x/*` paths
- (app) [`581b7d8`](https://github.com/mocachain/moca/commit/581b7d80) Restore complete `x/inflation` module from evmos v12
- (evm) [`f6b3b01`](https://github.com/mocachain/moca/commit/f6b3b01b) CRIT-002: Gas consumption for precompile methods must not be hard-coded
- (storage, payment, permission) [`1a7c899`](https://github.com/mocachain/moca/commit/1a7c8994) CRIT-003: Remove `UpdateParams` from storage/payment/permission precompile modules
- (x/sp) [`0969ae9`](https://github.com/mocachain/moca/commit/0969ae91) CRIT-001: Delete old indices in `EditStorageProvider` to prevent index pollution

### Features

- (upgrade) [`8ef7761`](https://github.com/mocachain/moca/commit/8ef77616) Add upgrade handler for v1.1.0
- (docker) [`929ec80`](https://github.com/mocachain/moca/commit/929ec803) Add cosmovisor support to Dockerfile and entrypoint script
- (x/challenge) [`5fe1e6a`](https://github.com/mocachain/moca/commit/5fe1e6ac) HIGH-006: Ensure slash key uniqueness by including spID
- (x/storage) [`490c634`](https://github.com/mocachain/moca/commit/490c634c) LOW-015, INFO-019: Enforce `MaxBucketsPerAccount` limit and `PrimarySpApproval` validation
- (x/evm/precompiles) [`6ebec23`](https://github.com/mocachain/moca/commit/6ebec239) Add `cancelUpdateObjectContent` EVM precompile
- (gov) [`b4540d4`](https://github.com/mocachain/moca/commit/b4540d48) Add expedited mode for `submitProposal`
- (x/storage) [`25a4df6`](https://github.com/mocachain/moca/commit/25a4df60) Add message size and payload bytes validation checks
- (testing) [`810321d`](https://github.com/mocachain/moca/commit/810321d8) Add Foundry support and improve contract deployment tests

### Bug Fixes

- (x/storage) [`6858ca2`](https://github.com/mocachain/moca/commit/6858ca20) HIGH-007: Persist refund for zero-payload object updates
- (x/evm/precompiles) [`57b3ea9`](https://github.com/mocachain/moca/commit/57b3ea99) HIGH-008: Resolve ABI decoding panic in `UpdateSPPrice`
- (x/sp) [`1cea99d`](https://github.com/mocachain/moca/commit/1cea99de) HIGH-009: Enforce uniqueness in `EditStorageProvider` for addresses and BLS keys
- (x/storage) [`8ddf62c`](https://github.com/mocachain/moca/commit/8ddf62c1) MED-010: Remove nested `EstimateGas` to prevent inflated gas estimates
- (x/storage/cli) [`68db4b5`](https://github.com/mocachain/moca/commit/68db4b5c) MED-011, MED-012: Prevent slice index panic in group member operations
- (x/storage) [`bac19fb`](https://github.com/mocachain/moca/commit/bac19fb7) MED-013: Enable V2 cross-chain package deserialization with fallback
- (x/storage) [`37c8edc`](https://github.com/mocachain/moca/commit/37c8edce) MED-014: Burn ERC-721 NFT when deleting sealed objects
- (x/storage) [`ef133dd`](https://github.com/mocachain/moca/commit/ef133ddb) LOW-017: Enforce `PrimarySpApproval` validation in `CopyObject`
- (x/evm/precompiles) [`fa5b778`](https://github.com/mocachain/moca/commit/fa5b7785) Fix event topic encoding for indexed parameters in EVM precompiles
- (x/evm/precompiles/staking) [`3cad634`](https://github.com/mocachain/moca/commit/3cad634a) MINOR-020: Rename `Redelegatge` to `Redelegate` and update dispatch
- (chain) [`44d20d9`](https://github.com/mocachain/moca/commit/44d20d9c) Fix chain ID consistency issue to prevent double suffix
- (x/payment) [`a87e1e9`](https://github.com/mocachain/moca/commit/a87e1e94) Fix `MergeUserFlows` bug
- (x/sp) [`5fbdd76`](https://github.com/mocachain/moca/commit/5fbdd768) Fix `registerTx` ordering
- (x/sp) [`bf6167f`](https://github.com/mocachain/moca/commit/bf6167f4) Handle `NewPrivateKeyManager` error
- (x/storage) [`594ce2f`](https://github.com/mocachain/moca/commit/594ce2f1) Remove hardcoded addresses in `isKnownLockBalanceIssue`
- (x/sp) [`7013b42`](https://github.com/mocachain/moca/commit/7013b428) Fix SP withdraw bug
- (x/storage) [`2a78f17`](https://github.com/mocachain/moca/commit/2a78f177) Add `PayloadSize` check to prevent burn on empty objects
- (x/storage) [`4659bc5`](https://github.com/mocachain/moca/commit/4659bc51) Add explicit version identification for `CreateBucket` cross-chain packages
- (gov) [`77b3f5a`](https://github.com/mocachain/moca/commit/77b3f5a4) Add gov module address to blocked accounts
- (rpc) [`3a810a1`](https://github.com/mocachain/moca/commit/3a810a13) Fix RPC goroutine leak issues

### Improvements

- (ci) [`a87ed41`](https://github.com/mocachain/moca/commit/a87ed41b) Enhance goreleaser for multi-arch Docker support
- (ci) [`e0f0fe4`](https://github.com/mocachain/moca/commit/e0f0fe4c) Update GitHub Actions workflow for lowercase repository owner
- (deps) [`c299e77`](https://github.com/mocachain/moca/commit/c299e770) Bump btcec to v2.3.4

## [v0.1.0] - 2024-03-22

### Features

- (chain) [#9](https://github.com/mocachain/moca/pull/9) Set prefix to mc and denom to amoca, chain name to moca
- (precompile) [#101](https://github.com/mocachain/moca/pull/101) Add storage module precompile skeleton
- (storage) [#148](https://github.com/mocachain/moca/pull/148) Add system contract for object NFT

### Improvement

- (chore) [#33](https://github.com/mocachain/moca/pull/33) Fix test after remove recovery/incentives/revenue/vesting/inflation/claims module and remove upgrades.
- (dev) [#38](https://github.com/mocachain/moca/pull/38) Add dev.js script for development and testing.
- (dev) [#40](https://github.com/mocachain/moca/pull/40) Add four quick command and fix stop node bug.
- (deps) [#50](https://github.com/mocachain/moca/pull/50) Bump btcd version to [`v0.23.0`](https://github.com/btcsuite/btcd/releases/tag/v0.23.0)
- (dev) [#68](https://github.com/mocachain/moca/pull/68) Fix the issue of dev.js script not working after replacing moca-cosmos-sdk.
- (deps) [#69](https://github.com/mocachain/moca/pull/69) Bump moca-cosmos-sdk version to v0.1.0

### Bug Fixes

- (cli) [#46](https://github.com/mocachain/moca/pull/47) Use empty string as default value in `chain-id` flag to use the chain id from the genesis file when not specified.
- (evm) [#81](https://github.com/mocachain/moca/pull/81) Fix deploy the contract but cannot call the contract.

### State Machine Breaking

- (recovery) [#27](https://github.com/mocachain/moca/pull/27) Remove `x/recovery` module.
- (incentives) [#28](https://github.com/mocachain/moca/pull/28) Remove `x/incentives` module.
- (revenue) [#29](https://github.com/mocachain/moca/pull/29) Remove `x/revenue` module.
- (vesting) [#30](https://github.com/mocachain/moca/pull/30) Remove `x/vesting` module.
- (inflation) [#31](https://github.com/mocachain/moca/pull/31) Remove `x/inflation` module.
- (claims) [#32](https://github.com/mocachain/moca/pull/32) Remove `x/claims` module.
- (evm) [#35](https://github.com/mocachain/moca/pull/35) Enable EIP 3855 for solidity push0 instruction.
- (deps) [#43](https://github.com/mocachain/moca/pull/43) Bump Cosmos-SDK to v0.47.2 and ibc-go to v7.2.0.
- (evm) [#236](https://github.com/mocachain/moca/pull/236) Implement EIP 6780.

### API Breaking

- (evm) [#238](https://github.com/mocachain/moca/pull/238) Implement EIP-1153 transient storage.
