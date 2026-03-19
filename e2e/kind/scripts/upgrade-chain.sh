#!/usr/bin/env bash
# Upgrades the Moca chain running in Kind to a new version.
#
# Required env vars:
#   UPGRADE_NAME       - Name of the upgrade handler (must match a registered handler in the new binary)
#   UPGRADE_HEIGHT     - Block height at which to upgrade
#   UPGRADE_MODE       - "hardfork" or "governance"
#   NEW_DOCKER_IMAGE   - Full image:tag for the new binary
#
# For governance mode, the script submits an upgrade proposal, votes YES with
# all validators, and waits for the upgrade height. For hardfork mode, it simply
# waits for the upgrade height then patches all StatefulSets.

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

: "${UPGRADE_NAME:?UPGRADE_NAME is required}"
: "${UPGRADE_HEIGHT:?UPGRADE_HEIGHT is required}"
: "${UPGRADE_MODE:?UPGRADE_MODE must be 'hardfork' or 'governance'}"
: "${NEW_DOCKER_IMAGE:?NEW_DOCKER_IMAGE is required}"

log_info "=== Chain Upgrade ==="
log_info "  Mode:   ${UPGRADE_MODE}"
log_info "  Name:   ${UPGRADE_NAME}"
log_info "  Height: ${UPGRADE_HEIGHT}"
log_info "  Image:  ${NEW_DOCKER_IMAGE}"

# ── Governance mode ──────────────────────────────────────────────────────────
_upgrade_governance() {
    local fees="200000000000000amoca"

    log_info "Submitting software-upgrade proposal..."

    # Get the gov module authority address
    local gov_authority=""
    gov_authority=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad query auth module-account gov \
        --node tcp://localhost:26657 \
        --home /root/.mocad \
        --output json 2>/dev/null | jq -r '.account.base_account.address // .account.value.address // empty' 2>/dev/null) || true

    if [ -z "$gov_authority" ]; then
        log_warn "Could not query gov module address, trying alternative..."
        gov_authority=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
            mocad query auth module-accounts \
            --node tcp://localhost:26657 \
            --home /root/.mocad \
            --output json 2>/dev/null | jq -r '.accounts[] | select(.name=="gov") | .base_account.address // .value.address // empty' 2>/dev/null) || true
    fi

    if [ -z "$gov_authority" ]; then
        log_error "Could not determine gov module authority address"
        return 1
    fi

    log_info "Gov module authority: ${gov_authority}"

    # Write proposal JSON (SDK v0.50+ format)
    local proposal_json
    proposal_json=$(cat <<PROPOSAL_EOF
{
  "messages": [
    {
      "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
      "authority": "${gov_authority}",
      "plan": {
        "name": "${UPGRADE_NAME}",
        "height": "${UPGRADE_HEIGHT}",
        "info": "E2E test upgrade"
      }
    }
  ],
  "deposit": "${GOV_MIN_DEPOSIT_AMOUNT}${BASIC_DENOM}",
  "title": "Upgrade to ${UPGRADE_NAME}",
  "summary": "E2E test software upgrade to ${UPGRADE_NAME} at height ${UPGRADE_HEIGHT}"
}
PROPOSAL_EOF
    )

    # Write proposal file into the pod
    echo "$proposal_json" | kubectl exec -i -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        bash -c "cat > /tmp/upgrade-proposal.json" || true

    # Submit proposal
    local submit_out=""
    submit_out=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad tx gov submit-proposal /tmp/upgrade-proposal.json \
        --from validator0 \
        --keyring-backend test \
        --chain-id "${CHAIN_ID}" \
        --node tcp://localhost:26657 \
        --fees "$fees" \
        --home /root/.mocad \
        -y 2>&1) || true
    echo "$submit_out"

    sleep 5

    # Find the proposal ID (latest proposal)
    local proposal_id=""
    local query_out=""
    query_out=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad query gov proposals \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --home /root/.mocad \
        --output json 2>&1) || true

    proposal_id=$(echo "$query_out" | jq -r '.proposals[-1].id // .proposals[-1].proposal_id // empty' 2>/dev/null) || true

    if [ -z "$proposal_id" ]; then
        log_error "Could not find upgrade proposal"
        log_error "Query output: ${query_out}"
        return 1
    fi

    log_info "Proposal ID: ${proposal_id}"

    # Vote YES from all validators
    for ((i = 0; i < NUM_VALIDATORS; i++)); do
        log_info "  validator${i} voting YES..."
        kubectl exec -n "${K8S_NAMESPACE}" "validator-${i}-0" -c mocad -- \
            mocad tx gov vote "$proposal_id" yes \
            --from "validator${i}" \
            --keyring-backend test \
            --chain-id "${CHAIN_ID}" \
            --node tcp://localhost:26657 \
            --fees "$fees" \
            --home /root/.mocad \
            -y 2>&1 || true
        sleep 2
    done

    log_info "Waiting for voting period to end and upgrade height ${UPGRADE_HEIGHT}..."
    _wait_for_upgrade_halt
}

