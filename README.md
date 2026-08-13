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

## E2E Integration Tests

Network-level tests verify security and concurrency properties:

- Organization authorization enforcement
- Private data isolation between parties
- Invalid state transition prevention
- Public block privacy (no transient/PDC leakage)
- Double-financing prevention (single-winner under SBE)
- MVCC read conflict detection for concurrent state updates

```
make e2e         # Run E2E tests (requires running network with deployed chaincode)
make verify-e2e  # Full clean verification cycle: reset, up, deploy, test, down
```

## REST API

The Go backend provides organization-specific REST APIs using Fabric Gateway. Each organization runs its own API instance with non-admin client identities.

Local ports:
- Supplier API: 8081
- Buyer API: 8082
- Finance API: 8083

Build and test:

```
make api-build   # Build the API binary
make api-test    # Run backend unit tests
```

Manual verification (requires running network with deployed chaincode):

```
# Terminal 1: Start Supplier API
FINTRUST_ORG=supplier FINTRUST_NETWORK_DIR=$PWD/network ./bin/fintrust-api

# Terminal 2: Start Buyer API
FINTRUST_ORG=buyer FINTRUST_NETWORK_DIR=$PWD/network ./bin/fintrust-api

# Terminal 3: Start Finance API
FINTRUST_ORG=finance FINTRUST_NETWORK_DIR=$PWD/network ./bin/fintrust-api

# Terminal 4: Run smoke test
./scripts/api-smoke.sh
```

API endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /healthz | Health check |
| POST | /api/v1/invoices | Create invoice (Supplier) |
| GET | /api/v1/invoices?status=X | Query by status |
| GET | /api/v1/invoices/{id} | Get public invoice |
| GET | /api/v1/invoices/{id}/private | Get private commercial data |
| GET | /api/v1/invoices/{id}/financing | Get financing terms |
| GET | /api/v1/invoices/{id}/events | Get invoice events |
| POST | /api/v1/invoices/{id}/approve | Approve (Buyer) |
| POST | /api/v1/invoices/{id}/reject | Reject (Buyer) |
| POST | /api/v1/invoices/{id}/financing-request | Request financing (Supplier) |
| POST | /api/v1/invoices/{id}/finance | Finance (Finance) |
| POST | /api/v1/invoices/{id}/settle | Settle (Buyer) |
| GET | /api/v1/events | Query event projection |

Confidential data is submitted via transient fields and stored in private data collections.

## Event Projection

The API subscribes to chaincode events and projects them into SQLite for efficient querying. On startup, the consumer resumes from its last checkpoint, replaying any missed events. Idempotent inserts prevent duplicates after restarts.

Projected events: InvoiceCreated, InvoiceApproved, InvoiceRejected, FinancingRequested, InvoiceFinanced, InvoiceSettled

## Architecture

- Hyperledger Fabric 2.5 LTS with Fabric CA
- Go chaincode with MSP-based authorization
- Three member organizations: SupplierMSP, BuyerMSP, FinanceMSP
- Single `fintrust` channel with Raft ordering
- Private data collections for confidential invoice and financing data
- Transient data for sensitive inputs (never logged or committed publicly)
- State-based endorsement for lifecycle-phase security
- Fabric Gateway for client connectivity
- CouchDB for rich queries
- Chaincode events for off-chain projection
- SQLite read model for API queries with checkpoint-based resume

## Status

Core implementation complete. Under active development.

## License

Apache License 2.0
