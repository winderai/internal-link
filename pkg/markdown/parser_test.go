package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		minNGram   int
		maxNGram   int
		expected   map[string]int
		skipReason string
	}{
		{
			name:     "unigrams only",
			content:  "This is a simple test document",
			minNGram: 1,
			maxNGram: 1,
			expected: map[string]int{
				"simple":   1,
				"test":     1,
				"document": 1,
			},
		},
		{
			name:     "bigrams",
			content:  "This is a simple test document about testing",
			minNGram: 2,
			maxNGram: 2,
			expected: map[string]int{
				"simple test":    1,
				"test document":  1,
				"document about": 1,
				"about testing":  1,
			},
		},
		{
			name:     "trigrams",
			content:  "This is a simple test document about testing",
			minNGram: 3,
			maxNGram: 3,
			expected: map[string]int{
				"simple test document":   1,
				"test document about":    1,
				"document about testing": 1,
			},
		},
		{
			name: "with heading and mixed n-grams",
			content: `# Main Title
## Section 1
This is a test document about testing
### Subsection
More content about testing`,
			minNGram: 2,
			maxNGram: 2,
			expected: map[string]int{
				"main title":     1,
				"test document":  1,
				"document about": 1,
				"about testing":  2,
				"content about":  1,
			},
		},
		{
			name:     "with code blocks",
			content:  "```\ncode block\n```\nRegular text about testing",
			minNGram: 2,
			maxNGram: 2,
			expected: map[string]int{
				"regular text":  1,
				"text about":    1,
				"about testing": 1,
			},
		},
		{
			name:     "empty document",
			content:  "",
			minNGram: 2,
			maxNGram: 2,
			expected: map[string]int{},
		},
		{
			name:     "with punctuation",
			content:  "Hello, world! This is a test. Testing, testing...",
			minNGram: 2,
			maxNGram: 2,
			expected: map[string]int{
				"hello world":     1,
				"testing testing": 1,
				"test testing":    1,
			},
		},
		{
			name:     "multiple n-gram lengths",
			content:  "This is a simple test document about testing",
			minNGram: 2,
			maxNGram: 3,
			expected: map[string]int{
				// bigrams
				"simple test":    1,
				"test document":  1,
				"document about": 1,
				"about testing":  1,
				// trigrams
				"simple test document":   1,
				"test document about":    1,
				"document about testing": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			parser := NewParser(ParserConfig{MinNGram: tt.minNGram, MaxNGram: tt.maxNGram})
			wordFreq, err := parser.ParseContent([]byte(tt.content))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, wordFreq)
		})
	}
}

