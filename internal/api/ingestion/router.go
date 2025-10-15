package ingestion

import (
	"github.com/Conversly/db-ingestor/internal/config"
	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, db *loaders.PostgresClient, cfg *config.Config) {
	queueCapacity := cfg.BatchSize * cfg.WorkerCount
	if queueCapacity <= 0 {
		queueCapacity = 100
	}

	workers := NewWorkerPool(cfg.WorkerCount, queueCapacity)
	workers.Start()

	service := NewService(db, workers)
	controller := NewController(service)
	router.POST("/process", controller.Process)
}
