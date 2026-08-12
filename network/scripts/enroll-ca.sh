#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

write_node_ou_config() {
  local msp_dir="$1"
  local ca_cert_file
  ca_cert_file=$(basename "$(ls "${msp_dir}/cacerts/"*.pem 2>/dev/null | head -1)")
  cat > "${msp_dir}/config.yaml" << EOF
NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/${ca_cert_file}
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/${ca_cert_file}
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/${ca_cert_file}
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/${ca_cert_file}
    OrganizationalUnitIdentifier: orderer
EOF
}

create_org_msp() {
  local org_dir="$1"
  mkdir -p "${org_dir}/msp/cacerts" "${org_dir}/msp/tlscacerts"
  cp "${org_dir}/ca/ca-cert.pem" "${org_dir}/msp/cacerts/"
  cp "${org_dir}/tlsca/tlsca-cert.pem" "${org_dir}/msp/tlscacerts/"
  cat > "${org_dir}/msp/config.yaml" << EOF
NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: orderer
EOF
}

enroll_supplier() {
  local org_dir="${ORGANIZATIONS_DIR}/supplierOrg"
  local ca_dir="${ORGANIZATIONS_DIR}/fabric-ca/supplier"
  local ca_cert="${ca_dir}/tls-cert.pem"
  local ca_url="https://localhost:7054"
  export FABRIC_CA_CLIENT_HOME="${org_dir}/ca/client"

  mkdir -p "${org_dir}" "${FABRIC_CA_CLIENT_HOME}"

  echo "Enrolling CA admin for SupplierOrg..."
  fabric-ca-client enroll -u "https://admin:adminpw@localhost:7054" \
    --caname ca-supplier --tls.certfiles "$ca_cert" \
    -M "${org_dir}/ca/admin"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/ca/ca-cert.pem"

  echo "Registering peer0.supplier..."
  fabric-ca-client register --caname ca-supplier -u "$ca_url" \
    --id.name peer0 --id.secret peer0pw --id.type peer \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Registering admin user..."
  fabric-ca-client register --caname ca-supplier -u "$ca_url" \
    --id.name supplieradmin --id.secret adminpw --id.type admin \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling peer0.supplier MSP..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:7054" \
    --caname ca-supplier --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.supplier/msp"
  write_node_ou_config "${org_dir}/peers/peer0.supplier/msp"

  echo "Enrolling peer0.supplier TLS..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:7054" \
    --caname ca-supplier --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.supplier/tls" \
    --enrollment.profile tls --csr.hosts peer0.supplier,localhost

  cp "${org_dir}/peers/peer0.supplier/tls/tlscacerts/"* "${org_dir}/peers/peer0.supplier/tls/ca.crt"
  cp "${org_dir}/peers/peer0.supplier/tls/signcerts/"* "${org_dir}/peers/peer0.supplier/tls/server.crt"
  cp "${org_dir}/peers/peer0.supplier/tls/keystore/"* "${org_dir}/peers/peer0.supplier/tls/server.key"

  echo "Enrolling Admin@supplierOrg..."
  fabric-ca-client enroll -u "https://supplieradmin:adminpw@localhost:7054" \
    --caname ca-supplier --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/Admin@supplierOrg/msp"
  write_node_ou_config "${org_dir}/users/Admin@supplierOrg/msp"

  echo "Registering supplier-client..."
  fabric-ca-client register --caname ca-supplier -u "$ca_url" \
    --id.name supplier-client --id.secret clientpw --id.type client \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling supplier-client..."
  fabric-ca-client enroll -u "https://supplier-client:clientpw@localhost:7054" \
    --caname ca-supplier --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/supplier-client/msp"
  write_node_ou_config "${org_dir}/users/supplier-client/msp"

  mkdir -p "${org_dir}/tlsca"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/tlsca/tlsca-cert.pem"
  create_org_msp "$org_dir"
}

