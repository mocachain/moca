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
- (e2e) [#105](https://github.com/mocachain/moca/pull/105) Add Kind-based e2e test framework with smoke and upgrade tests
- (cli) [#243](https://github.com/mocachain/moca/pull/243) Add `mocad snapshots` command tree (list/delete/dump/export/load/restore) for managing local state-sync snapshots
- (upgrade) [#246](https://github.com/mocachain/moca/pull/246) Add `v1.3.0` upgrade handler (noop `RunMigrations`)
- (upgrade) [#263](https://github.com/mocachain/moca/pull/263) Add `v1.3.0` upgrade handler (noop `RunMigrations`). The validator→gov `StakeAuthorization` and SP funding→gov `DepositAuthorization` grants that can appear "missing" are **consumed and auto-deleted by normal authz flow** at validator/SP creation (`CheckStakeAuthorization`/`CheckDepositAuthorization` call `Accept` then `DeleteGrant` once the scoped limit is exhausted) — they are not dropped by the moca-iavl commit-time bug, so nothing needs restoring. `main` tracks upstream `cosmos/iavl` (which carries the `GetNode` reformatted-root fallback / `cosmos/iavl#1009`); the residual-fastnode-phantom cleanup for the commit-time bug is delivered by the `v1.3.0` release on `release/1.3.x` (via the `moca-iavl` `fastStorageVersionValue` bump that forces an in-binary fastnode rebuild), so the handler here is a pure noop

### Improvements

- (precompiles) [#359](https://github.com/mocachain/moca/pull/359) Move the EVM precompiles from `x/evm/precompiles` to a top-level `precompiles/` package, matching the cosmos/evm layout (a precompile wraps another module, so it does not belong under the vm module). Pure `git mv` relocation: Go import paths change from `.../v2/x/evm/precompiles/...` to `.../v2/precompiles/...`, and `compile.sh` + the `Makefile` precompile-gen target are updated. No ABI, selector, address, gas, or logic change
- (e2e) [#315](https://github.com/mocachain/moca/pull/315) Restore dynamic `--gas auto` for cosmos txs in the kind upgrade suites (reverting the fixed-gas workaround from #294) via a shared `cosmos_broadcast` helper. moca's `--gas auto` derives the fee from the node `minimum-gas-prices`, so `--gas-prices`/`--fees` are dropped (both error `cannot provide both fees and gas prices` on either binary) and the e2e node `minimum-gas-prices` is bumped 5 → 25 gwei to clear the post-v2 20 gwei feemarket floor; `--gas-adjustment` is 1.5 because post-v2 `WriteFlat` store gas runs ~30% above the simulate estimate. The helper retries on `account sequence mismatch` — validator0's account sequence is shared across the suite's cosmos and EVM txs (cosmos/evm maps the EVM nonce onto it) and `--gas auto`'s simulate round-trip widens the stale-read window.
- (proto) [#313](https://github.com/mocachain/moca/pull/313) Align the gRPC-gateway swagger and Buf Schema Registry tooling with what moca actually runs: drop the stale `evmos/*`, `ethermint/*`, and `ibc/*` entries in `client/docs/config.json` (modules moca deleted) and add cosmos/evm `x/vm`+`x/feemarket` plus moca's own gateway-served modules (challenge, payment, permission, sp, storage, virtualgroup); title Evmos → Moca. Fix `scripts/protoc-swagger-gen.sh` (it still did `cp -r ./proto/evmos`, a removed dir, silently breaking `make proto-swagger-gen` under `set -e`) and move the third-party proto download out of the Makefile into `scripts/proto-download-deps.sh`, extended to fetch cosmos/evm's protos. Protos are pushed to `buf.build/moca/moca` manually (`buf push proto`) for now.
- (dockerup) Remove legacy Node.js tooling from `deployment/dockerup` (dev.js, npm manifests,
  Node-only configs); Docker multi-validator flow uses `localup.sh` via `docker-compose.yml`
- (localup) [#118](https://github.com/mocachain/moca/pull/118) Remove legacy Node.js tooling from `deployment/localup` (dev.js, join.js, npm manifests, configs, sample JSONs); local chains should use `localup.sh`
- (ci/docs) [#119](https://github.com/mocachain/moca/pull/119) Markdown lint: align README/e2e ERC20 and Kind docs
  with markdownlint; disable MD060; extend `.markdownlintignore` (CONTRIBUTING, RELEASE_GUIDE); fix
  `markdown-lint.yml` comments; remove obsolete `deployment/localup/README.md` and
  `deployment/dockerup/README.md` (legacy Node dev.js docs)
- (docs) [#66](https://github.com/mocachain/moca/pull/66) Update RELEASE_GUIDE.md security notes for GITHUB_TOKEN
- (rpc) [#309](https://github.com/mocachain/moca/pull/309) Replace the in-tree JSON-RPC server (`rpc/`, ~16k LOC forked from ethermint/evmos) with `github.com/cosmos/evm/rpc`: the backend, namespaces, and the newHeads/logs/pendingTx subscriptions now come from cosmos/evm (fed by an in-process CometBFT event stream and an ante `PendingTxListener` hook on `*Moca`), and the indexer moves to cosmos/evm's `server/types.TxResult`. Gains `safe`/`finalized` block tags, `eth_getBlockReceipts`, `eth_getHeaderBy*`, `eth_createAccessList`, `debug_traceBlock`, EIP-4844/7702 tx fields, and upstream tracking. New operator knobs (app.toml + flags): `json-rpc.ws-origins`, `json-rpc.enable-profiling`, `json-rpc.allow-insecure-unlock`, `json-rpc.batch-request-limit`, `json-rpc.batch-response-max-size`; the checked-in asset configs also pin `evm.evm-chain-id` per network. A thin `server/websockets.go` shim is retained so `newHeads` emits the canonical CometBFT block hash (completes `cosmos/evm#725`); removable once that lands upstream and we bump.
- (indexer) [#310](https://github.com/mocachain/moca/pull/310) Use `github.com/cosmos/evm/indexer` directly instead of the in-tree functional copy (same `TxResult` proto and key encoding — no reindex needed). The KV-indexer test is retained under `tests/integration/indexer` (mirroring upstream's layout), retargeted at the upstream package, since cosmos/evm publishes no test files in its module — keeping the indexer path covered against moca's forked SDK in CI. The kind e2e now runs validators with `enable-indexer = true` and asserts the tx hash actually lands in `evmindexer.db` (receipt lookups silently fall back to CometBFT `tx_search`, so a green suite alone cannot prove the indexer works).
- (testutil) [#312](https://github.com/mocachain/moca/pull/312) Slim `testutil/`: delete dead helpers (`contract.go`, `integration.go`, `statedb.go`, `CreateEthTx`) and use `cosmos/evm/testutil`'s fork-neutral `NoOpNextFn`/`NewHeader` instead of the in-tree copies. Fork-coupled helpers (key/codec/signing, `testutil/network`, funding) are deliberately kept in-tree — swapping them would change which `ethsecp256k1` type tests exercise.
- (proto) [#316](https://github.com/mocachain/moca/pull/316) Drop the orphaned `ethermint.feemarket.v1` protos and their generated `api/` pulsar code: zero importers since the in-tree `x/feemarket` was replaced by cosmos/evm's (`cosmos.evm.feemarket.v1`), and the dead protos were being republished to moca's BSR. `ethermint/types/v1/account.proto` stays — `EthAccount` is the stored account type.
- (types,utils) [#320](https://github.com/mocachain/moca/pull/320) Delete dead ethermint-era files orphaned by the `rpc/`/`x/evm`/`x/erc20` removals: `types/{block,gasmeter,int,protocol,validation,constant}.go`, `types/openapiutil/`, and `utils/eth/eth.go` — every exported symbol verified unreferenced across the repo and all known external consumers (moca-storage-provider, moca-go-sdk, moca-challenger). Externally-used files (`chain_id.go` — `ParseChainID` is production code in the storage provider's signer — `grn.go`, `s3util`, `resource`, `common`) are untouched.
- (client) [#321](https://github.com/mocachain/moca/pull/321) Delete the dead `client/testnet.go` (579 LOC): an unreferenced near-duplicate of `cmd/mocad/cmd/testnet.go`, which is what the `mocad testnet` command actually wires.
- (types,server) [#322](https://github.com/mocachain/moca/pull/322) Use cosmos/evm's `HasDynamicFeeExtensionOption` and `NewIndexTxCmd` instead of the in-tree copies (semantically identical; both already operated on cosmos/evm's own types) — the only two fork-neutral swaps the five-folder sweep found.
- (cleanup) [#323](https://github.com/mocachain/moca/pull/323) Trim dead exported symbols inside live files: `utils` (`IsTestnet`, `IsSupportedKey`, `GetMocaAddressFromBech32`, `ValAddressMustToHexAddress`), `client.InitConfig`, `server` rpc-era leftovers (`ConnectTmWS`, `MountGRPCWebServices`), and `types` coin helpers (`NewMocaCoin*`, `DefaultGasPrice`) — each verified unreferenced repo-wide (namesakes in `e2e/core` and `sdk/types` are distinct symbols and untouched) and across the known external consumers.
- (contracts) [#314](https://github.com/mocachain/moca/pull/314) Slim `contracts/` to what moca actually uses (mirroring cosmos/evm's top-level `contracts/` convention): delete the four ERC20 test doubles (`MinterBurnerDecimals`, `Burnable`, `DirectBalanceManipulation`, `MaliciousDelayed`) — dead since the `x/erc20` removal (#221), available upstream where still needed — plus their `.sol` sources, artifacts, and npm manifest. The moca-specific `ERC721NonTransferable` (bucket/object/group NFT facade the storage keeper mints/burns through) stays put, now pinned by a guard test covering its ABI surface, fixed token/hub addresses, and its intentionally bytecode-less (ABI-only) nature. Slither retargets to `solidity/`; the now-subjectless solhint workflow and broken `lint-contracts` Make targets are removed.

### State Machine Breaking

- (precompiles) [#280](https://github.com/mocachain/moca/issues/280) Adopt direct-caller semantics for transaction precompiles: smart contracts can compose with native modules, `contract.Caller()` is the sole acting identity for Cosmos message fields and EVM event topics, and transaction origin is no longer an authorization input. This behavior activates through a coordinated versioned chain upgrade.
- (erc20) [#221](https://github.com/mocachain/moca/pull/221) Remove the dormant `x/erc20` module and the erc20 precompile; register the `erc20` store for deletion in the `v2.0.0` store upgrade
- (deps) [#240](https://github.com/mocachain/moca/pull/240) Bump `moca-cosmos-sdk` to the upstream gas meter (remove greenfield RW metering): store writes/deletes now consume tx gas (reads stay free under `KVGasConfigAfterNagqu`), and the `GasInfo.rw_used` / `TxMsgData.extra_data` fields are removed

### Client Breaking

- (rpc) [#309](https://github.com/mocachain/moca/pull/309) The EVM JSON-RPC is now served by `cosmos/evm`: the non-standard `eth_coinbase`, `eth_mining`, `eth_hashrate`, `eth_getPendingTransactions`, and `debug_seedHash` endpoints are removed; `eth_subscribe("newPendingTransactions")` now streams from the local mempool (CheckTx) rather than committed-tx events; the in-RPC `eth_getLogs` rate limiter and query timeout are removed along with their `json-rpc.query-timeout` / `json-rpc.getlogs-rate-limit` / `json-rpc.getlogs-burst-limit` keys (rate-limit public endpoints at the infra layer; `logs-cap` / `block-range-cap` still bound queries) and the dead `json-rpc.fix-revert-gas-refund-height` key is dropped; JSON-RPC batch requests are now limited (1000 calls / 25 MB response by default, configurable via `json-rpc.batch-request-limit` / `json-rpc.batch-response-max-size`, 0 = unlimited); the profiling `debug_*` endpoints (previously always on when the `debug` namespace was enabled) are now gated behind `json-rpc.enable-profiling` (default off); keyring-backed account RPCs (`eth_accounts`, `eth_sendTransaction`, `personal_*`) are gated behind `json-rpc.allow-insecure-unlock` (default on, matching previous behavior — public nodes should disable it); `net_version` is now sourced from `evm.evm-chain-id` instead of the cosmos chain-id string; and WebSocket (`eth_subscribe`) connections now honor a `json-rpc.ws-origins` allow-list (default `["127.0.0.1", "localhost"]`; non-browser clients without an Origin header are unaffected). Standard ethers/viem polling and transaction submission are unaffected.

### Bug Fixes

- (precompiles) [#360](https://github.com/mocachain/moca/pull/360) Fix the precompile ABI-regen tooling (`precompiles/compile.sh`): pin abigen to `v1.16.2`, the go-ethereum version cosmos/evm v0.6.0 tracks (moca's effective go-ethereum is `github.com/cosmos/go-ethereum v1.16.2-cosmos-1` via `replace`; the minimal fork keeps the upstream `github.com/ethereum/go-ethereum` module path, so upstream abigen at the same version generates identical bindings). The script previously declared abigen `1.15.11` but installed `1.14.5`. Also fix the node_modules existence check (`solidity/contracts/node_modules` → `solidity/node_modules`) and drop the dead `IErc20` entry (no `precompiles/erc20` package). Regenerating all 11 bindings with the corrected pin produces no diff — they are already current with the go-ethereum bump.
- (config) [#288](https://github.com/mocachain/moca/issues/288) Auto-populate `evm.evm-chain-id` in the rendered `app.toml` at `mocad init` from `--chain-id` (devnet 5151 / testnet 222888 / mainnet 2288), so a fresh node gets the correct EIP-155 EVM chain ID with no operator step — instead of `0`, which makes cosmos/evm silently fall back to `262144`. The value is threaded through `initAppConfig` into the app.toml template the config interceptor renders at init. (Nodes upgraded in place keep their existing `app.toml`, which is not rewritten on upgrade — handled separately.)
- (config) [#288](https://github.com/mocachain/moca/issues/288) Derive `evm.evm-chain-id` from the **genesis** cosmos chain-id at node start when it is unset, and self-heal `app.toml` with the value — so a validator upgraded in place (whose `app.toml` is **not** rewritten on upgrade) gets the correct EIP-155 EVM chain ID with no operator action instead of cosmos/evm's silent `262144` fallback. The genesis `chain_id` is read via a minimal decode (robust to genesis schema changes) — it is the id CometBFT validates the node against, so a mistyped `--chain-id` can't persist a wrong value. The node **fails fast** (panic) when the id is unset and cannot be derived, and when a set `evm-chain-id` disagrees with the genesis-derived one (a consensus-critical mismatch); the configured value is trusted only when genesis is not derivable. The self-heal edits just the `evm-chain-id` line with `tomledit` (the comment-preserving TOML editor already vendored transitively via `confix`, promoted to a direct dep) — setting it in place if present, else adding it under `[evm]` (creating the section if absent) — preserving every other key, comment, the file mode, and any symlink; it never re-renders the whole file from a config struct, and a read-only config mount (e.g. a Kubernetes ConfigMap) is handled gracefully (logged, not fatal).
- (e2e) [#361](https://github.com/mocachain/moca/pull/361) Assert the EVM `eth_chainId` matches the value derived from the cosmos chain-id (`moca_<evmid>-<epoch>`) on **every validator pod** in the kind RPC suite, instead of adopting whatever `cast chain-id` reports on the NodePort endpoint (validator-0 only). Previously a node that fell back to cosmos/evm's `262144` default (`evm.evm-chain-id` unset in `app.toml`) would self-adjust and pass green — and because only validator-0 was queried, a single diverging validator (still <1/3 voting power) stayed invisible; now the suite signs `cast` txs with the derived id and requires all validators to report it, so the misconfiguration is caught.
- (wallets) [#150](https://github.com/mocachain/moca/issues/150) Add the missing Ledger USB product IDs to `wallets/usbwallet/hub.go` — `0x0008` (Ledger Nano Gen5), `0x7000` (Ledger Flex, newer firmware) and `0x8000` (Ledger Nano Gen5, newer firmware) — so Ledger Flex and Nano Gen5 are discoverable via USB enumeration, aligning with upstream go-ethereum. moca previously listed only the legacy `0x0007` Flex ID; cosmos/evm v0.6.0 adds `0x7000` but is itself still missing the Nano Gen5 IDs.
- (evm) [#332](https://github.com/mocachain/moca/pull/332) Align all EVM precompiles with cosmos/evm v0.6.0's native-action protocol, fixing a native-token inflation bug. The precompiles kept the legacy Greenfield `GetCacheContext()`→keeper-write→`commit()` pattern, so a bank/staking/distribution/gov/etc. keeper coin-move updated the bank store but not the EVM StateDB stateObject balance. Combined with EIP-7702 (active from genesis via Prague), an attacker could self-delegate their EOA to a contract that calls `bank.send`, making the EOA a dirty stateObject with a stale balance; `StateDB.Commit`'s reconciliation (`SetBalance` → mint the delta) then minted the debited amount back to the caller — net supply inflation, repeatable per block. Every precompile now embeds `cmn.Precompile` with a `BalanceHandlerFactory` and routes dispatch through `RunNativeAction`, which translates the bank `coin_spent`/`coin_received` events into `StateDB.SubBalance`/`AddBalance` (keeps balances reconciled → no spurious mint), snapshots the multistore via `AddPrecompileFn` for atomic revert on an outer-frame revert, and meters store gas against `contract.Gas`. Note: precompile calls now consume real store gas in addition to the flat `RequiredGas`, so callers must supply adequate gas limits. Only affects v2/`main` (cosmos/evm); mainnet v1.3.0 (in-tree `x/evm`, pre-Prague geth fork) is unaffected.
- (deps) [#329](https://github.com/mocachain/moca/pull/329) Bump `moca-cosmos-sdk` to fork `main` (`06ad3faf98` → `1ff7bfd0e4`), importing four `x/staking` store-iterator leak fixes (MOCA-416 through MOCA-420: `defer iterator.Close()` in the delegation keeper, `IterateLastValidatorPowers`, `GetValidators`, and the v5 delegations-by-validator migration). Pseudo-version bump only — no dependency-graph, proto, or state-machine changes.
- (tests) [#325](https://github.com/mocachain/moca/pull/325) De-flake `TestIndexerServiceRetriesAfterFetchError`: recovery needed two wake-up tokens but the new-block signal channel coalesces to one, stranding the second retry on the 60s timeout under the `-race` scheduler. Restructured to a deterministic two-phase design (quiescence check catches the busy-loop regression; a single-token recovery catches the latch) — verified 30× under `-race` and still failing on both historical defects.
- (e2e) [#324](https://github.com/mocachain/moca/pull/324) Fix the hardfork-upgrade e2e flake ("Chain did not reach height N within 240s"): the new-binary image load into kind could fail silently (`kind load ... || true`) and readiness accepted a single healthy pod as "chain resumed" while 2/3 quorum was still absent. Load images via `docker save | ctr import` with verification (`kind_load_image`), and gate post-restart on every validator's own RPC plus the EVM JSON-RPC (`wait_for_all_validator_rpcs`, `wait_for_evm_rpc_ready`); also delete stale kind clusters on fresh builds so containerd can't serve an old image SHA under the new tag.
- (e2e) [#319](https://github.com/mocachain/moca/pull/319) Route `log_error`/`log_warn` to stderr in the kind e2e harness so error/warning text isn't swallowed into `x=$(fn)` command substitutions (which masked failures as silent "Test exited early" and polluted captured values).
- (server) [#311](https://github.com/mocachain/moca/pull/311) Harden the EVM tx indexer service's fetch loop: back off on transient `Block`/`BlockResults` fetch errors instead of busy-looping, clear the error before retrying so indexing cannot stall until restart (upstream cosmos/evm as of v0.6.0 latches it), and make the shared latest-height access atomic (data race between the new-block subscription goroutine and the indexing loop). Guarded by a mock-driven regression test that fails on all three defects.
- (virtualgroup,storage) [#306](https://github.com/mocachain/moca/pull/306) Tighten storage provider exit preconditions and make discontinued-resource cleanup resolve its primary SP defensively.
- (storage) [#298](https://github.com/mocachain/moca/pull/298) Close the object iterator in `isNonEmptyBucket` to fix a per-call store-iterator leak (MOCA-413)
- (sp) [#299](https://github.com/mocachain/moca/pull/299) Close the iterator in `GetAllStorageProviders` to fix a store-iterator leak
- (sp) [#302](https://github.com/mocachain/moca/pull/302) Close the iterator in `ForceUpdateMaintenanceRecords` to fix a store-iterator leak
- (payment) [#300](https://github.com/mocachain/moca/pull/300) Close the auto-resume frozen-flow iterator per account to fix a store-iterator leak (MOCA-415)
- (payment) [#293](https://github.com/mocachain/moca/pull/293) Scope the auto-settle out-flow iterator per account to fix a store-iterator leak (MOCA-412)
- (storage) [#301](https://github.com/mocachain/moca/pull/301) Close the per-bucket object iterator in RunPaymentCheck to fix a store-iterator leak
- (app/upgrades) [#289](https://github.com/mocachain/moca/pull/289) Pin v2 feemarket `min_gas_price` to 20 gwei (moca's intended floor) so upgraded chains match genesis.
- (evm) [#290](https://github.com/mocachain/moca/pull/290) Restore `CallEVM` error-context wrap (method + contract in error message), fix copy-pasted "evil token" comment in `erc721NonTransferable.go`, update stale geth v1.15→v1.16 comments, remove unreachable `AddBalance` blocks in distribution precompile, fix grammar in precompile Run() default cases, and drop dead test-helper expressions.
- (evm) [#291](https://github.com/mocachain/moca/pull/291) Reject native value sent to precompiles to prevent a balance-reconciliation mint.
- (rpc) [#292](https://github.com/mocachain/moca/pull/292) Decode EVM tx logs from the tx response data (fixes empty `eth_getLogs`, receipt `logs`/`logsBloom`, and log subscriptions) and align receipt signer, `effectiveGasPrice` (now set on all receipt types), and pending-nonce signer with cosmos/evm v0.6.0.
- (x/payment) [#287](https://github.com/mocachain/moca/pull/287) Fix an Int64-overflow panic in the settle-timestamp math of `UpdateStreamRecord`/`TryResumeStreamRecord` (a chain-halt DoS reachable via a large balance + tiny netflow rate): compute in `sdkmath.Int` and bound to int64 range — reject an over-funding user deposit (`ErrSettleTimestampOverflow`), saturate on forced/EndBlocker paths so the chain can't halt (MOCA-385).
- (x/challenge) [#286](https://github.com/mocachain/moca/pull/286) Retire a challenge from the active set once it is attested, making attestation idempotent so duplicate submissions (e.g. redundant relayers or resubmissions by the in-turn submitter) are rejected instead of re-running heartbeat rewards and re-emitting attestation events.
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
