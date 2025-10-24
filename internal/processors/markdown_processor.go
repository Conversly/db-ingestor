package processors

import (
	"context"
	"fmt"
	"time"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type MarkdownProcessor struct {
	Content  []byte
	Filename string
}

func NewMarkdownProcessorFromBytes(content []byte, filename string) *MarkdownProcessor {
	return &MarkdownProcessor{
		Content:  content,
		Filename: filename,
	}
}

func (p *MarkdownProcessor) GetSourceType() types.SourceType {
	return types.SourceTypeText
}

func (p *MarkdownProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing Markdown with Eino splitter",
		zap.String("filename", p.Filename),
		zap.String("chatbotId", chatbotID))

	fullContent := string(p.Content)

	// Initialize markdown header splitter
	splitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":    "h1",
			"##":   "h2",
			"###":  "h3",
			"####": "h4",
		},
		TrimHeaders: false, // Keep headers in the content
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown splitter: %w", err)
	}

	// Create a document from the content
	docs := []*schema.Document{
		{
			ID:      p.Filename,
			Content: fullContent,
			MetaData: map[string]any{
				"filename": p.Filename,
			},
		},
	}

	// Split by headers
	splitDocs, err := splitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown: %w", err)
	}

	// Convert Eino documents to our content chunks
	chunks := make([]types.ContentChunk, 0, len(splitDocs))

	for i, doc := range splitDocs {
		chunk := types.ContentChunk{
			Content:    doc.Content,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"filename": p.Filename,
			},
		}
		// Merge any metadata from the split document (headers)
		for k, v := range doc.MetaData {
			chunk.Metadata[k] = v
		}
		chunks = append(chunks, chunk)
	}

	utils.Zlog.Info("Markdown processed successfully",
		zap.String("filename", p.Filename),
		zap.Int("chunks", len(chunks)))

	return &types.ProcessedContent{
		SourceType: types.SourceTypeText,
		Content:    fullContent,
		Topic:      p.Filename,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"filename":    p.Filename,
			"fileSize":    len(p.Content),
			"contentType": "text/markdown",
			"chatbotId":   chatbotID,
			"userId":      userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

