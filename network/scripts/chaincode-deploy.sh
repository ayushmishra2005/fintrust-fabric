#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

CHAINCODE_NAME="invoice"
CHAINCODE_VERSION="1.0"
CHAINCODE_SEQUENCE="1"
CHAINCODE_PATH="${SCRIPT_DIR}/../../chaincode/invoice"
COLLECTIONS_CONFIG="${CHAINCODE_PATH}/collections_config.json"
PACKAGE_DIR="${NETWORK_DIR}/chaincode-packages"

FABRIC_BIN_DIR="$(dirname "$(command -v peer)")"
export FABRIC_CFG_PATH="${FABRIC_BIN_DIR}/../config"

ENDORSEMENT_POLICY="OR('SupplierMSP.peer','BuyerMSP.peer','FinanceMSP.peer')"

mkdir -p "${PACKAGE_DIR}"

echo "Packaging chaincode..."
peer lifecycle chaincode package "${PACKAGE_DIR}/${CHAINCODE_NAME}.tar.gz" \
  --path "${CHAINCODE_PATH}" \
  --lang golang \
  --label "${CHAINCODE_NAME}_${CHAINCODE_VERSION}"

install_chaincode() {
  local org="$1"
  echo "Installing chaincode on ${org}..."
  set_peer_env "$org"
  peer lifecycle chaincode install "${PACKAGE_DIR}/${CHAINCODE_NAME}.tar.gz"
}

install_chaincode supplier
install_chaincode buyer
install_chaincode finance

echo "Getting package ID..."
set_peer_env supplier
PACKAGE_ID=$(peer lifecycle chaincode queryinstalled | grep "${CHAINCODE_NAME}_${CHAINCODE_VERSION}" | sed -n 's/.*Package ID: \([^,]*\).*/\1/p')

if [[ -z "${PACKAGE_ID}" ]]; then
  echo "Failed to get package ID" >&2
  exit 1
fi
echo "Package ID: ${PACKAGE_ID}"

approve_chaincode() {
  local org="$1"
  echo "Approving chaincode for ${org}..."
  set_peer_env "$org"
  set_orderer_env
  peer lifecycle chaincode approveformyorg \
    -o localhost:7050 \
    --ordererTLSHostnameOverride orderer.fintrust \
    --tls \
    --cafile "${ORDERER_CA}" \
    --channelID "${CHANNEL_NAME}" \
    --name "${CHAINCODE_NAME}" \
    --version "${CHAINCODE_VERSION}" \
    --package-id "${PACKAGE_ID}" \
    --sequence "${CHAINCODE_SEQUENCE}" \
    --signature-policy "${ENDORSEMENT_POLICY}" \
    --collections-config "${COLLECTIONS_CONFIG}"
}

approve_chaincode supplier
approve_chaincode buyer
approve_chaincode finance

echo "Checking commit readiness..."
set_peer_env supplier
peer lifecycle chaincode checkcommitreadiness \
  --channelID "${CHANNEL_NAME}" \
  --name "${CHAINCODE_NAME}" \
  --version "${CHAINCODE_VERSION}" \
  --sequence "${CHAINCODE_SEQUENCE}" \
  --signature-policy "${ENDORSEMENT_POLICY}" \
  --collections-config "${COLLECTIONS_CONFIG}" \
  --output json

echo "Committing chaincode..."
set_orderer_env
peer lifecycle chaincode commit \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.fintrust \
  --tls \
  --cafile "${ORDERER_CA}" \
  --channelID "${CHANNEL_NAME}" \
  --name "${CHAINCODE_NAME}" \
  --version "${CHAINCODE_VERSION}" \
  --sequence "${CHAINCODE_SEQUENCE}" \
  --signature-policy "${ENDORSEMENT_POLICY}" \
  --collections-config "${COLLECTIONS_CONFIG}" \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${ORGANIZATIONS_DIR}/supplierOrg/peers/peer0.supplier/tls/ca.crt" \
  --peerAddresses localhost:8051 \
  --tlsRootCertFiles "${ORGANIZATIONS_DIR}/buyerOrg/peers/peer0.buyer/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${ORGANIZATIONS_DIR}/financeOrg/peers/peer0.finance/tls/ca.crt"

echo ""
echo "Chaincode deployed successfully!"
