package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"internal-link/pkg/cache"
	"internal-link/pkg/markdown"
	"internal-link/pkg/scorer"
)

// Config holds the analyzer configuration
type Config struct {
	MinScore     float64
	DryRun       bool
	SingleFile   string
	TargetDir    string
	CacheDir     string
	ParserConfig markdown.ParserConfig
}

// Analyzer coordinates document analysis and link suggestions
type Analyzer struct {
	parser *markdown.Parser
	scorer scorer.Scorer
	cache  *cache.Cache
	config Config
	docs   map[string]*scorer.Document
}

// NewAnalyzer creates a new analyzer with the given configuration
func NewAnalyzer(config Config) (*Analyzer, error) {
	cache, err := cache.NewCache(config.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return &Analyzer{
		parser: markdown.NewParser(config.ParserConfig),
		scorer: scorer.NewBM25Scorer(config.ParserConfig.MaxNGram),
		cache:  cache,
		config: config,
		docs:   make(map[string]*scorer.Document),
	}, nil
}

// Analyze processes markdown files and generates link suggestions
func (a *Analyzer) Analyze() ([]scorer.LinkSuggestion, error) {
	// Load documents
	if err := a.loadDocuments(); err != nil {
		return nil, fmt.Errorf("failed to load documents: %w", err)
	}
	fmt.Println("Loaded ", len(a.docs), " documents")

	var suggestions []scorer.LinkSuggestion

	// If analyzing a single file
	if a.config.SingleFile != "" {
		fmt.Println("Analyzing single file: ", a.config.SingleFile)
		doc, exists := a.docs[a.config.SingleFile]
		if !exists {
			return nil, fmt.Errorf("file %s not found", a.config.SingleFile)
		}
		return a.analyzeSingleDocument(doc)
	}

	// Analyze all documents
	for _, doc := range a.docs {
		docSuggestions, err := a.analyzeSingleDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze %s: %w", doc.Path, err)
		}
		suggestions = append(suggestions, docSuggestions...)
	}

	return suggestions, nil
}

// loadDocuments reads and processes all markdown files
func (a *Analyzer) loadDocuments() error {
	return filepath.Walk(a.config.TargetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// Try to get from cache first
		cached, err := a.cache.Get(path)
		if err != nil {
			return fmt.Errorf("failed to check cache for %s: %w", path, err)
		}

		var wordFreq map[string]int

		if cached != nil {
			wordFreq = cached.WordFreq
		} else {
			fmt.Println("Parsing file: ", path)
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			wordFreq, err = a.parser.ParseContent(content)
			if err != nil {
				return fmt.Errorf("failed to parse file %s: %w", path, err)
			}

			// Cache the results
			if err := a.cache.Set(path, wordFreq); err != nil {
				return fmt.Errorf("failed to cache results for %s: %w", path, err)
			}
		}

		doc := &scorer.Document{
			Path:     path,
			WordFreq: wordFreq,
		}

		if err := a.scorer.ProcessDocument(doc); err != nil {
			return fmt.Errorf("failed to process document %s: %w", path, err)
		}

		a.docs[path] = doc
		return nil
	})
}

// analyzeSingleDocument generates link suggestions for a single document
func (a *Analyzer) analyzeSingleDocument(doc *scorer.Document) ([]scorer.LinkSuggestion, error) {
	var suggestions []scorer.LinkSuggestion

	// Read the document content
	content, err := os.ReadFile(doc.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", doc.Path, err)
	}

	// Find word occurrences in the document
	occurrences, err := a.parser.FindWordOccurrences(content, 3) // Skip words shorter than 3 chars
	if err != nil {
		return nil, fmt.Errorf("failed to find word occurrences in %s: %w", doc.Path, err)
	}

	// Group occurrences by word
	wordOccurrences := make(map[string][]markdown.WordOccurrence)
	for _, occ := range occurrences {
		wordOccurrences[occ.Word] = append(wordOccurrences[occ.Word], occ)
	}

	// Check each target document for potential links
	positionSuggestions := make(map[int]scorer.LinkSuggestion)

	for targetPath, targetDoc := range a.docs {
		if targetPath == doc.Path {
			continue
		}

		score := a.scorer.Score(string(content), targetDoc)
		if score >= a.config.MinScore {
			// Find the best word to link based on frequency and presence in target
			var bestOccurrence *markdown.WordOccurrence
			var maxFreq int

			for word, occs := range wordOccurrences {
				if freq, exists := targetDoc.WordFreq[word]; exists && freq > maxFreq {
					maxFreq = freq
					// Use the first occurrence of the most frequent matching word
					bestOccurrence = &occs[0]
				}
			}

			if bestOccurrence != nil {
				suggestion := scorer.LinkSuggestion{
					SourcePath: doc.Path,
					TargetPath: targetPath,
					Score:      score,
					WordToLink: bestOccurrence.Word,
					Position:   bestOccurrence.Position,
					Context:    bestOccurrence.Context,
				}

				// Only keep the suggestion if it has a higher score than any existing one at this position
				if existing, exists := positionSuggestions[bestOccurrence.Position]; !exists || suggestion.Score > existing.Score {
					positionSuggestions[bestOccurrence.Position] = suggestion
				}
			}
		}
	}

	// Convert map to slice
	suggestions = make([]scorer.LinkSuggestion, 0, len(positionSuggestions))
	for _, suggestion := range positionSuggestions {
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// ApplyChanges applies the suggested changes to the documents
func (a *Analyzer) ApplyChanges(suggestions []scorer.LinkSuggestion) error {
	if a.config.DryRun {
		return nil
	}

	for _, suggestion := range suggestions {
		content, err := os.ReadFile(suggestion.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", suggestion.SourcePath, err)
		}

		newContent, err := a.parser.InsertLink(content, suggestion.WordToLink, suggestion.TargetPath, suggestion.Position)
		if err != nil {
			return fmt.Errorf("failed to insert link in %s: %w", suggestion.SourcePath, err)
		}

		if err := os.WriteFile(suggestion.SourcePath, newContent, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", suggestion.SourcePath, err)
		}
	}

	return nil
}
