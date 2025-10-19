package processors

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type TextProcessor struct {
	Text     string
	Topic    string
	Config   *types.Config
	FromFile bool
	File     *multipart.FileHeader
}

func NewTextProcessor(text, topic string, config *types.Config) *TextProcessor {
	if config == nil {
		config = types.DefaultConfig()
	}
	if topic == "" {
		topic = "Direct text input"
	}
	return &TextProcessor{
		Text:     text,
		Topic:    topic,
		Config:   config,
		FromFile: false,
	}
}

func NewTextFileProcessor(file *multipart.FileHeader, config *types.Config) *TextProcessor {
	if config == nil {
		config = types.DefaultConfig()
	}
	return &TextProcessor{
		Topic:    file.Filename,
		Config:   config,
		FromFile: true,
		File:     file,
	}
}

// GetSourceType returns the source type
func (p *TextProcessor) GetSourceType() types.SourceType {
	return types.SourceTypeText
}

// Process splits and processes the text content
func (p *TextProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing text with Eino recursive splitter",
		zap.String("topic", p.Topic),
		zap.Bool("fromFile", p.FromFile),
		zap.String("chatbotId", chatbotID))

	var content string
	var err error

	if p.FromFile {
		// Read from file
		file, err := p.File.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open text file: %w", err)
		}
		defer file.Close()

		contentBytes, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read text file: %w", err)
		}
		content = string(contentBytes)
	} else {
		content = p.Text
	}

	if content == "" {
		return nil, fmt.Errorf("text content is empty")
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

	// Create a document from the content
	docs := []*schema.Document{
		{
			ID:      p.Topic,
			Content: content,
			MetaData: map[string]any{
				"topic": p.Topic,
			},
		},
	}

	// Split into chunks
	splitDocs, err := splitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to split text: %w", err)
	}

	// Convert Eino documents to our content chunks
	chunks := make([]types.ContentChunk, 0, len(splitDocs))

	for i, doc := range splitDocs {
		chunk := types.ContentChunk{
			Content:    doc.Content,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"topic": p.Topic,
			},
		}
		// Merge any metadata from the split document
		for k, v := range doc.MetaData {
			chunk.Metadata[k] = v
		}
		chunks = append(chunks, chunk)
	}

	metadata := map[string]interface{}{
		"topic":     p.Topic,
		"chatbotId": chatbotID,
		"userId":    userID,
	}

	if p.FromFile {
		metadata["filename"] = p.File.Filename
		metadata["fileSize"] = p.File.Size
		metadata["contentType"] = p.File.Header.Get("Content-Type")
	}

	utils.Zlog.Info("Text processed successfully",
		zap.String("topic", p.Topic),
		zap.Int("chunks", len(chunks)))

	return &types.ProcessedContent{
		SourceType:  types.SourceTypeText,
		Content:     content,
		Topic:       p.Topic,
		Chunks:      chunks,
		Metadata:    metadata,
		ProcessedAt: time.Now().UTC(),
	}, nil
}

