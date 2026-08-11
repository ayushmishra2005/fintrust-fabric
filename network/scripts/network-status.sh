#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

FABRIC_BIN_DIR="$(dirname "$(command -v peer)")"
export FABRIC_CFG_PATH="${FABRIC_BIN_DIR}/../config"

echo "Container status:"
docker compose -f "$COMPOSE_FILE" ps

echo ""
echo "Channel membership:"

for org in supplier buyer finance; do
  set_peer_env "$org"
  echo ""
  echo "=== ${org} ==="
  peer channel list 2>/dev/null || echo "  (peer not reachable)"
done

echo ""
echo "Orderer channels:"
set_orderer_env
osnadmin channel list -o localhost:7053 \
  --ca-file "$ORDERER_CA" \
  --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" \
  --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY" 2>/dev/null || echo "  (orderer not reachable)"
