package ingestion

import (
	"context"
	"sync"
	"time"

	"github.com/Conversly/db-ingestor/internal/embedder"
	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

type EmbeddingJob struct {
	JobID     string
	UserID    string
	ChatbotID string
	Chunks    []ContentChunk
	CreatedAt time.Time
}

type WorkerPool struct {
	jobs       chan EmbeddingJob
	quit       chan struct{}
	started    bool
	wg         sync.WaitGroup
	numWorkers int
	embedder   *embedder.GeminiEmbedder
}

func NewWorkerPool(numWorkers int, queueCapacity int, geminiEmbedder *embedder.GeminiEmbedder) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if queueCapacity <= 0 {
		queueCapacity = 100
	}
	return &WorkerPool{
		jobs:       make(chan EmbeddingJob, queueCapacity),
		quit:       make(chan struct{}),
		numWorkers: numWorkers,
		embedder:   geminiEmbedder,
	}
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
				case job := <-wp.jobs:
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
	case wp.jobs <- job:
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
	utils.Zlog.Info("Completed embedding job",
		zap.Int("workerId", workerID),
		zap.String("jobId", job.JobID),
		zap.String("chatbotId", job.ChatbotID),
		zap.Int("successful", successCount),
		zap.Int("failed", failCount),
		zap.Duration("duration", duration))

	// TODO: Persist embeddings to database
	// For now, embeddings are stored in memory in job.Chunks[].Embedding
}
