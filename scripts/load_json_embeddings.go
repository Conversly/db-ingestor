package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Conversly/db-ingestor/internal/embedder"
	"github.com/Conversly/db-ingestor/internal/loaders"
	"go.uber.org/zap"
)

// JSONRecord represents the structure of each record in the JSON file
type JSONRecord struct {
	ID           int     `json:"id"`
	UserID       string  `json:"userId"`
	ChatbotID    string  `json:"chatbotid"`
	Text         string  `json:"text"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
	DataSourceID *int    `json:"dataSourceId"`
	Citation     *string `json:"citation"`
}

func main() {
	// Command line flags
	jsonFile := flag.String("file", "data.json", "Path to the JSON file")
	dbDSN := flag.String("db", "", "PostgreSQL DSN connection string")
	apiKeys := flag.String("keys", "", "Comma-separated Gemini API keys")
	batchSize := flag.Int("batch", 10, "Batch size for processing")
	flag.Parse()

	// Validate required flags
	if *dbDSN == "" {
		fmt.Println("Error: Database DSN is required. Use -db flag")
		flag.Usage()
		os.Exit(1)
	}

	if *apiKeys == "" {
		fmt.Println("Error: Gemini API keys are required. Use -keys flag")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx := context.Background()

	// Load JSON file
	logger.Info("Loading JSON file", zap.String("file", *jsonFile))
	records, err := loadJSONFile(*jsonFile)
	if err != nil {
		logger.Fatal("Failed to load JSON file", zap.Error(err))
	}
	logger.Info("Loaded records from JSON", zap.Int("count", len(records)))

	// Initialize Gemini embedder
	logger.Info("Initializing Gemini embedder")
	keys := parseAPIKeys(*apiKeys)
	geminiEmbedder, err := embedder.NewGeminiEmbedder(keys)
	if err != nil {
		logger.Fatal("Failed to initialize Gemini embedder", zap.Error(err))
	}

	// Initialize PostgreSQL client
	logger.Info("Connecting to PostgreSQL database")
	pgClient, err := loaders.NewPostgresClient(*dbDSN, 4, *batchSize)
	if err != nil {
		logger.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer pgClient.Close()

	// Process records in batches
	logger.Info("Starting to process records", zap.Int("totalRecords", len(records)))
	totalProcessed := 0
	totalFailed := 0

	for i := 0; i < len(records); i += *batchSize {
		end := i + *batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		logger.Info("Processing batch",
			zap.Int("batchStart", i),
			zap.Int("batchEnd", end),
			zap.Int("batchSize", len(batch)))

		processed, failed := processBatch(ctx, batch, geminiEmbedder, pgClient, logger)
		totalProcessed += processed
		totalFailed += failed

		// Add a small delay between batches to avoid rate limiting
		if end < len(records) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Info("Completed processing all records",
		zap.Int("totalRecords", len(records)),
		zap.Int("successful", totalProcessed),
		zap.Int("failed", totalFailed))
}

// loadJSONFile reads and parses the JSON file
func loadJSONFile(filePath string) ([]JSONRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var records []JSONRecord
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return records, nil
}

// parseAPIKeys splits comma-separated API keys
func parseAPIKeys(keysStr string) []string {
	var keys []string
	var currentKey string

	for _, char := range keysStr {
		if char == ',' {
			if currentKey != "" {
				keys = append(keys, currentKey)
				currentKey = ""
			}
		} else {
			currentKey += string(char)
		}
	}

	if currentKey != "" {
		keys = append(keys, currentKey)
	}

	return keys
}

// processBatch processes a batch of records: generates embeddings and saves to database
func processBatch(
	ctx context.Context,
	batch []JSONRecord,
	geminiEmbedder *embedder.GeminiEmbedder,
	pgClient *loaders.PostgresClient,
	logger *zap.Logger,
) (processed, failed int) {
	for _, record := range batch {
		if err := processRecord(ctx, record, geminiEmbedder, pgClient, logger); err != nil {
			logger.Error("Failed to process record",
				zap.Int("id", record.ID),
				zap.Error(err))
			failed++
			continue
		}
		processed++
	}

	return processed, failed
}

// processRecord processes a single record: generates embedding and saves to database
func processRecord(
	ctx context.Context,
	record JSONRecord,
	geminiEmbedder *embedder.GeminiEmbedder,
	pgClient *loaders.PostgresClient,
	logger *zap.Logger,
) error {
	// Validate record
	if record.Text == "" {
		return fmt.Errorf("text field is empty")
	}

	// Use values from record or defaults
	userID := record.UserID
	if userID == "" || userID == "NaN" {
		userID = "19280499-9952-4275-99e9-3cde452b31fa" // fallback default
	}

	chatbotID := record.ChatbotID
	if chatbotID == "" {
		chatbotID = "clxxx-default-chatbot-id" // fallback default cuid2 format
	}

	dataSourceID := 12

	logger.Info("Generating embedding",
		zap.Int("id", record.ID),
		zap.String("chatbotId", chatbotID),
		zap.Int("textLength", len(record.Text)))

	// Generate embedding with timeout
	embedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	embedding, err := geminiEmbedder.EmbedText(embedCtx, record.Text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	logger.Info("Embedding generated successfully",
		zap.Int("id", record.ID),
		zap.Int("embeddingDimensions", len(embedding)))

	// Prepare embedding data for insertion
	embeddingData := []loaders.EmbeddingData{
		{
			Text:         record.Text,
			Vector:       embedding,
			DataSourceID: &dataSourceID,
			Citation:     record.Citation,
		},
	}

	// Insert into database
	insertCtx, insertCancel := context.WithTimeout(ctx, 10*time.Second)
	defer insertCancel()

	if err := pgClient.BatchInsertEmbeddings(insertCtx, userID, chatbotID, embeddingData); err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	logger.Info("Successfully saved embedding to database",
		zap.Int("id", record.ID),
		zap.String("userId", userID),
		zap.String("chatbotId", chatbotID))

	return nil
}