enroll_buyer() {
  local org_dir="${ORGANIZATIONS_DIR}/buyerOrg"
  local ca_dir="${ORGANIZATIONS_DIR}/fabric-ca/buyer"
  local ca_cert="${ca_dir}/tls-cert.pem"
  local ca_url="https://localhost:8054"
  export FABRIC_CA_CLIENT_HOME="${org_dir}/ca/client"

  mkdir -p "${org_dir}" "${FABRIC_CA_CLIENT_HOME}"

  echo "Enrolling CA admin for BuyerOrg..."
  fabric-ca-client enroll -u "https://admin:adminpw@localhost:8054" \
    --caname ca-buyer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/ca/admin"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/ca/ca-cert.pem"

  echo "Registering peer0.buyer..."
  fabric-ca-client register --caname ca-buyer -u "$ca_url" \
    --id.name peer0 --id.secret peer0pw --id.type peer \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Registering admin user..."
  fabric-ca-client register --caname ca-buyer -u "$ca_url" \
    --id.name buyeradmin --id.secret adminpw --id.type admin \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling peer0.buyer MSP..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:8054" \
    --caname ca-buyer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.buyer/msp"
  write_node_ou_config "${org_dir}/peers/peer0.buyer/msp"

  echo "Enrolling peer0.buyer TLS..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:8054" \
    --caname ca-buyer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.buyer/tls" \
    --enrollment.profile tls --csr.hosts peer0.buyer,localhost

  cp "${org_dir}/peers/peer0.buyer/tls/tlscacerts/"* "${org_dir}/peers/peer0.buyer/tls/ca.crt"
  cp "${org_dir}/peers/peer0.buyer/tls/signcerts/"* "${org_dir}/peers/peer0.buyer/tls/server.crt"
  cp "${org_dir}/peers/peer0.buyer/tls/keystore/"* "${org_dir}/peers/peer0.buyer/tls/server.key"

  echo "Enrolling Admin@buyerOrg..."
  fabric-ca-client enroll -u "https://buyeradmin:adminpw@localhost:8054" \
    --caname ca-buyer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/Admin@buyerOrg/msp"
  write_node_ou_config "${org_dir}/users/Admin@buyerOrg/msp"

  echo "Registering buyer-client..."
  fabric-ca-client register --caname ca-buyer -u "$ca_url" \
    --id.name buyer-client --id.secret clientpw --id.type client \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling buyer-client..."
  fabric-ca-client enroll -u "https://buyer-client:clientpw@localhost:8054" \
    --caname ca-buyer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/buyer-client/msp"
  write_node_ou_config "${org_dir}/users/buyer-client/msp"

  mkdir -p "${org_dir}/tlsca"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/tlsca/tlsca-cert.pem"
  create_org_msp "$org_dir"
}

enroll_finance() {
  local org_dir="${ORGANIZATIONS_DIR}/financeOrg"
  local ca_dir="${ORGANIZATIONS_DIR}/fabric-ca/finance"
  local ca_cert="${ca_dir}/tls-cert.pem"
  local ca_url="https://localhost:9054"
  export FABRIC_CA_CLIENT_HOME="${org_dir}/ca/client"

  mkdir -p "${org_dir}" "${FABRIC_CA_CLIENT_HOME}"

  echo "Enrolling CA admin for FinanceOrg..."
  fabric-ca-client enroll -u "https://admin:adminpw@localhost:9054" \
    --caname ca-finance --tls.certfiles "$ca_cert" \
    -M "${org_dir}/ca/admin"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/ca/ca-cert.pem"

  echo "Registering peer0.finance..."
  fabric-ca-client register --caname ca-finance -u "$ca_url" \
    --id.name peer0 --id.secret peer0pw --id.type peer \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Registering admin user..."
  fabric-ca-client register --caname ca-finance -u "$ca_url" \
    --id.name financeadmin --id.secret adminpw --id.type admin \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling peer0.finance MSP..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:9054" \
    --caname ca-finance --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.finance/msp"
  write_node_ou_config "${org_dir}/peers/peer0.finance/msp"

  echo "Enrolling peer0.finance TLS..."
  fabric-ca-client enroll -u "https://peer0:peer0pw@localhost:9054" \
    --caname ca-finance --tls.certfiles "$ca_cert" \
    -M "${org_dir}/peers/peer0.finance/tls" \
    --enrollment.profile tls --csr.hosts peer0.finance,localhost

  cp "${org_dir}/peers/peer0.finance/tls/tlscacerts/"* "${org_dir}/peers/peer0.finance/tls/ca.crt"
  cp "${org_dir}/peers/peer0.finance/tls/signcerts/"* "${org_dir}/peers/peer0.finance/tls/server.crt"
  cp "${org_dir}/peers/peer0.finance/tls/keystore/"* "${org_dir}/peers/peer0.finance/tls/server.key"

  echo "Enrolling Admin@financeOrg..."
  fabric-ca-client enroll -u "https://financeadmin:adminpw@localhost:9054" \
    --caname ca-finance --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/Admin@financeOrg/msp"
  write_node_ou_config "${org_dir}/users/Admin@financeOrg/msp"

  echo "Registering finance-client..."
  fabric-ca-client register --caname ca-finance -u "$ca_url" \
    --id.name finance-client --id.secret clientpw --id.type client \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling finance-client..."
  fabric-ca-client enroll -u "https://finance-client:clientpw@localhost:9054" \
    --caname ca-finance --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/finance-client/msp"
  write_node_ou_config "${org_dir}/users/finance-client/msp"

  mkdir -p "${org_dir}/tlsca"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/tlsca/tlsca-cert.pem"
  create_org_msp "$org_dir"
}

