# E2E Tests: Kind-based Upgrade & Integration Testing

## Status: Draft / In Development

## Context

Moca chain upgrades are currently tested manually or through limited CI checks that don't exercise a real multi-validator chain. This makes it difficult to catch upgrade regressions, state migration bugs, or EVM compatibility issues before they reach testnet or mainnet.

We need automated end-to-end tests that:
- Run a real multi-validator Moca chain locally
- Execute diverse Cosmos and EVM transactions
- Perform chain upgrades (governance and hardfork paths)
- Verify state preservation across upgrades
- Can run against release images (ghcr.io) and source builds

## Decision

Use **Kind (Kubernetes in Docker)** to run a local 4-validator Moca chain for e2e testing. Kind provides:
- Realistic multi-node topology (separate pods per validator)
- Network isolation via Kubernetes networking
- Easy image swapping for upgrade testing (patch StatefulSet, restart pods)
- Reproducible environments (no cloud infra needed, runs on developer machines and CI)

### Why Kind over Docker Compose or single-node?

| Approach | Pros | Cons |
|----------|------|------|
| Single-node (`mocad start`) | Simplest | No consensus, no real networking, can't test upgrades |
| Docker Compose | Multi-node, no K8s dependency | Manual orchestration for upgrades, no native rollout |
| **Kind (chosen)** | Multi-node, K8s-native rollout, image swapping, CI-friendly | Requires Docker + Kind + kubectl |

## Goals

### Phase 1: Core Framework (current)
- [x] Reusable test framework (`framework/framework.sh`) with lifecycle management
- [x] One-liner chain setup: `fw_start_chain` / `fw_start_chain_from_version`
- [x] One-liner upgrades: `fw_upgrade_chain --name X --mode governance|hardfork`
- [x] Test runner with pass/fail tracking and debug log collection
- [x] Support for both source builds and release image testing

### Phase 2: Smoke & Upgrade Tests
- [x] Basic smoke test: block production, bank sends, validator queries
- [x] Hardfork upgrade test: pre-state -> upgrade -> verify state preserved
- [x] Governance upgrade test: proposal -> vote -> upgrade -> verify
- [x] EVM upgrade test with ERC20 contracts (mint/burn/transfer/approve/transferFrom)
- [x] Comprehensive upgrade test: 50+ EVM txs + 50+ Cosmos txs

### Phase 3: Extended Coverage (planned)
- [ ] Slashing / jailing tests
- [ ] Parameter change proposals
- [ ] EVM precompile tests
- [ ] Gas estimation and fee market tests
- [ ] Multi-hop upgrade tests (v1.0 -> v1.1 -> v1.2)
- [ ] Chaos testing (kill validators mid-upgrade, network partitions)

### Phase 4: CI Integration (planned)
- [ ] GitHub Actions workflow for PR checks
- [ ] Nightly runs against latest main
- [ ] Release gating (run full suite before tagging)
- [ ] Test result reporting / dashboards

## Architecture

```
e2e/kind/
├── e2e.env                    # Chain configuration (chain ID, denom, validators, etc.)
├── Dockerfile.e2e             # Build from source (current commit)
├── Dockerfile.e2e-gitref      # Build from git tag (old versions)
├── Dockerfile.e2e-release     # Build from pre-built release binary
├── contracts/                 # Solidity contracts for EVM testing
│   └── TestERC20.sol          # Full ERC20 with mint/burn/transfer/approve/transferFrom
├── scripts/                   # Low-level plumbing
│   ├── lib.sh                 # Shared utilities (exec_mocad, assertions, logging)
│   ├── setup-kind.sh          # Create Kind cluster with port forwarding
│   ├── build-images.sh        # Build Docker images (source or gitref)
│   ├── deploy.sh              # Deploy chain (init genesis, create K8s resources)
│   ├── init-chain.sh          # Genesis initialization (K8s Job)
│   ├── upgrade-chain.sh       # Upgrade orchestrator (governance + hardfork modes)
│   ├── cleanup.sh             # Tear down cluster
│   ├── run-suite.sh           # Legacy suite runner
│   └── run-tests.sh           # Legacy test runner
├── framework/                 # High-level test API
│   ├── framework.sh           # Lifecycle, chain setup, upgrade, test execution
│   └── runner.sh              # Test discovery and batch execution
├── tests/                     # Self-contained test files
│   ├── test_smoke.sh          # Basic chain operations (7 tests)
│   ├── test_upgrade_hardfork.sh      # Hardfork upgrade path
│   ├── test_upgrade_governance.sh    # Governance upgrade path
│   ├── test_upgrade_evm.sh           # ERC20 contract upgrade test (14 tests)
│   └── test_upgrade_comprehensive.sh # Full 50+50 tx upgrade test (14 tests)
├── suites/                    # Legacy test suites
└── manifests/                 # K8s manifests (StatefulSets, Services, ConfigMaps)
```

