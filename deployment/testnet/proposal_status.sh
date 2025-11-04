#!/bin/bash
curl -s https://devnet-api.mocachain.org/cosmos/gov/v1/proposals/$1 | jq -r '.proposal.status'
