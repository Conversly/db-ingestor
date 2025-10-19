# Processors Package

This package provides content processors for different data types using Eino data loaders, parsers, and splitters.

## Overview

Each processor implements the `Processor` interface and handles a specific content type:

```go
type Processor interface {
    Process(ctx context.Context, chatbotID, userID string) (*ProcessedContent, error)
    GetSourceType() ingestion.SourceType
}
```

## Processors

### 1. WebsiteProcessor (`website_processor.go`)
**Purpose**: Process website URLs

**Technology**: 
- Eino URL Loader
- Recursive Splitter

**Usage**:
```go
processor := processors.NewWebsiteProcessor("https://example.com", config)
content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Fetches and parses HTML content
- Extracts main text content
- Splits into chunks using recursive strategy
- 30-second HTTP timeout

---

### 2. PDFProcessor (`pdf_processor.go`)
**Purpose**: Process PDF documents

**Technology**: 
- Eino PDF Parser
- Recursive Splitter

**Usage**:
```go
processor := processors.NewPDFProcessor(fileHeader, config)
content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Extracts text from PDF files
- Handles fonts and encoding
- Splits into semantic chunks
- Preserves document structure

---

### 3. MarkdownProcessor (`markdown_processor.go`)
**Purpose**: Process Markdown files

**Technology**: 
- Eino Markdown Header Splitter

**Usage**:
```go
processor := processors.NewMarkdownProcessor(fileHeader)
content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Splits by header hierarchy (H1-H4)
- Preserves headers in content
- Maintains header metadata
- Creates semantically meaningful chunks

---

### 4. CSVProcessor (`csv_processor.go`)
**Purpose**: Process CSV files

**Technology**: 
- Go standard library `encoding/csv`

**Usage**:
```go
processor := processors.NewCSVProcessor(fileHeader)
content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Each row becomes one chunk
- First row treated as headers
- Structured output format
- Row data stored in metadata

**Output Format** (per chunk):
```
ColumnName1: Value1
ColumnName2: Value2
ColumnName3: Value3
```

---

### 5. TextProcessor (`text_processor.go`)
**Purpose**: Process text content and text files

**Technology**: 
- Eino Recursive Splitter

**Usage**:
```go
// For raw text
processor := processors.NewTextProcessor(text, topic, config)

// For text files
processor := processors.NewTextFileProcessor(fileHeader, config)

content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Handles both strings and files
- Recursive splitting strategy
- Configurable chunk size
- Maintains context with overlap

---

### 6. QAProcessor (`qa_processor.go`)
**Purpose**: Process Q&A pairs

**Technology**: 
- Native Go (no splitting)

**Usage**:
```go
qaPair := ingestion.QAPair{
    Question: "What is AI?",
    Answer: "Artificial Intelligence is...",
}
processor := processors.NewQAProcessor(qaPair)
content, err := processor.Process(ctx, chatbotID, userID)
```

**Features**:
- Single chunk per Q&A pair
- Preserves question-answer integrity
- No splitting applied
- Optional metadata support

---

## Factory Pattern

Use the `Factory` to create processors with consistent configuration:

```go
import "github.com/Conversly/db-ingestor/internal/processors"

// Create factory with configuration
config := &processors.Config{
    ChunkSize:    1000,
    ChunkOverlap: 200,
}
factory := processors.NewFactory(config)

