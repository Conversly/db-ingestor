package ingestion

import "time"

// Request DTOs
type IngestRequest struct {
	Data      string            `json:"data" binding:"required"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp time.Time         `json:"timestamp"`
}

type BulkIngestRequest struct {
	Items []IngestRequest `json:"items" binding:"required,min=1"`
}

// Response DTOs
type IngestResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type BulkIngestResponse struct {
	Successful int               `json:"successful"`
	Failed     int               `json:"failed"`
	Results    []IngestResponse  `json:"results"`
	Timestamp  time.Time         `json:"timestamp"`
}

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
