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

	// Register routes
	router.POST("/ingest", controller.Ingest)
	router.POST("/ingest/bulk", controller.BulkIngest)
	router.GET("/ingest/:id", controller.GetByID)
}
