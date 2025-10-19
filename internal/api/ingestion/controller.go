package ingestion

import (
	"encoding/json"
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

	var req types.ProcessRequest
	var err error

	if contentType == "application/json" {
		err = ctrl.parseJSONRequest(c, &req)
	} else {
		err = ctrl.parseMultipartRequest(c, &req)
	}

	if err != nil {
		utils.Zlog.Error("Invalid request", zap.Error(err))
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

func (ctrl *Controller) parseJSONRequest(c *gin.Context, req *types.ProcessRequest) error {
	if err := c.ShouldBindJSON(req); err != nil {
		return err
	}
	return nil
}

func (ctrl *Controller) parseMultipartRequest(c *gin.Context, req *types.ProcessRequest) error {
	req.UserID = c.PostForm("userId")
	req.ChatbotID = c.PostForm("chatbotId")

	if websiteURLsStr := c.PostForm("websiteUrls"); websiteURLsStr != "" {
		var urls []string
		if err := json.Unmarshal([]byte(websiteURLsStr), &urls); err == nil {
			req.WebsiteURLs = urls
		} else {
			req.WebsiteURLs = c.PostFormArray("websiteUrls")
		}
	} else {
		req.WebsiteURLs = c.PostFormArray("websiteUrls")
	}

	if qandaStr := c.PostForm("qandaData"); qandaStr != "" {
		var qaPairs []types.QAPair
		if err := json.Unmarshal([]byte(qandaStr), &qaPairs); err == nil {
			req.QandAData = qaPairs
		} else {
			qandaStrings := c.PostFormArray("qandaData")
			parsed, err := ParseFormQAData(qandaStrings)
			if err != nil {
				return err
			}
			req.QandAData = parsed
		}
	}

	req.TextContent = c.PostFormArray("textContent")

	form, err := c.MultipartForm()
	if err == nil && form != nil {
		if files, ok := form.File["documents"]; ok {
			req.Documents = files
		}
	}

	// Parse options if provided
	if optionsStr := c.PostForm("options"); optionsStr != "" {
		var options types.ProcessingOptions
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

func (ctrl *Controller) ProcessWebsites(c *gin.Context) {
	var req struct {
		UserID    string                `json:"userId" binding:"required"`
		ChatbotID string                `json:"chatbotId" binding:"required"`
		URLs      []string              `json:"urls" binding:"required,min=1"`
		Options   *types.WebsiteConfig  `json:"options,omitempty"`
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
		Options: &types.ProcessingOptions{
			WebsiteConfig: req.Options,
		},
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
