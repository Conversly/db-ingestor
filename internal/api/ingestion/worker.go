package ingestion

import (
	"context"
	"sync"
	"time"

	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

// EmbeddingJob represents a unit of work to generate embeddings and persist them
type EmbeddingJob struct {
	JobID     string
	UserID    string
	ChatbotID string
	Chunks    []ContentChunk
	CreatedAt time.Time
}

// WorkerPool manages a pool of workers consuming embedding jobs
type WorkerPool struct {
	jobs       chan EmbeddingJob
	quit       chan struct{}
	started    bool
	wg         sync.WaitGroup
	numWorkers int
}

func NewWorkerPool(numWorkers int, queueCapacity int) *WorkerPool {
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

// Stop gracefully stops workers, waiting for ongoing jobs to finish
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

// Enqueue tries to add a job to the queue non-blockingly
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
	// TODO: Generate embeddings and persist to DB
	// Intentionally left blank per requirements; only logging for now
	utils.Zlog.Info("Processing embedding job",
		zap.Int("workerId", workerID),
		zap.String("jobId", job.JobID),
		zap.String("chatbotId", job.ChatbotID),
		zap.Int("chunks", len(job.Chunks)))

	// Simulate quick processing path without side effects
	_ = start
}
