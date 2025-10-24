# Migration Summary: File Upload to URL-Based Document Processing

## Overview
This migration changes the document processing flow from receiving actual file uploads to receiving document metadata with public URLs. The system now downloads files from provided URLs before processing them.

## Changes Made

### 1. Type Definitions (`internal/types/datasource.go`)

**Added:**
- `DocumentMetadata` struct with fields:
  - `URL`: Public URL of the document
  - `DownloadURL`: URL to download the document from
  - `Pathname`: File pathname/name
  - `ContentType`: MIME type of the document
  - `ContentDisposition`: Content disposition header value
  
- `Citations` field to `QAPair` struct (optional)

- `DetermineSourceTypeFromContentType()` function to determine source type from MIME type

**Modified:**
- `ProcessRequest.Documents` changed from `[]*multipart.FileHeader` to `[]DocumentMetadata`
- Removed `mime/multipart` import
- Updated JSON tags to remove form tags

### 2. Schema Validation (`internal/api/ingestion/schema.go`)

**Removed:**
- `validateDocuments()` - old file upload validation
- `getFileExtension()` - helper for file extensions
- `DetermineSourceType()` - old filename-based type detection
- `GetFileInfo()` - multipart file info extraction
- `mime/multipart` import

**Added:**
- `validateDocumentMetadata()` - validates document URLs and metadata
  - Validates URL format (must be http/https)
  - Validates download URL
  - Validates pathname is not empty
  - Validates content type against allowed types
  - Validates content disposition is present
  
- `DetermineSourceTypeFromContentType()` - determines type from MIME type

**Allowed Content Types:**
- `application/pdf`
- `text/plain`
- `text/csv` / `application/csv`
- `application/json`
- `application/msword`
- `application/vnd.openxmlformats-officedocument.wordprocessingml.document`

### 3. Controller (`internal/api/ingestion/controller.go`)

**Removed:**
- Multipart form data handling
- `parseJSONRequest()` and `parseMultipartRequest()` methods
- Content type detection logic
- `encoding/json` import

**Simplified:**
- `Process()` endpoint now only accepts JSON requests
- Direct JSON binding with validation
- Cleaner error handling

### 4. File Downloader Utility (`internal/utils/downloader.go`)

**New File Created:**
- `FileDownloader` struct for managing downloads
- `DownloadedFile` struct to hold downloaded file data
- `DownloadFile()` method with:
  - Context-aware HTTP requests
  - Content type validation
  - File size limits (100MB max)
  - Timeout handling (5 minutes default)
  - Proper error handling and logging

### 5. Service Layer (`internal/api/ingestion/service.go`)

**Modified:**
- `processAllSources()` now:
  - Initializes `FileDownloader`
  - Downloads documents before processing
  - Tracks download failures separately
  - Processes downloaded content in parallel
  - Returns appropriate error results for failed downloads

**Document Processing Flow:**
1. Receive document metadata with URLs
2. Download file from `DownloadURL`
3. If download fails → record as failed source
4. If download succeeds → process downloaded bytes
5. Track and report results

### 6. Processor Factory (`internal/processors/factory.go`)

**Removed:**
- `CreateDocumentProcessor()` - old multipart file processor
- `mime/multipart` import

**Added:**
- `CreateDocumentProcessorFromBytes()` - creates processor from byte content
  - Accepts `content []byte`, `filename`, and `contentType`
  - Routes to appropriate processor based on content type and extension

### 7. Individual Processors

**All processors updated to work with byte content:**

#### PDF Processor (`pdf_processor.go`)
- `NewPDFProcessorFromBytes()` - accepts byte content
- Uses `bytes.NewReader()` to create reader for Eino parser
- Removed multipart file handling

#### Text Processor (`text_processor.go`)
- `NewTextFileProcessorFromBytes()` - accepts byte content
- Stores content as `[]byte` instead of multipart file
- Direct string conversion from bytes

#### CSV Processor (`csv_processor.go`)
- `NewCSVProcessorFromBytes()` - accepts byte content
- Uses `bytes.NewReader()` for CSV parsing
- Removed multipart file handling

#### Markdown Processor (`markdown_processor.go`)
- `NewMarkdownProcessorFromBytes()` - accepts byte content
- Direct string conversion from bytes
- Removed file I/O operations

## API Request Schema Changes

### Old Format (Multipart):
```
POST /api/v1/process
Content-Type: multipart/form-data

userId: string
chatbotId: string
websiteUrls: string[] (form array)
documents: file[] (uploaded files)
qandaData: JSON string
textContent: string[] (form array)
```

### New Format (JSON Only):
```json
POST /api/v1/process
Content-Type: application/json

{
  "userId": "string",
  "chatbotId": "string",
  "websiteUrls": ["string"],
  "documents": [
    {
      "url": "https://example.com/file.pdf",
      "downloadUrl": "https://storage.example.com/signed-url",
      "pathname": "document.pdf",
      "contentType": "application/pdf",
      "contentDisposition": "attachment; filename=\"document.pdf\""
    }
  ],
  "qandaData": [
    {
      "question": "string",
      "answer": "string",
      "citations": "string (optional)"
    }
  ],
  "textContent": ["string"],
  "options": {
    "documentConfig": {
      "chunkSize": 1000,
      "chunkOverlap": 200
    }
  }
}
```

## Benefits

1. **No File Size Limits on Server**: Files are downloaded from external storage
2. **Better Scalability**: Offloads file storage to external services (S3, etc.)
3. **Cleaner API**: JSON-only, no multipart complexity
4. **Async Processing**: Can queue downloads and process separately
5. **Better Error Tracking**: Download failures tracked separately from processing failures
6. **Cloud-Native**: Works well with cloud storage solutions

## Error Handling

### Download Failures:
- Logged with full context
- Recorded as failed source in results
- Does not stop processing of other documents
- Returns detailed error message in response

### Processing Failures:
- Same as before
- Logged and tracked separately

## Configuration

### Download Limits:
- **Max File Size**: 100MB
- **Timeout**: 5 minutes
- **Content Types**: Validated against allowed list

These can be adjusted in `internal/utils/downloader.go`

## Migration Notes

### Breaking Changes:
- ⚠️ API now only accepts JSON (no multipart)
- ⚠️ Document field changed from files to metadata objects
- ⚠️ Clients must provide pre-signed URLs for documents

### Backward Compatibility:
- None - this is a breaking change
- Clients must update to new request format

## Testing Recommendations

1. Test document download from various sources
2. Test download failures and error handling
3. Test with maximum file sizes
4. Test content type validation
5. Test concurrent document downloads
6. Test timeout scenarios
7. Verify all document types (PDF, CSV, TXT, etc.)

## Future Enhancements

1. Add retry logic for failed downloads
2. Implement download progress tracking
3. Add support for streaming large files
4. Implement file size validation before download
5. Add URL security validation (prevent SSRF)
6. Cache downloaded files for reprocessing
