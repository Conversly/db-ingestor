package ingestion

import (
	"encoding/json"
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

// Process godoc
// @Summary Process multiple data sources
// @Description Process websites, documents, Q&A pairs, and text content for a chatbot
// @Tags ingestion
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param userId formData string true "User ID"
// @Param chatbotId formData string true "Chatbot ID"
// @Param websiteUrls formData []string false "Website URLs to scrape"
// @Param documents formData []file false "Documents to process (PDF, TXT, CSV)"
// @Param textContent formData []string false "Raw text content"
// @Success 200 {object} ProcessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/process [post]
func (ctrl *Controller) Process(c *gin.Context) {
	contentType := c.ContentType()

	var req ProcessRequest
	var err error

	// Handle different content types
	if contentType == "application/json" {
		err = ctrl.parseJSONRequest(c, &req)
	} else {
		err = ctrl.parseMultipartRequest(c, &req)
	}

	if err != nil {
		utils.Zlog.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Process the request
	response, err := ctrl.service.Process(c.Request.Context(), req)
	if err != nil {
		utils.Zlog.Error("Failed to process sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// parseJSONRequest parses a JSON request body
func (ctrl *Controller) parseJSONRequest(c *gin.Context, req *ProcessRequest) error {
	if err := c.ShouldBindJSON(req); err != nil {
		return err
	}
	return nil
}

// parseMultipartRequest parses a multipart form data request
func (ctrl *Controller) parseMultipartRequest(c *gin.Context, req *ProcessRequest) error {
	// Parse form fields
	req.UserID = c.PostForm("userId")
	req.ChatbotID = c.PostForm("chatbotId")

	// Parse website URLs
	if websiteURLsStr := c.PostForm("websiteUrls"); websiteURLsStr != "" {
		// Try to parse as JSON array
		var urls []string
		if err := json.Unmarshal([]byte(websiteURLsStr), &urls); err == nil {
			req.WebsiteURLs = urls
		} else {
			// Fallback to comma-separated values
			req.WebsiteURLs = c.PostFormArray("websiteUrls")
		}
	} else {
		req.WebsiteURLs = c.PostFormArray("websiteUrls")
	}

	// Parse Q&A data
	if qandaStr := c.PostForm("qandaData"); qandaStr != "" {
		// Try to parse as JSON array
		var qaPairs []QAPair
		if err := json.Unmarshal([]byte(qandaStr), &qaPairs); err == nil {
			req.QandAData = qaPairs
		} else {
			// Fallback to parsing form array
			qandaStrings := c.PostFormArray("qandaData")
			parsed, err := ParseFormQAData(qandaStrings)
			if err != nil {
				return err
			}
			req.QandAData = parsed
		}
	}

	// Parse text content
	req.TextContent = c.PostFormArray("textContent")

	// Parse file uploads
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		if files, ok := form.File["documents"]; ok {
			req.Documents = files
		}
	}

	// Parse options if provided
	if optionsStr := c.PostForm("options"); optionsStr != "" {
		var options ProcessingOptions
		if err := json.Unmarshal([]byte(optionsStr), &options); err == nil {
			req.Options = &options
		}
	}

	// Validate required fields
	if req.UserID == "" {
		return &ValidationError{Field: "userId", Message: "userId is required"}
	}
	if req.ChatbotID == "" {
		return &ValidationError{Field: "chatbotId", Message: "chatbotId is required"}
	}

	return nil
}

// GetIngestionStatus godoc
// @Summary Get ingestion job status
// @Description Get the status of an ingestion job by its ID
// @Tags ingestion
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} IngestionRecord
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/process/{id} [get]
func (ctrl *Controller) GetIngestionStatus(c *gin.Context) {
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

// Chat godoc
// @Summary Chat with the chatbot
// @Description Send a question to the chatbot and get an answer with context
// @Tags chat
// @Accept json
// @Produce json
// @Param X-API-Key header string true "API Key"
// @Param request body ChatRequest true "Chat request"
// @Success 200 {object} ChatResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/chat [post]
func (ctrl *Controller) Chat(c *gin.Context) {
	// Get API key from header
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:     "Unauthorized",
			Message:   "X-API-Key header is required",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Parse request
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Zlog.Error("Invalid chat request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Process chat request
	response, err := ctrl.service.Chat(c.Request.Context(), req, apiKey)
	if err != nil {
		utils.Zlog.Error("Failed to process chat request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ProcessWebsites godoc
// @Summary Process website URLs
// @Description Process one or more website URLs for a chatbot
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body WebsiteProcessRequest true "Website process request"
// @Success 200 {object} ProcessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/process/websites [post]
func (ctrl *Controller) ProcessWebsites(c *gin.Context) {
	var req struct {
		UserID    string         `json:"userId" binding:"required"`
		ChatbotID string         `json:"chatbotId" binding:"required"`
		URLs      []string       `json:"urls" binding:"required,min=1"`
		Options   *WebsiteConfig `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	processReq := ProcessRequest{
		UserID:      req.UserID,
		ChatbotID:   req.ChatbotID,
		WebsiteURLs: req.URLs,
		Options: &ProcessingOptions{
			WebsiteConfig: req.Options,
		},
	}

	response, err := ctrl.service.Process(c.Request.Context(), processReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal Server Error",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ProcessQA godoc
// @Summary Process Q&A pairs
// @Description Process question-answer pairs for a chatbot
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body QAProcessRequest true "Q&A process request"
// @Success 200 {object} ProcessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/process/qa [post]
func (ctrl *Controller) ProcessQA(c *gin.Context) {
	var req struct {
		UserID    string   `json:"userId" binding:"required"`
		ChatbotID string   `json:"chatbotId" binding:"required"`
		QAPairs   []QAPair `json:"qaPairs" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	processReq := ProcessRequest{
		UserID:    req.UserID,
		ChatbotID: req.ChatbotID,
		QandAData: req.QAPairs,
	}

	response, err := ctrl.service.Process(c.Request.Context(), processReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
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
