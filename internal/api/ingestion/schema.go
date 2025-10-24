package ingestion

import (
	"fmt"
	"strings"

	"github.com/Conversly/db-ingestor/internal/types"
)

type Validator interface {
	Validate() error
}

func ValidateProcessRequest(r *types.ProcessRequest) error {
	hasWebsites := len(r.WebsiteURLs) > 0
	hasQA := len(r.QandAData) > 0
	hasDocuments := len(r.Documents) > 0
	hasText := len(r.TextContent) > 0

	if !hasWebsites && !hasQA && !hasDocuments && !hasText {
		return fmt.Errorf("at least one data source must be provided (websiteUrls, qandaData, documents, or textContent)")
	}

	if hasWebsites {
		if err := validateWebsiteURLs(r.WebsiteURLs); err != nil {
			return fmt.Errorf("invalid website URLs: %w", err)
		}
	}

	if hasQA {
		for i, qa := range r.QandAData {
			if err := ValidateQAPair(&qa); err != nil {
				return fmt.Errorf("invalid Q&A pair at index %d: %w", i, err)
			}
		}
	}

	if hasDocuments {
		if err := validateDocumentMetadata(r.Documents); err != nil {
			return fmt.Errorf("invalid documents: %w", err)
		}
	}

	return nil
}

func ValidateQAPair(q *types.QAPair) error {
	if q.DatasourceID <= 0 {
		return fmt.Errorf("datasource ID must be positive")
	}
	if strings.TrimSpace(q.Question) == "" {
		return fmt.Errorf("question cannot be empty")
	}
	if strings.TrimSpace(q.Answer) == "" {
		return fmt.Errorf("answer cannot be empty")
	}
	return nil
}

func validateWebsiteURLs(urls []types.WebsiteURL) error {
	for i, websiteURL := range urls {
		if websiteURL.DatasourceID <= 0 {
			return fmt.Errorf("datasource ID at index %d must be positive", i)
		}
		url := strings.TrimSpace(websiteURL.URL)
		if url == "" {
			return fmt.Errorf("URL at index %d is empty", i)
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("URL at index %d must start with http:// or https://", i)
		}
	}
	return nil
}

func validateDocumentMetadata(docs []types.DocumentMetadata) error {
	allowedContentTypes := map[string]bool{
		"application/pdf":                                                      true,
		"text/plain":                                                           true,
		"text/csv":                                                             true,
		"application/csv":                                                      true,
		"application/json":                                                     true,
		"application/msword":                                                   true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	}

	for i, doc := range docs {
		// Validate DatasourceID
		if doc.DatasourceID <= 0 {
			return fmt.Errorf("document at index %d has invalid datasource ID", i)
		}

		// Validate URL
		if strings.TrimSpace(doc.URL) == "" {
			return fmt.Errorf("document at index %d has empty URL", i)
		}
		if !strings.HasPrefix(doc.URL, "http://") && !strings.HasPrefix(doc.URL, "https://") {
			return fmt.Errorf("document URL at index %d must start with http:// or https://", i)
		}

		// Validate DownloadURL
		if strings.TrimSpace(doc.DownloadURL) == "" {
			return fmt.Errorf("document at index %d has empty download URL", i)
		}
		if !strings.HasPrefix(doc.DownloadURL, "http://") && !strings.HasPrefix(doc.DownloadURL, "https://") {
			return fmt.Errorf("document download URL at index %d must start with http:// or https://", i)
		}

		// Validate Pathname
		if strings.TrimSpace(doc.Pathname) == "" {
			return fmt.Errorf("document at index %d has empty pathname", i)
		}

		// Validate ContentType
		if strings.TrimSpace(doc.ContentType) == "" {
			return fmt.Errorf("document at index %d has empty content type", i)
		}
		if !allowedContentTypes[doc.ContentType] {
			return fmt.Errorf("document at index %d has unsupported content type: %s", i, doc.ContentType)
		}

		// ContentDisposition is required but we don't validate its format
		if strings.TrimSpace(doc.ContentDisposition) == "" {
			return fmt.Errorf("document at index %d has empty content disposition", i)
		}
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

func ParseFormQAData(qandaStrings []string) ([]types.QAPair, error) {
	var qaPairs []types.QAPair

	for i, qaStr := range qandaStrings {
		qaStr = strings.TrimSpace(qaStr)
		if qaStr == "" {
			continue
		}

		parts := strings.SplitN(qaStr, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Q&A format at index %d: expected 'question:answer' format", i)
		}

		qa := types.QAPair{
			Question: strings.TrimSpace(parts[0]),
			Answer:   strings.TrimSpace(parts[1]),
		}

		if err := ValidateQAPair(&qa); err != nil {
			return nil, fmt.Errorf("invalid Q&A at index %d: %w", i, err)
		}

		qaPairs = append(qaPairs, qa)
	}

	return qaPairs, nil
}
