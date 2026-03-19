#!/usr/bin/env bash
# init-chain.sh — Runs inside the K8s genesis-init Job container.
# Initializes a multi-validator Moca chain, producing a ready-to-use
# DATA_DIR tree that is later distributed to each validator pod.
set -euo pipefail

###############################################################################
# Defaults (overridable via env)
###############################################################################
NUM_VALIDATORS="${NUM_VALIDATORS:-4}"
NUM_STORAGE_PROVIDERS="${NUM_STORAGE_PROVIDERS:-1}"
DATA_DIR="${DATA_DIR:-/data}"
CHAIN_ID="${CHAIN_ID:-moca_5151-1}"

###############################################################################
# Chain parameters (mirrors e2e.env / deployment/localup/.env)
###############################################################################
STAKING_BOND_DENOM="${STAKING_BOND_DENOM:-amoca}"
BASIC_DENOM="${BASIC_DENOM:-amoca}"
STAKING_BOND_AMOUNT="${STAKING_BOND_AMOUNT:-10000000000000000000000000}"
GENESIS_ACCOUNT_BALANCE="${GENESIS_ACCOUNT_BALANCE:-1000000000000000000000000000}"
COMMISSION_MAX_CHANGE_RATE="${COMMISSION_MAX_CHANGE_RATE:-0.01}"
COMMISSION_MAX_RATE="${COMMISSION_MAX_RATE:-1.0}"
COMMISSION_RATE="${COMMISSION_RATE:-0.07}"
NATIVE_COIN_DESC='{"description":"The native staking token of the Moca.","denom_units":[{"denom":"amoca","exponent":0,"aliases":["wei"]}],"base":"amoca","display":"amoca"}'
DEPOSIT_VOTE_PERIOD="${DEPOSIT_VOTE_PERIOD:-15s}"
GOV_MIN_DEPOSIT_AMOUNT="${GOV_MIN_DEPOSIT_AMOUNT:-10000000000000000}"
SP_MIN_DEPOSIT_AMOUNT="${SP_MIN_DEPOSIT_AMOUNT:-10000000000000000000000000}"

# Cross-chain IDs
SRC_CHAIN_ID="${SRC_CHAIN_ID:-5151}"
DEST_CHAIN_ID="${DEST_CHAIN_ID:-97}"
DEST_OP_CHAIN_ID="${DEST_OP_CHAIN_ID:-5611}"
DEST_POLYGON_CHAIN_ID="${DEST_POLYGON_CHAIN_ID:-137}"
DEST_SCROLL_CHAIN_ID="${DEST_SCROLL_CHAIN_ID:-534352}"
DEST_LINEA_CHAIN_ID="${DEST_LINEA_CHAIN_ID:-59144}"
DEST_MANTLE_CHAIN_ID="${DEST_MANTLE_CHAIN_ID:-5000}"
DEST_ARBITRUM_CHAIN_ID="${DEST_ARBITRUM_CHAIN_ID:-42161}"
DEST_OPTIMISM_CHAIN_ID="${DEST_OPTIMISM_CHAIN_ID:-10}"
DEST_BASE_CHAIN_ID="${DEST_BASE_CHAIN_ID:-8453}"

# Snapshot config
SNAPSHOT_INTERVAL="${SNAPSHOT_INTERVAL:-10}"
SNAPSHOT_KEEP_RECENT="${SNAPSHOT_KEEP_RECENT:-0}"

###############################################################################
# Preset private keys (validator0 / devaccount / relayer0 / sp0)
###############################################################################
DEVACCOUNT_PRIKEY="${DEVACCOUNT_PRIKEY:-2228e392584d902843272c37fd62b8c73c10c81a5ecb901773c9ebe366e937bb}"
VALIDATOR0_PRIKEY="${VALIDATOR0_PRIKEY:-e54bff83fc945cba77ca3e45d69adc5b57ad8db6073736c8422692abecfb5fe2}"
RELAYER0_PRIKEY="${RELAYER0_PRIKEY:-3c7ea76ddb53539174caae1dd960b308981933bd6e95196556ba29063200df9c}"
SP0_PRIKEY="${SP0_PRIKEY:-ebbeb28b89bc7ec5da6441ed70452cc413f96ea33a7c790aba06810ae441b776}"

