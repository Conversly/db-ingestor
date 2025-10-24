package ingestion

import (
	"net/http"
	"time"

	"github.com/Conversly/db-ingestor/internal/types"
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

// Process godoc
// @Summary Process multiple data sources
// @Description Process websites, documents, Q&A pairs, and text content for a chatbot
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body types.ProcessRequest true "Process Request"
// @Success 200 {object} ProcessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/process [post]
func (ctrl *Controller) Process(c *gin.Context) {
	var req types.ProcessRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Zlog.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Validate request using the validator
	if err := ValidateProcessRequest(&req); err != nil {
		utils.Zlog.Error("Validation failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	response, err := ctrl.service.Process(c.Request.Context(), req)
	if err != nil {
		utils.Zlog.Error("Failed to process sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}
	c.JSON(http.StatusOK, response)
}