#!/usr/bin/env bash
# Deploys the Moca chain to a Kind K8s cluster.
# Usage: ./deploy.sh
# Env:   DEPLOY_IMAGE  Override the container image (default: $DOCKER_IMAGE:$DOCKER_TAG)

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

MANIFESTS_DIR="${E2E_DIR}/manifests/base"
DEPLOY_IMAGE="${DEPLOY_IMAGE:-${DOCKER_IMAGE}:${DOCKER_TAG}}"
INIT_DIR="/tmp/moca-e2e-init"

log_info "=== Deploying Moca chain to Kind cluster ==="
log_info "Image: ${DEPLOY_IMAGE}"
log_info "Namespace: ${K8S_NAMESPACE}"
log_info "Validators: ${NUM_VALIDATORS}"

# ── 1. Clean up previous deployment ─────────────────────────────────────────
log_info "Cleaning up previous deployment..."
if kubectl get namespace "${K8S_NAMESPACE}" &>/dev/null; then
    kubectl delete namespace "${K8S_NAMESPACE}" --wait=true --timeout=120s || true
    # Wait for namespace to be fully removed
    local_wait=0
    while kubectl get namespace "${K8S_NAMESPACE}" &>/dev/null && [ $local_wait -lt 60 ]; do
        sleep 2
        local_wait=$((local_wait + 2))
    done
fi
rm -rf "${INIT_DIR}"
mkdir -p "${INIT_DIR}"

# ── 2. Create namespace ─────────────────────────────────────────────────────
log_info "Creating namespace '${K8S_NAMESPACE}'..."
kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml"

# ── 3. Create ConfigMap from init-chain.sh ───────────────────────────────────
log_info "Creating init-chain-script ConfigMap..."
kubectl create configmap init-chain-script \
    --from-file=init-chain.sh="${SCRIPT_DIR}/init-chain.sh" \
    -n "${K8S_NAMESPACE}"

# ── 4. Apply genesis-init-job (patching the image) ──────────────────────────
log_info "Applying genesis-init job with image ${DEPLOY_IMAGE}..."
sed "s|image: .*|image: ${DEPLOY_IMAGE}|" "${MANIFESTS_DIR}/genesis-init-job.yaml" \
    | kubectl apply -n "${K8S_NAMESPACE}" -f -

# ── 5. Wait for init pod to be ready ────────────────────────────────────────
log_info "Waiting for init pod to be ready..."
# Wait for the pod to exist first (kubectl wait fails if no pods match yet)
pod_wait=0
while ! kubectl get pods -n "${K8S_NAMESPACE}" -l job-name=genesis-init --no-headers 2>/dev/null | grep -q .; do
    if [ $pod_wait -ge 30 ]; then
        log_error "Init pod did not appear within 30s"
        exit 1
    fi
    sleep 2
    pod_wait=$((pod_wait + 2))
done
kubectl wait --for=condition=ready pod \
    -l job-name=genesis-init \
    -n "${K8S_NAMESPACE}" \
    --timeout=120s

POD=$(kubectl get pods -n "${K8S_NAMESPACE}" -l job-name=genesis-init -o jsonpath='{.items[0].metadata.name}')
log_info "Init pod: ${POD}"

# Wait for init-chain.sh to complete (check logs for "Init done")
log_info "Waiting for chain init to complete..."
init_wait=0
while ! kubectl logs -n "${K8S_NAMESPACE}" "${POD}" 2>/dev/null | grep -q "Init done"; do
    if [ $init_wait -ge 120 ]; then
        log_error "Init did not complete within 120s"
        kubectl logs -n "${K8S_NAMESPACE}" "${POD}" --tail=50 || true
        exit 1
    fi
    sleep 3
    init_wait=$((init_wait + 3))
done
log_success "Chain init completed"