###############################################################################
# Pre-allocated test addresses
###############################################################################
TEST_ADDRS=(
  "0x1111102dd32160b064f2a512cdef74bfdb6a9f96"
  "0x2222207b1f7b8d37566d9a2778732451dbfbc5d0"
  "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
  "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
  "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
  "0x90F79bf6EB2c4f870365E785982E1f101E93b906"
  "0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"
  "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"
  "0x976EA74026E726554dB657fA54763abd0C3a0aa9"
  "0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
  "0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f"
  "0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"
  "0x000000000000000000000000000000000000dead"
)

###############################################################################
# K8s DNS
###############################################################################
K8S_NAMESPACE="${K8S_NAMESPACE:-moca-e2e}"

# Build K8s persistent-peer DNS name for validator i
peer_dns() {
  local idx=$1
  echo "validator-${idx}-0.validator-headless.${K8S_NAMESPACE}.svc.cluster.local"
}

###############################################################################
# Binary
###############################################################################
BIN="mocad"

###############################################################################
# Helpers
###############################################################################
log() { echo "[init-chain] $(date -u +%H:%M:%S) $*"; }

joinByString() {
  local sep="$1"; shift
  local first="$1"; shift
  printf "%s" "$first" "${@/#/$sep}"
}

###############################################################################
# 1. INIT — initialise directories, chain config, and keyrings
###############################################################################
init() {
  local size=$NUM_VALIDATORS
  local sp_size=$NUM_STORAGE_PROVIDERS

  log "Initializing ${size} validators and ${sp_size} storage providers"

  for ((i = 0; i < size; i++)); do
    mkdir -p "${DATA_DIR}/validator${i}"
    mkdir -p "${DATA_DIR}/relayer${i}"
    mkdir -p "${DATA_DIR}/challenger${i}"

    # init chain
    ${BIN} init "validator${i}" \
      --chain-id "${CHAIN_ID}" \
      --default-denom "${STAKING_BOND_DENOM}" \
      --home "${DATA_DIR}/validator${i}"

    # --- key generation ---
    if [ "$i" -eq 0 ]; then
      log "Importing preset keys for validator0"
      ${BIN} keys import devaccount "${DEVACCOUNT_PRIKEY}" \
        --secp256k1-private-key --keyring-backend test \
        --home "${DATA_DIR}/validator0"
      ${BIN} keys import validator0 "${VALIDATOR0_PRIKEY}" \
        --secp256k1-private-key --keyring-backend test \
        --home "${DATA_DIR}/validator0"
      ${BIN} keys import relayer0 "${RELAYER0_PRIKEY}" \
        --secp256k1-private-key --keyring-backend test \
        --home "${DATA_DIR}/relayer0"
    else
      log "Generating keys for validator${i}"
      ${BIN} keys add "validator${i}" \
        --keyring-backend test --home "${DATA_DIR}/validator${i}"
      ${BIN} keys add "relayer${i}" \
        --keyring-backend test --home "${DATA_DIR}/relayer${i}"
    fi

    # BLS key (all validators)
    ${BIN} keys add "validator_bls${i}" \
      --keyring-backend test --home "${DATA_DIR}/validator${i}" --algo eth_bls

    # Challenger key
    ${BIN} keys add "challenger${i}" \
      --keyring-backend test --home "${DATA_DIR}/challenger${i}"
  done

  # --- Storage Provider keys ---
  for ((i = 0; i < sp_size; i++)); do
    mkdir -p "${DATA_DIR}/sp${i}"
    if [ "$i" -eq 0 ]; then
      log "Importing preset keys for sp0"
      ${BIN} keys import sp0 "${SP0_PRIKEY}" \
        --secp256k1-private-key --keyring-backend test \
        --home "${DATA_DIR}/sp0"
    else
      log "Generating keys for sp${i}"
      ${BIN} keys add "sp${i}" \
        --keyring-backend test --home "${DATA_DIR}/sp${i}"
    fi
    ${BIN} keys add "sp${i}_fund" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}"
    ${BIN} keys add "sp${i}_seal" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}"
    ${BIN} keys add "sp${i}_bls" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}" --algo eth_bls
    ${BIN} keys add "sp${i}_approval" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}"
    ${BIN} keys add "sp${i}_gc" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}"
    ${BIN} keys add "sp${i}_maintenance" \
      --keyring-backend test --home "${DATA_DIR}/sp${i}"
  done

  log "Initialization complete"
}

