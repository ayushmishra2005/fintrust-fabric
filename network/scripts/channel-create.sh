#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

FABRIC_BIN_DIR="$(dirname "$(command -v peer)")"
FABRIC_CONFIG_DIR="${FABRIC_BIN_DIR}/../config"

export FABRIC_CFG_PATH="${CONFIG_DIR}"

echo "Generating channel genesis block..."
configtxgen -profile FintrustGenesis -outputBlock "${CHANNEL_ARTIFACTS_DIR}/${CHANNEL_NAME}.block" -channelID "$CHANNEL_NAME"

set_orderer_env

echo "Creating channel via osnadmin..."
osnadmin channel join \
  --channelID "$CHANNEL_NAME" \
  --config-block "${CHANNEL_ARTIFACTS_DIR}/${CHANNEL_NAME}.block" \
  -o localhost:7053 \
  --ca-file "$ORDERER_CA" \
  --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" \
  --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"

sleep 2

export FABRIC_CFG_PATH="${FABRIC_CONFIG_DIR}"

echo "Joining peer0.supplier..."
set_peer_env supplier
peer channel join -b "${CHANNEL_ARTIFACTS_DIR}/${CHANNEL_NAME}.block"

echo "Joining peer0.buyer..."
set_peer_env buyer
peer channel join -b "${CHANNEL_ARTIFACTS_DIR}/${CHANNEL_NAME}.block"

echo "Joining peer0.finance..."
set_peer_env finance
peer channel join -b "${CHANNEL_ARTIFACTS_DIR}/${CHANNEL_NAME}.block"

echo "Channel $CHANNEL_NAME created and all peers joined."
