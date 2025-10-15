package ingestion

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

// Processor defines the interface for processing different data sources
type Processor interface {
	Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error)
	GetSourceType() SourceType
}

// WebsiteProcessor processes website URLs
type WebsiteProcessor struct {
	URL    string
	Config *WebsiteConfig
}

func NewWebsiteProcessor(url string, config *WebsiteConfig) *WebsiteProcessor {
	if config == nil {
		config = &WebsiteConfig{
			MaxDepth: 1,
			MaxPages: 10,
			Timeout:  30,
		}
	}
	return &WebsiteProcessor{
		URL:    url,
		Config: config,
	}
}

func (p *WebsiteProcessor) GetSourceType() SourceType {
	return SourceTypeWebsite
}

func (p *WebsiteProcessor) Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error) {
	utils.Zlog.Info("Processing website",
		zap.String("url", p.URL),
		zap.String("chatbotId", chatbotID))

	// TODO: Implement actual website scraping
	// This is a placeholder implementation
	// You would typically:
	// 1. Fetch the webpage content
	// 2. Parse HTML
	// 3. Extract text content
	// 4. Clean and format the content
	// 5. Create chunks

	content := fmt.Sprintf("Content scraped from %s", p.URL)
	chunks := createChunks(content, 1000, 100)

	return &ProcessedContent{
		SourceType: SourceTypeWebsite,
		Content:    content,
		Topic:      p.URL,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"url":       p.URL,
			"chatbotId": chatbotID,
			"userId":    userID,
			"scrapedAt": time.Now().UTC(),
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

// QAProcessor processes Q&A pairs
type QAProcessor struct {
	QAPair QAPair
}

func NewQAProcessor(qa QAPair) *QAProcessor {
	return &QAProcessor{QAPair: qa}
}

func (p *QAProcessor) GetSourceType() SourceType {
	return SourceTypeQA
}

func (p *QAProcessor) Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error) {
	utils.Zlog.Info("Processing Q&A pair",
		zap.String("question", p.QAPair.Question),
		zap.String("chatbotId", chatbotID))

	content := fmt.Sprintf("Question: %s\nAnswer: %s", p.QAPair.Question, p.QAPair.Answer)

	// Q&A pairs are typically stored as single chunks
	chunk := ContentChunk{
		Content:    content,
		ChunkIndex: 0,
		Metadata: map[string]interface{}{
			"question": p.QAPair.Question,
			"answer":   p.QAPair.Answer,
		},
	}

	return &ProcessedContent{
		SourceType: SourceTypeQA,
		Content:    content,
		Topic:      p.QAPair.Question,
		Chunks:     []ContentChunk{chunk},
		Metadata: map[string]interface{}{
			"question":  p.QAPair.Question,
			"chatbotId": chatbotID,
			"userId":    userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

// DocumentProcessor processes uploaded documents (PDF, TXT, CSV, etc.)
type DocumentProcessor struct {
	File       *multipart.FileHeader
	SourceType SourceType
	Config     *DocumentConfig
}

func NewDocumentProcessor(file *multipart.FileHeader, config *DocumentConfig) *DocumentProcessor {
	if config == nil {
		config = &DocumentConfig{
			ChunkSize:    1000,
			ChunkOverlap: 100,
		}
	}
	return &DocumentProcessor{
		File:       file,
		SourceType: DetermineSourceType(file.Filename),
		Config:     config,
	}
}

func (p *DocumentProcessor) GetSourceType() SourceType {
	return p.SourceType
}

func (p *DocumentProcessor) Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error) {
	utils.Zlog.Info("Processing document",
		zap.String("filename", p.File.Filename),
		zap.String("sourceType", string(p.SourceType)),
		zap.String("chatbotId", chatbotID))

	// Open the file
	file, err := p.File.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read file content
	content, err := p.readFileContent(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// Create chunks
	chunks := createChunks(content, p.Config.ChunkSize, p.Config.ChunkOverlap)

	return &ProcessedContent{
		SourceType: p.SourceType,
		Content:    content,
		Topic:      p.File.Filename,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"filename":    p.File.Filename,
			"fileSize":    p.File.Size,
			"contentType": p.File.Header.Get("Content-Type"),
			"chatbotId":   chatbotID,
			"userId":      userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

func (p *DocumentProcessor) readFileContent(file multipart.File) (string, error) {
	switch p.SourceType {
	case SourceTypePDF:
		return p.readPDF(file)
	case SourceTypeText:
		return p.readText(file)
	case SourceTypeCSV:
		return p.readCSV(file)
	case SourceTypeJSON:
		return p.readJSON(file)
	default:
		return p.readText(file) // Default to text reading
	}
}

func (p *DocumentProcessor) readText(file multipart.File) (string, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read text file: %w", err)
	}
	return string(content), nil
}

func (p *DocumentProcessor) readPDF(file multipart.File) (string, error) {
	// TODO: Implement PDF reading using a library like pdfcpu or go-fitz
	// For now, return placeholder
	utils.Zlog.Warn("PDF processing not yet implemented", zap.String("filename", p.File.Filename))
	return "PDF content extraction not yet implemented", nil
}

func (p *DocumentProcessor) readCSV(file multipart.File) (string, error) {
	// TODO: Implement CSV reading and formatting
	// You might want to convert CSV to a structured text format
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read CSV file: %w", err)
	}
	return string(content), nil
}

func (p *DocumentProcessor) readJSON(file multipart.File) (string, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read JSON file: %w", err)
	}
	return string(content), nil
}

// TextProcessor processes raw text content
type TextProcessor struct {
	Text  string
	Topic string
}

func NewTextProcessor(text, topic string) *TextProcessor {
	if topic == "" {
		topic = "Direct text input"
	}
	return &TextProcessor{
		Text:  text,
		Topic: topic,
	}
}

func (p *TextProcessor) GetSourceType() SourceType {
	return SourceTypeText
}

func (p *TextProcessor) Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error) {
	utils.Zlog.Info("Processing text content",
		zap.String("topic", p.Topic),
		zap.String("chatbotId", chatbotID))

	chunks := createChunks(p.Text, 1000, 100)

	return &ProcessedContent{
		SourceType: SourceTypeText,
		Content:    p.Text,
		Topic:      p.Topic,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"topic":     p.Topic,
			"chatbotId": chatbotID,
			"userId":    userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

// createChunks splits content into chunks with overlap
func createChunks(content string, chunkSize, overlap int) []ContentChunk {
	var chunks []ContentChunk
	contentLen := len(content)

	if contentLen == 0 {
		return chunks
	}

	// If content is smaller than chunk size, return as single chunk
	if contentLen <= chunkSize {
		chunks = append(chunks, ContentChunk{
			Content:    content,
			ChunkIndex: 0,
			Metadata:   map[string]interface{}{},
		})
		return chunks
	}

	// Split into chunks with overlap
	start := 0
	chunkIndex := 0

	for start < contentLen {
		end := start + chunkSize
		if end > contentLen {
			end = contentLen
		}

		// Try to break at word boundary
		if end < contentLen {
			// Look for last space in the chunk
			lastSpace := strings.LastIndex(content[start:end], " ")
			if lastSpace > 0 {
				end = start + lastSpace
			}
		}

		chunk := ContentChunk{
			Content:    strings.TrimSpace(content[start:end]),
			ChunkIndex: chunkIndex,
			Metadata: map[string]interface{}{
				"startPos": start,
				"endPos":   end,
			},
		}
		chunks = append(chunks, chunk)

		chunkIndex++
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

// ProcessorFactory creates processors based on source type
type ProcessorFactory struct {
	websiteConfig  *WebsiteConfig
	documentConfig *DocumentConfig
}

func NewProcessorFactory(options *ProcessingOptions) *ProcessorFactory {
	factory := &ProcessorFactory{}
	if options != nil {
		factory.websiteConfig = options.WebsiteConfig
		factory.documentConfig = options.DocumentConfig
	}
	return factory
}

func (f *ProcessorFactory) CreateWebsiteProcessor(url string) Processor {
	return NewWebsiteProcessor(url, f.websiteConfig)
}

func (f *ProcessorFactory) CreateQAProcessor(qa QAPair) Processor {
	return NewQAProcessor(qa)
}

func (f *ProcessorFactory) CreateDocumentProcessor(file *multipart.FileHeader) Processor {
	return NewDocumentProcessor(file, f.documentConfig)
}

func (f *ProcessorFactory) CreateTextProcessor(text, topic string) Processor {
	return NewTextProcessor(text, topic)
}
