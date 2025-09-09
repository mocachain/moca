#!/bin/bash

ACCOUNTS=("v0" "v1" "v2" "v3")
PROPOSAL_ID=$1
CHAIN_ID="moca_5151-1"

# Keyring backend
KEYRING_BACKEND="test"

# Gas prices
GAS_PRICES="10000amoca"
VOTE_OPTION="yes"

echo "Starting voting process for proposal ID: $PROPOSAL_ID..."

# Traverse the account and execute the vote
for ACCOUNT in "${ACCOUNTS[@]}"; do
    echo "Voting for account: $ACCOUNT..."

    # Execute voting
    mocad tx gov vote "$PROPOSAL_ID" "$VOTE_OPTION" \
        --from="$ACCOUNT" \
        --chain-id="$CHAIN_ID" \
        --keyring-backend="$KEYRING_BACKEND" \
        --gas-prices="$GAS_PRICES" \
        --node "https://devnet-lcd.mocachain.org:443" \
        -y

    # Check command execution result
    if [ $? -eq 0 ]; then
        echo "Vote from $ACCOUNT submitted successfully."
    else
        echo "Failed to submit vote from $ACCOUNT."
    fi
done

echo "Voting process completed."
