package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// EmbeddingRequest represents the request payload for Gemini embedding API
type EmbeddingRequest struct {
	Model               string           `json:"model"`
	Content             EmbeddingContent `json:"content"`
	TaskType            string           `json:"taskType,omitempty"`
	OutputDimensionality int             `json:"outputDimensionality,omitempty"`
}

// EmbeddingContent represents the content structure for embedding
type EmbeddingContent struct {
	Parts []Part `json:"parts"`
}

// Part represents a single part of the content
type Part struct {
	Text string `json:"text"`
}

// EmbeddingResponse represents the response from Gemini embedding API
type EmbeddingResponse struct {
	Embedding Embedding `json:"embedding"`
}

// Embedding represents the embedding vector
type Embedding struct {
	Values []float64 `json:"values"`
}

// GeminiEmbedder handles embedding generation with rotating API keys
type GeminiEmbedder struct {
	apiKeys []string
	client  *http.Client
	baseURL string
}

// NewGeminiEmbedder creates a new embedder with API keys
func NewGeminiEmbedder(keys []string) (*GeminiEmbedder, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one API key is required")
	}
	return &GeminiEmbedder{
		apiKeys: keys,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
	}, nil
}

// getRandomKey returns a random API key from the pool
func (g *GeminiEmbedder) getRandomKey() string {
	if len(g.apiKeys) == 1 {
		return g.apiKeys[0]
	}
	return g.apiKeys[rand.Intn(len(g.apiKeys))]
}

// normalize normalizes a vector to unit length
func normalize(vec []float64) []float64 {
	if len(vec) == 0 {
		return vec
	}

	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm == 0 || math.IsNaN(norm) || math.IsInf(norm, 0) {
		return vec
	}

	normalized := make([]float64, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}
	return normalized
}

func (g *GeminiEmbedder) EmbedText(ctx context.Context, text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := EmbeddingRequest{
		Model: "text-embedding-004",
		Content: EmbeddingContent{
			Parts: []Part{
				{Text: text},
			},
		},
		TaskType:            "RETRIEVAL_DOCUMENT",
		OutputDimensionality: 768,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiKey := g.getRandomKey()
	url := fmt.Sprintf("%s/text-embedding-004:embedContent?key=%s", g.baseURL, apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embeddingResp.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embeddings returned from API")
	}

	// Verify we got the expected dimension
	if len(embeddingResp.Embedding.Values) != 768 {
		return nil, fmt.Errorf("expected 768 dimensions, got %d", len(embeddingResp.Embedding.Values))
	}

	normalized := normalize(embeddingResp.Embedding.Values)
	return normalized, nil
}

// EmbedBatch embeds multiple texts in parallel (with context for cancellation)
// NOTE: This makes individual API calls for each text (not using Gemini's batch API)
// It's suitable for free tier but will be slower than batch API for large volumes
// Uses the same RETRIEVAL_DOCUMENT task type and 768 dimensions as EmbedText
func (g *GeminiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	embeddings := make([][]float64, len(texts))
	errors := make([]error, len(texts))

	// Limit concurrent requests to avoid rate limiting on free tier
	sem := make(chan struct{}, 5) // max 5 concurrent requests

	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
			go func(idx int, txt string) {
				defer func() { <-sem }()
				embedding, err := g.EmbedText(ctx, txt)
				embeddings[idx] = embedding
				errors[idx] = err
			}(i, text)
		}
	}

	// Wait for all goroutines to finish
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}

	// Check for errors
	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("failed to embed text at index %d: %w", i, err)
		}
	}

	return embeddings, nil
}