###############################################################################
# 2. GENERATE GENESIS
###############################################################################
generate_genesis() {
  local size=$NUM_VALIDATORS
  local sp_size=$NUM_STORAGE_PROVIDERS

  log "Generating genesis (validators=${size}, sps=${sp_size})"

  # --- Collect addresses ---
  declare -a validator_addrs=()
  for ((i = 0; i < size; i++)); do
    validator_addrs+=("$(${BIN} keys show "validator${i}" -a --keyring-backend test --home "${DATA_DIR}/validator${i}")")
  done

  declare -a relayer_addrs=()
  for ((i = 0; i < size; i++)); do
    relayer_addrs+=("$(${BIN} keys show "relayer${i}" -a --keyring-backend test --home "${DATA_DIR}/relayer${i}")")
  done

  declare -a challenger_addrs=()
  for ((i = 0; i < size; i++)); do
    challenger_addrs+=("$(${BIN} keys show "challenger${i}" -a --keyring-backend test --home "${DATA_DIR}/challenger${i}")")
  done

  # --- Fund genesis accounts ---
  mkdir -p "${DATA_DIR}/gentx"

  for ((i = 0; i < size; i++)); do
    # On validator0, pre-allocate test addresses and devaccount
    if [ "$i" -eq 0 ]; then
      for addr in "${TEST_ADDRS[@]}"; do
        ${BIN} add-genesis-account "$addr" \
          "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" \
          --home "${DATA_DIR}/validator${i}"
      done
      local devaccount_addr
      devaccount_addr=$(${BIN} keys show devaccount -a --keyring-backend test --home "${DATA_DIR}/validator0")
      ${BIN} add-genesis-account "${devaccount_addr}" \
        "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" \
        --home "${DATA_DIR}/validator${i}"
    fi

    # Add all validator addresses as genesis accounts
    for validator_addr in "${validator_addrs[@]}"; do
      ${BIN} add-genesis-account "$validator_addr" \
        "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" \
        --home "${DATA_DIR}/validator${i}"
    done

    # Add all relayer addresses as genesis accounts
    for relayer_addr in "${relayer_addrs[@]}"; do
      ${BIN} add-genesis-account "$relayer_addr" \
        "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" \
        --home "${DATA_DIR}/validator${i}"
    done

    # Add all challenger addresses as genesis accounts
    for challenger_addr in "${challenger_addrs[@]}"; do
      ${BIN} add-genesis-account "$challenger_addr" \
        "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" \
        --home "${DATA_DIR}/validator${i}"
    done

    # Clean any stale gentx dir on this validator
    rm -rf "${DATA_DIR}/validator${i}/config/gentx/"

    # --- Create gentx ---
    local validatorAddr="${validator_addrs[$i]}"
    local delegatorAddr="${validator_addrs[$i]}"
    local relayerAddr
    relayerAddr="$(${BIN} keys show "relayer${i}" -a --keyring-backend test --home "${DATA_DIR}/relayer${i}")"
    local challengerAddr
    challengerAddr="$(${BIN} keys show "challenger${i}" -a --keyring-backend test --home "${DATA_DIR}/challenger${i}")"
    local blsKey
    blsKey="$(${BIN} keys show "validator_bls${i}" --keyring-backend test --home "${DATA_DIR}/validator${i}" --output json | jq -r .pubkey_hex)"
    local blsProof
    blsProof="$(${BIN} keys sign "${blsKey}" --from "validator_bls${i}" --keyring-backend test --home "${DATA_DIR}/validator${i}")"

    ${BIN} gentx "validator${i}" \
      "${STAKING_BOND_AMOUNT}${STAKING_BOND_DENOM}" \
      "$validatorAddr" "$delegatorAddr" "$relayerAddr" "$challengerAddr" \
      "$blsKey" "$blsProof" \
      --home "${DATA_DIR}/validator${i}" \
      --keyring-backend=test \
      --chain-id="${CHAIN_ID}" \
      --moniker="validator${i}" \
      --commission-max-change-rate="${COMMISSION_MAX_CHANGE_RATE}" \
      --commission-max-rate="${COMMISSION_MAX_RATE}" \
      --commission-rate="${COMMISSION_RATE}" \
      --details="validator${i}" \
      --website="http://website" \
      --node "tcp://localhost:26657" \
      --node-id "validator${i}" \
      --ip "127.0.0.1" \
      --gas ""

    cp "${DATA_DIR}/validator${i}/config/gentx/gentx-validator${i}.json" "${DATA_DIR}/gentx/"
  done

  # --- Collect gentxs on each validator ---
  local node_ids=""
  for ((i = 0; i < size; i++)); do
    cp "${DATA_DIR}"/gentx/* "${DATA_DIR}/validator${i}/config/gentx/"
    ${BIN} collect-gentxs --home "${DATA_DIR}/validator${i}"
    local node_id
    node_id="$(${BIN} tendermint show-node-id --home "${DATA_DIR}/validator${i}")"
    node_ids="${node_id}@$(peer_dns ${i}):26656 ${node_ids}"
  done

  # --- Generate SP genesis ---
  generate_sp_genesis "$size" "$sp_size"

  # --- Build persistent peers string ---
  local persistent_peers
  persistent_peers=$(joinByString ',' ${node_ids})

  # --- Apply config patches to every validator ---
  for ((i = 0; i < size; i++)); do
    # Copy canonical genesis from validator0 to others
    if [ "$i" -gt 0 ]; then
      cp "${DATA_DIR}/validator0/config/genesis.json" "${DATA_DIR}/validator${i}/config/"
    fi

    local genesis="${DATA_DIR}/validator${i}/config/genesis.json"
    local config="${DATA_DIR}/validator${i}/config/config.toml"
    local app="${DATA_DIR}/validator${i}/config/app.toml"
    local client="${DATA_DIR}/validator${i}/config/client.toml"

    # ---- genesis.json patches ----
    sed -i "s/\"stake\"/\"${BASIC_DENOM}\"/g" "$genesis"
    # Use jq for denom_metadata (sed can't safely handle nested JSON)
    jq --argjson meta "[${NATIVE_COIN_DESC}]" '.app_state.bank.denom_metadata = $meta' "$genesis" > "${genesis}.tmp" && mv "${genesis}.tmp" "$genesis"
    sed -i "s/\"reserve_time\": \"15552000\"/\"reserve_time\": \"60\"/g" "$genesis"
    sed -i "s/\"forced_settle_time\": \"86400\"/\"forced_settle_time\": \"30\"/g" "$genesis"
    sed -i "s/\"signed_blocks_window\": \"100\"/\"signed_blocks_window\": \"10000\"/g" "$genesis"
    sed -i "s/\"min_gas_price\": \"0.000000000000000000\"/\"min_gas_price\": \"1000000000.000000000000000000\"/g" "$genesis"
    sed -i "s/172800s/${DEPOSIT_VOTE_PERIOD}/g" "$genesis"
    sed -i "s/\"10000000\"/\"${GOV_MIN_DEPOSIT_AMOUNT}\"/g" "$genesis"
    sed -i "s/\"max_bytes\": \"22020096\"/\"max_bytes\": \"1048576\"/g" "$genesis"
    sed -i "s/\"challenge_count_per_block\": \"1\"/\"challenge_count_per_block\": \"5\"/g" "$genesis"
    sed -i "s/\"challenge_keep_alive_period\": \"300\"/\"challenge_keep_alive_period\": \"10\"/g" "$genesis"
    sed -i "s/\"heartbeat_interval\": \"1000\"/\"heartbeat_interval\": \"100\"/g" "$genesis"
    sed -i "s/\"attestation_inturn_interval\": \"120\"/\"attestation_inturn_interval\": \"10\"/g" "$genesis"
    sed -i "s/\"discontinue_confirm_period\": \"604800\"/\"discontinue_confirm_period\": \"5\"/g" "$genesis"
    sed -i "s/\"discontinue_deletion_max\": \"100\"/\"discontinue_deletion_max\": \"2\"/g" "$genesis"
    sed -i "s/\"update_global_price_interval\": \"0\"/\"update_global_price_interval\": \"1\"/g" "$genesis"
    sed -i "s/\"update_price_disallowed_days\": 2/\"update_price_disallowed_days\": 0/g" "$genesis"
    sed -i "s/\"redundant_data_chunk_num\": 4/\"redundant_data_chunk_num\": 1/g" "$genesis"
    sed -i "s/\"redundant_parity_chunk_num\": 2/\"redundant_parity_chunk_num\": 1/g" "$genesis"

    # ---- config.toml patches (fast block times, peers) ----
    sed -i "s/seeds = \"[^\"]*\"/seeds = \"\"/g" "$config"
    sed -i "s/persistent_peers = \".*\"/persistent_peers = \"${persistent_peers}\"/g" "$config"
    sed -i "s/timeout_propose = \"3s\"/timeout_propose = \"600ms\"/g" "$config"
    sed -i "s/timeout_propose_delta = \"500ms\"/timeout_propose_delta = \"200ms\"/g" "$config"
    sed -i "s/timeout_prevote = \"1s\"/timeout_prevote = \"500ms\"/g" "$config"
    sed -i "s/timeout_prevote_delta = \"500ms\"/timeout_prevote_delta = \"200ms\"/g" "$config"
    sed -i "s/timeout_precommit = \"1s\"/timeout_precommit = \"500ms\"/g" "$config"
    sed -i "s/timeout_precommit_delta = \"500ms\"/timeout_precommit_delta = \"200ms\"/g" "$config"
    sed -i "s/timeout_commit = \"3s\"/timeout_commit = \"500ms\"/g" "$config"
    sed -i "s/addr_book_strict = true/addr_book_strict = false/g" "$config"
    sed -i "s/allow_duplicate_ip = false/allow_duplicate_ip = true/g" "$config"
    sed -i "s/log_level = \"info\"/log_level = \"debug\"/g" "$config"
    sed -i "s/cors_allowed_origins = \[\]/cors_allowed_origins = \[\"*\"\]/g" "$config"

    # ---- app.toml patches ----
    sed -i "s/minimum-gas-prices = \"0amoca\"/minimum-gas-prices = \"5000000000${BASIC_DENOM}\"/g" "$app"
    sed -i "s/snapshot-interval = 0/snapshot-interval = ${SNAPSHOT_INTERVAL}/g" "$app"
    sed -i "s/snapshot-keep-recent = 2/snapshot-keep-recent = ${SNAPSHOT_KEEP_RECENT}/g" "$app"
    sed -i "s/src-chain-id = 1/src-chain-id = ${SRC_CHAIN_ID}/g" "$app"
    sed -i "s/dest-bsc-chain-id = 2/dest-bsc-chain-id = ${DEST_CHAIN_ID}/g" "$app"
    sed -i "s/dest-op-chain-id = 3/dest-op-chain-id = ${DEST_OP_CHAIN_ID}/g" "$app"
    sed -i "s/dest-polygon-chain-id = 4/dest-polygon-chain-id = ${DEST_POLYGON_CHAIN_ID}/g" "$app"
    sed -i "s/dest-scroll-chain-id = 5/dest-scroll-chain-id = ${DEST_SCROLL_CHAIN_ID}/g" "$app"
    sed -i "s/dest-linea-chain-id = 6/dest-linea-chain-id = ${DEST_LINEA_CHAIN_ID}/g" "$app"
    sed -i "s/dest-mantle-chain-id = 7/dest-mantle-chain-id = ${DEST_MANTLE_CHAIN_ID}/g" "$app"
    sed -i "s/dest-arbitrum-chain-id = 8/dest-arbitrum-chain-id = ${DEST_ARBITRUM_CHAIN_ID}/g" "$app"
    sed -i "s/dest-optimism-chain-id = 9/dest-optimism-chain-id = ${DEST_OPTIMISM_CHAIN_ID}/g" "$app"
    sed -i "s/dest-base-chain-id = 10/dest-base-chain-id = ${DEST_BASE_CHAIN_ID}/g" "$app"
    sed -i "s/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g" "$app"
    sed -i "s/pruning = \"default\"/pruning = \"nothing\"/g" "$app"
    sed -i "s/eth,net,web3/eth,txpool,personal,net,debug,web3/g" "$app"

    # ---- client.toml — point at standard RPC port ----
    sed -i "s#node = \"tcp://localhost:26657\"#node = \"tcp://localhost:26657\"#g" "$client"
  done

  # Enable swagger API and telemetry for validator0
  sed -i "/Enable defines if the API server should be enabled/{N;s/enable = false/enable = true/;}" \
    "${DATA_DIR}/validator0/config/app.toml"
  sed -i "s/swagger = false/swagger = true/" \
    "${DATA_DIR}/validator0/config/app.toml"
  sed -i "/other sinks such as Prometheus/{N;s/enable = false/enable = true/;}" \
    "${DATA_DIR}/validator0/config/app.toml"

  log "Genesis generation complete"
}

###############################################################################
# 2a. SP GENESIS — add storage providers to genesis state
###############################################################################
generate_sp_genesis() {
  local size=$1
  local sp_size=$2

  log "Generating SP genesis for ${sp_size} storage provider(s)"

  # Fund SP accounts via validator0
  for ((i = 0; i < sp_size; i++)); do
    local spoperator_addr spfund_addr spseal_addr spapproval_addr spgc_addr spmaintenance_addr
    spoperator_addr="$(${BIN} keys show "sp${i}" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spfund_addr="$(${BIN} keys show "sp${i}_fund" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spseal_addr="$(${BIN} keys show "sp${i}_seal" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spapproval_addr="$(${BIN} keys show "sp${i}_approval" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spgc_addr="$(${BIN} keys show "sp${i}_gc" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spmaintenance_addr="$(${BIN} keys show "sp${i}_maintenance" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"

    ${BIN} add-genesis-account "$spoperator_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
    ${BIN} add-genesis-account "$spfund_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
    ${BIN} add-genesis-account "$spseal_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
    ${BIN} add-genesis-account "$spapproval_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
    ${BIN} add-genesis-account "$spgc_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
    ${BIN} add-genesis-account "$spmaintenance_addr" "${GENESIS_ACCOUNT_BALANCE}${STAKING_BOND_DENOM}" --home "${DATA_DIR}/validator0"
  done

  # Generate SP gentxs
  rm -rf "${DATA_DIR}/gensptx"
  mkdir -p "${DATA_DIR}/gensptx"

  for ((i = 0; i < sp_size; i++)); do
    cp "${DATA_DIR}/validator0/config/genesis.json" "${DATA_DIR}/sp${i}/config/"

    local spoperator_addr spfund_addr spseal_addr bls_pub_key bls_proof spapproval_addr spgc_addr spmaintenance_addr
    spoperator_addr="$(${BIN} keys show "sp${i}" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spfund_addr="$(${BIN} keys show "sp${i}_fund" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spseal_addr="$(${BIN} keys show "sp${i}_seal" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    bls_pub_key="$(${BIN} keys show "sp${i}_bls" --keyring-backend test --home "${DATA_DIR}/sp${i}" --output json | jq -r .pubkey_hex)"
    bls_proof="$(${BIN} keys sign "${bls_pub_key}" --from "sp${i}_bls" --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spapproval_addr="$(${BIN} keys show "sp${i}_approval" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spgc_addr="$(${BIN} keys show "sp${i}_gc" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"
    spmaintenance_addr="$(${BIN} keys show "sp${i}_maintenance" -a --keyring-backend test --home "${DATA_DIR}/sp${i}")"

    ${BIN} spgentx "sp${i}" "${SP_MIN_DEPOSIT_AMOUNT}${STAKING_BOND_DENOM}" \
      --home "${DATA_DIR}/sp${i}" \
      --creator="${spoperator_addr}" \
      --operator-address="${spoperator_addr}" \
      --funding-address="${spfund_addr}" \
      --seal-address="${spseal_addr}" \
      --bls-pub-key="${bls_pub_key}" \
      --bls-proof="${bls_proof}" \
      --approval-address="${spapproval_addr}" \
      --gc-address="${spgc_addr}" \
      --maintenance-address="${spmaintenance_addr}" \
      --keyring-backend=test \
      --chain-id="${CHAIN_ID}" \
      --moniker="sp${i}" \
      --details="detail_sp${i}" \
      --website="http://website" \
      --endpoint="http://sp-${i}.${K8S_NAMESPACE}.svc.cluster.local:9033" \
      --node "tcp://localhost:26657" \
      --node-id "sp${i}" \
      --ip "127.0.0.1" \
      --gas "" \
      --output-document="${DATA_DIR}/gensptx/gentx-sp${i}.json"
  done

  rm -rf "${DATA_DIR}/validator0/config/gensptx/"
  mkdir -p "${DATA_DIR}/validator0/config/gensptx"
  cp "${DATA_DIR}"/gensptx/* "${DATA_DIR}/validator0/config/gensptx/"
  ${BIN} collect-spgentxs \
    --gentx-dir "${DATA_DIR}/validator0/config/gensptx" \
    --home "${DATA_DIR}/validator0"

  log "SP genesis complete"
}

###############################################################################
# MAIN
###############################################################################
log "=========================================="
log "Moca E2E Chain Initializer (K8s)"
log "  NUM_VALIDATORS=${NUM_VALIDATORS}"
log "  NUM_STORAGE_PROVIDERS=${NUM_STORAGE_PROVIDERS}"
log "  CHAIN_ID=${CHAIN_ID}"
log "  DATA_DIR=${DATA_DIR}"
log "=========================================="

init
generate_genesis

log "=========================================="
log "DATA_DIR structure ready at ${DATA_DIR}"
log "=========================================="
ls -la "${DATA_DIR}"/

log "Init done"