## How It Works

### Test Lifecycle

Each test file is self-contained:

```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../framework/framework.sh"
fw_init                              # 1. Initialize framework (traps, counters)

fw_start_chain                       # 2. Create Kind cluster, build image, deploy chain
# -- or for upgrade tests --
fw_start_chain_from_version "v1.1.2" # 2. Deploy old version

# 3. Run pre-upgrade transactions...

fw_upgrade_chain --name "v1.2.0" --mode governance  # 4. Upgrade

# 5. Define test functions
test_something() {
    local val; val=$(exec_mocad query ...)
    assert_eq "$val" "expected" "description"
}

fw_run_test "Something works" test_something  # 6. Execute tests

fw_done                                       # 7. Print summary, cleanup
```

### Upgrade Modes

**Governance**: Submits `MsgSoftwareUpgrade` proposal, votes YES with all validators, waits for upgrade height, scales down pods, swaps image, scales back up.

**Hardfork**: Waits for upgrade height (triggered by upgrade handler in binary), scales down, swaps image, scales back up.

### Release Image Testing

Tests can run against published ghcr.io release images:
```bash
RELEASE_TAG=v12.2.0-rc1 bash tests/test_upgrade_comprehensive.sh
```

The framework handles architecture detection and Docker multi-platform manifest workarounds automatically.

## Prerequisites

- Docker Desktop (or Docker Engine)
- [Kind](https://kind.sigs.k8s.io/) (`go install sigs.k8s.io/kind@latest`)
- kubectl
- [Foundry](https://getfoundry.sh/) (forge + cast) for EVM tests
- jq
- ~8GB free RAM (4 validator pods)

## Running Tests

```bash
# Run a single test
make e2e-fw-test TEST=smoke
make e2e-fw-test TEST=upgrade_evm

# Run all tests
make e2e-fw

# Debug mode (leave cluster running after test)
FW_SKIP_CLEANUP=true make e2e-fw-test TEST=smoke

# Test against a specific release
OLD_VERSION=v1.1.2 RELEASE_TAG=v12.2.0-rc1 make e2e-fw-test TEST=upgrade_comprehensive
```

## EVM Testing Approach

Rather than using raw bytecode toy contracts, we deploy real Solidity contracts (ERC20) and exercise:
- **Mappings**: `balanceOf`, `allowance` — tests complex storage slot persistence
- **Multi-account interactions**: approve + transferFrom across different signers
- **Access control**: only owner can mint/burn
- **Events**: Transfer, Approval events emitted correctly
- **Multiple contracts**: deploy 2-3 ERC20s with different configurations (decimals, names)
- **Cross-upgrade verification**: state from pre-upgrade contracts accessible post-upgrade

This tests more than just SSTORE/SLOAD — it validates that the EVM's account model, storage trie, and contract execution environment survive upgrades intact.

## Cosmos Testing Approach

Beyond simple bank sends, the comprehensive test exercises:
- **Staking**: edit-validator (moniker changes), delegate, unbond
- **Distribution**: withdraw-rewards
- **Governance**: text proposals, voting from all validators
- **State queries**: validator set, balances, module parameters

## References

- [Kind documentation](https://kind.sigs.k8s.io/)
- [Cosmos SDK upgrade module](https://docs.cosmos.network/main/build/modules/upgrade)
- [Foundry book](https://book.getfoundry.sh/)
- ghcr.io release images: `ghcr.io/mocachain/mocad:<tag>`
