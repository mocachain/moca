#!/bin/bash

KEYRING_BACKEND="test"
KEYS_FILE="keys.json"

if [ ! -f "$KEYS_FILE" ]; then
    echo "Keys file $KEYS_FILE not found. Exiting..."
    exit 1
fi

echo "Starting to import keys..."

# Traverse Attribute - Value Pair in JSON file using jq
for NAME in $(jq -r 'keys[]' "$KEYS_FILE"); do
    PRIVATE_KEY=$(jq -r --arg NAME "$NAME" '.[$NAME]' "$KEYS_FILE")
    echo "Importing key for $NAME..."

    # Import the private key using a pipeline
    mocad keys import "${NAME}" ${PRIVATE_KEY} --secp256k1-private-key --keyring-backend "${KEYRING_BACKEND}"

    # Check import results
    if [ $? -eq 0 ]; then
        echo "Key for $NAME imported successfully."
    else
        echo "Failed to import key for $NAME."
    fi
done

echo "All keys have been imported successfully."
