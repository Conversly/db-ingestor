package types

import (
	"mime/multipart"
	"time"
)

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

type QAPair struct {
	Question string                 `json:"question" binding:"required"`
	Answer   string                 `json:"answer" binding:"required"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type SourceResult struct {
	SourceType  SourceType `json:"sourceType"`
	Source      string     `json:"source"`
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	Error       string     `json:"error,omitempty"`
	ChunkCount  int        `json:"chunkCount"`
	ProcessedAt time.Time  `json:"processedAt"`
}

type WebsiteConfig struct {
	MaxDepth        int      `json:"maxDepth,omitempty"`
	MaxPages        int      `json:"maxPages,omitempty"`
	IncludePatterns []string `json:"includePatterns,omitempty"`
	ExcludePatterns []string `json:"excludePatterns,omitempty"`
	Timeout         int      `json:"timeout,omitempty"`
}

type DocumentConfig struct {
	ChunkSize     int  `json:"chunkSize,omitempty"`
	ChunkOverlap  int  `json:"chunkOverlap,omitempty"`
	ExtractImages bool `json:"extractImages,omitempty"`
	ExtractTables bool `json:"extractTables,omitempty"`
}

type ProcessingOptions struct {
	WebsiteConfig    *WebsiteConfig  `json:"websiteConfig,omitempty"`
	DocumentConfig   *DocumentConfig `json:"documentConfig,omitempty"`
	AsyncMode        bool            `json:"asyncMode,omitempty"`
	NotifyOnComplete string          `json:"notifyOnComplete,omitempty"`
}

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

type ProcessRequest struct {
	UserID    string `json:"userId" form:"userId" binding:"required"`
	ChatbotID string `json:"chatbotId" form:"chatbotId" binding:"required"`

	WebsiteURLs []string                `json:"websiteUrls" form:"websiteUrls"`
	QandAData   []QAPair                `json:"qandaData"`
	Documents   []*multipart.FileHeader `form:"documents"`
	TextContent []string                `json:"textContent" form:"textContent"`

	Options *ProcessingOptions `json:"options,omitempty"`
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

type ChatRequest struct {
	Question  string                 `json:"question" binding:"required"`
	ChatbotID string                 `json:"chatbotId" binding:"required"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type ChatResponse struct {
	Answer    string                   `json:"answer"`
	Context   []map[string]interface{} `json:"context,omitempty"`
	Sources   []string                 `json:"sources,omitempty"`
	Timestamp time.Time                `json:"timestamp"`
}
