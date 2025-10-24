package processors

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"
	"github.com/Conversly/db-ingestor/internal/types"
	"github.com/Conversly/db-ingestor/internal/utils"
	"go.uber.org/zap"
)

type CSVProcessor struct {
	Content  []byte
	Filename string
}

func NewCSVProcessorFromBytes(content []byte, filename string) *CSVProcessor {
	return &CSVProcessor{
		Content:  content,
		Filename: filename,
	}
}

func (p *CSVProcessor) GetSourceType() types.SourceType {
	return types.SourceTypeCSV
}

func (p *CSVProcessor) Process(ctx context.Context, chatbotID, userID string) (*types.ProcessedContent, error) {
	utils.Zlog.Info("Processing CSV file",
		zap.String("filename", p.Filename),
		zap.String("chatbotId", chatbotID))

	// Create a reader from byte content
	reader := csv.NewReader(bytes.NewReader(p.Content))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	headers := records[0]
	dataRows := records[1:]

	if len(dataRows) == 0 {
		return nil, fmt.Errorf("CSV file has no data rows")
	}

	chunks := make([]types.ContentChunk, 0, len(dataRows))
	var fullContentBuilder strings.Builder

	for i, row := range dataRows {
		var rowContent strings.Builder
		rowData := make(map[string]interface{})

		for j, value := range row {
			if j < len(headers) {
				header := headers[j]
				rowContent.WriteString(fmt.Sprintf("%s: %s\n", header, value))
				rowData[header] = value
			}
		}

		content := strings.TrimSpace(rowContent.String())
		
		chunk := types.ContentChunk{
			Content:    content,
			ChunkIndex: i,
			Metadata: map[string]interface{}{
				"filename":   p.Filename,
				"row_number": i + 2,
				"row_data":   rowData,
			},
		}
		chunks = append(chunks, chunk)

		fullContentBuilder.WriteString(content)
		fullContentBuilder.WriteString("\n---\n")
	}

	fullContent := fullContentBuilder.String()

	utils.Zlog.Info("CSV processed successfully",
		zap.String("filename", p.Filename),
		zap.Int("chunks", len(chunks)),
		zap.Int("rows", len(dataRows)))

	return &types.ProcessedContent{
		SourceType: types.SourceTypeCSV,
		Content:    fullContent,
		Topic:      p.Filename,
		Chunks:     chunks,
		Metadata: map[string]interface{}{
			"filename":    p.Filename,
			"fileSize":    len(p.Content),
			"contentType": "text/csv",
			"headers":     headers,
			"rowCount":    len(dataRows),
			"chatbotId":   chatbotID,
			"userId":      userID,
		},
		ProcessedAt: time.Now().UTC(),
	}, nil
}

