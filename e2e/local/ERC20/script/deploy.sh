#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./script/deploy.sh local
#   ./script/deploy.sh devnet
#   ./script/deploy.sh testnet
# Optional envs:
#   PRIVATE_KEY=0x...
#   INITIAL_SUPPLY_WEI=1000000000000000000000000
#   FOUNDRY_EVM_VERSION=paris
#   RPC_URL=... (overrides preset)

NETWORK="${1:-local}"
case "$NETWORK" in
  local)
    DEFAULT_RPC="http://127.0.0.1:8545"
    ;;
  devnet)
    DEFAULT_RPC="https://devnet-rpc.mocachain.org"
    ;;
  testnet)
    DEFAULT_RPC="https://testnet-rpc.mocachain.org"
    ;;
  *)
    echo "Unknown network: $NETWORK. Use local|devnet|testnet"; exit 1;
    ;;
esac

RPC_URL="${RPC_URL:-$DEFAULT_RPC}"
PRIVATE_KEY="${PRIVATE_KEY:-}"
INITIAL_SUPPLY_WEI="${INITIAL_SUPPLY_WEI:-1000000000000000000000000}"

command -v forge >/dev/null || { echo "forge not found"; exit 1; }
command -v cast >/dev/null || { echo "cast not found"; exit 1; }

if [ -z "$PRIVATE_KEY" ]; then
  echo "PRIVATE_KEY is not set"
  exit 1
fi

export FOUNDRY_EVM_VERSION=${FOUNDRY_EVM_VERSION:-paris}

echo "Network: $NETWORK"
echo "RPC: $RPC_URL"

DEPLOY_OUTPUT=$(forge create \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  src/MyToken.sol:MyToken \
  --constructor-args "$INITIAL_SUPPLY_WEI")

echo "$DEPLOY_OUTPUT"

ADDR=$(echo "$DEPLOY_OUTPUT" | awk '/Deployed to:/ {print $3}')
if [ -z "$ADDR" ]; then
  echo "Failed to parse deployed address"
  exit 1
fi

echo "Contract address: $ADDR"
echo -n "symbol: "
cast call "$ADDR" "symbol()(string)" --rpc-url "$RPC_URL"
echo -n "totalSupply: "
cast call "$ADDR" "totalSupply()(uint256)" --rpc-url "$RPC_URL"


