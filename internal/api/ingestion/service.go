package ingestion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/Conversly/db-ingestor/internal/processors"
	"github.com/Conversly/db-ingestor/internal/types"
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

// Process enqueues the request for background processing and returns immediately
func (s *Service) Process(ctx context.Context, req types.ProcessRequest) (*types.ProcessResponse, error) {
	jobID := uuid.New().String()

	utils.Zlog.Info("Enqueueing ingestion job",
		zap.String("jobId", jobID),
		zap.String("userId", req.UserID),
		zap.String("chatbotId", req.ChatbotID),
		zap.Int("websites", len(req.WebsiteURLs)),
		zap.Int("qanda", len(req.QandAData)),
		zap.Int("documents", len(req.Documents)),
		zap.Int("textContent", len(req.TextContent)))

	job := IngestionJob{
		JobID:   jobID,
		Request: req,
	}

	if ok := s.workers.EnqueueIngestion(job); !ok {
		return nil, fmt.Errorf("ingestion queue is full, try again later")
	}

	response := &types.ProcessResponse{
		JobID:        jobID,
		Status:       types.StatusProcessing,
		Message:      "Job queued for processing",
		TotalSources: s.calculateTotalSources(req),
		Timestamp:    time.Now().UTC(),
	}

	return response, nil
}

// ProcessIngestionJob processes an ingestion job in the background (called by workers)
func (s *Service) ProcessIngestionJob(ctx context.Context, job IngestionJob) {
	req := job.Request
	jobID := job.JobID

	utils.Zlog.Info("Processing ingestion job",
		zap.String("jobId", jobID),
		zap.String("userId", req.UserID),
		zap.String("chatbotId", req.ChatbotID),
		zap.Int("websites", len(req.WebsiteURLs)),
		zap.Int("qanda", len(req.QandAData)),
		zap.Int("documents", len(req.Documents)),
		zap.Int("textContent", len(req.TextContent)))

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

	var status types.ProcessStatus
	if failed == 0 {
		status = types.StatusCompleted
	} else if successful == 0 {
		status = types.StatusFailed
	} else {
		status = types.StatusPartial
	}

	if s.workers != nil && len(allChunks) > 0 {
		// Group chunks by datasourceID for parallel processing
		chunksByDatasource := make(map[string][]types.ContentChunk)
		for _, chunk := range allChunks {
			chunksByDatasource[chunk.DatasourceID] = append(chunksByDatasource[chunk.DatasourceID], chunk)
		}

		// Enqueue separate jobs for each datasource to enable parallel processing
		enqueuedJobs := 0
		droppedJobs := 0
		for datasourceID, chunks := range chunksByDatasource {
			embJob := EmbeddingJob{
				JobID:     fmt.Sprintf("%s-ds-%s", jobID, datasourceID),
				UserID:    req.UserID,
				ChatbotID: req.ChatbotID,
				Chunks:    chunks,
				CreatedAt: time.Now().UTC(),
			}
			if ok := s.workers.Enqueue(embJob); !ok {
				utils.Zlog.Warn("Embedding queue is full; dropping job",
					zap.String("jobId", embJob.JobID),
					zap.String("datasourceId", datasourceID),
					zap.Int("chunks", len(chunks)))
				droppedJobs++
			} else {
				enqueuedJobs++
			}
		}

		utils.Zlog.Info("Embedding jobs enqueued",
			zap.String("jobId", jobID),
			zap.Int("datasources", len(chunksByDatasource)),
			zap.Int("enqueuedJobs", enqueuedJobs),
			zap.Int("droppedJobs", droppedJobs),
			zap.Int("totalChunks", len(allChunks)))
	}

	utils.Zlog.Info("Ingestion job completed",
		zap.String("jobId", jobID),
		zap.String("status", string(status)),
		zap.Int("successful", successful),
		zap.Int("failed", failed),
		zap.Int("totalChunks", totalChunks))
}

