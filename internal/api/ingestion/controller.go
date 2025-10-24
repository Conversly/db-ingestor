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

	// Validate required fields
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   "userId is required",
			Timestamp: time.Now().UTC(),
		})
		return
	}
	if req.ChatbotID == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   "chatbotId is required",
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

func (ctrl *Controller) ProcessWebsites(c *gin.Context) {
	var req struct {
		UserID    string                    `json:"userId" binding:"required"`
		ChatbotID string                    `json:"chatbotId" binding:"required"`
		URLs      []string                  `json:"urls" binding:"required,min=1"`
		Options   *types.ProcessingOptions  `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	processReq := types.ProcessRequest{
		UserID:      req.UserID,
		ChatbotID:   req.ChatbotID,
		WebsiteURLs: req.URLs,
		Options:     req.Options,
	}

	response, err := ctrl.service.Process(c.Request.Context(), processReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (ctrl *Controller) ProcessQA(c *gin.Context) {
	var req struct {
		UserID    string          `json:"userId" binding:"required"`
		ChatbotID string          `json:"chatbotId" binding:"required"`
		QAPairs   []types.QAPair  `json:"qaPairs" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	processReq := types.ProcessRequest{
		UserID:    req.UserID,
		ChatbotID: req.ChatbotID,
		QandAData: req.QAPairs,
	}

	response, err := ctrl.service.Process(c.Request.Context(), processReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
