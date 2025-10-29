# JSON Embeddings Loader Script

This script loads data from a JSON file, generates embeddings using Gemini API, and saves them to the PostgreSQL database.

## Usage

### Build the script

```bash
go build -o load_json_embeddings scripts/load_json_embeddings.go
```

### Run the script

```bash
./load_json_embeddings \
  -file=data.json \
  -db="postgresql://user:password@localhost:5432/dbname" \
  -keys="GEMINI_API_KEY_1,GEMINI_API_KEY_2" \
  -batch=10
```

### Command-line flags

- `-file` (default: "data.json"): Path to the JSON file containing the data
- `-db` (required): PostgreSQL DSN connection string
- `-keys` (required): Comma-separated Gemini API keys for generating embeddings
- `-batch` (default: 10): Number of records to process in each batch

### JSON File Format

The JSON file should be an array of objects with the following structure:

```json
[
  {
    "id": 170,
    "userId": "user123",
    "chatbotid": 14,
    "topic": "why intentjs and what is intentjs?",
    "text": "Question: why intentjs...",
    "createdAt": "2025-02-16 10:36:28.132316+00",
    "updatedAt": "2025-02-16 10:36:28.132316+00",
    "dataSourceId": null,
    "citation": "why intentjs and what is intentjs?"
  }
]
```

### Example with environment variables

```bash
# Set environment variables
export POSTGRES_DSN="postgresql://user:password@localhost:5432/dbname"
export GEMINI_API_KEYS="key1,key2,key3"

# Run the script
./load_json_embeddings \
  -file=data.json \
  -db="$POSTGRES_DSN" \
  -keys="$GEMINI_API_KEYS" \
  -batch=20
```

## Features

- **Batch Processing**: Processes records in configurable batches to manage memory and API rate limits
- **Multiple API Keys**: Supports multiple Gemini API keys for load balancing
- **Error Handling**: Continues processing even if individual records fail
- **Progress Logging**: Detailed logging of progress and errors
- **Rate Limiting**: Built-in delays between batches to avoid API rate limits
- **Database Transaction Safety**: Uses transactions for data integrity

## Notes

- The script handles "NaN" values in the `userId` field by converting them to "unknown"
- Empty text fields are skipped with an error log
- Each embedding generation has a 30-second timeout
- Database insertions have a 10-second timeout
- The script uses the existing `GeminiEmbedder` and `PostgresClient` from the main application
