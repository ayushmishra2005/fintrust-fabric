# FinTrust

FinTrust is a confidential B2B invoice financing network built with Hyperledger Fabric. It models invoice approval, financing, and settlement across supplier, buyer, and financier organizations while keeping sensitive commercial and financing data private.

## Why FinTrust

Traditional invoice financing involves multiple independent parties who need to agree on invoice validity and financing terms without exposing confidential data to each other. FinTrust addresses:

- Shared invoice lifecycle across supplier, buyer, and financier organizations
- Confidential commercial data that should not leak between parties
- Prevention of double financing through coordinated ledger state
- Fabric-native security model rather than cryptocurrency or token mechanics

## Local Network

The development network runs three application organizations (SupplierMSP, BuyerMSP, FinanceMSP) with one peer each, backed by CouchDB. A single Raft orderer handles consensus for local development.

Prerequisites:
- Docker and Docker Compose
- Fabric CLI binaries: `peer`, `configtxgen`, `osnadmin`, `fabric-ca-client`
- `jq`

```
make network-up      # Start network and create fintrust channel
make network-status  # Show container and channel status
make network-down    # Stop containers
make network-reset   # Stop and remove all generated files
```

## Invoice Chaincode

The `invoice` chaincode implements a confidential B2B invoice financing workflow with strict lifecycle state machine:

```
CREATED → APPROVED → FINANCING_REQUESTED → FINANCED → SETTLED
       ↘ REJECTED
```

Private data collections:
- `collectionInvoiceParties`: Commercial terms and payment details shared between supplier and buyer
- `collectionSupplierFinance`: Financing request, disclosure, and agreement shared between supplier and financier

Deploy and query chaincode status:

```
make chaincode-deploy  # Package, install, approve, and commit
make chaincode-status  # Show installation and commit status
```

## Planned Architecture

- Hyperledger Fabric 2.5 LTS
- Go chaincode and API
- Three member organizations: SupplierMSP, BuyerMSP, FinanceMSP
- Single Fabric channel
- Private data collections for confidential invoice and financing data
- Transient data for sensitive inputs
- Fabric Gateway for client connectivity
- CouchDB for rich queries
- Chaincode events for off-chain projection
- SQLite read model for API queries

## Status

Under active implementation.

## License

Apache License 2.0