// Create processors
websiteProc := factory.CreateWebsiteProcessor(url)
pdfProc := factory.CreateDocumentProcessor(pdfFile)  // Auto-detects type
qaProc := factory.CreateQAProcessor(qaPair)
textProc := factory.CreateTextProcessor(text, topic)
```

### Auto-Detection

`CreateDocumentProcessor()` automatically routes based on file extension:
- `.pdf` → PDFProcessor
- `.csv` → CSVProcessor
- `.md`, `.markdown` → MarkdownProcessor
- `.txt` → TextFileProcessor
- Others → TextFileProcessor (default)

---

## Configuration

### Config Structure
```go
type Config struct {
    ChunkSize    int  // Target chunk size in characters
    ChunkOverlap int  // Overlap between chunks
}
```

### Default Configuration
```go
config := processors.DefaultConfig()
// ChunkSize: 1000
// ChunkOverlap: 200
```

### Custom Configuration
```go
config := &processors.Config{
    ChunkSize:    1500,
    ChunkOverlap: 300,
}
```

---

## Chunk Structure

All processors return `ProcessedContent` with chunks:

```go
type ProcessedContent struct {
    SourceType  ingestion.SourceType
    Content     string                 // Full original content
    Topic       string                 // Filename or URL
    Chunks      []ContentChunk         // Processed chunks
    Metadata    map[string]interface{} // Source metadata
    ProcessedAt time.Time
}

type ContentChunk struct {
    Content    string
    Embedding  []float64              // Populated later by embedder
    Metadata   map[string]interface{} // Chunk-specific metadata
    ChunkIndex int
}
```

---

## Metadata by Type

### Website Chunks
```go
{
  "source": "https://example.com",
  "url": "https://example.com",
  "scrapedAt": "2025-10-18T..."
}
```

### PDF Chunks
```go
{
  "filename": "document.pdf",
  "fileSize": 1024000,
  "contentType": "application/pdf"
}
```

### Markdown Chunks
```go
{
  "filename": "README.md",
  "h1": "Main Title",
  "h2": "Section Title",
  "h3": "Subsection"
}
```

### CSV Chunks
```go
{
  "filename": "data.csv",
  "row_number": 2,
  "row_data": {
    "Name": "John",
    "Age": "30"
  }
}
```

### Q&A Chunks
```go
{
  "question": "What is AI?",
  "answer": "Artificial Intelligence is..."
}
```

---

## Dependencies

### Required Packages
```go
github.com/cloudwego/eino v0.3.12
github.com/cloudwego/eino-ext v0.3.12
```

### Install
```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext@latest
go mod tidy
```

---

## Error Handling

All processors return errors for:
- File reading failures
- Parser initialization errors
- Content extraction errors
- Empty content
- Invalid formats

Example:
```go
content, err := processor.Process(ctx, chatbotID, userID)
if err != nil {
    log.Error("Processing failed", zap.Error(err))
    // Handle error appropriately
}
```

---

## Best Practices

1. **Use the Factory**: Centralized configuration and processor creation
2. **Configure Appropriately**: Adjust chunk size based on your embedding model's limits
3. **Handle Errors**: All processors can fail, always check errors
4. **Context**: Pass appropriate context for cancellation and timeouts
5. **Metadata**: Leverage metadata for citation and source tracking

---

## Performance Tips

1. **Chunk Size**: Larger chunks = fewer API calls but less precision
2. **Overlap**: More overlap = better context but more storage
3. **Parallel Processing**: Process multiple sources concurrently
4. **Memory**: Large files may require streaming (not yet implemented)

---

## Testing

Example test structure:
```go
func TestWebsiteProcessor(t *testing.T) {
    ctx := context.Background()
    processor := processors.NewWebsiteProcessor(
        "https://example.com",
        &processors.Config{ChunkSize: 500, ChunkOverlap: 50},
    )
    
    content, err := processor.Process(ctx, "chatbot-1", "user-1")
    
    assert.NoError(t, err)
    assert.NotEmpty(t, content.Chunks)
    assert.Equal(t, ingestion.SourceTypeWebsite, content.SourceType)
}
```

---

## Future Enhancements

- [ ] Add support for DOCX files
- [ ] Add support for XLSX files
- [ ] Implement streaming for large files
- [ ] Add custom separator configuration
- [ ] Support for more markdown features (tables, code blocks)
- [ ] Image extraction from PDFs
- [ ] Multi-language support
- [ ] Custom HTML selectors for web scraping

---

## Contributing

When adding new processors:

1. Create a new file: `{type}_processor.go`
2. Implement the `Processor` interface
3. Add to factory's `CreateDocumentProcessor()` if file-based
4. Add tests
5. Update this README

---

## License

Part of the Conversly project.

