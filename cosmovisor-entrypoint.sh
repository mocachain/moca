#!/bin/bash
set -e

# Ensure atleast the genesis binary exists, if not copy it from the docker images
# to the daemon home directory.
if [ ! -f "$DAEMON_HOME/cosmovisor/genesis/bin/$DAEMON_NAME" ]; then
    echo "Binary not found at $DAEMON_HOME/cosmovisor/genesis/bin/$DAEMON_NAME"
    echo "Copying from /usr/local/bin/$DAEMON_NAME..."
    mkdir -p "$DAEMON_HOME/cosmovisor/genesis/bin"
    cp "/usr/local/bin/$DAEMON_NAME" "$DAEMON_HOME/cosmovisor/genesis/bin/$DAEMON_NAME"
    echo "Binary copied successfully"
fi

exec "$@"
