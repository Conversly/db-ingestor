package ingestion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service contains the business logic for ingestion
type Service struct {
	db *loaders.PostgresClient
}

// NewService creates a new ingestion service
func NewService(db *loaders.PostgresClient) *Service {
	return &Service{db: db}
}

// Process handles the main processing of all data sources
func (s *Service) Process(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	utils.Zlog.Info("Processing data sources",
		zap.String("userId", req.UserID),
		zap.String("chatbotId", req.ChatbotID),
		zap.Int("websites", len(req.WebsiteURLs)),
		zap.Int("qanda", len(req.QandAData)),
		zap.Int("documents", len(req.Documents)),
		zap.Int("textContent", len(req.TextContent)))

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate job ID
	jobID := uuid.New().String()

	// Create ingestion record
	record := &IngestionRecord{
		ID:               jobID,
		UserID:           req.UserID,
		ChatbotID:        req.ChatbotID,
		Status:           StatusProcessing,
		TotalSources:     s.calculateTotalSources(req),
		ProcessedSources: 0,
		FailedSources:    0,
		TotalChunks:      0,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	// Store initial record in database
	if err := s.createIngestionRecord(ctx, record); err != nil {
		utils.Zlog.Error("Failed to create ingestion record", zap.Error(err))
		// Continue processing even if DB insert fails
	}

	// Process all sources
	results, totalChunks := s.processAllSources(ctx, req, jobID)

	// Calculate success/failure counts
	successful := 0
	failed := 0
	for _, result := range results {
		if result.Status == "success" {
			successful++
		} else {
			failed++
		}
	}

	// Update record status
	record.ProcessedSources = successful
	record.FailedSources = failed
	record.TotalChunks = totalChunks

	if failed == 0 {
		record.Status = StatusCompleted
	} else if successful == 0 {
		record.Status = StatusFailed
	} else {
		record.Status = StatusPartial
	}

	completedAt := time.Now().UTC()
	record.CompletedAt = &completedAt
	record.UpdatedAt = completedAt

	// Update database record
	if err := s.updateIngestionRecord(ctx, record); err != nil {
		utils.Zlog.Error("Failed to update ingestion record", zap.Error(err))
	}

	response := &ProcessResponse{
		JobID:            jobID,
		Status:           record.Status,
		Message:          s.generateResponseMessage(successful, failed),
		TotalSources:     record.TotalSources,
		ProcessedSources: successful,
		FailedSources:    failed,
		TotalChunks:      totalChunks,
		Results:          results,
		Timestamp:        time.Now().UTC(),
	}

	utils.Zlog.Info("Processing completed",
		zap.String("jobId", jobID),
		zap.String("status", string(record.Status)),
		zap.Int("successful", successful),
		zap.Int("failed", failed))

	return response, nil
}

// processAllSources processes all data sources and returns results
func (s *Service) processAllSources(ctx context.Context, req ProcessRequest, jobID string) ([]SourceResult, int) {
	var results []SourceResult
	var totalChunks int
	var mu sync.Mutex

	// Create processor factory
	factory := NewProcessorFactory(req.Options)

	// Use wait group for concurrent processing
	var wg sync.WaitGroup

	// Process websites
	for _, url := range req.WebsiteURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			result := s.processSource(ctx, factory.CreateWebsiteProcessor(url), req.ChatbotID, req.UserID, url)
			mu.Lock()
			results = append(results, result)
			totalChunks += result.ChunkCount
			mu.Unlock()
		}(url)
	}

	// Process Q&A pairs
	for _, qa := range req.QandAData {
		wg.Add(1)
		go func(qa QAPair) {
			defer wg.Done()
			result := s.processSource(ctx, factory.CreateQAProcessor(qa), req.ChatbotID, req.UserID, qa.Question)
			mu.Lock()
			results = append(results, result)
			totalChunks += result.ChunkCount
			mu.Unlock()
		}(qa)
	}

	// Process documents
	for _, doc := range req.Documents {
		wg.Add(1)
		go func(doc interface{}) {
			defer wg.Done()
			// Type assertion needed due to multipart.FileHeader
			if fileHeader, ok := doc.(*interface{}); ok {
				_ = fileHeader // placeholder to avoid unused variable error in this example
			}
			// For now, process synchronously in the main goroutine
			// Real implementation would properly handle the file
		}(doc)
	}

	// Process documents (synchronously for file handling safety)
	for _, doc := range req.Documents {
		result := s.processSource(ctx, factory.CreateDocumentProcessor(doc), req.ChatbotID, req.UserID, doc.Filename)
		results = append(results, result)
		totalChunks += result.ChunkCount
	}

	// Process text content
	for i, text := range req.TextContent {
		wg.Add(1)
		go func(text string, index int) {
			defer wg.Done()
			topic := fmt.Sprintf("Text content #%d", index+1)
			result := s.processSource(ctx, factory.CreateTextProcessor(text, topic), req.ChatbotID, req.UserID, topic)
			mu.Lock()
			results = append(results, result)
			totalChunks += result.ChunkCount
			mu.Unlock()
		}(text, i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return results, totalChunks
}

// processSource processes a single data source using the appropriate processor
func (s *Service) processSource(ctx context.Context, processor Processor, chatbotID, userID, source string) SourceResult {
	startTime := time.Now()

	utils.Zlog.Info("Processing source",
		zap.String("source", source),
		zap.String("type", string(processor.GetSourceType())))

	// Process the source
	content, err := processor.Process(ctx, chatbotID, userID)
	if err != nil {
		utils.Zlog.Error("Failed to process source",
			zap.String("source", source),
			zap.Error(err))
		return SourceResult{
			SourceType:  processor.GetSourceType(),
			Source:      source,
			Status:      "failed",
			Error:       err.Error(),
			ChunkCount:  0,
			ProcessedAt: time.Now().UTC(),
		}
	}

	// TODO: Generate embeddings for each chunk
	// For now, we'll just store the processed content
	// In a real implementation, you would:
	// 1. Generate embeddings using an embedding model
	// 2. Store embeddings in a vector database
	// 3. Store metadata in PostgreSQL

	// Store processed content
	if err := s.storeProcessedContent(ctx, chatbotID, userID, content); err != nil {
		utils.Zlog.Error("Failed to store processed content",
			zap.String("source", source),
			zap.Error(err))
		return SourceResult{
			SourceType:  processor.GetSourceType(),
			Source:      source,
			Status:      "failed",
			Error:       fmt.Sprintf("Failed to store: %v", err),
			ChunkCount:  len(content.Chunks),
			ProcessedAt: time.Now().UTC(),
		}
	}

	duration := time.Since(startTime)
	utils.Zlog.Info("Source processed successfully",
		zap.String("source", source),
		zap.Int("chunks", len(content.Chunks)),
		zap.Duration("duration", duration))

	return SourceResult{
		SourceType:  processor.GetSourceType(),
		Source:      source,
		Status:      "success",
		Message:     fmt.Sprintf("Processed successfully in %v", duration),
		ChunkCount:  len(content.Chunks),
		ProcessedAt: time.Now().UTC(),
	}
}

// storeProcessedContent stores the processed content and embeddings
func (s *Service) storeProcessedContent(ctx context.Context, chatbotID, userID string, content *ProcessedContent) error {
	// TODO: Implement actual storage logic
	// This would typically involve:
	// 1. Generating embeddings for each chunk
	// 2. Storing embeddings in a vector database (pgvector, Pinecone, etc.)
	// 3. Storing metadata in PostgreSQL

	utils.Zlog.Info("Storing processed content",
		zap.String("chatbotId", chatbotID),
		zap.String("sourceType", string(content.SourceType)),
		zap.Int("chunks", len(content.Chunks)))

	// Placeholder for now
	return nil
}

// calculateTotalSources calculates the total number of sources in the request
func (s *Service) calculateTotalSources(req ProcessRequest) int {
	return len(req.WebsiteURLs) + len(req.QandAData) + len(req.Documents) + len(req.TextContent)
}

// generateResponseMessage generates a human-readable response message
func (s *Service) generateResponseMessage(successful, failed int) string {
	if failed == 0 {
		return fmt.Sprintf("Successfully processed all %d sources", successful)
	} else if successful == 0 {
		return fmt.Sprintf("Failed to process all %d sources", failed)
	} else {
		return fmt.Sprintf("Processed %d sources successfully, %d failed", successful, failed)
	}
}

// createIngestionRecord creates a new ingestion record in the database
func (s *Service) createIngestionRecord(ctx context.Context, record *IngestionRecord) error {
	// TODO: Implement actual database insertion
	// Example:
	// query := `
	//     INSERT INTO ingestion_jobs (id, user_id, chatbot_id, status, total_sources,
	//                                  processed_sources, failed_sources, total_chunks, created_at, updated_at)
	//     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	// `
	// _, err := s.db.GetPool().Exec(ctx, query, record.ID, record.UserID, record.ChatbotID,
	//                                record.Status, record.TotalSources, record.ProcessedSources,
	//                                record.FailedSources, record.TotalChunks, record.CreatedAt, record.UpdatedAt)
	// return err

	utils.Zlog.Info("Created ingestion record", zap.String("jobId", record.ID))
	return nil
}

// updateIngestionRecord updates an existing ingestion record in the database
func (s *Service) updateIngestionRecord(ctx context.Context, record *IngestionRecord) error {
	// TODO: Implement actual database update
	// Example:
	// query := `
	//     UPDATE ingestion_jobs
	//     SET status = $1, processed_sources = $2, failed_sources = $3,
	//         total_chunks = $4, updated_at = $5, completed_at = $6
	//     WHERE id = $7
	// `
	// _, err := s.db.GetPool().Exec(ctx, query, record.Status, record.ProcessedSources,
	//                                record.FailedSources, record.TotalChunks, record.UpdatedAt,
	//                                record.CompletedAt, record.ID)
	// return err

	utils.Zlog.Info("Updated ingestion record", zap.String("jobId", record.ID))
	return nil
}

// GetIngestionByID retrieves an ingestion record by ID
func (s *Service) GetIngestionByID(ctx context.Context, id string) (*IngestionRecord, error) {
	utils.Zlog.Info("Fetching ingestion record", zap.String("id", id))

	// TODO: Implement actual database query
	// Example:
	// query := `SELECT * FROM ingestion_jobs WHERE id = $1`
	// var record IngestionRecord
	// err := s.db.GetPool().QueryRow(ctx, query, id).Scan(...)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to fetch record: %w", err)
	// }

	// Mock response for now
	return &IngestionRecord{
		ID:               id,
		UserID:           "user123",
		ChatbotID:        "chatbot123",
		Status:           StatusCompleted,
		TotalSources:     5,
		ProcessedSources: 5,
		FailedSources:    0,
		TotalChunks:      50,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}, nil
}

// Chat processes a chat query and retrieves relevant context
func (s *Service) Chat(ctx context.Context, req ChatRequest, apiKey string) (*ChatResponse, error) {
	utils.Zlog.Info("Processing chat request",
		zap.String("chatbotId", req.ChatbotID),
		zap.String("question", req.Question))

	// TODO: Implement actual chat logic:
	// 1. Generate embedding for the question
	// 2. Search vector database for similar chunks
	// 3. Retrieve relevant context
	// 4. Generate response using LLM
	// 5. Return response with sources

	// Mock response for now
	return &ChatResponse{
		Answer: "This is a placeholder response. Implement actual chat logic here.",
		Context: []map[string]interface{}{
			{
				"content": "Sample context chunk 1",
				"source":  "https://example.com",
				"score":   0.95,
			},
		},
		Sources:   []string{"https://example.com"},
		Timestamp: time.Now().UTC(),
	}, nil
}