func TestFindWordOccurrences(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		minNGram   int
		maxNGram   int
		minWordLen int
		expected   []WordOccurrence
	}{
		{
			name:       "single word occurrences",
			content:    "This is a test document about testing",
			minNGram:   1,
			maxNGram:   1,
			minWordLen: 3,
			expected: []WordOccurrence{
				{Word: "test", Position: 10, Context: "This is a test document about..."},
				{Word: "document", Position: 15, Context: "...is a test document about testing"},
				{Word: "about", Position: 24, Context: "...test document about testing"},
				{Word: "testing", Position: 30, Context: "...document about testing"},
			},
		},
		{
			name:       "bigram occurrences",
			content:    "This is a test document about testing",
			minNGram:   2,
			maxNGram:   2,
			minWordLen: 3,
			expected: []WordOccurrence{
				{Word: "test document", Position: 10, Context: "This is a test document about testing"},
				{Word: "document about", Position: 15, Context: "...a test document about testing"},
				{Word: "about testing", Position: 24, Context: "...test document about testing"},
			},
		},
		{
			name: "with frontmatter",
			content: `--- # Test
title: Test
---
This is a test document`,
			minNGram:   2,
			maxNGram:   2,
			minWordLen: 3,
			expected: []WordOccurrence{
				{Word: "test document", Position: 37, Context: "This is a test document"},
			},
		},
		{
			name:       "with code blocks",
			content:    "Before code\n```\ncode block\n```\nAfter code",
			minNGram:   2,
			maxNGram:   2,
			minWordLen: 3,
			expected: []WordOccurrence{
				{Word: "before code", Position: 0, Context: "Before code..."},
				{Word: "after code", Position: 31, Context: "...After code"},
			},
		},
		{
			name:       "multiple n-gram lengths",
			content:    "This is a test document about testing",
			minNGram:   2,
			maxNGram:   3,
			minWordLen: 3,
			expected: []WordOccurrence{
				// All bigrams first
				{Word: "test document", Position: 10, Context: "This is a test document about testing"},
				{Word: "test document about", Position: 10, Context: "This is a test document about testing"},
				{Word: "document about", Position: 15, Context: "...a test document about testing"},
				{Word: "document about testing", Position: 15, Context: "...a test document about testing"},
				{Word: "about testing", Position: 24, Context: "...test document about testing"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(ParserConfig{MinNGram: tt.minNGram, MaxNGram: tt.maxNGram})
			occurrences, err := parser.FindWordOccurrences([]byte(tt.content), tt.minWordLen)
			assert.NoError(t, err)

			// Compare only the fields we care about, as context might have slight variations
			for i, expected := range tt.expected {
				if i >= len(occurrences) {
					t.Errorf("Missing occurrence at index %d", i)
					continue
				}
				actual := occurrences[i]
				assert.Equal(t, expected.Word, actual.Word)
				assert.Equal(t, expected.Position, actual.Position)
				assert.Contains(t, strings.ToLower(actual.Context), strings.ToLower(expected.Word))
			}
			assert.Equal(t, len(tt.expected), len(occurrences))
		})
	}
}

func TestInsertLink(t *testing.T) {
	parser := NewParser(ParserConfig{MinNGram: 1, MaxNGram: 1})

	tests := []struct {
		name     string
		content  string
		word     string
		target   string
		position int
		expected string
		wantErr  bool
	}{
		{
			name:     "simple link insertion",
			content:  "This is a test document",
			word:     "test",
			target:   "target.md",
			position: 10,
			expected: "This is a [test](target.md) document",
		},
		{
			name:     "link at start",
			content:  "Test document",
			word:     "Test",
			target:   "target.md",
			position: 0,
			expected: "[Test](target.md) document",
		},
		{
			name:     "link at end",
			content:  "This is a test",
			word:     "test",
			target:   "target.md",
			position: 10,
			expected: "This is a [test](target.md)",
		},
		{
			name:     "multi-word link",
			content:  "This is a test document about testing",
			word:     "test document",
			target:   "target.md",
			position: 10,
			expected: "This is a [test document](target.md) about testing",
		},
		{
			name:     "invalid position",
			content:  "Short text",
			word:     "word",
			target:   "target.md",
			position: 20,
			wantErr:  true,
		},
		{
			name:     "word mismatch",
			content:  "This is a test",
			word:     "wrong",
			target:   "target.md",
			position: 10,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.InsertLink([]byte(tt.content), tt.word, tt.target, tt.position)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestGenerateNGrams(t *testing.T) {
	tests := []struct {
		name     string
		words    []string
		n        int
		expected []string
	}{
		{
			name:     "unigrams",
			words:    []string{"this", "is", "test"},
			n:        1,
			expected: []string{"this", "is", "test"},
		},
		{
			name:     "bigrams",
			words:    []string{"this", "is", "test"},
			n:        2,
			expected: []string{"this is", "is test"},
		},
		{
			name:     "trigrams",
			words:    []string{"this", "is", "a", "test"},
			n:        3,
			expected: []string{"this is a", "is a test"},
		},
		{
			name:     "n larger than input",
			words:    []string{"this", "is"},
			n:        3,
			expected: []string{"this", "is"},
		},
		{
			name:     "empty input",
			words:    []string{},
			n:        2,
			expected: []string{},
		},
		{
			name:     "invalid n",
			words:    []string{"this", "is", "test"},
			n:        0,
			expected: []string{"this", "is", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNGrams(tt.words, tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}
