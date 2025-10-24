package processors

import (
	"github.com/Conversly/db-ingestor/internal/types"

	"strings"
)

type Factory struct {
	config *types.Config
}

func NewFactory(config *types.Config) *Factory {
	if config == nil {
		config = types.DefaultConfig()
	}
	return &Factory{
		config: config,
	}
}

func (f *Factory) CreateWebsiteProcessor(url string) types.Processor {
	return NewWebsiteProcessor(url, f.config)
}

func (f *Factory) CreateQAProcessor(qa types.QAPair) types.Processor {
	return NewQAProcessor(qa)
}

// CreateDocumentProcessorFromBytes creates a processor for document content from bytes
func (f *Factory) CreateDocumentProcessorFromBytes(content []byte, filename, contentType string) types.Processor {
	filename = strings.ToLower(filename)

	switch {
	case strings.Contains(contentType, "pdf") || strings.HasSuffix(filename, ".pdf"):
		return NewPDFProcessorFromBytes(content, filename, f.config)
	case strings.Contains(contentType, "csv") || strings.HasSuffix(filename, ".csv"):
		return NewCSVProcessorFromBytes(content, filename)
	case strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".markdown"):
		return NewMarkdownProcessorFromBytes(content, filename)
	case strings.Contains(contentType, "text") || strings.HasSuffix(filename, ".txt"):
		return NewTextFileProcessorFromBytes(content, filename, f.config)
	default:
		return NewTextFileProcessorFromBytes(content, filename, f.config)
	}
}

func (f *Factory) CreateTextProcessor(text, topic string) types.Processor {
	return NewTextProcessor(text, topic, f.config)
}

