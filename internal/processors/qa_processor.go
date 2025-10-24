package processors

import (
	"context"
	"fmt"
	"time"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

// QAProcessor processes Q&A pairs
// Q&A pairs are stored as single chunks without splitting
type QAProcessor struct {
	QAPair types.QAPair
}

func NewQAProcessor(qa types.QAPair) *QAProcessor {
	return &QAProcessor{QAPair: qa}
}

func (p *QAProcessor) GetSourceType() types.SourceType {
	return types.SourceTypeQA
}

func (p *QAProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing Q&A pair",
		zap.String("question", p.QAPair.Question),
		zap.String("chatbotId", chatbotID))

	content := fmt.Sprintf("Question: %s\nAnswer: %s", p.QAPair.Question, p.QAPair.Answer)

	chunk := types.ContentChunk{
		Content:    content,
		ChunkIndex: 0,
		Metadata: map[string]interface{}{
			"question": p.QAPair.Question,
			"answer":   p.QAPair.Answer,
		},
	}

	return &types.ProcessedContent{
		SourceType: types.SourceTypeQA,
		Content:    content,
		Topic:      p.QAPair.Question,
		Chunks:     []types.ContentChunk{chunk},
		Metadata: map[string]interface{}{
			"question":  p.QAPair.Question,
			"chatbotId": chatbotID,
			"userId":    userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

