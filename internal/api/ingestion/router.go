package ingestion

import (
	"github.com/Conversly/db-ingestor/internal/loaders"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all ingestion-related routes
func RegisterRoutes(router *gin.RouterGroup, db *loaders.PostgresClient) {
	// Initialize service and controller
	service := NewService(db)
	controller := NewController(service)

	// Main processing endpoint - handles all data source types
	router.POST("/process", controller.Process)

	// Get processing job status
	router.GET("/process/:id", controller.GetIngestionStatus)

	// Specific endpoints for individual data source types (optional, for cleaner API)
	router.POST("/process/websites", controller.ProcessWebsites)
	router.POST("/process/qa", controller.ProcessQA)

	// Chat endpoint
	router.POST("/chat", controller.Chat)

	// Legacy endpoints (for backward compatibility if needed)
	// router.POST("/ingest", controller.Ingest)
	// router.POST("/ingest/bulk", controller.BulkIngest)
	// router.GET("/ingest/:id", controller.GetByID)
}
