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
	JobID      string
	UserID     string
	ChatbotID  string
	Chunks     []types.ContentChunk
	CreatedAt  time.Time
	RetryCount int // Track retry attempts to prevent infinite loops
}

const maxEmbeddingRetries = 3

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
		zap.Int("chunks", len(job.Chunks)),
		zap.Int("retryCount", job.RetryCount))

	if wp.embedder == nil {
		utils.Zlog.Warn("Embedder not configured, skipping embedding generation",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var successfulChunks []types.ContentChunk
	var failedChunks []types.ContentChunk

	// Generate embeddings for all chunks
	for i := range job.Chunks {
		embedding, err := wp.embedder.EmbedText(ctx, job.Chunks[i].Content)
		if err != nil {
			utils.Zlog.Error("Failed to generate embedding",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Int("chunkIndex", job.Chunks[i].ChunkIndex),
				zap.Error(err))
			failedChunks = append(failedChunks, job.Chunks[i])
			continue
		}

		utils.Zlog.Debug("Embedding generated",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID),
			zap.Int("chunkIndex", job.Chunks[i].ChunkIndex),
			zap.Int("embeddingLength", len(embedding)))

		job.Chunks[i].Embedding = embedding
		successfulChunks = append(successfulChunks, job.Chunks[i])

		if (len(successfulChunks)+len(failedChunks))%10 == 0 {
			utils.Zlog.Info("Embedding progress",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Int("processed", len(successfulChunks)+len(failedChunks)),
				zap.Int("total", len(job.Chunks)))
		}
	}

	duration := time.Since(start)
	utils.Zlog.Info("Completed embedding generation",
		zap.Int("workerId", workerID),
		zap.String("jobId", job.JobID),
		zap.String("chatbotId", job.ChatbotID),
		zap.Int("successful", len(successfulChunks)),
		zap.Int("failed", len(failedChunks)),
		zap.Duration("duration", duration))

	// Persist successful embeddings to database
	if wp.db != nil && len(successfulChunks) > 0 {
		// Create a job copy with only successful chunks for persistence
		successJob := EmbeddingJob{
			JobID:     job.JobID,
			UserID:    job.UserID,
			ChatbotID: job.ChatbotID,
			Chunks:    successfulChunks,
		}

		// Only mark COMPLETED if there are no failed chunks to retry
		markCompleted := len(failedChunks) == 0

		if err := wp.persistEmbeddingsWithStatus(ctx, successJob, markCompleted); err != nil {
			utils.Zlog.Error("Failed to persist embeddings to database",
				zap.Int("workerId", workerID),
				zap.String("jobId", job.JobID),
				zap.Error(err))
			// Requeue entire job on persist failure
			wp.requeueFailedChunks(workerID, job, job.Chunks)
			return
		}

		utils.Zlog.Info("Successfully persisted embeddings to database",
			zap.Int("workerId", workerID),
			zap.String("jobId", job.JobID),
			zap.Int("embeddingsCount", len(successfulChunks)),
			zap.Bool("markedCompleted", markCompleted))
	}

	// Requeue failed chunks for retry
	if len(failedChunks) > 0 {
		wp.requeueFailedChunks(workerID, job, failedChunks)
	}
}

// requeueFailedChunks requeues failed chunks for retry
func (wp *WorkerPool) requeueFailedChunks(workerID int, originalJob EmbeddingJob, failedChunks []types.ContentChunk) {
	if originalJob.RetryCount >= maxEmbeddingRetries {
		utils.Zlog.Error("Max retries exceeded for embedding job, marking datasources as FAILED",
			zap.Int("workerId", workerID),
			zap.String("jobId", originalJob.JobID),
			zap.Int("failedChunks", len(failedChunks)),
			zap.Int("retryCount", originalJob.RetryCount))

		// Mark affected datasources as FAILED
		if wp.db != nil {
			datasourceIDs := make(map[string]bool)
			for _, chunk := range failedChunks {
				if chunk.DatasourceID != "" {
					datasourceIDs[chunk.DatasourceID] = true
				}
			}
			if len(datasourceIDs) > 0 {
				ids := make([]string, 0, len(datasourceIDs))
				for id := range datasourceIDs {
					ids = append(ids, id)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := wp.db.UpdateDataSourceStatus(ctx, ids, "FAILED"); err != nil {
					utils.Zlog.Error("Failed to update datasource status to FAILED",
						zap.String("jobId", originalJob.JobID),
						zap.Error(err))
				} else {
					utils.Zlog.Info("Marked datasources as FAILED after max retries",
						zap.String("jobId", originalJob.JobID),
						zap.Int("datasourceCount", len(ids)))
				}
			}
		}
		return
	}

	retryJob := EmbeddingJob{
		JobID:      originalJob.JobID + "-retry",
		UserID:     originalJob.UserID,
		ChatbotID:  originalJob.ChatbotID,
		Chunks:     failedChunks,
		CreatedAt:  time.Now().UTC(),
		RetryCount: originalJob.RetryCount + 1,
	}

	if ok := wp.Enqueue(retryJob); !ok {
		utils.Zlog.Error("Failed to requeue embedding job (queue full), marking datasources as FAILED",
			zap.Int("workerId", workerID),
			zap.String("jobId", retryJob.JobID),
			zap.Int("failedChunks", len(failedChunks)))

		// Mark affected datasources as FAILED since we can't retry
		if wp.db != nil {
			datasourceIDs := make(map[string]bool)
			for _, chunk := range failedChunks {
				if chunk.DatasourceID != "" {
					datasourceIDs[chunk.DatasourceID] = true
				}
			}
			if len(datasourceIDs) > 0 {
				ids := make([]string, 0, len(datasourceIDs))
				for id := range datasourceIDs {
					ids = append(ids, id)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := wp.db.UpdateDataSourceStatus(ctx, ids, "FAILED"); err != nil {
					utils.Zlog.Error("Failed to update datasource status to FAILED",
						zap.String("jobId", originalJob.JobID),
						zap.Error(err))
				}
			}
		}
	} else {
		utils.Zlog.Info("Requeued failed chunks for retry",
			zap.Int("workerId", workerID),
			zap.String("jobId", retryJob.JobID),
			zap.Int("chunks", len(failedChunks)),
			zap.Int("retryCount", retryJob.RetryCount))
	}
}

// persistEmbeddings saves embeddings to the database and optionally updates data source status
func (wp *WorkerPool) persistEmbeddings(ctx context.Context, job EmbeddingJob) error {
	return wp.persistEmbeddingsWithStatus(ctx, job, true)
}

// persistEmbeddingsWithStatus saves embeddings and optionally marks datasources as COMPLETED
func (wp *WorkerPool) persistEmbeddingsWithStatus(ctx context.Context, job EmbeddingJob, markCompleted bool) error {
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

	// Update data source status to COMPLETED only if all chunks succeeded
	if markCompleted && len(dataSourceIDsMap) > 0 {
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

// service.go - Mark FAILED on chunking/download failures:
// When processSource fails → marks datasource as FAILED in DB
// When document download fails → marks datasource as FAILED in DB
// worker.go - Requeue failed embeddings:
// Added RetryCount field to EmbeddingJob (max 3 retries)
// Failed chunks get requeued instead of being dropped
// Only marks datasource COMPLETED when ALL chunks succeed
// After max retries exhausted → marks datasource as FAILED
// If queue full during requeue → marks datasource as FAILED
