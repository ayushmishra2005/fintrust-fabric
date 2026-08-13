package projection

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS invoice_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		block_number INTEGER NOT NULL,
		transaction_id TEXT NOT NULL,
		event_name TEXT NOT NULL,
		invoice_id TEXT NOT NULL,
		status TEXT NOT NULL,
		supplier_msp_id TEXT NOT NULL,
		buyer_msp_id TEXT NOT NULL,
		financier_msp_id TEXT,
		event_timestamp TEXT NOT NULL,
		created_at TEXT NOT NULL,
		UNIQUE(transaction_id, event_name)
	);

	CREATE INDEX IF NOT EXISTS idx_events_invoice_id ON invoice_events(invoice_id);
	CREATE INDEX IF NOT EXISTS idx_events_event_name ON invoice_events(event_name);
	CREATE INDEX IF NOT EXISTS idx_events_block ON invoice_events(block_number);

	CREATE TABLE IF NOT EXISTS event_checkpoint (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		block_number INTEGER NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

type InvoiceEvent struct {
	ID             int64
	BlockNumber    uint64
	TransactionID  string
	EventName      string
	InvoiceID      string
	Status         string
	SupplierMSPID  string
	BuyerMSPID     string
	FinancierMSPID string
	EventTimestamp string
	CreatedAt      string
}

type EventPayload struct {
	InvoiceID      string `json:"invoiceId"`
	SupplierMSPID  string `json:"supplierMspId"`
	BuyerMSPID     string `json:"buyerMspId"`
	FinancierMSPID string `json:"financierMspId,omitempty"`
	Status         string `json:"status"`
	Timestamp      string `json:"timestamp"`
	TxID           string `json:"txId"`
}

// InsertEventAndCheckpoint inserts an event and updates checkpoint atomically.
// This ensures we never lose events or duplicate them on restart.
func (s *Store) InsertEventAndCheckpoint(ctx context.Context, blockNum uint64, txID, eventName string, payload []byte) error {
	var evt EventPayload
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("parse event payload: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO invoice_events
			(block_number, transaction_id, event_name, invoice_id, status,
			 supplier_msp_id, buyer_msp_id, financier_msp_id, event_timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(transaction_id, event_name) DO NOTHING
	`, blockNum, txID, eventName, evt.InvoiceID, evt.Status,
		evt.SupplierMSPID, evt.BuyerMSPID, nullString(evt.FinancierMSPID), evt.Timestamp, now)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO event_checkpoint (id, block_number, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET block_number = excluded.block_number, updated_at = excluded.updated_at
	`, blockNum, now)
	if err != nil {
		return fmt.Errorf("update checkpoint: %w", err)
	}

	return tx.Commit()
}

func (s *Store) GetCheckpoint(ctx context.Context) (uint64, bool, error) {
	var blockNum uint64
	err := s.db.QueryRowContext(ctx, "SELECT block_number FROM event_checkpoint WHERE id = 1").Scan(&blockNum)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return blockNum, true, nil
}

type EventFilter struct {
	InvoiceID string
	EventName string
	Limit     int
}

func (s *Store) QueryEvents(ctx context.Context, filter EventFilter) ([]InvoiceEvent, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}

	query := `SELECT id, block_number, transaction_id, event_name, invoice_id, status,
		supplier_msp_id, buyer_msp_id, COALESCE(financier_msp_id, ''), event_timestamp, created_at
		FROM invoice_events WHERE 1=1`
	var args []any

	if filter.InvoiceID != "" {
		query += " AND invoice_id = ?"
		args = append(args, filter.InvoiceID)
	}
	if filter.EventName != "" {
		query += " AND event_name = ?"
		args = append(args, filter.EventName)
	}

	query += " ORDER BY block_number DESC, id DESC LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []InvoiceEvent
	for rows.Next() {
		var e InvoiceEvent
		err := rows.Scan(&e.ID, &e.BlockNumber, &e.TransactionID, &e.EventName,
			&e.InvoiceID, &e.Status, &e.SupplierMSPID, &e.BuyerMSPID,
			&e.FinancierMSPID, &e.EventTimestamp, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) EventCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoice_events").Scan(&count)
	return count, err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
