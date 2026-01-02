package utils

import (
	"strings"
	"unicode/utf8"
)

// Chunker splits text into overlapping chunks
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string
}

// NewChunker creates a new Chunker with specified chunk size and overlap
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
		Separators:   []string{"\n\n", "\n", ". ", "? ", "! ", "; ", ", ", " "},
	}
}

// ChunkText splits text into chunks with overlap
func (c *Chunker) ChunkText(text string) []string {
	if text == "" {
		return nil
	}

	text = strings.TrimSpace(text)
	textLen := utf8.RuneCountInString(text)

	if textLen <= c.ChunkSize {
		return []string{text}
	}

	return c.recursiveSplit(text, c.Separators)
}

func (c *Chunker) recursiveSplit(text string, separators []string) []string {
	if utf8.RuneCountInString(text) <= c.ChunkSize {
		if strings.TrimSpace(text) != "" {
			return []string{strings.TrimSpace(text)}
		}
		return nil
	}

	// Find the best separator
	var bestSep string
	for _, sep := range separators {
		if strings.Contains(text, sep) {
			bestSep = sep
			break
		}
	}

	// If no separator found, split by character count
	if bestSep == "" {
		return c.splitBySize(text)
	}

	// Split by the separator
	parts := strings.Split(text, bestSep)
	var chunks []string
	var currentChunk strings.Builder

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Add separator back (except for first part)
		testContent := currentChunk.String()
		if testContent != "" {
			testContent += bestSep + part
		} else {
			testContent = part
		}

		if utf8.RuneCountInString(testContent) <= c.ChunkSize {
			if currentChunk.Len() > 0 {
				currentChunk.WriteString(bestSep)
			}
			currentChunk.WriteString(part)
		} else {
			// Current chunk is full, save it and start new one
			if currentChunk.Len() > 0 {
				chunk := strings.TrimSpace(currentChunk.String())
				if chunk != "" {
					chunks = append(chunks, chunk)
				}
			}

			// Handle part that might be too large
			if utf8.RuneCountInString(part) > c.ChunkSize {
				// Try with next separator level
				nextSeps := separators
				for j, sep := range separators {
					if sep == bestSep && j+1 < len(separators) {
						nextSeps = separators[j+1:]
						break
					}
				}
				subChunks := c.recursiveSplit(part, nextSeps)
				chunks = append(chunks, subChunks...)
				currentChunk.Reset()
			} else {
				currentChunk.Reset()
				currentChunk.WriteString(part)
			}
		}

		// Add overlap from previous chunk
		if i > 0 && len(chunks) > 0 && c.ChunkOverlap > 0 && currentChunk.Len() > 0 {
			// Overlap is handled by including content from end of previous chunk
			// This is simplified - just ensure chunks aren't empty
		}
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		chunk := strings.TrimSpace(currentChunk.String())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}

	// Apply overlap between chunks
	if c.ChunkOverlap > 0 && len(chunks) > 1 {
		chunks = c.applyOverlap(chunks)
	}

	return chunks
}

func (c *Chunker) splitBySize(text string) []string {
	runes := []rune(text)
	var chunks []string

	for i := 0; i < len(runes); {
		end := i + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := strings.TrimSpace(string(runes[i:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		// Move forward by (chunkSize - overlap)
		step := c.ChunkSize - c.ChunkOverlap
		if step <= 0 {
			step = c.ChunkSize
		}
		i += step
	}

	return chunks
}

func (c *Chunker) applyOverlap(chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}

	result := make([]string, len(chunks))
	result[0] = chunks[0]

	for i := 1; i < len(chunks); i++ {
		prevChunk := chunks[i-1]
		prevRunes := []rune(prevChunk)

		// Get overlap from end of previous chunk
		overlapStart := len(prevRunes) - c.ChunkOverlap
		if overlapStart < 0 {
			overlapStart = 0
		}

		overlap := string(prevRunes[overlapStart:])

		// Find a good break point (word boundary)
		if idx := strings.LastIndex(overlap, " "); idx > 0 {
			overlap = overlap[idx+1:]
		}

		// Prepend overlap to current chunk if it doesn't already start with it
		if !strings.HasPrefix(chunks[i], overlap) && overlap != "" {
			result[i] = overlap + " " + chunks[i]
		} else {
			result[i] = chunks[i]
		}
	}

	return result
}