enroll_orderer() {
  local org_dir="${ORGANIZATIONS_DIR}/ordererOrg"
  local ca_dir="${ORGANIZATIONS_DIR}/fabric-ca/orderer"
  local ca_cert="${ca_dir}/tls-cert.pem"
  local ca_url="https://localhost:10054"
  export FABRIC_CA_CLIENT_HOME="${org_dir}/ca/client"

  mkdir -p "${org_dir}" "${FABRIC_CA_CLIENT_HOME}"

  echo "Enrolling CA admin for OrdererOrg..."
  fabric-ca-client enroll -u "https://admin:adminpw@localhost:10054" \
    --caname ca-orderer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/ca/admin"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/ca/ca-cert.pem"

  echo "Registering orderer..."
  fabric-ca-client register --caname ca-orderer -u "$ca_url" \
    --id.name orderer --id.secret ordererpw --id.type orderer \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Registering orderer admin..."
  fabric-ca-client register --caname ca-orderer -u "$ca_url" \
    --id.name ordereradmin --id.secret adminpw --id.type admin \
    --tls.certfiles "$ca_cert" -M "${org_dir}/ca/admin"

  echo "Enrolling orderer.fintrust MSP..."
  fabric-ca-client enroll -u "https://orderer:ordererpw@localhost:10054" \
    --caname ca-orderer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/orderers/orderer.fintrust/msp"
  write_node_ou_config "${org_dir}/orderers/orderer.fintrust/msp"

  echo "Enrolling orderer.fintrust TLS..."
  fabric-ca-client enroll -u "https://orderer:ordererpw@localhost:10054" \
    --caname ca-orderer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/orderers/orderer.fintrust/tls" \
    --enrollment.profile tls --csr.hosts orderer.fintrust,localhost

  cp "${org_dir}/orderers/orderer.fintrust/tls/tlscacerts/"* "${org_dir}/orderers/orderer.fintrust/tls/ca.crt"
  cp "${org_dir}/orderers/orderer.fintrust/tls/signcerts/"* "${org_dir}/orderers/orderer.fintrust/tls/server.crt"
  cp "${org_dir}/orderers/orderer.fintrust/tls/keystore/"* "${org_dir}/orderers/orderer.fintrust/tls/server.key"

  echo "Enrolling Admin@ordererOrg..."
  fabric-ca-client enroll -u "https://ordereradmin:adminpw@localhost:10054" \
    --caname ca-orderer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/Admin@ordererOrg/msp"
  write_node_ou_config "${org_dir}/users/Admin@ordererOrg/msp"

  echo "Enrolling Admin@ordererOrg TLS..."
  fabric-ca-client enroll -u "https://ordereradmin:adminpw@localhost:10054" \
    --caname ca-orderer --tls.certfiles "$ca_cert" \
    -M "${org_dir}/users/Admin@ordererOrg/tls" \
    --enrollment.profile tls --csr.hosts orderer.fintrust,localhost

  cp "${org_dir}/users/Admin@ordererOrg/tls/tlscacerts/"* "${org_dir}/users/Admin@ordererOrg/tls/ca.crt"
  cp "${org_dir}/users/Admin@ordererOrg/tls/signcerts/"* "${org_dir}/users/Admin@ordererOrg/tls/client.crt"
  cp "${org_dir}/users/Admin@ordererOrg/tls/keystore/"* "${org_dir}/users/Admin@ordererOrg/tls/client.key"

  mkdir -p "${org_dir}/tlsca"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/tlsca/tlsca-cert.pem"
  cp "${ca_dir}/ca-cert.pem" "${org_dir}/tlsca/tlsca.ordererOrg-cert.pem"
  create_org_msp "$org_dir"
}

main() {
  echo "Enrolling identities from Fabric CAs..."
  enroll_supplier
  enroll_buyer
  enroll_finance
  enroll_orderer
  echo "All identities enrolled."
}

main "$@"