# ── 6. Extract configs from init pod ────────────────────────────────────────
log_info "Extracting validator configs from init pod..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    log_info "  Extracting validator${i} config..."
    mkdir -p "${INIT_DIR}/validator${i}/config"
    mkdir -p "${INIT_DIR}/validator${i}/keyring-test"
    mkdir -p "${INIT_DIR}/validator${i}/data"

    # Extract config directory
    kubectl exec -n "${K8S_NAMESPACE}" "${POD}" -- \
        tar cf - -C / "data/validator${i}/config/" 2>/dev/null \
        | tar xf - -C "${INIT_DIR}/" 2>/dev/null
    # Move extracted files to the right place
    if [ -d "${INIT_DIR}/data/validator${i}/config" ]; then
        cp -a "${INIT_DIR}/data/validator${i}/config/." "${INIT_DIR}/validator${i}/config/"
    fi

    # Extract keyring-test directory
    kubectl exec -n "${K8S_NAMESPACE}" "${POD}" -- \
        tar cf - -C / "data/validator${i}/keyring-test/" 2>/dev/null \
        | tar xf - -C "${INIT_DIR}/" 2>/dev/null
    if [ -d "${INIT_DIR}/data/validator${i}/keyring-test" ]; then
        cp -a "${INIT_DIR}/data/validator${i}/keyring-test/." "${INIT_DIR}/validator${i}/keyring-test/"
    fi

    # Extract priv_validator_state.json
    kubectl exec -n "${K8S_NAMESPACE}" "${POD}" -- \
        tar cf - -C / "data/validator${i}/data/priv_validator_state.json" 2>/dev/null \
        | tar xf - -C "${INIT_DIR}/" 2>/dev/null
    if [ -f "${INIT_DIR}/data/validator${i}/data/priv_validator_state.json" ]; then
        cp "${INIT_DIR}/data/validator${i}/data/priv_validator_state.json" \
           "${INIT_DIR}/validator${i}/data/priv_validator_state.json"
    fi
done

# Clean up the nested extraction directory
rm -rf "${INIT_DIR}/data"
log_success "Validator configs extracted to ${INIT_DIR}"

# ── 7. Delete the init job ──────────────────────────────────────────────────
log_info "Deleting genesis-init job..."
kubectl delete job genesis-init -n "${K8S_NAMESPACE}" --wait=true --timeout=60s

