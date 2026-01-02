package ingestion

import (
	"context"
	"sync"
	"time"

	"github.com/Conversly/db-ingestor/internal/embedder"
	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

type EmbeddingJob struct {
	JobID     string
	UserID    string
	ChatbotID string
	Chunks    []types.ContentChunk
	CreatedAt time.Time
}

type IngestionJob struct {
	JobID   string
	Request types.ProcessRequest
}

type WorkerPool struct {
	embeddingJobs chan EmbeddingJob
	ingestionJobs chan IngestionJob
	quit          chan struct{}
	started       bool
	wg            sync.WaitGroup
	numWorkers    int
	embedder      *embedder.GeminiEmbedder
	db            *loaders.PostgresClient
	processFunc   func(ctx context.Context, job IngestionJob)
}

func NewWorkerPool(numWorkers int, queueCapacity int, geminiEmbedder *embedder.GeminiEmbedder, db *loaders.PostgresClient) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if queueCapacity <= 0 {
		queueCapacity = 100
	}
	return &WorkerPool{
		embeddingJobs: make(chan EmbeddingJob, queueCapacity),
		ingestionJobs: make(chan IngestionJob, queueCapacity),
		quit:          make(chan struct{}),
		numWorkers:    numWorkers,
		embedder:      geminiEmbedder,
		db:            db,
	}
}

func (wp *WorkerPool) SetProcessFunc(fn func(ctx context.Context, job IngestionJob)) {
	wp.processFunc = fn
}

func (wp *WorkerPool) Start() {
	if wp.started {
		return
	}
	wp.started = true
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			utils.Zlog.Info("Worker started", zap.Int("workerId", workerID))
			for {
				select {
				case <-wp.quit:
					utils.Zlog.Info("Worker stopping", zap.Int("workerId", workerID))
					return
				case job := <-wp.ingestionJobs:
					if wp.processFunc != nil {
						ctx := context.Background()
						wp.processFunc(ctx, job)
					}
				case job := <-wp.embeddingJobs:
					wp.processEmbeddingJob(workerID, job)
				}
			}
		}(i + 1)
	}
}

func (wp *WorkerPool) Stop(ctx context.Context) {
	if !wp.started {
		return
	}
	close(wp.quit)
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		utils.Zlog.Warn("Timeout waiting for workers to stop")
	case <-done:
		utils.Zlog.Info("All workers stopped")
	}
}

func (wp *WorkerPool) Enqueue(job EmbeddingJob) bool {
	select {
	case <-wp.quit:
		return false
	default:
	}
	select {
	case wp.embeddingJobs <- job:
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) EnqueueIngestion(job IngestionJob) bool {
	select {
	case <-wp.quit:
		return false
	default:
	}
	select {
	case wp.ingestionJobs <- job:
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) processEmbeddingJob(workerID int, job EmbeddingJob) {
	start := time.Now()
	utils.Zlog.Info("Processing embedding job",
		zap.Int("workerId", workerID),
		zap.String("jobId", job.JobID),
		zap.String("chatbotId", job.ChatbotID),
		zap.Int("chunks", len(job.Chunks)))

	if wp.embedder == nil {
		utils.Zlog.Warn("Embedder not configured, skipping embedding generation",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	successCount := 0
	failCount := 0

	// Generate embeddings for all chunks
	for i := range job.Chunks {
		embedding, err := wp.embedder.EmbedText(ctx, job.Chunks[i].Content)
		utils.Zlog.Info("Embedding generated",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID),
			zap.Int("chunkIndex", job.Chunks[i].ChunkIndex),
			zap.Int("embeddingLength", len(embedding)))
		if err != nil {
			utils.Zlog.Error("Failed to generate embedding",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Int("chunkIndex", job.Chunks[i].ChunkIndex),
				zap.Error(err))
			failCount++
			continue
		}

		job.Chunks[i].Embedding = embedding
		successCount++

		if (i+1)%10 == 0 {
			utils.Zlog.Info("Embedding progress",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Int("processed", i+1),
				zap.Int("total", len(job.Chunks)))
		}
	}

	duration := time.Since(start)
	utils.Zlog.Info("Completed embedding generation",
		zap.Int("workerId", workerID),
		zap.String("jobId", job.JobID),
		zap.String("chatbotId", job.ChatbotID),
		zap.Int("successful", successCount),
		zap.Int("failed", failCount),
		zap.Duration("duration", duration))

	// Persist embeddings to database
	if wp.db != nil && successCount > 0 {
		if err := wp.persistEmbeddings(ctx, job); err != nil {
			utils.Zlog.Error("Failed to persist embeddings to database",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Error(err))
			return
		}

		utils.Zlog.Info("Successfully persisted embeddings to database",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID),
			zap.Int("embeddingsCount", successCount))
	}
}

// persistEmbeddings saves embeddings to the database and updates data source status
func (wp *WorkerPool) persistEmbeddings(ctx context.Context, job EmbeddingJob) error {
	// Prepare embedding data for insertion
	var embeddingData []loaders.EmbeddingData
	dataSourceIDsMap := make(map[string]bool)

	for _, chunk := range job.Chunks {
		if len(chunk.Embedding) == 0 {
			continue
		}

		// Extract citation from metadata if available
		var citation *string
		if citationVal, ok := chunk.Metadata["citation"].(string); ok && citationVal != "" {
			citation = &citationVal
		}

		var dataSourceID *string
		if chunk.DatasourceID != "" {
			dataSourceID = &chunk.DatasourceID
			dataSourceIDsMap[chunk.DatasourceID] = true
		}

		embeddingData = append(embeddingData, loaders.EmbeddingData{
			Text:         chunk.Content,
			Vector:       chunk.Embedding,
			DataSourceID: dataSourceID,
			Citation:     citation,
		})
	}

	if len(embeddingData) == 0 {
		return nil
	}

	// Insert embeddings into database
	if err := wp.db.BatchInsertEmbeddings(ctx, job.UserID, job.ChatbotID, embeddingData); err != nil {
		return err
	}

	// Update data source status to COMPLETED
	if len(dataSourceIDsMap) > 0 {
		var dataSourceIDs []string
		for id := range dataSourceIDsMap {
			dataSourceIDs = append(dataSourceIDs, id)
		}

		if err := wp.db.UpdateDataSourceStatus(ctx, dataSourceIDs, "COMPLETED"); err != nil {
			utils.Zlog.Error("Failed to update data source status",
				zap.String("jobId", job.JobID),
				zap.Error(err))
			return err
		}

		utils.Zlog.Info("Updated data source status to COMPLETED",
			zap.String("jobId", job.JobID),
			zap.Int("dataSourceCount", len(dataSourceIDs)))
	}

	return nil
}
