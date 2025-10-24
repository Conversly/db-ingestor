package ingestion

import (
	"errors"
	"fmt"

	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func ValidateProcessRequest(r *types.ProcessRequest) error {
	// Run struct-based validation
	if err := validate.Struct(r); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}

	// At least one source must be present
	if len(r.WebsiteURLs) == 0 && len(r.QandAData) == 0 && len(r.Documents) == 0 && len(r.TextContent) == 0 {
		return errors.New("at least one data source must be provided (websiteUrls, qandaData, documents, or textContent)")
	}

	return nil
}


func DetermineSourceTypeFromContentType(contentType string) types.SourceType {
	switch contentType {
	case "application/pdf":
		return types.SourceTypePDF
	case "text/plain":
		return types.SourceTypeText
	case "text/csv", "application/csv":
		return types.SourceTypeCSV
	case "application/json":
		return types.SourceTypeJSON
	default:
		return types.SourceTypeText
	}
}