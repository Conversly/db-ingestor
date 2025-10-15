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

type Service struct {
	db      *loaders.PostgresClient
	workers *WorkerPool
}

func NewService(db *loaders.PostgresClient, workers *WorkerPool) *Service {
	return &Service{db: db, workers: workers}
}

func (s *Service) Process(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	utils.Zlog.Info("Processing data sources",
		zap.String("userId", req.UserID),
		zap.String("chatbotId", req.ChatbotID),
		zap.Int("websites", len(req.WebsiteURLs)),
		zap.Int("qanda", len(req.QandAData)),
		zap.Int("documents", len(req.Documents)),
		zap.Int("textContent", len(req.TextContent)))

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	jobID := uuid.New().String()

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

	results, totalChunks, allChunks := s.processAllSources(ctx, req, jobID)

	successful := 0
	failed := 0
	for _, result := range results {
		if result.Status == "success" {
			successful++
		} else {
			failed++
		}
	}

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

	if s.workers != nil && len(allChunks) > 0 {
		job := EmbeddingJob{
			JobID:     jobID,
			UserID:    req.UserID,
			ChatbotID: req.ChatbotID,
			Chunks:    allChunks,
			CreatedAt: time.Now().UTC(),
		}
		if ok := s.workers.Enqueue(job); !ok {
			utils.Zlog.Warn("Embedding queue is full; dropping job",
				zap.String("jobId", jobID),
				zap.Int("chunks", len(allChunks)))
		} else {
			utils.Zlog.Info("Embedding job enqueued",
				zap.String("jobId", jobID),
				zap.Int("chunks", len(allChunks)))
		}
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

func (s *Service) processAllSources(ctx context.Context, req ProcessRequest, jobID string) ([]SourceResult, int, []ContentChunk) {
	var results []SourceResult
	var totalChunks int
	var allChunks []ContentChunk
	var mu sync.Mutex

	factory := NewProcessorFactory(req.Options)

	var wg sync.WaitGroup

	for _, url := range req.WebsiteURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			result, content := s.processSource(ctx, factory.CreateWebsiteProcessor(url), req.ChatbotID, req.UserID, url)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.addCitationToChunks(content)...)
			}
			mu.Unlock()
		}(url)
	}

	for _, qa := range req.QandAData {
		wg.Add(1)
		go func(qa QAPair) {
			defer wg.Done()
			result, content := s.processSource(ctx, factory.CreateQAProcessor(qa), req.ChatbotID, req.UserID, qa.Question)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.addCitationToChunks(content)...)
			}
			mu.Unlock()
		}(qa)
	}

	for _, doc := range req.Documents {
		result, content := s.processSource(ctx, factory.CreateDocumentProcessor(doc), req.ChatbotID, req.UserID, doc.Filename)
		results = append(results, result)
		if content != nil {
			totalChunks += len(content.Chunks)
			allChunks = append(allChunks, s.addCitationToChunks(content)...)
		}
	}

	for i, text := range req.TextContent {
		wg.Add(1)
		go func(text string, index int) {
			defer wg.Done()
			topic := fmt.Sprintf("Text content #%d", index+1)
			result, content := s.processSource(ctx, factory.CreateTextProcessor(text, topic), req.ChatbotID, req.UserID, topic)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.addCitationToChunks(content)...)
			}
			mu.Unlock()
		}(text, i)
	}

	wg.Wait()

	return results, totalChunks, allChunks
}

func (s *Service) processSource(ctx context.Context, processor Processor, chatbotID, userID, source string) (SourceResult, *ProcessedContent) {
	startTime := time.Now()

	utils.Zlog.Info("Processing source",
		zap.String("source", source),
		zap.String("type", string(processor.GetSourceType())))

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
		}, nil
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
	}, content
}

func (s *Service) storeProcessedContent(ctx context.Context, chatbotID, userID string, content *ProcessedContent) error {

	utils.Zlog.Info("Storing processed content",
		zap.String("chatbotId", chatbotID),
		zap.String("sourceType", string(content.SourceType)),
		zap.Int("chunks", len(content.Chunks)))

	return nil
}

func (s *Service) calculateTotalSources(req ProcessRequest) int {
	return len(req.WebsiteURLs) + len(req.QandAData) + len(req.Documents) + len(req.TextContent)
}

func (s *Service) generateResponseMessage(successful, failed int) string {
	if failed == 0 {
		return fmt.Sprintf("Successfully processed all %d sources", successful)
	} else if successful == 0 {
		return fmt.Sprintf("Failed to process all %d sources", failed)
	} else {
		return fmt.Sprintf("Processed %d sources successfully, %d failed", successful, failed)
	}
}

func (s *Service) addCitationToChunks(content *ProcessedContent) []ContentChunk {
	citation := determineCitation(content)
	for i := range content.Chunks {
		if content.Chunks[i].Metadata == nil {
			content.Chunks[i].Metadata = map[string]interface{}{}
		}
		content.Chunks[i].Metadata["citation"] = citation
		content.Chunks[i].Metadata["sourceType"] = string(content.SourceType)
		content.Chunks[i].Metadata["topic"] = content.Topic
	}
	return content.Chunks
}

func determineCitation(content *ProcessedContent) string {
	switch content.SourceType {
	case SourceTypeWebsite:
		if urlVal, ok := content.Metadata["url"].(string); ok && urlVal != "" {
			return urlVal
		}
		return content.Topic
	case SourceTypeQA:
		return "QnA"
	case SourceTypePDF, SourceTypeCSV, SourceTypeText, SourceTypeJSON:
		if filename, ok := content.Metadata["filename"].(string); ok && filename != "" {
			return filename
		}
		return content.Topic
	default:
		return content.Topic
	}
}
