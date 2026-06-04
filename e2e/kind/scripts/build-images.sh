#!/usr/bin/env bash
# Builds Docker images for Moca E2E tests and loads them into Kind.

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

REPO_ROOT=$(cd -- "${E2E_DIR}/../.." && pwd)

log_info "=== Building Docker images for Moca E2E tests ==="

FULL_IMAGE="${DOCKER_IMAGE}:${DOCKER_TAG}"

# Create a Docker-safe gitconfig (HTTPS-only, no SSH rewrite)
_DOCKER_GITCONFIG=$(mktemp)
trap "rm -f ${_DOCKER_GITCONFIG}" EXIT
grep -A1 'url.*gho_\|url.*ghp_\|url.*github_pat_' "${HOME}/.gitconfig" > "${_DOCKER_GITCONFIG}" 2>/dev/null || true
# If no token found, try GOPRIVATE + netrc approach
if [ ! -s "${_DOCKER_GITCONFIG}" ]; then
    log_warn "No GitHub token found in ~/.gitconfig, private deps may fail"
    cp "${HOME}/.gitconfig" "${_DOCKER_GITCONFIG}"
fi

# NEW_VERSION allows building the "current" image from a specific git ref (e.g. main)
# instead of local source. Useful when the local branch lacks upgrade handlers.
if [ -n "${NEW_VERSION:-}" ]; then
    log_info "Building mocad Docker image from git ref '${NEW_VERSION}': ${FULL_IMAGE}..."
    DOCKER_BUILDKIT=1 docker build \
        --secret id=gitconfig,src="${_DOCKER_GITCONFIG}" \
        --build-arg "GIT_REF=${NEW_VERSION}" \
        -f "${E2E_DIR}/Dockerfile.e2e-gitref" \
        -t "${FULL_IMAGE}" \
        "${REPO_ROOT}"
else
    log_info "Building mocad Docker image from local source: ${FULL_IMAGE}..."
    DOCKER_BUILDKIT=1 docker build \
        --secret id=gitconfig,src="${_DOCKER_GITCONFIG}" \
        -f "${E2E_DIR}/Dockerfile.e2e" \
        -t "${FULL_IMAGE}" \
        "${REPO_ROOT}"
fi
log_success "Docker image built: ${FULL_IMAGE}"

# Load into Kind
log_info "Loading image into Kind cluster '${KIND_CLUSTER_NAME}'..."
kind load docker-image "${FULL_IMAGE}" --name "${KIND_CLUSTER_NAME}"
log_success "Image loaded into Kind cluster"

# Build old version image if OLD_VERSION is set (for upgrade tests)
if [ -n "${OLD_VERSION:-}" ]; then
    OLD_IMAGE="${DOCKER_IMAGE}:${OLD_VERSION}"
    log_info "Building old version image from git ref: ${OLD_IMAGE}..."

    DOCKER_BUILDKIT=1 docker build \
        --secret id=gitconfig,src="${_DOCKER_GITCONFIG}" \
        --build-arg "GIT_REF=${OLD_VERSION}" \
        -f "${E2E_DIR}/Dockerfile.e2e-gitref" \
        -t "${OLD_IMAGE}" \
        "${REPO_ROOT}"
    log_success "Old version image built: ${OLD_IMAGE}"

    log_info "Loading old version image into Kind cluster..."
    kind load docker-image "${OLD_IMAGE}" --name "${KIND_CLUSTER_NAME}"
    log_success "Old version image loaded into Kind cluster"
fi

log_success "=== Image build and load complete ==="
