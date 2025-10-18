package ingestion

import (
	"github.com/Conversly/db-ingestor/internal/config"
	"github.com/Conversly/db-ingestor/internal/embedder"
	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RegisterRoutes(router *gin.RouterGroup, db *loaders.PostgresClient, cfg *config.Config) {
	queueCapacity := cfg.BatchSize * cfg.WorkerCount
	if queueCapacity <= 0 {
		queueCapacity = 100
	}

	var geminiEmbedder *embedder.GeminiEmbedder
	if len(cfg.GeminiAPIKeys) > 0 {
		var err error
		geminiEmbedder, err = embedder.NewGeminiEmbedder(cfg.GeminiAPIKeys)
		if err != nil {
			utils.Zlog.Error("Failed to initialize Gemini embedder", zap.Error(err))
			geminiEmbedder = nil
		} else {
			utils.Zlog.Info("Gemini embedder initialized successfully", zap.Int("apiKeyCount", len(cfg.GeminiAPIKeys)))
		}
	} else {
		utils.Zlog.Warn("No Gemini API keys provided, embedder will not be initialized")
	}

	workers := NewWorkerPool(cfg.WorkerCount, queueCapacity, geminiEmbedder)
	workers.Start()

	service := NewService(db, workers)
	controller := NewController(service)
	router.POST("/process", controller.Process)
}
