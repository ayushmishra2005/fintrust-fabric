#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
NETWORK_DIR="$ROOT_DIR/network"
BIN_DIR="$ROOT_DIR/bin"

SUPPLIER_PORT=8081
BUYER_PORT=8082
FINANCE_PORT=8083

LOG_DIR="$ROOT_DIR/.demo-logs"
PIDS=""

cleanup() {
    echo ""
    echo "=== Cleanup ==="
    for pid in $PIDS; do
        kill "$pid" 2>/dev/null || true
    done
    sleep 1
    rm -rf "$LOG_DIR"
    rm -f "$ROOT_DIR"/fintrust-supplier.db "$ROOT_DIR"/fintrust-supplier.db-wal "$ROOT_DIR"/fintrust-supplier.db-shm 2>/dev/null
    rm -f "$ROOT_DIR"/fintrust-buyer.db "$ROOT_DIR"/fintrust-buyer.db-wal "$ROOT_DIR"/fintrust-buyer.db-shm 2>/dev/null
    rm -f "$ROOT_DIR"/fintrust-finance.db "$ROOT_DIR"/fintrust-finance.db-wal "$ROOT_DIR"/fintrust-finance.db-shm 2>/dev/null
    "$NETWORK_DIR/scripts/network-down.sh" 2>/dev/null || true
    echo "Done."
}

trap cleanup EXIT

start_api() {
    local org=$1
    local port=$2
    local log_file="$LOG_DIR/${org}.log"
    FINTRUST_ORG="$org" FINTRUST_NETWORK_DIR="$NETWORK_DIR" "$BIN_DIR/fintrust-api" > "$log_file" 2>&1 &
    local pid=$!
    PIDS="$PIDS $pid"
    echo "$pid"
}

wait_for_api() {
    local name=$1
    local port=$2
    local pid=$3
    for attempt in $(seq 1 30); do
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "FAIL: $name exited"
            return 1
        fi
        if curl -sf "http://localhost:$port/healthz" >/dev/null 2>&1; then
            return 0
        fi
        sleep 0.5
    done
    echo "FAIL: $name not ready"
    return 1
}

echo "============================================"
echo "       FinTrust Invoice Financing Demo"
echo "============================================"
echo ""
echo "This demo shows a confidential B2B invoice financing workflow"
echo "using Hyperledger Fabric with private data collections."
echo ""

echo "=== Setting Up Network ==="
"$NETWORK_DIR/scripts/network-reset.sh" 2>/dev/null || true
"$NETWORK_DIR/scripts/network-up.sh"
"$NETWORK_DIR/scripts/chaincode-deploy.sh"

echo ""
echo "=== Building and Starting APIs ==="
cd "$ROOT_DIR/backend" && go build -o "$BIN_DIR/fintrust-api" ./cmd/fintrust-api
mkdir -p "$LOG_DIR"
rm -f "$ROOT_DIR"/fintrust-*.db* 2>/dev/null || true

SUPPLIER_PID=$(start_api supplier $SUPPLIER_PORT)
BUYER_PID=$(start_api buyer $BUYER_PORT)
FINANCE_PID=$(start_api finance $FINANCE_PORT)

wait_for_api "Supplier API" $SUPPLIER_PORT "$SUPPLIER_PID"
wait_for_api "Buyer API" $BUYER_PORT "$BUYER_PID"
wait_for_api "Finance API" $FINANCE_PORT "$FINANCE_PID"
echo "All APIs ready."

INVOICE_ID="DEMO-$(date +%s)"
DOC_HASH="sha256:$(openssl rand -hex 32)"
SALT1="$(openssl rand -hex 16)"
SALT2="$(openssl rand -hex 16)"
SALT3="$(openssl rand -hex 16)"
SALT4="$(openssl rand -hex 16)"
SALT5="$(openssl rand -hex 16)"

echo ""
echo "============================================"
echo "          Invoice Lifecycle Demo"
echo "============================================"
echo ""
echo "Invoice: $INVOICE_ID"
echo ""

echo "--- 1. Supplier creates invoice ---"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$SUPPLIER_PORT/api/v1/invoices" \
    -H "Content-Type: application/json" \
    -d '{
        "invoiceId": "'"$INVOICE_ID"'",
        "buyerMspId": "BuyerMSP",
        "documentHash": "'"$DOC_HASH"'",
        "commercialTerms": {"amountMinor": 100000, "currency": "USD", "dueDate": "2026-12-31", "paymentTerms": "NET-30", "salt": "'"$SALT1"'"},
        "paymentDetails": {"accountName": "Supplier Corp", "bankName": "Test Bank", "accountIdentifier": "ACC-123", "routingCode": "RTG-456", "paymentReference": "REF-789", "salt": "'"$SALT2"'"}
    }')
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: $STATUS"

echo ""
echo "--- 2. Buyer approves invoice ---"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$BUYER_PORT/api/v1/invoices/$INVOICE_ID/approve")
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: $STATUS"

