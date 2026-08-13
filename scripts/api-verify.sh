#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
NETWORK_DIR="$ROOT_DIR/network"
BIN_DIR="$ROOT_DIR/bin"
E2E_DIR="$ROOT_DIR/test/e2e"

SUPPLIER_PORT=8081
BUYER_PORT=8082
FINANCE_PORT=8083

LOG_DIR="$ROOT_DIR/.api-logs"
CHECKPOINT_DB="$ROOT_DIR/fintrust-checkpoint-test.db"

PIDS=""

FAILED=false

cleanup() {
    echo ""
    echo "--- Cleanup ---"
    for pid in $PIDS; do
        kill "$pid" 2>/dev/null || true
    done
    sleep 1
    if [ "$FAILED" = true ]; then
        echo "Preserving logs in $LOG_DIR for debugging"
        for logfile in "$LOG_DIR"/*.log; do
            if [ -f "$logfile" ]; then
                echo "=== $(basename "$logfile") ==="
                tail -30 "$logfile"
            fi
        done
    else
        rm -rf "$LOG_DIR"
    fi
    rm -f "$ROOT_DIR"/fintrust-supplier.db "$ROOT_DIR"/fintrust-supplier.db-wal "$ROOT_DIR"/fintrust-supplier.db-shm 2>/dev/null
    rm -f "$ROOT_DIR"/fintrust-buyer.db "$ROOT_DIR"/fintrust-buyer.db-wal "$ROOT_DIR"/fintrust-buyer.db-shm 2>/dev/null
    rm -f "$ROOT_DIR"/fintrust-finance.db "$ROOT_DIR"/fintrust-finance.db-wal "$ROOT_DIR"/fintrust-finance.db-shm 2>/dev/null
    rm -f "$CHECKPOINT_DB" "$CHECKPOINT_DB"-wal "$CHECKPOINT_DB"-shm 2>/dev/null
}

trap cleanup EXIT

wait_for_chaincode() {
    echo "--- Waiting for chaincode readiness ---"
    go clean -testcache
    local max_attempts=30
    for attempt in $(seq 1 $max_attempts); do
        echo "Chaincode probe attempt $attempt/$max_attempts..."
        set +e
        cd "$E2E_DIR"
        OUTPUT=$(FINTRUST_E2E=1 go test -v -run TestChaincodeReady -timeout 60s ./... 2>&1)
        RESULT=$?
        set -e
        if [ $RESULT -eq 0 ]; then
            echo "Chaincode query ready, warming up with E2E write test..."
            set +e
            cd "$E2E_DIR"
            OUTPUT=$(FINTRUST_E2E=1 go test -v -run TestHappyPath -timeout 120s ./... 2>&1)
            RESULT=$?
            set -e
            if [ $RESULT -eq 0 ]; then
                echo "Chaincode fully warmed up"
                return 0
            fi
            echo "Warmup test failed, retrying..."
            echo "$OUTPUT" | tail -5
        else
            echo "$OUTPUT" | tail -3
        fi
        sleep 2
    done
    echo "FAIL: Chaincode not ready after $max_attempts attempts"
    return 1
}

start_api() {
    local org=$1
    local port=$2
    local db_path="${3:-}"
    local log_file="$LOG_DIR/${org}-${port}.log"

    if [ -n "$db_path" ]; then
        DATABASE_PATH="$db_path" FINTRUST_ORG="$org" FINTRUST_NETWORK_DIR="$NETWORK_DIR" \
            "$BIN_DIR/fintrust-api" > "$log_file" 2>&1 &
    else
        FINTRUST_ORG="$org" FINTRUST_NETWORK_DIR="$NETWORK_DIR" \
            "$BIN_DIR/fintrust-api" > "$log_file" 2>&1 &
    fi
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
            echo "FAIL: $name (PID $pid) exited"
            cat "$LOG_DIR/${name}-${port}.log" 2>/dev/null || true
            return 1
        fi
        if curl -sf "http://localhost:$port/healthz" >/dev/null 2>&1; then
            echo "OK: $name ready on port $port (PID $pid)"
            return 0
        fi
        sleep 0.5
    done
    echo "FAIL: $name not ready"
    tail -20 "$LOG_DIR/${name}-${port}.log" 2>/dev/null || true
    return 1
}

wait_for_events() {
    local api_url=$1
    local invoice_id=$2
    local expected_count=$3
    local count=0

    for attempt in $(seq 1 60); do
        local events
        events=$(curl -sf "$api_url/api/v1/invoices/$invoice_id/events" 2>/dev/null || echo "[]")
        set +o pipefail
        count=$(echo "$events" | grep -o '"EventName"' 2>/dev/null | wc -l | tr -d ' ')
        count=${count:-0}
        set -o pipefail
        if [ "$count" -ge "$expected_count" ]; then
            echo "OK: Found $count events (expected >= $expected_count)"
            return 0
        fi
        sleep 0.5
    done
    echo "FAIL: Expected $expected_count events, got $count"
    return 1
}

get_event_count() {
    local db=$1
    local invoice_id=$2
    sqlite3 "$db" "SELECT COUNT(*) FROM invoice_events WHERE invoice_id='$invoice_id'" 2>/dev/null || echo "0"
}

get_checkpoint() {
    local db=$1
    sqlite3 "$db" "SELECT block_number FROM event_checkpoint WHERE id=1" 2>/dev/null || echo "0"
}

stop_pid() {
    local pid=$1
    kill "$pid" 2>/dev/null || true
    for i in $(seq 1 10); do
        if ! kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
        sleep 0.2
    done
    kill -9 "$pid" 2>/dev/null || true
}

echo "=== Full API Verification ==="

mkdir -p "$LOG_DIR"
rm -f "$ROOT_DIR"/fintrust-supplier.db "$ROOT_DIR"/fintrust-supplier.db-wal "$ROOT_DIR"/fintrust-supplier.db-shm 2>/dev/null || true
rm -f "$ROOT_DIR"/fintrust-buyer.db "$ROOT_DIR"/fintrust-buyer.db-wal "$ROOT_DIR"/fintrust-buyer.db-shm 2>/dev/null || true
rm -f "$ROOT_DIR"/fintrust-finance.db "$ROOT_DIR"/fintrust-finance.db-wal "$ROOT_DIR"/fintrust-finance.db-shm 2>/dev/null || true
rm -f "$CHECKPOINT_DB" "$CHECKPOINT_DB"-wal "$CHECKPOINT_DB"-shm 2>/dev/null || true

wait_for_chaincode

echo ""
echo "--- Starting APIs for Smoke Test ---"

SUPPLIER_PID=$(start_api supplier $SUPPLIER_PORT)
BUYER_PID=$(start_api buyer $BUYER_PORT)
FINANCE_PID=$(start_api finance $FINANCE_PORT)

wait_for_api supplier $SUPPLIER_PORT "$SUPPLIER_PID"
wait_for_api buyer $BUYER_PORT "$BUYER_PID"
wait_for_api finance $FINANCE_PORT "$FINANCE_PID"

echo ""
echo "--- Running HTTP Smoke Test ---"
if ! "$SCRIPT_DIR/api-smoke.sh"; then
    echo "Smoke test failed"
    echo "=== supplier log ===" && tail -30 "$LOG_DIR/supplier-$SUPPLIER_PORT.log" 2>/dev/null || true
    echo "=== buyer log ===" && tail -30 "$LOG_DIR/buyer-$BUYER_PORT.log" 2>/dev/null || true
    echo "=== finance log ===" && tail -30 "$LOG_DIR/finance-$FINANCE_PORT.log" 2>/dev/null || true
    FAILED=true
    exit 1
fi

echo ""
echo "--- Stopping all APIs ---"
stop_pid "$SUPPLIER_PID"
stop_pid "$BUYER_PID"
stop_pid "$FINANCE_PID"
PIDS=""
sleep 1

rm -f "$ROOT_DIR"/fintrust-supplier.db "$ROOT_DIR"/fintrust-supplier.db-wal "$ROOT_DIR"/fintrust-supplier.db-shm 2>/dev/null || true
rm -f "$ROOT_DIR"/fintrust-buyer.db "$ROOT_DIR"/fintrust-buyer.db-wal "$ROOT_DIR"/fintrust-buyer.db-shm 2>/dev/null || true
rm -f "$ROOT_DIR"/fintrust-finance.db "$ROOT_DIR"/fintrust-finance.db-wal "$ROOT_DIR"/fintrust-finance.db-shm 2>/dev/null || true

echo ""
echo "=== Checkpoint Restart Verification ==="
echo ""
echo "--- Phase 1: Create and Approve invoice with checkpoint DB ---"

SUPPLIER_PID=$(start_api supplier $SUPPLIER_PORT "$CHECKPOINT_DB")
BUYER_PID=$(start_api buyer $BUYER_PORT)

wait_for_api supplier $SUPPLIER_PORT "$SUPPLIER_PID"
wait_for_api buyer $BUYER_PORT "$BUYER_PID"

INVOICE_ID="CHKPT-$(date +%s)-$(printf '%04X' $RANDOM)"
DOC_HASH="sha256:$(openssl rand -hex 32)"
SALT1="$(openssl rand -hex 16)"
SALT2="$(openssl rand -hex 16)"

echo "Creating invoice: $INVOICE_ID"
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$SUPPLIER_PORT/api/v1/invoices" \
    -H "Content-Type: application/json" \
    -d '{
        "invoiceId": "'"$INVOICE_ID"'",
        "buyerMspId": "BuyerMSP",
        "documentHash": "'"$DOC_HASH"'",
        "commercialTerms": {"amountMinor": 50000, "currency": "USD", "dueDate": "2026-12-31", "paymentTerms": "NET-30", "salt": "'"$SALT1"'"},
        "paymentDetails": {"accountName": "Test", "bankName": "Bank", "accountIdentifier": "123", "routingCode": "RTG", "paymentReference": "REF", "salt": "'"$SALT2"'"}
    }')
echo "Create response: $RESP"

echo "Approving invoice..."
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$BUYER_PORT/api/v1/invoices/$INVOICE_ID/approve")
echo "Approve response: $RESP"

echo "Waiting for events to be projected..."
if ! wait_for_events "http://localhost:$SUPPLIER_PORT" "$INVOICE_ID" 2; then
    echo "Event projection failed - checking API logs"
    echo "=== supplier log ===" && tail -30 "$LOG_DIR/supplier-$SUPPLIER_PORT.log" 2>/dev/null || true
    FAILED=true
    exit 1
fi

EVENTS_BEFORE=$(get_event_count "$CHECKPOINT_DB" "$INVOICE_ID")
CHECKPOINT_BEFORE=$(get_checkpoint "$CHECKPOINT_DB")

echo ""
echo "Before shutdown:"
echo "  Projected event count: $EVENTS_BEFORE"
echo "  Checkpoint block: $CHECKPOINT_BEFORE"

echo ""
echo "--- Phase 2: Stop event consumer, submit more transactions ---"

echo "Stopping Supplier API (event consumer using checkpoint DB)..."
stop_pid "$SUPPLIER_PID"
PIDS="$BUYER_PID"
sleep 1

echo "Starting temporary Supplier API (separate DB) for submitting transactions..."
TEMP_SUPPLIER_PID=$(start_api supplier $SUPPLIER_PORT)
wait_for_api supplier $SUPPLIER_PORT "$TEMP_SUPPLIER_PID"

SALT3="$(openssl rand -hex 16)"
SALT4="$(openssl rand -hex 16)"

echo "Submitting RequestFinancing while checkpoint consumer is offline..."
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$SUPPLIER_PORT/api/v1/invoices/$INVOICE_ID/financing-request" \
    -H "Content-Type: application/json" \
    -d '{
        "disclosure": {"amountMinor": 50000, "currency": "USD", "dueDate": "2026-12-31", "paymentTerms": "NET-30", "salt": "'"$SALT1"'"},
        "financingRequest": {"requestedAmountMinor": 40000, "requestedTenor": "30 days", "salt": "'"$SALT3"'"},
        "disbursementDetails": {"accountName": "Supplier", "bankName": "Bank", "accountIdentifier": "456", "routingCode": "RTG2", "salt": "'"$SALT4"'"}
    }')
echo "RequestFinancing response: $RESP"

stop_pid "$TEMP_SUPPLIER_PID"
stop_pid "$BUYER_PID"
PIDS=""
sleep 1

echo "Starting Finance API for financing..."
FINANCE_PID=$(start_api finance $FINANCE_PORT)
wait_for_api finance $FINANCE_PORT "$FINANCE_PID"

SALT5="$(openssl rand -hex 16)"
echo "Submitting FinanceInvoice while checkpoint consumer is offline..."
RESP=$(curl -s --max-time 60 -X POST "http://localhost:$FINANCE_PORT/api/v1/invoices/$INVOICE_ID/finance" \
    -H "Content-Type: application/json" \
    -d '{
        "financingAgreement": {"financedAmountMinor": 38000, "discountBps": 200, "maturityTerms": "30 days", "salt": "'"$SALT5"'"}
    }')
echo "FinanceInvoice response: $RESP"

stop_pid "$FINANCE_PID"
PIDS=""
sleep 1

echo ""
echo "--- Phase 3: Restart consumer with checkpoint DB, verify catch-up ---"

SUPPLIER_PID=$(start_api supplier $SUPPLIER_PORT "$CHECKPOINT_DB")
wait_for_api supplier $SUPPLIER_PORT "$SUPPLIER_PID"

echo "Waiting for missed events to be caught up..."
if ! wait_for_events "http://localhost:$SUPPLIER_PORT" "$INVOICE_ID" 4; then
    echo "Event catch-up failed - checking API logs"
    echo "=== supplier log ===" && tail -50 "$LOG_DIR/supplier-$SUPPLIER_PORT.log" 2>/dev/null || true
    FAILED=true
    exit 1
fi

EVENTS_AFTER=$(get_event_count "$CHECKPOINT_DB" "$INVOICE_ID")
CHECKPOINT_AFTER=$(get_checkpoint "$CHECKPOINT_DB")

TOTAL_ROWS=$(sqlite3 "$CHECKPOINT_DB" "SELECT COUNT(*) FROM invoice_events WHERE invoice_id='$INVOICE_ID'" 2>/dev/null || echo "0")
DUPLICATES=$(sqlite3 "$CHECKPOINT_DB" "SELECT COUNT(*) - COUNT(DISTINCT transaction_id || event_name) FROM invoice_events WHERE invoice_id='$INVOICE_ID'" 2>/dev/null || echo "0")

echo ""
echo "After restart:"
echo "  Projected event count: $EVENTS_AFTER"
echo "  Checkpoint block: $CHECKPOINT_AFTER"
echo "  Total DB rows for invoice: $TOTAL_ROWS"
echo "  Duplicate events: $DUPLICATES"

PASS=true

if [ "$EVENTS_AFTER" -gt "$EVENTS_BEFORE" ]; then
    echo ""
    echo "OK: Missed events were caught up ($EVENTS_BEFORE -> $EVENTS_AFTER)"
else
    echo ""
    echo "FAIL: Events not caught up (before=$EVENTS_BEFORE, after=$EVENTS_AFTER)"
    PASS=false
fi

if [ "$CHECKPOINT_AFTER" -gt "$CHECKPOINT_BEFORE" ]; then
    echo "OK: Checkpoint advanced ($CHECKPOINT_BEFORE -> $CHECKPOINT_AFTER)"
else
    echo "FAIL: Checkpoint did not advance"
    PASS=false
fi

if [ "$DUPLICATES" -eq 0 ]; then
    echo "OK: No duplicate events"
else
    echo "FAIL: Found $DUPLICATES duplicate events"
    PASS=false
fi

stop_pid "$SUPPLIER_PID"
PIDS=""

if [ "$PASS" = true ]; then
    echo ""
    echo "=== All Verification Passed ==="
    exit 0
else
    echo ""
    echo "=== Verification FAILED ==="
    FAILED=true
    exit 1
fi
