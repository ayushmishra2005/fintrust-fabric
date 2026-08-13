package projection

import (
	"context"
	"os"
	"testing"
)

func TestStoreInitSchema(t *testing.T) {
	dbPath := tempDB(t)
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	count, err := store.EventCount(context.Background())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 events, got %d", count)
	}
}

func TestCheckpointPersistence(t *testing.T) {
	dbPath := tempDB(t)
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	ctx := context.Background()

	checkpoint, exists, err := store.GetCheckpoint(ctx)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if exists {
		t.Error("expected no checkpoint initially")
	}

	payload := `{"invoiceId":"INV-001","supplierMspId":"SupplierMSP","buyerMspId":"BuyerMSP","status":"CREATED","timestamp":"2026-01-01T00:00:00Z","txId":"tx1"}`
	err = store.InsertEventAndCheckpoint(ctx, 5, "tx1", "InvoiceCreated", []byte(payload))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	checkpoint, exists, err = store.GetCheckpoint(ctx)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if !exists {
		t.Error("expected checkpoint to exist")
	}
	if checkpoint != 5 {
		t.Errorf("got checkpoint %d, want 5", checkpoint)
	}

	store.Close()

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()

	checkpoint, exists, err = store2.GetCheckpoint(ctx)
	if err != nil {
		t.Fatalf("get checkpoint after reopen: %v", err)
	}
	if !exists || checkpoint != 5 {
		t.Errorf("checkpoint not persisted: exists=%v, checkpoint=%d", exists, checkpoint)
	}
}

func TestEventIdempotency(t *testing.T) {
	store, err := NewStore(tempDB(t))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	payload := `{"invoiceId":"INV-001","supplierMspId":"SupplierMSP","buyerMspId":"BuyerMSP","status":"CREATED","timestamp":"2026-01-01T00:00:00Z","txId":"tx1"}`

	err = store.InsertEventAndCheckpoint(ctx, 5, "tx1", "InvoiceCreated", []byte(payload))
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	err = store.InsertEventAndCheckpoint(ctx, 5, "tx1", "InvoiceCreated", []byte(payload))
	if err != nil {
		t.Fatalf("second insert should not error: %v", err)
	}

	count, err := store.EventCount(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 event after duplicate insert, got %d", count)
	}
}

func TestQueryEvents(t *testing.T) {
	store, err := NewStore(tempDB(t))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	events := []struct {
		block uint64
		txID  string
		name  string
		invID string
	}{
		{1, "tx1", "InvoiceCreated", "INV-001"},
		{2, "tx2", "InvoiceApproved", "INV-001"},
		{3, "tx3", "InvoiceCreated", "INV-002"},
	}

	for _, e := range events {
		payload := `{"invoiceId":"` + e.invID + `","supplierMspId":"SupplierMSP","buyerMspId":"BuyerMSP","status":"CREATED","timestamp":"2026-01-01T00:00:00Z","txId":"` + e.txID + `"}`
		if err := store.InsertEventAndCheckpoint(ctx, e.block, e.txID, e.name, []byte(payload)); err != nil {
			t.Fatalf("insert %s: %v", e.txID, err)
		}
	}

	t.Run("filter by invoice_id", func(t *testing.T) {
		result, err := store.QueryEvents(ctx, EventFilter{InvoiceID: "INV-001"})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
	})

	t.Run("filter by event_name", func(t *testing.T) {
		result, err := store.QueryEvents(ctx, EventFilter{EventName: "InvoiceCreated"})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
	})

	t.Run("limit", func(t *testing.T) {
		result, err := store.QueryEvents(ctx, EventFilter{Limit: 1})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
	})

	t.Run("max limit enforced", func(t *testing.T) {
		result, err := store.QueryEvents(ctx, EventFilter{Limit: 1000})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("got %d events, want 3", len(result))
		}
	})
}

func TestCheckpointResume(t *testing.T) {
	dbPath := tempDB(t)
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	ctx := context.Background()

	for i := uint64(1); i <= 5; i++ {
		payload := `{"invoiceId":"INV-001","supplierMspId":"SupplierMSP","buyerMspId":"BuyerMSP","status":"CREATED","timestamp":"2026-01-01T00:00:00Z","txId":"tx` + string(rune('0'+i)) + `"}`
		if err := store.InsertEventAndCheckpoint(ctx, i, "tx"+string(rune('0'+i)), "Event", []byte(payload)); err != nil {
			t.Fatalf("insert block %d: %v", i, err)
		}
	}

	checkpoint, exists, _ := store.GetCheckpoint(ctx)
	if !exists || checkpoint != 5 {
		t.Fatalf("expected checkpoint 5, got %d", checkpoint)
	}

	count1, _ := store.EventCount(ctx)
	store.Close()

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	checkpoint2, exists2, _ := store2.GetCheckpoint(ctx)
	if !exists2 || checkpoint2 != 5 {
		t.Errorf("checkpoint not preserved: exists=%v, val=%d", exists2, checkpoint2)
	}

	count2, _ := store2.EventCount(ctx)
	if count1 != count2 {
		t.Errorf("event count changed: before=%d, after=%d", count1, count2)
	}
}

func TestAtomicEventAndCheckpoint(t *testing.T) {
	store, err := NewStore(tempDB(t))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	payload := `{"invoiceId":"INV-001","supplierMspId":"SupplierMSP","buyerMspId":"BuyerMSP","status":"CREATED","timestamp":"2026-01-01T00:00:00Z","txId":"tx1"}`
	err = store.InsertEventAndCheckpoint(ctx, 10, "tx1", "InvoiceCreated", []byte(payload))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	checkpoint, _, _ := store.GetCheckpoint(ctx)
	count, _ := store.EventCount(ctx)

	if checkpoint != 10 {
		t.Errorf("checkpoint=%d, want 10", checkpoint)
	}
	if count != 1 {
		t.Errorf("count=%d, want 1", count)
	}
}

func tempDB(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "fintrust-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	t.Cleanup(func() {
		os.Remove(path)
		os.Remove(path + "-shm")
		os.Remove(path + "-wal")
	})
	return path
}
