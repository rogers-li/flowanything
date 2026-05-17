package domain

import (
	"fmt"

	"flow-anything/internal/platform/contracts/knowledge"
	"flow-anything/internal/platform/kernel/id"
)

const (
	defaultChunkSize    = 800
	defaultChunkOverlap = 120
)

func ChunkDocument(document knowledge.Document) []knowledge.Chunk {
	text := []rune(document.Text)
	if len(text) == 0 {
		return nil
	}

	chunkSize := defaultChunkSize
	overlap := defaultChunkOverlap
	if len(text) <= chunkSize {
		return []knowledge.Chunk{newChunk(document, 0, string(text))}
	}

	chunks := make([]knowledge.Chunk, 0, len(text)/chunkSize+1)
	for start, index := 0, 0; start < len(text); index++ {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, newChunk(document, index, string(text[start:end])))
		if end == len(text) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = end
		}
	}

	return chunks
}

func newChunk(document knowledge.Document, index int, text string) knowledge.Chunk {
	metadata := copyMetadata(document.Metadata)
	metadata["title"] = document.Title
	metadata["chunk_index"] = index

	return knowledge.Chunk{
		ID:       id.ID(fmt.Sprintf("%s_chunk_%d", document.ID.String(), index)),
		DocID:    document.ID,
		KBID:     document.KBID,
		Text:     text,
		Metadata: metadata,
	}
}

func copyMetadata(metadata map[string]any) map[string]any {
	result := make(map[string]any, len(metadata)+2)
	for key, value := range metadata {
		result[key] = value
	}
	return result
}
