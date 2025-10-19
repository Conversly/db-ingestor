package ingestion

import (
    "context"
    "fmt"
    "sync"
    "time"
    "github.com/Conversly/db-ingestor/internal/types"
    "github.com/Conversly/db-ingestor/internal/loaders"
    "github.com/Conversly/db-ingestor/internal/processors"
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

func (s *Service) Process(ctx context.Context, req types.ProcessRequest) (*types.ProcessResponse, error) {
    utils.Zlog.Info("Processing data sources",
        zap.String("userId", req.UserID),
        zap.String("chatbotId", req.ChatbotID),
        zap.Int("websites", len(req.WebsiteURLs)),
        zap.Int("qanda", len(req.QandAData)),
        zap.Int("documents", len(req.Documents)),
        zap.Int("textContent", len(req.TextContent)))

    // Validation method should be implemented in types package or here
    // if err := req.Validate(); err != nil {
    // 	return nil, fmt.Errorf("validation failed: %w", err)
    // }

    jobID := uuid.New().String()

    record := &types.IngestionRecord{
        ID:               jobID,
        UserID:           req.UserID,
        ChatbotID:        req.ChatbotID,
        Status:           types.StatusProcessing,
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
        record.Status = types.StatusCompleted
    } else if successful == 0 {
        record.Status = types.StatusFailed
    } else {
        record.Status = types.StatusPartial
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

    response := &types.ProcessResponse{
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
    if req.Options != nil && req.Options.DocumentConfig != nil {
        if req.Options.DocumentConfig.ChunkSize > 0 {
            config.ChunkSize = req.Options.DocumentConfig.ChunkSize
        }
        if req.Options.DocumentConfig.ChunkOverlap > 0 {
            config.ChunkOverlap = req.Options.DocumentConfig.ChunkOverlap
        }
    }
    factory := processors.NewFactory(config)

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
                allChunks = append(allChunks, s.convertAndAddCitationToChunks(content)...)
            }
            mu.Unlock()
        }(url)
    }

    for _, qa := range req.QandAData {
        wg.Add(1)
        go func(qa types.QAPair) {
            defer wg.Done()
            result, content := s.processSource(ctx, factory.CreateQAProcessor(qa), req.ChatbotID, req.UserID, qa.Question)
            mu.Lock()
            results = append(results, result)
            if content != nil {
                totalChunks += len(content.Chunks)
                allChunks = append(allChunks, s.convertAndAddCitationToChunks(content)...)
            }
            mu.Unlock()
        }(qa)
    }

    for _, doc := range req.Documents {
        result, content := s.processSource(ctx, factory.CreateDocumentProcessor(doc), req.ChatbotID, req.UserID, doc.Filename)
        results = append(results, result)
        if content != nil {
            totalChunks += len(content.Chunks)
            allChunks = append(allChunks, s.convertAndAddCitationToChunks(content)...)
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
                allChunks = append(allChunks, s.convertAndAddCitationToChunks(content)...)
            }
            mu.Unlock()
        }(text, i)
    }

    wg.Wait()

    return results, totalChunks, allChunks
}

func (s *Service) processSource(ctx context.Context, processor types.Processor, chatbotID, userID, source string) (types.SourceResult, *types.ProcessedContent) {
    startTime := time.Now()

    utils.Zlog.Info("Processing source",
        zap.String("source", source),
        zap.String("type", string(processor.GetSourceType())))

    content, err := processor.Process(ctx, chatbotID, userID)
    if err != nil {
        utils.Zlog.Error("Failed to process source",
            zap.String("source", source),
            zap.Error(err))
        return types.SourceResult{
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

    return types.SourceResult{
        SourceType:  processor.GetSourceType(),
        Source:      source,
        Status:      "success",
        Message:     fmt.Sprintf("Processed successfully in %v", duration),
        ChunkCount:  len(content.Chunks),
        ProcessedAt: time.Now().UTC(),
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
func (s *Service) convertAndAddCitationToChunks(content *types.ProcessedContent) []types.ContentChunk {
    citation := determineCitation(content)
    chunks := make([]types.ContentChunk, len(content.Chunks))
    
    for i, chunk := range content.Chunks {
        chunks[i] = types.ContentChunk{
            Content:    chunk.Content,
            Embedding:  chunk.Embedding,
            Metadata:   chunk.Metadata,
            ChunkIndex: chunk.ChunkIndex,
        }
        
        if chunks[i].Metadata == nil {
            chunks[i].Metadata = map[string]interface{}{}
        }
        chunks[i].Metadata["citation"] = citation
        chunks[i].Metadata["sourceType"] = string(content.SourceType)
        chunks[i].Metadata["topic"] = content.Topic
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