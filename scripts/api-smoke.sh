#!/usr/bin/env bash
set -euo pipefail

SUPPLIER_API="${SUPPLIER_API:-http://localhost:8081}"
BUYER_API="${BUYER_API:-http://localhost:8082}"
FINANCE_API="${FINANCE_API:-http://localhost:8083}"

INVOICE_ID="SMOKE-$(date +%s)-$(printf '%04X' $RANDOM)"
DOC_HASH="sha256:$(openssl rand -hex 32)"
SALT1="$(openssl rand -hex 16)"
SALT2="$(openssl rand -hex 16)"
SALT3="$(openssl rand -hex 16)"
SALT4="$(openssl rand -hex 16)"
SALT5="$(openssl rand -hex 16)"

echo "=== API Smoke Test ==="
echo "Invoice ID: $INVOICE_ID"

check_status() {
    local expected=$1
    local response=$2
    local status
    status=$(echo "$response" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ "$status" != "$expected" ]; then
        echo "FAIL: expected $expected, got: $response"
        exit 1
    fi
}

echo ""
echo "--- Health checks ---"
curl -sf "$SUPPLIER_API/healthz" >/dev/null && echo "OK: Supplier API healthy" || { echo "FAIL: Supplier API"; exit 1; }
curl -sf "$BUYER_API/healthz" >/dev/null && echo "OK: Buyer API healthy" || { echo "FAIL: Buyer API"; exit 1; }
curl -sf "$FINANCE_API/healthz" >/dev/null && echo "OK: Finance API healthy" || { echo "FAIL: Finance API"; exit 1; }

echo ""
echo "--- Create invoice (Supplier) ---"
RESP=$(curl -s --max-time 60 -X POST "$SUPPLIER_API/api/v1/invoices" \
    -H "Content-Type: application/json" \
    -d '{
        "invoiceId": "'"$INVOICE_ID"'",
        "buyerMspId": "BuyerMSP",
        "documentHash": "'"$DOC_HASH"'",
        "commercialTerms": {
            "amountMinor": 100000,
            "currency": "USD",
            "dueDate": "2026-12-31",
            "paymentTerms": "NET-30",
            "salt": "'"$SALT1"'"
        },
        "paymentDetails": {
            "accountName": "Supplier Corp",
            "bankName": "Test Bank",
            "accountIdentifier": "123456789",
            "routingCode": "TESTBANK",
            "paymentReference": "INV-REF",
            "salt": "'"$SALT2"'"
        }
    }')
echo "Response: $RESP"
check_status "CREATED" "$RESP"
echo "OK: Invoice created"

echo ""
echo "--- Get invoice (Supplier) ---"
RESP=$(curl -sf --max-time 30 "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID")
check_status "CREATED" "$RESP"
echo "OK: Status is CREATED"

echo ""
echo "--- Approve invoice (Buyer) ---"
RESP=$(curl -s --max-time 60 -X POST "$BUYER_API/api/v1/invoices/$INVOICE_ID/approve")
echo "Response: $RESP"
check_status "APPROVED" "$RESP"
echo "OK: Invoice approved"

echo ""
echo "--- Request financing (Supplier) ---"
RESP=$(curl -s --max-time 60 -X POST "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID/financing-request" \
    -H "Content-Type: application/json" \
    -d '{
        "disclosure": {
            "amountMinor": 100000,
            "currency": "USD",
            "dueDate": "2026-12-31",
            "paymentTerms": "NET-30",
            "salt": "'"$SALT1"'"
        },
        "financingRequest": {
            "requestedAmountMinor": 80000,
            "requestedTenor": "30 days",
            "salt": "'"$SALT3"'"
        },
        "disbursementDetails": {
            "accountName": "Supplier Corp",
            "bankName": "Disb Bank",
            "accountIdentifier": "DISB-123",
            "routingCode": "DISBBANK",
            "salt": "'"$SALT4"'"
        }
    }')
echo "Response: $RESP"
check_status "FINANCING_REQUESTED" "$RESP"
echo "OK: Financing requested"

echo ""
echo "--- Finance invoice (Finance) ---"
RESP=$(curl -s --max-time 60 -X POST "$FINANCE_API/api/v1/invoices/$INVOICE_ID/finance" \
    -H "Content-Type: application/json" \
    -d '{
        "financingAgreement": {
            "financedAmountMinor": 75000,
            "discountBps": 250,
            "maturityTerms": "30 days from invoice date",
            "salt": "'"$SALT5"'"
        }
    }')
echo "Response: $RESP"
check_status "FINANCED" "$RESP"
echo "OK: Invoice financed"

echo ""
echo "--- Settle invoice (Buyer) ---"
RESP=$(curl -s --max-time 60 -X POST "$BUYER_API/api/v1/invoices/$INVOICE_ID/settle")
echo "Response: $RESP"
check_status "SETTLED" "$RESP"
echo "OK: Invoice settled"

echo ""
echo "--- Verify final state ---"
RESP=$(curl -sf --max-time 30 "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID")
check_status "SETTLED" "$RESP"
FINANCED=$(echo "$RESP" | grep -o '"financed":[^,}]*' | cut -d':' -f2)
if [ "$FINANCED" != "true" ]; then
    echo "FAIL: expected financed=true, got: $RESP"
    exit 1
fi
echo "OK: Final state verified"

echo ""
echo "--- Check event projection ---"
EVENT_COUNT=0
for i in 1 2 3 4 5 6 7 8 9 10; do
    EVENTS=$(curl -sf "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID/events" 2>/dev/null || echo "[]")
    set +o pipefail
    EVENT_COUNT=$(echo "$EVENTS" | grep -o '"EventName"' 2>/dev/null | wc -l | tr -d ' ')
    EVENT_COUNT=${EVENT_COUNT:-0}
    set -o pipefail
    if [ "$EVENT_COUNT" -ge 5 ]; then
        echo "OK: Event projection has $EVENT_COUNT events"
        break
    fi
    sleep 0.5
done
if [ "$EVENT_COUNT" -lt 5 ]; then
    echo "WARN: expected at least 5 events, got $EVENT_COUNT"
fi

echo ""
echo "=== Smoke Test Passed ==="
