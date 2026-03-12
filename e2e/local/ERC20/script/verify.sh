#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./script/verify.sh devnet <contract_address>
#   ./script/verify.sh testnet <contract_address>
# Optional envs:
#   INITIAL_SUPPLY_WEI (default 1e24)
#   FOUNDRY_EVM_VERSION (default paris)

NETWORK="${1:-}"
ADDR="${2:-}"

if [ -z "$NETWORK" ] || [ -z "$ADDR" ]; then
  echo "Usage: $0 <devnet|testnet> <contract_address>"
  exit 1
fi

case "$NETWORK" in
  devnet)
    VERIFIER_URL="https://devnet-scan.mocachain.org/api"
    CHAIN_ID=5151
    ;;
  testnet)
    VERIFIER_URL="https://testnet-scan.mocachain.org/api"
    CHAIN_ID=222888
    ;;
  *)
    echo "Unknown network: $NETWORK"; exit 1;
    ;;
esac

export FOUNDRY_EVM_VERSION=${FOUNDRY_EVM_VERSION:-paris}
INITIAL_SUPPLY_WEI="${INITIAL_SUPPLY_WEI:-1000000000000000000000000}"

echo "Network: $NETWORK (chain $CHAIN_ID)"
echo "Verifier: $VERIFIER_URL"
echo "Contract: $ADDR"

ARGS=$(cast abi-encode "constructor(uint256)" "$INITIAL_SUPPLY_WEI")

forge verify-contract \
  --verifier blockscout \
  --verifier-url "$VERIFIER_URL" \
  --chain "$CHAIN_ID" \
  --compiler-version "v0.8.20+commit.a1b79de6" \
  "$ADDR" \
  src/MyToken.sol:MyToken \
  --constructor-args "$ARGS"


