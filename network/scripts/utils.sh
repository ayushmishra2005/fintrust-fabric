#!/usr/bin/env bash

export NETWORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export COMPOSE_FILE="${NETWORK_DIR}/compose/compose.yaml"
export ORGANIZATIONS_DIR="${NETWORK_DIR}/organizations"
export CHANNEL_ARTIFACTS_DIR="${NETWORK_DIR}/channel-artifacts"
export CONFIG_DIR="${NETWORK_DIR}/config"

export CHANNEL_NAME="fintrust"

check_prereqs() {
  local missing=()
  for cmd in docker jq; do
    command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
  done
  if ! docker compose version >/dev/null 2>&1; then
    missing+=("docker compose")
  fi
  for cmd in peer configtxgen osnadmin fabric-ca-client; do
    command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Missing required tools: ${missing[*]}" >&2
    exit 1
  fi
}

wait_for_container() {
  local container="$1"
  local max_attempts="${2:-30}"
  local attempt=0
  while [[ $attempt -lt $max_attempts ]]; do
    if docker compose -f "$COMPOSE_FILE" ps --format json 2>/dev/null | jq -e --arg c "$container" 'select(.Service == $c and .State == "running")' >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    ((attempt++))
  done
  echo "Container $container did not start" >&2
  return 1
}

wait_for_ca() {
  local ca_url="$1"
  local ca_cert="$2"
  local max_attempts="${3:-30}"
  local attempt=0
  while [[ $attempt -lt $max_attempts ]]; do
    if curl -s --cacert "$ca_cert" "${ca_url}/cainfo" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    ((attempt++))
  done
  echo "CA at $ca_url not ready" >&2
  return 1
}

set_peer_env() {
  local org="$1"
  case "$org" in
    supplier)
      export CORE_PEER_LOCALMSPID="SupplierMSP"
      export CORE_PEER_TLS_ROOTCERT_FILE="${ORGANIZATIONS_DIR}/supplierOrg/peers/peer0.supplier/tls/ca.crt"
      export CORE_PEER_MSPCONFIGPATH="${ORGANIZATIONS_DIR}/supplierOrg/users/Admin@supplierOrg/msp"
      export CORE_PEER_ADDRESS="localhost:7051"
      ;;
    buyer)
      export CORE_PEER_LOCALMSPID="BuyerMSP"
      export CORE_PEER_TLS_ROOTCERT_FILE="${ORGANIZATIONS_DIR}/buyerOrg/peers/peer0.buyer/tls/ca.crt"
      export CORE_PEER_MSPCONFIGPATH="${ORGANIZATIONS_DIR}/buyerOrg/users/Admin@buyerOrg/msp"
      export CORE_PEER_ADDRESS="localhost:8051"
      ;;
    finance)
      export CORE_PEER_LOCALMSPID="FinanceMSP"
      export CORE_PEER_TLS_ROOTCERT_FILE="${ORGANIZATIONS_DIR}/financeOrg/peers/peer0.finance/tls/ca.crt"
      export CORE_PEER_MSPCONFIGPATH="${ORGANIZATIONS_DIR}/financeOrg/users/Admin@financeOrg/msp"
      export CORE_PEER_ADDRESS="localhost:9051"
      ;;
    *)
      echo "Unknown org: $org" >&2
      return 1
      ;;
  esac
  export CORE_PEER_TLS_ENABLED="true"
}

set_orderer_env() {
  export ORDERER_CA="${ORGANIZATIONS_DIR}/ordererOrg/tlsca/tlsca.ordererOrg-cert.pem"
  export ORDERER_ADMIN_TLS_SIGN_CERT="${ORGANIZATIONS_DIR}/ordererOrg/users/Admin@ordererOrg/tls/client.crt"
  export ORDERER_ADMIN_TLS_PRIVATE_KEY="${ORGANIZATIONS_DIR}/ordererOrg/users/Admin@ordererOrg/tls/client.key"
}
