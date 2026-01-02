package processors

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/dslipak/pdf"
	"go.uber.org/zap"
)

type PDFProcessor struct {
	Content  []byte
	Config   *types.Config
	Filename string
}

func NewPDFProcessorFromBytes(content []byte, filename string, config *types.Config) *PDFProcessor {
	if config == nil {
		config = types.DefaultConfig()
	}
	return &PDFProcessor{
		Content:  content,
		Config:   config,
		Filename: filename,
	}
}

func (p *PDFProcessor) GetSourceType() types.SourceType {
	return types.SourceTypePDF
}

func (p *PDFProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing PDF with pure Go parser",
		zap.String("filename", p.Filename),
		zap.String("chatbotId", chatbotID),
		zap.Int("contentSize", len(p.Content)))

	// Create reader from bytes
	reader := bytes.NewReader(p.Content)

	// Parse PDF using dslipak/pdf (pure Go, no CGO required)
	pdfReader, err := pdf.NewReader(reader, int64(len(p.Content)))
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	numPages := pdfReader.NumPage()
	utils.Zlog.Info("PDF opened successfully",
		zap.String("filename", p.Filename),
		zap.Int("pages", numPages))

	if numPages == 0 {
		return nil, fmt.Errorf("PDF has no pages")
	}

	// Extract text from all pages
	var textBuilder strings.Builder
	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			utils.Zlog.Warn("Failed to extract text from page",
				zap.String("filename", p.Filename),
				zap.Int("page", i),
				zap.Error(err))
			continue
		}

		if text != "" {
			textBuilder.WriteString(text)
			textBuilder.WriteString("\n\n")
		}
	}

	fullContent := strings.TrimSpace(textBuilder.String())
	if fullContent == "" {
		return nil, fmt.Errorf("no text content extracted from PDF")
	}

	utils.Zlog.Info("PDF text extracted",
		zap.String("filename", p.Filename),
		zap.Int("contentLength", len(fullContent)))

	// Chunk the content using the shared chunker
	chunker := utils.NewChunker(p.Config.ChunkSize, p.Config.ChunkOverlap)
	textChunks := chunker.ChunkText(fullContent)

	// Convert to ContentChunks
	chunks := make([]types.ContentChunk, len(textChunks))
	for i, text := range textChunks {
		chunks[i] = types.ContentChunk{
			Content:    text,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"filename": p.Filename,
			},
		}
	}

	utils.Zlog.Info("PDF processed successfully",
		zap.String("filename", p.Filename),
		zap.Int("chunks", len(chunks)))

	return &types.ProcessedContent{
		SourceType: types.SourceTypePDF,
		Content:    fullContent,
		Topic:      p.Filename,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"filename":    p.Filename,
			"fileSize":    len(p.Content),
			"contentType": "application/pdf",
			"pageCount":   numPages,
			"chatbotId":   chatbotID,
			"userId":      userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}
