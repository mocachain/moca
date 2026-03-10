#!/bin/bash
# Setup .env file for repositories that don't have one
# This script copies .env from moca directory to other repositories

set -e

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
MOCA_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)/moca
ENV_FILE="${MOCA_DIR}/.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "Error: .env file not found in ${MOCA_DIR}"
    exit 1
fi

REPOS=(
    "moca-storage-provider"
    "moca-callisto"
    "moca-relayer"
    "moca-cmd"
)

for repo in "${REPOS[@]}"; do
    repo_path="${MOCA_DIR}/../${repo}"
    if [ -d "${repo_path}" ]; then
        if [ ! -f "${repo_path}/.env" ]; then
            cp "${ENV_FILE}" "${repo_path}/.env"
            echo "Copied .env to ${repo}"
        else
            echo "${repo} already has .env file"
        fi
        
        if ! grep -q "^\.env$" "${repo_path}/.gitignore" 2>/dev/null; then
            echo ".env" >> "${repo_path}/.gitignore"
            echo "Added .env to ${repo}/.gitignore"
        fi
    else
        echo "Warning: ${repo} directory not found"
    fi
done

echo "Setup complete!"

