package scorer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBM25Scorer(t *testing.T) {
	scorer := NewBM25Scorer(3)

	// Create test documents
	doc1 := &Document{
		Path:    "doc1.md",
		Content: "This is a test document about Go programming",
		WordFreq: map[string]int{
			"test":           1,
			"document":       1,
			"test document":  1,
			"document go":    1,
			"go":             1,
			"programming":    1,
			"go programming": 1,
		},
	}

	doc2 := &Document{
		Path:    "doc2.md",
		Content: "Another document about programming languages",
		WordFreq: map[string]int{
			"another":               1,
			"document":              1,
			"programming":           1,
			"languages":             1,
			"another document":      1,
			"document programming":  1,
			"programming languages": 1,
		},
	}

	// Process documents
	err := scorer.ProcessDocument(doc1)
	assert.NoError(t, err)

	err = scorer.ProcessDocument(doc2)
	assert.NoError(t, err)

	// Test scoring with individual words
	score1 := scorer.Score("programming languages", doc1)
	score2 := scorer.Score("programming languages", doc2)

	// Doc2 should have a higher score as it contains both terms and the bigram
	assert.Greater(t, score2, score1)

	// Test scoring with exact n-grams
	score3 := scorer.Score("test document", doc1)
	score4 := scorer.Score("test document", doc2)

	// Doc1 should have a higher score as it contains the exact bigram
	assert.Greater(t, score3, score4)

	// Test with non-existent terms
	score5 := scorer.Score("nonexistent", doc1)
	assert.Equal(t, float64(0), score5)
}

func TestBM25ScorerEmpty(t *testing.T) {
	scorer := NewBM25Scorer(3)

	// Test with empty document
	emptyDoc := &Document{
		Path:     "empty.md",
		Content:  "",
		WordFreq: map[string]int{},
	}

	err := scorer.ProcessDocument(emptyDoc)
	assert.NoError(t, err)

	score := scorer.Score("test", emptyDoc)
	assert.Equal(t, float64(0), score)
}
