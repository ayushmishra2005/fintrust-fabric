#!/usr/bin/env bash
set -euo pipefail

SUPPLIER_API="${SUPPLIER_API:-http://localhost:8081}"
BUYER_API="${BUYER_API:-http://localhost:8082}"
FINANCE_API="${FINANCE_API:-http://localhost:8083}"

INVOICE_ID="SMOKE-$(date +%s)-$(printf '%04x' $RANDOM)"
DOC_HASH="sha256:$(openssl rand -hex 32)"
SALT1="$(openssl rand -hex 16)"
SALT2="$(openssl rand -hex 16)"
SALT3="$(openssl rand -hex 16)"
SALT4="$(openssl rand -hex 16)"
SALT5="$(openssl rand -hex 16)"

echo "=== API Smoke Test ==="
echo "Invoice ID: $INVOICE_ID"

check_health() {
    local name=$1 url=$2
    if ! curl -sf "$url/healthz" >/dev/null; then
        echo "FAIL: $name not healthy"
        exit 1
    fi
    echo "OK: $name healthy"
}

echo ""
echo "--- Health checks ---"
check_health "Supplier API" "$SUPPLIER_API"
check_health "Buyer API" "$BUYER_API"
check_health "Finance API" "$FINANCE_API"

echo ""
echo "--- Create invoice (Supplier) ---"
RESP=$(curl -s -X POST "$SUPPLIER_API/api/v1/invoices" \
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
if [ -z "$RESP" ]; then
    echo "FAIL: empty response"
    exit 1
fi
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "CREATED" ]; then
    echo "FAIL: expected CREATED, got response: $RESP"
    exit 1
fi
echo "OK: Invoice created"

echo ""
echo "--- Get invoice (Supplier) ---"
RESP=$(curl -sf "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID")
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "CREATED" ]; then
    echo "FAIL: expected CREATED, got $STATUS"
    exit 1
fi
echo "OK: Status is CREATED"

echo ""
echo "--- Approve invoice (Buyer) ---"
RESP=$(curl -sf -X POST "$BUYER_API/api/v1/invoices/$INVOICE_ID/approve")
echo "Response: $RESP"
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "APPROVED" ]; then
    echo "FAIL: expected APPROVED, got $STATUS"
    exit 1
fi
echo "OK: Invoice approved"

echo ""
echo "--- Request financing (Supplier) ---"
RESP=$(curl -sf -X POST "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID/financing-request" \
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
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "FINANCING_REQUESTED" ]; then
    echo "FAIL: expected FINANCING_REQUESTED, got $STATUS"
    exit 1
fi
echo "OK: Financing requested"

echo ""
echo "--- Finance invoice (Finance) ---"
RESP=$(curl -sf -X POST "$FINANCE_API/api/v1/invoices/$INVOICE_ID/finance" \
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
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "FINANCED" ]; then
    echo "FAIL: expected FINANCED, got $STATUS"
    exit 1
fi
echo "OK: Invoice financed"

echo ""
echo "--- Settle invoice (Buyer) ---"
RESP=$(curl -sf -X POST "$BUYER_API/api/v1/invoices/$INVOICE_ID/settle")
echo "Response: $RESP"
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$STATUS" != "SETTLED" ]; then
    echo "FAIL: expected SETTLED, got $STATUS"
    exit 1
fi
echo "OK: Invoice settled"

echo ""
echo "--- Verify final state ---"
RESP=$(curl -sf "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID")
STATUS=$(echo "$RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
FINANCED=$(echo "$RESP" | grep -o '"financed":[^,}]*' | cut -d':' -f2)
if [ "$STATUS" != "SETTLED" ]; then
    echo "FAIL: final status expected SETTLED, got $STATUS"
    exit 1
fi
if [ "$FINANCED" != "true" ]; then
    echo "FAIL: expected financed=true"
    exit 1
fi
echo "OK: Final state verified"

echo ""
echo "--- Check event projection ---"
sleep 2
EVENTS=$(curl -sf "$SUPPLIER_API/api/v1/invoices/$INVOICE_ID/events")
EVENT_COUNT=$(echo "$EVENTS" | grep -o '"event_name"' | wc -l | tr -d ' ')
if [ "$EVENT_COUNT" -lt 5 ]; then
    echo "WARN: expected at least 5 events, got $EVENT_COUNT"
else
    echo "OK: Event projection has $EVENT_COUNT events"
fi

echo ""
echo "=== Smoke Test Passed ==="
