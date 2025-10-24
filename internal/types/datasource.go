package types

import (
	"time"
)

// ====== ENUMS ======

type SourceType string

const (
	SourceTypeWebsite SourceType = "website"
	SourceTypePDF     SourceType = "pdf"
	SourceTypeText    SourceType = "text"
	SourceTypeCSV     SourceType = "csv"
	SourceTypeQA      SourceType = "qa"
	SourceTypeJSON    SourceType = "json"
)

type ProcessStatus string

const (
	StatusPending    ProcessStatus = "pending"
	StatusProcessing ProcessStatus = "processing"
	StatusCompleted  ProcessStatus = "completed"
	StatusFailed     ProcessStatus = "failed"
	StatusPartial    ProcessStatus = "partial"
)

// ====== DATA STRUCTURES ======

type WebsiteURL struct {
	DatasourceID int    `json:"datasourceId" validate:"required,min=1"`
	URL          string `json:"url" validate:"required,url"`
}

type QAPair struct {
	DatasourceID int    `json:"datasourceId" validate:"required,min=1"`
	Question     string `json:"question" validate:"required"`
	Answer       string `json:"answer" validate:"required"`
	Citations    string `json:"citations,omitempty"`
}

type DocumentMetadata struct {
	DatasourceID       int    `json:"datasourceId" validate:"required,min=1"`
	URL                string `json:"url" validate:"required,url"`
	DownloadURL        string `json:"downloadUrl" validate:"required,url"`
	Pathname           string `json:"pathname" validate:"required"`
	ContentType        string `json:"contentType" validate:"required,oneof=application/pdf text/plain text/csv application/csv application/json application/msword application/vnd.openxmlformats-officedocument.wordprocessingml.document"`
	ContentDisposition string `json:"contentDisposition" validate:"required"`
}

type TextContent struct {
	DatasourceID int    `json:"datasourceId" validate:"required,min=1"`
	Content      string `json:"content" validate:"required"`
}

type ProcessingOptions struct {
	ChunkSize    int `json:"chunkSize,omitempty" validate:"omitempty,min=0"`
	ChunkOverlap int `json:"chunkOverlap,omitempty" validate:"omitempty,min=0"`
}

// request structure for processing ingestion

type ProcessRequest struct {
	UserID      string             `json:"userId" validate:"required"`
	ChatbotID   string             `json:"chatbotId" validate:"required"`
	WebsiteURLs []WebsiteURL       `json:"websiteUrls,omitempty" validate:"omitempty,dive"`
	QandAData   []QAPair           `json:"qandaData,omitempty" validate:"omitempty,dive"`
	Documents   []DocumentMetadata `json:"documents,omitempty" validate:"omitempty,dive"`
	TextContent []TextContent      `json:"textContent,omitempty" validate:"omitempty,dive"`
	Options     *ProcessingOptions `json:"options,omitempty"`
}


type SourceResult struct {
	DatasourceID int        `json:"datasourceId,omitempty"`
	SourceType   SourceType `json:"sourceType"`
	Source       string     `json:"source"`
	Status       string     `json:"status"`
	Message      string     `json:"message,omitempty"`
	Error        string     `json:"error,omitempty"`
	ChunkCount   int        `json:"chunkCount"`
	ProcessedAt  time.Time  `json:"processedAt"`
}

type ProcessResponse struct {
	JobID            string         `json:"jobId"`
	Status           ProcessStatus  `json:"status"`
	Message          string         `json:"message"`
	TotalSources     int            `json:"totalSources"`
	ProcessedSources int            `json:"processedSources"`
	FailedSources    int            `json:"failedSources"`
	TotalChunks      int            `json:"totalChunks"`
	Results          []SourceResult `json:"results"`
	Timestamp        time.Time      `json:"timestamp"`
}

type ErrorResponse struct {
	Error     string                 `json:"error"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ====== DATABASE MODELS ======

type IngestionRecord struct {
	ID               string                 `db:"id" json:"id"`
	UserID           string                 `db:"user_id" json:"userId"`
	ChatbotID        string                 `db:"chatbot_id" json:"chatbotId"`
	Status           ProcessStatus          `db:"status" json:"status"`
	TotalSources     int                    `db:"total_sources" json:"totalSources"`
	ProcessedSources int                    `db:"processed_sources" json:"processedSources"`
	FailedSources    int                    `db:"failed_sources" json:"failedSources"`
	TotalChunks      int                    `db:"total_chunks" json:"totalChunks"`
	Metadata         map[string]interface{} `db:"metadata" json:"metadata,omitempty"`
	ErrorMessage     string                 `db:"error_message" json:"errorMessage,omitempty"`
	CreatedAt        time.Time              `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time              `db:"updated_at" json:"updatedAt"`
	CompletedAt      *time.Time             `db:"completed_at" json:"completedAt,omitempty"`
}

type FileInfo struct {
	Filename    string     `json:"filename"`
	Size        int64      `json:"size"`
	ContentType string     `json:"contentType"`
	SourceType  SourceType `json:"sourceType"`
}

// ====== HELPERS ======

func DetermineSourceTypeFromContentType(contentType string) SourceType {
	switch {
	case contentType == "application/pdf":
		return SourceTypePDF
	case contentType == "text/plain":
		return SourceTypeText
	case contentType == "text/csv" || contentType == "application/csv":
		return SourceTypeCSV
	case contentType == "application/json":
		return SourceTypeJSON
	default:
		return SourceTypeText
	}
}