func (s *Service) processAllSources(ctx context.Context, req types.ProcessRequest, jobID string) ([]types.SourceResult, int, []types.ContentChunk) {
	var results []types.SourceResult
	var totalChunks int
	var allChunks []types.ContentChunk
	var mu sync.Mutex

	// Create processor factory with configuration
	config := &types.Config{
		ChunkSize:    1000,
		ChunkOverlap: 200,
	}
	if req.Options != nil {
		if req.Options.ChunkSize > 0 {
			config.ChunkSize = req.Options.ChunkSize
		}
		if req.Options.ChunkOverlap > 0 {
			config.ChunkOverlap = req.Options.ChunkOverlap
		}
	}
	factory := processors.NewFactory(config)

	// Initialize file downloader
	downloader := utils.NewFileDownloader()

	var wg sync.WaitGroup

	for _, websiteURL := range req.WebsiteURLs {
		wg.Add(1)
		go func(websiteURL types.WebsiteURL) {
			defer wg.Done()
			result, content := s.processSource(ctx, factory.CreateWebsiteProcessor(websiteURL.URL), req.ChatbotID, req.UserID, websiteURL.URL, websiteURL.DatasourceID)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.convertAndAddCitationToChunks(content, websiteURL.DatasourceID)...)
			}
			mu.Unlock()
		}(websiteURL)
	}

	for _, qa := range req.QandAData {
		wg.Add(1)
		go func(qa types.QAPair) {
			defer wg.Done()
			result, content := s.processSource(ctx, factory.CreateQAProcessor(qa), req.ChatbotID, req.UserID, qa.Question, qa.DatasourceID)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.convertAndAddCitationToChunks(content, qa.DatasourceID)...)
			}
			mu.Unlock()
		}(qa)
	}

	// Process documents - download first, then process
	for _, doc := range req.Documents {
		wg.Add(1)
		go func(doc types.DocumentMetadata) {
			defer wg.Done()

			// Download the file
			utils.Zlog.Info("Downloading document",
				zap.String("url", doc.DownloadURL),
				zap.String("pathname", doc.Pathname),
				zap.String("datasourceId", doc.DatasourceID))

			downloadedFile, err := downloader.DownloadFile(ctx, doc.DownloadURL, doc.ContentType)
			if err != nil {
				utils.Zlog.Error("Failed to download document",
					zap.String("url", doc.DownloadURL),
					zap.Error(err))

				mu.Lock()
				results = append(results, types.SourceResult{
					DatasourceID: doc.DatasourceID,
					SourceType:   types.DetermineSourceTypeFromContentType(doc.ContentType),
					Source:       doc.Pathname,
					Status:       "failed",
					Error:        fmt.Sprintf("Failed to download: %v", err),
					ChunkCount:   0,
					ProcessedAt:  time.Now().UTC(),
				})
				mu.Unlock()
				return
			}

			// Process the downloaded file
			processor := factory.CreateDocumentProcessorFromBytes(
				downloadedFile.Content,
				doc.Pathname,
				doc.ContentType,
			)

			result, content := s.processSource(ctx, processor, req.ChatbotID, req.UserID, doc.Pathname, doc.DatasourceID)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.convertAndAddCitationToChunks(content, doc.DatasourceID)...)
			}
			mu.Unlock()
		}(doc)
	}

	for i, textContent := range req.TextContent {
		wg.Add(1)
		go func(textContent types.TextContent, index int) {
			defer wg.Done()
			topic := fmt.Sprintf("Text content #%d", index+1)
			result, content := s.processSource(ctx, factory.CreateTextProcessor(textContent.Content, topic), req.ChatbotID, req.UserID, topic, textContent.DatasourceID)
			mu.Lock()
			results = append(results, result)
			if content != nil {
				totalChunks += len(content.Chunks)
				allChunks = append(allChunks, s.convertAndAddCitationToChunks(content, textContent.DatasourceID)...)
			}
			mu.Unlock()
		}(textContent, i)
	}

	wg.Wait()

	return results, totalChunks, allChunks
}

func (s *Service) processSource(ctx context.Context, processor types.Processor, chatbotID, userID, source string, datasourceID string) (types.SourceResult, *types.ProcessedContent) {
	startTime := time.Now()

	utils.Zlog.Info("Processing source",
		zap.String("source", source),
		zap.String("type", string(processor.GetSourceType())),
		zap.String("datasourceId", datasourceID))

	content, err := processor.Process(ctx, chatbotID, userID)
	if err != nil {
		utils.Zlog.Error("Failed to process source",
			zap.String("source", source),
			zap.Error(err))
		return types.SourceResult{
			DatasourceID: datasourceID,
			SourceType:   processor.GetSourceType(),
			Source:       source,
			Status:       "failed",
			Error:        err.Error(),
			ChunkCount:   0,
			ProcessedAt:  time.Now().UTC(),
		}, nil
	}

	duration := time.Since(startTime)
	utils.Zlog.Info("Source processed successfully",
		zap.String("source", source),
		zap.Int("chunks", len(content.Chunks)),
		zap.Duration("duration", duration))

	return types.SourceResult{
		DatasourceID: datasourceID,
		SourceType:   processor.GetSourceType(),
		Source:       source,
		Status:       "success",
		Message:      fmt.Sprintf("Processed successfully in %v", duration),
		ChunkCount:   len(content.Chunks),
		ProcessedAt:  time.Now().UTC(),
	}, content
}

func (s *Service) storeProcessedContent(ctx context.Context, chatbotID, userID string, content *types.ProcessedContent) error {

	utils.Zlog.Info("Storing processed content",
		zap.String("chatbotId", chatbotID),
		zap.String("sourceType", string(content.SourceType)),
		zap.Int("chunks", len(content.Chunks)))

	return nil
}

func (s *Service) calculateTotalSources(req types.ProcessRequest) int {
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

// convertAndAddCitationToChunks converts processor chunks to ingestion chunks and adds citation
func (s *Service) convertAndAddCitationToChunks(content *types.ProcessedContent, datasourceID string) []types.ContentChunk {
	citation := determineCitation(content)
	chunks := make([]types.ContentChunk, len(content.Chunks))

	for i, chunk := range content.Chunks {
		chunks[i] = types.ContentChunk{
			DatasourceID: datasourceID,
			Content:      chunk.Content,
			Embedding:    chunk.Embedding,
			Metadata:     chunk.Metadata,
			ChunkIndex:   chunk.ChunkIndex,
		}

		if chunks[i].Metadata == nil {
			chunks[i].Metadata = map[string]interface{}{}
		}
		chunks[i].Metadata["citation"] = citation
		chunks[i].Metadata["sourceType"] = string(content.SourceType)
		chunks[i].Metadata["topic"] = content.Topic
		chunks[i].Metadata["datasourceId"] = datasourceID
	}
	return chunks
}

func determineCitation(content *types.ProcessedContent) string {
	switch content.SourceType {
	case types.SourceTypeWebsite:
		if urlVal, ok := content.Metadata["url"].(string); ok && urlVal != "" {
			return urlVal
		}
		return content.Topic
	case types.SourceTypeQA:
		return "QnA"
	case types.SourceTypePDF, types.SourceTypeCSV, types.SourceTypeText, types.SourceTypeJSON:
		if filename, ok := content.Metadata["filename"].(string); ok && filename != "" {
			return filename
		}
		return content.Topic
	default:
		return content.Topic
	}
}
