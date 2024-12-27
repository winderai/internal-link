package scorer

import (
	"math"
	"strings"
)

// Document represents a markdown document with its content and metadata
type Document struct {
	Path     string
	Content  string
	WordFreq map[string]int
}

// LinkSuggestion represents a suggested internal link
type LinkSuggestion struct {
	SourcePath string
	TargetPath string
	Score      float64
	Context    string
	WordToLink string
	Position   int
}

// Scorer defines the interface for document scoring algorithms
type Scorer interface {
	// Score calculates the relevance score between a query and a document
	Score(query string, doc *Document) float64

	// ProcessDocument prepares a document for scoring
	ProcessDocument(doc *Document) error
}

// BM25Scorer implements the BM25 algorithm for document scoring
type BM25Scorer struct {
	k1       float64
	b        float64
	docs     []*Document
	avgdl    float64
	idf      map[string]float64
	maxNGram int
}

// NewBM25Scorer creates a new BM25 scorer with default parameters
func NewBM25Scorer(maxNGram int) *BM25Scorer {
	return &BM25Scorer{
		k1:       1.2,
		b:        0.75,
		idf:      make(map[string]float64),
		maxNGram: maxNGram,
	}
}

// ProcessDocument implements the Scorer interface
func (s *BM25Scorer) ProcessDocument(doc *Document) error {
	s.docs = append(s.docs, doc)

	// Recalculate average document length
	var totalLength int
	for _, d := range s.docs {
		totalLength += len(d.WordFreq)
	}
	s.avgdl = float64(totalLength) / float64(len(s.docs))

	// Update IDF scores
	s.calculateIDF()

	return nil
}

// Score implements the Scorer interface
func (s *BM25Scorer) Score(query string, doc *Document) float64 {
	var score float64
	docLen := float64(len(doc.WordFreq))

	// Split query into terms and normalize
	queryTerms := strings.Fields(strings.ToLower(query))

	// Generate n-grams from query terms
	var allQueryTerms []string
	ngramLimit := min(len(queryTerms), s.maxNGram)
	for n := 1; n <= ngramLimit; n++ {
		for i := 0; i <= len(queryTerms)-n; i++ {
			ngram := strings.Join(queryTerms[i:i+n], " ")
			allQueryTerms = append(allQueryTerms, ngram)
		}
	}

	// Check if any query terms exist in the document
	hasMatch := false
	for _, term := range allQueryTerms {
		termFreq, exists := doc.WordFreq[term]
		if !exists {
			continue
		}

		idf, exists := s.idf[term]
		if !exists {
			continue
		}

		hasMatch = true
		numerator := float64(termFreq) * (s.k1 + 1)
		denominator := float64(termFreq) + s.k1*(1-s.b+s.b*docLen/s.avgdl)

		// Add length-based weight factor: (1 + 0.5 * (length - 1))
		// This gives more weight to longer n-grams while still keeping single terms relevant
		termLength := float64(len(strings.Fields(term)))
		lengthBoost := 1.0 + 0.5*(termLength-1)

		score += idf * numerator / denominator * lengthBoost
	}

	// Return 0 if no query terms were found in the document
	if !hasMatch {
		return 0
	}

	return score
}

func (s *BM25Scorer) calculateIDF() {
	N := float64(len(s.docs))

	for _, doc := range s.docs {
		for term := range doc.WordFreq {
			if _, exists := s.idf[term]; exists {
				continue
			}

			// Count documents containing the term
			var docCount float64
			for _, d := range s.docs {
				if _, has := d.WordFreq[term]; has {
					docCount++
				}
			}

			// Calculate IDF
			s.idf[term] = math.Log(1 + (N-docCount+0.5)/(docCount+0.5))
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
