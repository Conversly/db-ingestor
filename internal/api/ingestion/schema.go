package ingestion

import "time"

// Database models/schemas
type IngestionRecord struct {
	ID        string            `db:"id"`
	Data      string            `db:"data"`
	Metadata  map[string]string `db:"metadata"`
	Status    string            `db:"status"`
	CreatedAt time.Time         `db:"created_at"`
	UpdatedAt time.Time         `db:"updated_at"`
}

// Status constants
const (
	StatusPending   = "pending"
	StatusProcessed = "processed"
	StatusFailed    = "failed"
)
