# Moca Chain E2E Tests (Kind)

End-to-end tests for the Moca blockchain using [Kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker).
Mirrors the production `moca-chain-infra` patterns with K8s manifests and shell scripts.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [jq](https://jqlang.github.io/jq/download/)

## Quick Start

```bash
# Run smoke tests (setup Kind, build image, deploy, test)
make e2e-fw-test TEST=smoke

# Run all tests
make e2e-fw
```

## Framework Tests (Recommended)

The `framework/` + `tests/` approach provides self-contained test files with minimal boilerplate.
Each test file handles its own setup, tests, and teardown.

```bash
# Run all framework tests
make e2e-fw

# Run a single test
make e2e-fw-test TEST=smoke
make e2e-fw-test TEST=validator_devcontainer_parity
make e2e-fw-test TEST=upgrade_hardfork
make e2e-fw-test TEST=upgrade_governance

# Run with OLD_VERSION for upgrade tests
OLD_VERSION=v12.0.1 make e2e-fw-test TEST=upgrade_hardfork

# Dev mode (skip cleanup, leave cluster running for debugging)
make e2e-fw-dev TEST=smoke
```

### Writing a Test

```bash
#!/usr/bin/env bash
source "$(dirname "$0")/../framework/framework.sh"
fw_init

# Optional config overrides (before fw_start_chain)
fw_config NUM_VALIDATORS 2
fw_genesis_patch '.app_state.staking.params.max_validators = 10'

# Setup (1 line)
fw_start_chain

# Test functions
test_something() {
    local h1; h1=$(get_block_height "http://localhost:26657")
    sleep 3
    local h2; h2=$(get_block_height "http://localhost:26657")
    assert_gt "$h2" "$h1" "Block height should increase"
}

# Run tests
fw_run_test "Something works" test_something
fw_done
```

### Framework API

| Function | Purpose |
|----------|---------|
| `fw_init` | Initialize framework (first call in every test) |
| `fw_done` | Print summary, cleanup, exit (last call) |
| `fw_start_chain [--version V] [--validators N]` | Create cluster, build, deploy, wait for blocks |
| `fw_start_chain_from_version VERSION` | Deploy old version (for upgrade tests) |
| `fw_config KEY VALUE` | Override e2e.env variable |
| `fw_genesis_patch 'JQ_EXPR'` | Register jq patch for genesis.json |
| `fw_apptoml_patch 'SED_EXPR'` | Register sed patch for app.toml |
| `fw_upgrade_chain --name N --mode M` | Trigger upgrade (auto-computes height) |
| `fw_run_test "name" func` | Run test function, track pass/fail |
| `fw_tx_send FROM TO AMOUNT` | Send bank transfer + wait for inclusion |
| `fw_wait_blocks N` | Wait for N blocks from current height |

| Variable | Default | Purpose |
|----------|---------|---------|
| `FW_SKIP_CLEANUP` | `false` | Leave cluster running for debugging |
| `FW_REUSE_CLUSTER` | `false` | Reuse existing Kind cluster |

## Suite Tests (Legacy)

Tests are organized into suites under `suites/`. Each suite can override base config via its own `e2e.env`.

### Smoke Tests (default)

Basic chain functionality: blocks, balances, transfers, validators, module params.

```bash
make e2e-kind-smoke          # Full: setup + build + deploy + test
make e2e-kind-suite SUITE=smoke  # Just run tests (if chain is already deployed)
```

### Upgrade Tests (hardfork mode)

Tests a binary upgrade from an old version to the current build using the app.toml `[hardforks]` config.

```bash
# Upgrade from a specific release (pulls/builds old version, deploys, upgrades, validates)
OLD_VERSION=v12.0.1 make e2e-kind-upgrade-hardfork
```

### Upgrade Tests (governance mode)

Same as hardfork mode but triggers the upgrade via a governance software-upgrade proposal.

```bash
OLD_VERSION=v12.0.1 make e2e-kind-upgrade-governance
```

## Step by Step

```bash
# 1. Create Kind cluster
make e2e-kind-setup

# 2. Build Docker image from source, load into Kind
make e2e-kind-build

# 3. Deploy chain (init genesis, create validators, wait for blocks)
make e2e-kind-deploy

# 4. Run tests
make e2e-kind-test

# 5. Cleanup
make e2e-kind-cleanup
```

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │           Kind Cluster (moca-e2e)       │
                    │                                         │
                    │  ┌──────────┐  ┌──────────┐             │
                    │  │validator-0│  │validator-1│             │
                    │  │ (mocad)  │  │ (mocad)  │  ...        │
                    │  └────┬─────┘  └────┬─────┘             │
                    │       │ P2P (26656) │                   │
                    │       └──────┬──────┘                   │
                    │              │                           │
                    │  ┌───────────┴──────────┐               │
                    │  │ validator-headless    │               │
                    │  │ (Headless Service)    │               │
                    │  └──────────────────────┘               │
                    │                                         │
                    │  ┌──────────────────────┐               │
                    │  │ validator-nodeport    │               │
                    │  │ (NodePort Service)    │               │
                    │  └──────────┬───────────┘               │
                    └─────────────┼───────────────────────────┘
                                  │
                    localhost:26657 (RPC)
                    localhost:9090  (gRPC)
                    localhost:8545  (EVM)
```

### Flow

1. **setup-kind.sh** — Creates a Kind cluster with port mappings
2. **build-images.sh** — Multi-stage Docker build from source, loads into Kind
3. **init-chain.sh** — Runs as a K8s Job inside the cluster:
   - Generates validator keys (validator, BLS, relayer, challenger)
   - Generates SP keys (operator, fund, seal, BLS, approval, gc, maintenance)
   - Creates genesis accounts, gentx, spgentx
   - Configures persistent peers using K8s DNS
   - Applies test timeouts (1s commit, 15s voting period)
4. **deploy.sh** — Extracts configs from init Job, creates per-validator ConfigMaps/Secrets, deploys StatefulSets
   with writable config volumes
5. **run-tests.sh** — Executes test cases via `kubectl exec` and RPC queries

## Test Cases

### Validator parity (moca-devcontainer)

`moca-devcontainer` runs `test/validator/check-validators.sh test`
(per-validator RPC, sync, voting power, timed block production). The Kind suite mirrors that with:

```bash
make e2e-fw-test TEST=validator_devcontainer_parity
```

Optional tuning (same defaults as devcontainer): `CHECK_INTERVAL`, `MAX_WAIT`, `MIN_BLOCKS`.

### RPC + staking parity (`tests/test_rpc_suite.sh`)

Aligns with `moca-devcontainer/test/validator/RPC/rpc.sh` (EVM/CometBFT checks) and
`check-validators.sh` subcommands `balances` / `validators`.

```bash
make e2e-fw-test TEST=rpc_suite
```

| Kind test | devcontainer source |
|-----------|---------------------|
| EVM HTTP connectivity | `RPC/rpc.sh` connectivity |
| CometBFT `/status`, `/health` | `RPC/rpc.sh` status, health |
| `eth_blockNumber` JSON-RPC 2.0 | `RPC/rpc.sh` jsonrpc |
| EVM block timestamp + monotonic height | `RPC/rpc.sh` blocks |
| Forge `TestERC20` deploy + transfer | `RPC/rpc.sh` erc20 |
| Validator operator `bank` balances | `check-validators.sh` balances |
| Staking validators count + moniker | `check-validators.sh` validators |

Comprehensive upgrade (`test_upgrade_comprehensive.sh`) also runs **`mod_validator.sh`** checks including
validator height spread (<= 2) and on-chain staking count vs `NUM_VALIDATORS` (parity with devcontainer
upgrade validator verification; cosmovisor checks are N/A in Kind).

### Smoke Suite

| Test | Description |
|------|-------------|
| Chain producing blocks | Verifies block height increases over time |
| Genesis account balances | Checks validator0 has non-zero amoca balance |
| Query validators | Verifies expected number of validators registered |
| Query module params | Queries EVM, feemarket, staking, gov params |
| Send tokens | Sends 1 MOCA between accounts, verifies balance |
| Multi-account transfers | Creates N accounts, chain of transfers, verifies |
| Query storage providers | Verifies SPs registered in genesis |

### Upgrade Suite

| Test | Description |
|------|-------------|
| Chain producing blocks post-upgrade | Chain continues after upgrade |
| Height past upgrade height | Block height surpassed the upgrade trigger |
| Balances preserved | Pre-upgrade account balances survived the upgrade |
| Upgrade handler applied | `mocad query upgrade applied` confirms execution |
| Token transfers post-upgrade | Bank send works with the new binary |

### Comprehensive upgrade (`tests/test_upgrade_comprehensive.sh`)

Drop-in modules under `tests/modules/mod_*.sh` are auto-loaded. They register randomized pre/post-upgrade
transactions and post-upgrade verification hooks.

| Module | Role |
|--------|------|
| `mod_bank.sh` | Bank sends and balance checks |
| `mod_staking.sh` | Staking operations |
| `mod_gov.sh` | Governance proposals |
| `mod_distribution.sh` | Distribution queries |
| `mod_evm.sh` | EVM transfers and ERC20 |
| `mod_validator.sh` | Same checks on **every** validator pod (RPC, sync, voting power, height growth, height spread, on-chain validator count) |

## Configuration

Edit `e2e.env` to change:

| Variable | Default | Description |
|----------|---------|-------------|
| `NUM_VALIDATORS` | 4 | Number of validator nodes |
| `NUM_STORAGE_PROVIDERS` | 1 | Number of storage providers |
| `CHAIN_ID` | moca_5151-1 | Chain ID |
| `DEPOSIT_VOTE_PERIOD` | 15s | Governance voting period |
| `GOV_MIN_DEPOSIT_AMOUNT` | 10000000000000000 | Min governance deposit (0.01 MOCA) |
| `DOCKER_IMAGE` | mocachain/moca | Docker image name |
| `DOCKER_TAG` | e2e-local | Docker image tag |

### Chain Defaults

- **Pruning**: `default` (keeps last 362880 states)
- **Block time**: ~1s (`timeout_commit = 1s`)
- **Voting period**: 15s
- **Min gas price**: 5000000000 amoca

## Debugging

```bash
# Check pod status
kubectl -n moca-e2e get pods -o wide

# View validator logs
kubectl -n moca-e2e logs validator-0-0 -c mocad --tail=100

# Describe pod for events
kubectl -n moca-e2e describe pod validator-0-0

# Exec into validator
kubectl -n moca-e2e exec -it validator-0-0 -c mocad -- /bin/bash

# Query chain status
curl http://localhost:26657/status | jq .

# Query latest block
curl http://localhost:26657/block | jq .result.block.header.height

# Check connected peers
curl http://localhost:26657/net_info | jq '.result.n_peers'
```

## CI

The GitHub Actions workflow (`.github/workflows/e2e-kind.yml`) runs automatically on PRs to `main`. It:

1. Installs Kind
2. Builds Docker image and deploys to Kind
4. Runs all e2e tests
5. Collects debug logs on failure
6. Cleans up the Kind cluster

## Storage Provider (Optional)

SP deployment requires the `moca-sp` Docker image. Set `MOCA_SP_IMAGE` to enable:

```bash
MOCA_SP_IMAGE=mocafoundation/moca-sp:latest make e2e-kind-build
```

The SP connects to:

- MySQL at `mysql.moca-e2e.svc.cluster.local:3306`
- Validator RPC at `validator-0-0.validator-headless.moca-e2e.svc.cluster.local:26657`