echo ""
echo "--- 3. Supplier requests financing ---"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$SUPPLIER_PORT/api/v1/invoices/$INVOICE_ID/financing-request" \
    -H "Content-Type: application/json" \
    -d '{
        "disclosure": {"amountMinor": 100000, "currency": "USD", "dueDate": "2026-12-31", "paymentTerms": "NET-30", "salt": "'"$SALT1"'"},
        "financingRequest": {"requestedAmountMinor": 80000, "requestedTenor": "30 days", "salt": "'"$SALT3"'"},
        "disbursementDetails": {"accountName": "Supplier Corp", "bankName": "Disb Bank", "accountIdentifier": "DISB-123", "routingCode": "DISB-RTG", "salt": "'"$SALT4"'"}
    }')
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: $STATUS"

echo ""
echo "--- 4. Finance provides financing ---"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$FINANCE_PORT/api/v1/invoices/$INVOICE_ID/finance" \
    -H "Content-Type: application/json" \
    -d '{
        "financingAgreement": {"financedAmountMinor": 75000, "discountBps": 250, "maturityTerms": "30 days", "salt": "'"$SALT5"'"}
    }')
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: $STATUS"

echo ""
echo "--- 5. Buyer settles invoice ---"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$BUYER_PORT/api/v1/invoices/$INVOICE_ID/settle")
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
echo "Status: $STATUS"

echo ""
echo "============================================"
echo "       Private Data Access Control"
echo "============================================"
echo ""

echo "--- Supplier reads private invoice data ---"
if curl -sf "http://localhost:$SUPPLIER_PORT/api/v1/invoices/$INVOICE_ID/private" >/dev/null 2>&1; then
    echo "Result: ALLOWED"
else
    echo "Result: DENIED"
fi

echo ""
echo "--- Buyer reads private invoice data ---"
if curl -sf "http://localhost:$BUYER_PORT/api/v1/invoices/$INVOICE_ID/private" >/dev/null 2>&1; then
    echo "Result: ALLOWED"
else
    echo "Result: DENIED"
fi

echo ""
echo "--- Finance reads private invoice data ---"
if curl -sf "http://localhost:$FINANCE_PORT/api/v1/invoices/$INVOICE_ID/private" >/dev/null 2>&1; then
    echo "Result: ALLOWED (unexpected)"
else
    echo "Result: DENIED (correct - not a party)"
fi

echo ""
echo "--- Supplier reads financing terms ---"
if curl -sf "http://localhost:$SUPPLIER_PORT/api/v1/invoices/$INVOICE_ID/financing" >/dev/null 2>&1; then
    echo "Result: ALLOWED"
else
    echo "Result: DENIED"
fi

echo ""
echo "--- Finance reads financing terms ---"
if curl -sf "http://localhost:$FINANCE_PORT/api/v1/invoices/$INVOICE_ID/financing" >/dev/null 2>&1; then
    echo "Result: ALLOWED"
else
    echo "Result: DENIED"
fi

echo ""
echo "--- Buyer reads financing terms ---"
if curl -sf "http://localhost:$BUYER_PORT/api/v1/invoices/$INVOICE_ID/financing" >/dev/null 2>&1; then
    echo "Result: ALLOWED (unexpected)"
else
    echo "Result: DENIED (correct - not a financing party)"
fi

echo ""
echo "============================================"
echo "      Double-Financing Prevention"
echo "============================================"
echo ""
echo "Attempting second financing on already-financed invoice..."
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$FINANCE_PORT/api/v1/invoices/$INVOICE_ID/finance" \
    -H "Content-Type: application/json" \
    -d '{
        "financingAgreement": {"financedAmountMinor": 50000, "discountBps": 300, "maturityTerms": "60 days", "salt": "'"$(openssl rand -hex 16)"'"}
    }')
if echo "$RESP" | grep -q '"error"'; then
    echo "Result: REJECTED (double financing prevented)"
else
    echo "Result: Unexpected success - $RESP"
fi

echo ""
echo "============================================"
echo "           Event Projection"
echo "============================================"
echo ""
echo "Waiting for events to be projected..."
sleep 2
EVENTS=$(curl -sf "http://localhost:$SUPPLIER_PORT/api/v1/invoices/$INVOICE_ID/events" 2>/dev/null || echo "[]")
set +o pipefail
EVENT_NAMES=$(echo "$EVENTS" | grep -o '"EventName":"[^"]*"' | cut -d'"' -f4 | sort)
set -o pipefail
echo "Lifecycle events for $INVOICE_ID:"
echo "$EVENT_NAMES" | while read -r name; do
    [ -n "$name" ] && echo "  - $name"
done

echo ""
echo "============================================"
echo "         Demo Complete"
echo "============================================"
echo ""
echo "This demo showed:"
echo "  - Full invoice lifecycle (Create -> Approve -> Request -> Finance -> Settle)"
echo "  - Private data isolation (commercial terms, financing terms)"
echo "  - Double-financing prevention"
echo "  - Event projection for off-chain queries"
echo ""
echo "For deeper verification, run:"
echo "  make verify-e2e   # E2E security and concurrency tests"
echo "  make verify-api   # API and checkpoint restart verification"
echo ""
