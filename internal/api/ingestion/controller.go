package ingestion

import (
	"net/http"
	"time"

	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller handles HTTP requests for ingestion
type Controller struct {
	service *Service
}

// NewController creates a new ingestion controller
func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

// Ingest godoc
// @Summary Ingest data
// @Description Ingest a single data item
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body IngestRequest true "Ingestion request"
// @Success 200 {object} IngestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ingest [post]
func (ctrl *Controller) Ingest(c *gin.Context) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Zlog.Error("Invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Set timestamp if not provided
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now().UTC()
	}

	response, err := ctrl.service.Ingest(c.Request.Context(), req)
	if err != nil {
		utils.Zlog.Error("Failed to process ingestion", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal Server Error",
			Message:   "Failed to process ingestion request",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// BulkIngest godoc
// @Summary Bulk ingest data
// @Description Ingest multiple data items
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body BulkIngestRequest true "Bulk ingestion request"
// @Success 200 {object} BulkIngestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ingest/bulk [post]
func (ctrl *Controller) BulkIngest(c *gin.Context) {
	var req BulkIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Zlog.Error("Invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	response, err := ctrl.service.BulkIngest(c.Request.Context(), req)
	if err != nil {
		utils.Zlog.Error("Failed to process bulk ingestion", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal Server Error",
			Message:   "Failed to process bulk ingestion request",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetByID godoc
// @Summary Get ingestion by ID
// @Description Get an ingestion record by its ID
// @Tags ingestion
// @Produce json
// @Param id path string true "Ingestion ID"
// @Success 200 {object} IngestionRecord
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ingest/{id} [get]
func (ctrl *Controller) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   "ID parameter is required",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	record, err := ctrl.service.GetIngestionByID(c.Request.Context(), id)
	if err != nil {
		utils.Zlog.Error("Failed to fetch ingestion record",
			zap.String("id", id),
			zap.Error(err))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     "Not Found",
			Message:   "Ingestion record not found",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, record)
}
