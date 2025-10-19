package processors

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"go.uber.org/zap"
)

type WebsiteProcessor struct {
	URL    string
	Config *types.Config
}

func NewWebsiteProcessor(urlStr string, config *types.Config) *WebsiteProcessor {
	if config == nil {
		config = types.DefaultConfig()
	}
	return &WebsiteProcessor{
		URL:    urlStr,
		Config: config,
	}
}


func (p *WebsiteProcessor) GetSourceType() types.SourceType {
	return types.SourceTypeWebsite
}

func (p *WebsiteProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing website with Eino loader",
		zap.String("url", p.URL),
		zap.String("chatbotId", chatbotID))

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	loader, err := url.NewLoader(ctx, &url.LoaderConfig{
		Client: client,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create URL loader: %w", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: p.URL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load URL: %w", err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no content loaded from URL")
	}

	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   p.Config.ChunkSize,
		OverlapSize: p.Config.ChunkOverlap,
		Separators:  []string{"\n\n", "\n", ". ", "? ", "! ", " "},
		KeepType:    recursive.KeepTypeNone,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create splitter: %w", err)
	}

	splitDocs, err := splitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to split documents: %w", err)
	}

	chunks := make([]types.ContentChunk, 0, len(splitDocs))
	fullContent := docs[0].Content

	for i, doc := range splitDocs {
		chunk := types.ContentChunk{
			Content:    doc.Content,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"source": p.URL,
			},
		}
		// Merge any metadata from the split document
		for k, v := range doc.MetaData {
			chunk.Metadata[k] = v
		}
		chunks = append(chunks, chunk)
	}

	utils.Zlog.Info("Website processed successfully",
		zap.String("url", p.URL),
		zap.Int("chunks", len(chunks)))

	return &types.ProcessedContent{
		SourceType: types.SourceTypeWebsite,
		Content:    fullContent,
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

