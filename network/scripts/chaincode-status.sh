#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

FABRIC_BIN_DIR="$(dirname "$(command -v peer)")"
export FABRIC_CFG_PATH="${FABRIC_BIN_DIR}/../config"

echo "=== Chaincode Status ==="
echo ""

for org in supplier buyer finance; do
  echo "--- Installed on ${org} peer ---"
  set_peer_env "$org"
  peer lifecycle chaincode queryinstalled 2>/dev/null || echo "None installed"
  echo ""
done

echo "--- Committed on channel ${CHANNEL_NAME} ---"
set_peer_env supplier
peer lifecycle chaincode querycommitted --channelID "${CHANNEL_NAME}" 2>/dev/null || echo "None committed"
