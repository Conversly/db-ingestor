package processors

import (
	"github.com/Conversly/db-ingestor/internal/types"

	"mime/multipart"
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

func (f *Factory) CreateDocumentProcessor(file *multipart.FileHeader) types.Processor {
	filename := strings.ToLower(file.Filename)

	switch {
	case strings.HasSuffix(filename, ".pdf"):
		return NewPDFProcessor(file, f.config)
	case strings.HasSuffix(filename, ".csv"):
		return NewCSVProcessor(file)
	case strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".markdown"):
		return NewMarkdownProcessor(file)
	case strings.HasSuffix(filename, ".txt"):
		return NewTextFileProcessor(file, f.config)
	default:
		return NewTextFileProcessor(file, f.config)
	}
}

func (f *Factory) CreateTextProcessor(text, topic string) types.Processor {
	return NewTextProcessor(text, topic, f.config)
}

