#!/usr/bin/env bash
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE_FILE="${SCRIPT_DIR}/.env.example"

if [ ! -f "${ENV_FILE}" ]; then
    if [ ! -f "${ENV_EXAMPLE_FILE}" ]; then
        echo "Error: missing ${ENV_FILE} and ${ENV_EXAMPLE_FILE}" >&2
        exit 1
    fi

    cp "${ENV_EXAMPLE_FILE}" "${ENV_FILE}"
    echo "Created ${ENV_FILE} from ${ENV_EXAMPLE_FILE}" >&2
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"
SP_DIR=$(realpath "${SCRIPT_DIR}/../../../moca-storage-provider/deployment/dockerup")
RELAY_DIR=$(realpath "${SCRIPT_DIR}/../../../moca-relayer/deployment/dockerup")

function change_persistent_peers() {
    persistent_peers=$(cat ${SCRIPT_DIR}/persistent_peers.txt)
    sed -i -e "s/PERSISTENT_PEERS=\".*\"/PERSISTENT_PEERS=\"${persistent_peers}\"/g" "${ENV_FILE}"
}

function copy_sp_relayer() {
    cp "${SCRIPT_DIR}/sp.json" "${SP_DIR}"
    cp "${SCRIPT_DIR}/validator.json" "${RELAY_DIR}"
}

CMD=$1

case ${CMD} in

backup)
    change_persistent_peers
    copy_sp_relayer
    ;;
*)
    echo "Usage backup.sh backup"
    ;;
esac