# ── 8. Create per-validator ConfigMaps and Secrets ──────────────────────────
log_info "Creating per-validator ConfigMaps and Secrets..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    config_dir="${INIT_DIR}/validator${i}/config"
    keyring_dir="${INIT_DIR}/validator${i}/keyring-test"

    # Build --from-file args for ConfigMap (all files in config dir)
    config_args=()
    for f in "${config_dir}"/*; do
        [ -f "$f" ] && config_args+=(--from-file="$(basename "$f")=${f}")
    done

    log_info "  Creating ConfigMap validator-${i}-config..."
    kubectl create configmap "validator-${i}-config" \
        "${config_args[@]}" \
        -n "${K8S_NAMESPACE}"

    # Build --from-file args for Secret (all files in keyring-test dir)
    keyring_args=()
    for f in "${keyring_dir}"/*; do
        [ -f "$f" ] && keyring_args+=(--from-file="$(basename "$f")=${f}")
    done

    log_info "  Creating Secret validator-${i}-keyring..."
    kubectl create secret generic "validator-${i}-keyring" \
        "${keyring_args[@]}" \
        -n "${K8S_NAMESPACE}"
done

# ── 9. Apply validator services (before StatefulSets so DNS is available) ──
log_info "Applying validator services..."
kubectl apply -f "${MANIFESTS_DIR}/validator-services.yaml" -n "${K8S_NAMESPACE}"

# ── 10. Deploy per-validator StatefulSets ───────────────────────────────────
log_info "Deploying validator StatefulSets..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    log_info "  Deploying StatefulSet validator-${i}..."

    cat <<EOF | kubectl apply -n "${K8S_NAMESPACE}" -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: validator-${i}
  namespace: ${K8S_NAMESPACE}
  labels:
    app: validator
    validator-index: "${i}"
spec:
  serviceName: validator-headless
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: validator
      validator-index: "${i}"
  template:
    metadata:
      labels:
        app: validator
        validator-index: "${i}"
    spec:
      terminationGracePeriodSeconds: 10
      initContainers:
        - name: init-config
          image: ${DEPLOY_IMAGE}
          imagePullPolicy: Never
          command: ["/bin/bash", "-c"]
          args:
            - |
              echo "Copying config for validator-${i}..."
              # config and keyring emptyDirs are mounted; copy from readonly ConfigMap/Secret
              cp -a /config-readonly/. /root/.mocad/config/ 2>/dev/null || true
              cp -a /keyring-readonly/. /root/.mocad/keyring-test/ 2>/dev/null || true
              if [ ! -f /root/.mocad/data/priv_validator_state.json ]; then
                echo '{"height":"0","round":0,"step":0}' > /root/.mocad/data/priv_validator_state.json
              fi
              echo "Config init done for validator-${i}"
          volumeMounts:
            - name: config-readonly
              mountPath: /config-readonly
              readOnly: true
            - name: keyring-readonly
              mountPath: /keyring-readonly
              readOnly: true
            - name: config
              mountPath: /root/.mocad/config
            - name: keyring
              mountPath: /root/.mocad/keyring-test
            - name: data
              mountPath: /root/.mocad/data
      containers:
        - name: mocad
          image: ${DEPLOY_IMAGE}
          imagePullPolicy: Never
          command: ["mocad"]
          args:
            - start
            - --home=/root/.mocad
            - --keyring-backend=test
            - --rpc.laddr=tcp://0.0.0.0:26657
            - --rpc.unsafe=true
            - --p2p.laddr=tcp://0.0.0.0:26656
            - --grpc.address=0.0.0.0:9090
            - --address=0.0.0.0:28750
            - --api.enabled-unsafe-cors=true
            - --json-rpc.address=0.0.0.0:8545
            - --json-rpc.ws-address=0.0.0.0:8546
            - --log_format=json
          ports:
            - name: rpc
              containerPort: 26657
            - name: p2p
              containerPort: 26656
            - name: grpc
              containerPort: 9090
            - name: api
              containerPort: 1317
            - name: address
              containerPort: 28750
            - name: evm-rpc
              containerPort: 8545
            - name: evm-ws
              containerPort: 8546
          readinessProbe:
            httpGet:
              path: /status
              port: 26657
            initialDelaySeconds: 10
            periodSeconds: 5
            failureThreshold: 12
          livenessProbe:
            httpGet:
              path: /status
              port: 26657
            initialDelaySeconds: 30
            periodSeconds: 10
            failureThreshold: 6
          resources:
            requests:
              cpu: 250m
              memory: 512Mi
            limits:
              memory: 2Gi
          volumeMounts:
            - name: config
              mountPath: /root/.mocad/config
            - name: keyring
              mountPath: /root/.mocad/keyring-test
            - name: data
              mountPath: /root/.mocad/data
      volumes:
        - name: config-readonly
          configMap:
            name: validator-${i}-config
        - name: keyring-readonly
          secret:
            secretName: validator-${i}-keyring
        - name: config
          emptyDir: {}
        - name: keyring
          emptyDir: {}
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
EOF
done

# Patch nodeport service to select validator-0-0 specifically
log_info "Patching nodeport service to target validator-0-0..."
kubectl patch service validator-nodeport -n "${K8S_NAMESPACE}" \
    -p '{"spec":{"selector":{"statefulset.kubernetes.io/pod-name":"validator-0-0"}}}'

# ── 11. Wait for all validator pods to be ready ─────────────────────────────
log_info "Waiting for all validator pods to be ready..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    log_info "  Waiting for validator-${i}-0..."
    kubectl wait --for=condition=ready pod \
        "validator-${i}-0" \
        -n "${K8S_NAMESPACE}" \
        --timeout=180s
done
log_success "All validator pods are ready"

# ── 12. Wait for chain to produce blocks ────────────────────────────────────
log_info "Waiting for chain to produce blocks..."
wait_for_chain_ready "http://localhost:26657" 120

log_success "=== Moca chain deployed successfully ==="
