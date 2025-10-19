package processors

import (
	"context"
	"fmt"
	"mime/multipart"
	"time"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	einoParser "github.com/cloudwego/eino/components/document/parser"
	"go.uber.org/zap"
)

type PDFProcessor struct {
	File     *multipart.FileHeader
	Config   *types.Config
	Filename string
}

func NewPDFProcessor(file *multipart.FileHeader, config *types.Config) *PDFProcessor {
	if config == nil {
		config = types.DefaultConfig()
	}
	return &PDFProcessor{
		File:     file,
		Config:   config,
		Filename: file.Filename,
	}
}

func (p *PDFProcessor) GetSourceType() types.SourceType {
	return types.SourceTypePDF
}

func (p *PDFProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing PDF with Eino parser",
		zap.String("filename", p.Filename),
		zap.String("chatbotId", chatbotID))

	file, err := p.File.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	parser, err := pdf.NewPDFParser(ctx, &pdf.Config{
		ToPages: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF parser: %w", err)
	}

	// Parse PDF
	docs, err := parser.Parse(ctx, file,
		einoParser.WithURI(p.Filename),
		einoParser.WithExtraMeta(map[string]any{
			"filename": p.Filename,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF: %w", err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no content extracted from PDF")
	}

	// Initialize recursive splitter
	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   p.Config.ChunkSize,
		OverlapSize: p.Config.ChunkOverlap,
		Separators:  []string{"\n\n", "\n", ". ", "? ", "! ", " "},
		KeepType:    recursive.KeepTypeNone,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create splitter: %w", err)
	}

	// Split documents into chunks
	splitDocs, err := splitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to split documents: %w", err)
	}

	// Convert Eino documents to our content chunks
	chunks := make([]types.ContentChunk, 0, len(splitDocs))
	fullContent := docs[0].Content

	for i, doc := range splitDocs {
		chunk := types.ContentChunk{
			Content:    doc.Content,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"filename": p.Filename,
			},
		}
		// Merge any metadata from the split document
		for k, v := range doc.MetaData {
			chunk.Metadata[k] = v
		}
		chunks = append(chunks, chunk)
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
			"fileSize":    p.File.Size,
			"contentType": p.File.Header.Get("Content-Type"),
			"chatbotId":   chatbotID,
			"userId":      userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

