#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

check_prereqs

echo "Creating directories..."
mkdir -p "${ORGANIZATIONS_DIR}/fabric-ca/supplier"
mkdir -p "${ORGANIZATIONS_DIR}/fabric-ca/buyer"
mkdir -p "${ORGANIZATIONS_DIR}/fabric-ca/finance"
mkdir -p "${ORGANIZATIONS_DIR}/fabric-ca/orderer"
mkdir -p "${CHANNEL_ARTIFACTS_DIR}"

echo "Starting CA containers..."
docker compose -f "$COMPOSE_FILE" up -d ca.supplier ca.buyer ca.finance ca.orderer

echo "Waiting for CAs to start..."
sleep 5

for ca in ca.supplier ca.buyer ca.finance ca.orderer; do
  wait_for_container "$ca"
done

echo "Enrolling identities..."
"${SCRIPT_DIR}/enroll-ca.sh"

echo "Starting orderer and peers..."
docker compose -f "$COMPOSE_FILE" up -d orderer.fintrust \
  couchdb.supplier peer0.supplier \
  couchdb.buyer peer0.buyer \
  couchdb.finance peer0.finance

echo "Waiting for containers..."
sleep 5

for svc in orderer.fintrust peer0.supplier peer0.buyer peer0.finance; do
  wait_for_container "$svc"
done

echo "Creating and joining channel..."
"${SCRIPT_DIR}/channel-create.sh"

echo "Network is up."
