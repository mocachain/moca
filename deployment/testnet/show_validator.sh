#!/bin/bash
curl -s https://devnet-api.mocachain.org/cosmos/staking/v1beta1/validators | jq '.pagination.total'
