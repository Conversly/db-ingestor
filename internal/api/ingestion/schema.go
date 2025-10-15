package ingestion

import (
	"fmt"
	"mime/multipart"
	"strings"
)

type Validator interface {
	Validate() error
}

func (r *ProcessRequest) Validate() error {
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
			if err := qa.Validate(); err != nil {
				return fmt.Errorf("invalid Q&A pair at index %d: %w", i, err)
			}
		}
	}

	if hasDocuments {
		if err := validateDocuments(r.Documents); err != nil {
			return fmt.Errorf("invalid documents: %w", err)
		}
	}

	return nil
}

func (q *QAPair) Validate() error {
	if strings.TrimSpace(q.Question) == "" {
		return fmt.Errorf("question cannot be empty")
	}
	if strings.TrimSpace(q.Answer) == "" {
		return fmt.Errorf("answer cannot be empty")
	}
	return nil
}

func validateWebsiteURLs(urls []string) error {
	for i, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("URL at index %d is empty", i)
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("URL at index %d must start with http:// or https://", i)
		}
	}
	return nil
}

func validateDocuments(files []*multipart.FileHeader) error {
	const maxFileSize = 50 * 1024 * 1024 // 50MB

	allowedExtensions := map[string]bool{
		".pdf":  true,
		".txt":  true,
		".csv":  true,
		".doc":  true,
		".docx": true,
		".json": true,
	}

	for i, file := range files {
		if file.Size > maxFileSize {
			return fmt.Errorf("file %s exceeds maximum size of 50MB", file.Filename)
		}

		ext := getFileExtension(file.Filename)
		if !allowedExtensions[ext] {
			return fmt.Errorf("file %s has unsupported extension %s. Allowed: .pdf, .txt, .csv, .doc, .docx, .json", file.Filename, ext)
		}

		if strings.TrimSpace(file.Filename) == "" {
			return fmt.Errorf("file at index %d has empty filename", i)
		}
	}

	return nil
}

func getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}
	return "." + strings.ToLower(parts[len(parts)-1])
}

func DetermineSourceType(filename string) SourceType {
	ext := getFileExtension(filename)

	switch ext {
	case ".pdf":
		return SourceTypePDF
	case ".txt":
		return SourceTypeText
	case ".csv":
		return SourceTypeCSV
	case ".json":
		return SourceTypeJSON
	default:
		return SourceTypeText
	}
}

func GetFileInfo(file *multipart.FileHeader) FileInfo {
	return FileInfo{
		Filename:    file.Filename,
		Size:        file.Size,
		ContentType: file.Header.Get("Content-Type"),
		SourceType:  DetermineSourceType(file.Filename),
	}
}

func ParseFormQAData(qandaStrings []string) ([]QAPair, error) {
	var qaPairs []QAPair

	for i, qaStr := range qandaStrings {
		qaStr = strings.TrimSpace(qaStr)
		if qaStr == "" {
			continue
		}

		parts := strings.SplitN(qaStr, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Q&A format at index %d: expected 'question:answer' format", i)
		}

		qa := QAPair{
			Question: strings.TrimSpace(parts[0]),
			Answer:   strings.TrimSpace(parts[1]),
		}

		if err := qa.Validate(); err != nil {
			return nil, fmt.Errorf("invalid Q&A at index %d: %w", i, err)
		}

		qaPairs = append(qaPairs, qa)
	}

	return qaPairs, nil
}