# ── Hardfork mode ────────────────────────────────────────────────────────────
_upgrade_hardfork() {
    log_info "Waiting for upgrade height ${UPGRADE_HEIGHT}..."
    _wait_for_upgrade_halt
}

# ── Common: wait for chain to halt at upgrade height, then restart ───────────
_wait_for_upgrade_halt() {
    local max_wait=300
    local elapsed=0

    while [ $elapsed -lt $max_wait ]; do
        local height
        height=$(get_block_height "http://localhost:26657" 2>/dev/null || echo "0")

        if [ "$height" -ge "$UPGRADE_HEIGHT" ] 2>/dev/null; then
            log_info "Chain reached upgrade height (current: ${height})"
            break
        fi

        # Check if chain has halted (height stuck — process logs consensus failures but doesn't exit)
        if [ "$height" -ge "$((UPGRADE_HEIGHT - 1))" ] 2>/dev/null; then
            log_info "Chain approaching upgrade height (current: ${height}), waiting for halt..."
            sleep 10
            local height2
            height2=$(get_block_height "http://localhost:26657" 2>/dev/null || echo "0")
            if [ "$height2" = "$height" ] || [ "$height2" = "0" ]; then
                log_info "Chain halted at height ${height}"
                break
            fi
        fi

        elapsed=$((elapsed + 3))
        sleep 3
    done

    if [ $elapsed -ge $max_wait ]; then
        log_warn "Timeout waiting for upgrade height, proceeding with image update anyway"
    fi

    # Chain halts but doesn't exit — scale down, replace binary, scale back up
    log_info "Scaling down validators..."
    for ((i = 0; i < NUM_VALIDATORS; i++)); do
        kubectl scale statefulset "validator-${i}" --replicas=0 -n "${K8S_NAMESPACE}" 2>/dev/null || true
    done
    kubectl wait --for=delete pod -l app=validator -n "${K8S_NAMESPACE}" --timeout=60s 2>/dev/null || true
    log_info "All validators stopped"

    # Update all validator StatefulSets to use the new image, then scale back up
    _update_validator_images
}

_update_validator_images() {
    log_info "Updating validator images to ${NEW_DOCKER_IMAGE}..."

    # Load new image into Kind if not already loaded
    kind load docker-image "${NEW_DOCKER_IMAGE}" --name "${KIND_CLUSTER_NAME}" 2>/dev/null || true

    # Patch each validator StatefulSet with the new image
    for ((i = 0; i < NUM_VALIDATORS; i++)); do
        log_info "  Updating validator-${i}..."

        # Patch both initContainer and main container images
        kubectl set image statefulset/"validator-${i}" \
            init-config="${NEW_DOCKER_IMAGE}" \
            mocad="${NEW_DOCKER_IMAGE}" \
            -n "${K8S_NAMESPACE}"
    done

    # Scale validators back up with new image
    log_info "Starting validators with new image..."
    for ((i = 0; i < NUM_VALIDATORS; i++)); do
        kubectl scale statefulset "validator-${i}" --replicas=1 -n "${K8S_NAMESPACE}" 2>/dev/null || true
    done

    # Wait for all validators to come back up
    log_info "Waiting for validators to be ready with new image..."
    for ((i = 0; i < NUM_VALIDATORS; i++)); do
        log_info "  Waiting for validator-${i}..."
        kubectl wait --for=condition=ready "pod/validator-${i}-0" \
            -n "${K8S_NAMESPACE}" --timeout=180s 2>/dev/null || {
            log_error "Validator-${i} failed to restart"
            kubectl logs -n "${K8S_NAMESPACE}" "validator-${i}-0" --tail=50 2>/dev/null || true
            return 1
        }
    done

    log_success "All validators restarted with new image"

    # Wait for chain to resume producing blocks
    wait_for_chain_ready "http://localhost:26657" 180
    log_success "Chain resumed after upgrade"
}

# ── Main ─────────────────────────────────────────────────────────────────────
case "${UPGRADE_MODE}" in
    governance)
        _upgrade_governance
        ;;
    hardfork)
        _upgrade_hardfork
        ;;
    *)
        log_error "Unknown UPGRADE_MODE: ${UPGRADE_MODE} (expected 'hardfork' or 'governance')"
        exit 1
        ;;
esac

log_success "=== Upgrade complete ==="
