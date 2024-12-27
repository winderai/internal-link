package markdown

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Common English function/grammatical words to skip
var functionWords = map[string]bool{
	// Articles
	"a": true, "an": true, "the": true,

	// Conjunctions
	"and": true, "but": true, "or": true, "nor": true, "for": true,
	"yet": true, "so": true, "because": true, "if": true, "unless": true,
	"while": true, "where": true, "when": true, "whether": true,

	// Pronouns
	"i": true, "you": true, "he": true, "she": true, "it": true,
	"we": true, "they": true, "me": true, "him": true, "her": true,
	"us": true, "them": true, "my": true, "your": true, "his": true,
	"its": true, "our": true, "their": true, "mine": true, "yours": true,
	"hers": true, "ours": true, "theirs": true, "this": true, "that": true,
	"these": true, "those": true, "who": true, "whom": true, "whose": true,
	"which": true, "what": true,

	// Auxiliary verbs
	"am": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true,
	"had": true, "having": true, "do": true, "does": true, "did": true,
	"doing": true, "will": true, "would": true, "shall": true, "should": true,
	"may": true, "might": true, "must": true, "can": true, "could": true,

	// Other common function words
	"there": true, "here": true, "now": true, "then": true, "today": true,
	"tomorrow": true, "yesterday": true, "not": true, "no": true, "yes": true,
	"okay": true, "oh": true, "well": true, "just": true, "very": true,
	"much": true, "many": true, "more": true, "most": true, "some": true,
	"any": true, "all": true, "both": true, "each": true, "few": true,
	"several": true, "too": true, "rather": true, "quite": true,
}

// WordOccurrence represents a word's location in the document
type WordOccurrence struct {
	Word     string
	Position int
	Context  string
}

// Parser handles markdown document parsing and manipulation
type Parser struct {
	md       goldmark.Markdown
	minNGram int
	maxNGram int
}

// ParserConfig holds configuration for the parser
type ParserConfig struct {
	MinNGram int // Minimum number of words in n-grams
	MaxNGram int // Maximum number of words in n-grams
}

// NewParser creates a new markdown parser
func NewParser(config ParserConfig) *Parser {
	if config.MinNGram < 1 {
		config.MinNGram = 1 // Default to unigrams if not specified
	}
	return &Parser{
		md:       goldmark.New(),
		minNGram: config.MinNGram,
		maxNGram: config.MaxNGram,
	}
}

// generateNGrams generates n-grams of exactly the specified length
func generateNGrams(words []string, n int) []string {
	if n <= 0 || len(words) < n {
		return words
	}

	var ngrams []string
	for i := 0; i <= len(words)-n; i++ {
		ngram := strings.Join(words[i:i+n], " ")
		ngrams = append(ngrams, ngram)
	}
	return ngrams
}

// isSignificantWord checks if a word carries lexical meaning
func isSignificantWord(word string) bool {
	// Skip function words
	if functionWords[word] {
		return false
	}

	// Skip words that are just numbers
	if strings.IndexFunc(word, func(r rune) bool { return !strings.ContainsRune("0123456789", r) }) == -1 {
		return false
	}

	return true
}

// skipFrontmatter returns the content without frontmatter and the number of bytes skipped
func (p *Parser) skipFrontmatter(content []byte) ([]byte, int) {
	// Check for YAML/TOML frontmatter
	if len(content) > 3 && (bytes.HasPrefix(content, []byte("---")) || bytes.HasPrefix(content, []byte("+++"))) {
		rest := content[3:]

		// Find first newline after opening delimiter
		if nl := bytes.IndexByte(rest, '\n'); nl != -1 {
			rest = rest[nl+1:]

			// Find the closing delimiter
			var delimiter []byte
			if bytes.HasPrefix(content, []byte("---")) {
				delimiter = []byte("---")
			} else {
				delimiter = []byte("+++")
			}

			if idx := bytes.Index(rest, delimiter); idx != -1 {
				// Find the newline after the closing delimiter
				closeRest := rest[idx+3:]
				if nl := bytes.IndexByte(closeRest, '\n'); nl != -1 {
					skipped := len(content) - len(closeRest[nl+1:])
					return closeRest[nl+1:], skipped
				}
			}
		}
	}
	return content, 0
}

// processTextNode extracts text from a text node and writes it to the buffer
func (p *Parser) processTextNode(text *ast.Text, content []byte, buf *bytes.Buffer) {
	segment := text.Segment
	buf.Write(segment.Value(content))
	buf.WriteRune(' ')
}

// walkTextNodes walks through nodes recursively and processes text nodes
func (p *Parser) walkTextNodes(n ast.Node, content []byte, buf *bytes.Buffer) ast.WalkStatus {
	switch n.Kind() {
	case ast.KindText:
		if text, ok := n.(*ast.Text); ok {
			p.processTextNode(text, content, buf)
		}
	case ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindCodeSpan:
		return ast.WalkSkipChildren
	default:
		// For all other nodes (including headings), process their children
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if status := p.walkTextNodes(child, content, buf); status != ast.WalkContinue {
				return status
			}
		}
	}
	return ast.WalkContinue
}

// ParseContent parses markdown content and returns a map of word/n-gram frequencies
func (p *Parser) ParseContent(content []byte) (map[string]int, error) {
	// Use FindWordOccurrences to get all word/n-gram occurrences
	occurrences, err := p.FindWordOccurrences(content, 1) // minWordLen=1 since we'll filter later
	if err != nil {
		return nil, err
	}

	// Convert occurrences to frequency map
	wordFreq := make(map[string]int)
	for _, occ := range occurrences {
		wordFreq[occ.Word]++
	}

	return wordFreq, nil
}

// processTextNodeWithPosition processes a text node and adds word occurrences to the slice
func (p *Parser) processTextNodeWithPosition(text *ast.Text, content []byte, currentPosition int, frontmatterOffset int, minWordLen int, occurrences *[]WordOccurrence) {
	textContent := text.Segment.Value(content)
	textStr := string(textContent)

	// Get all words and their positions
	words := strings.Fields(textStr)
	if len(words) == 0 {
		return
	}

	// Create normalized words and track their positions
	var significantWords []string
	var significantWordPositions []int
	pos := 0

	for i, word := range words {
		normalized := strings.ToLower(strings.Trim(word, ".,!?()[]{}\"'"))

		// Skip numbers and function words
		if strings.IndexFunc(normalized, func(r rune) bool { return !strings.ContainsRune("0123456789", r) }) == -1 {
			continue
		}
		if functionWords[normalized] {
			continue
		}
		if len(normalized) <= 2 {
			continue
		}

		// Find the exact position of the word in the original text
		wordPos := -1
		if i == 0 {
			wordPos = bytes.Index(textContent, []byte(word))
		} else {
			wordPos = bytes.Index(textContent[pos:], []byte(word))
			if wordPos != -1 {
				wordPos += pos
			}
		}
		if wordPos != -1 {
			pos = wordPos + len(word)
			significantWords = append(significantWords, normalized)
			significantWordPositions = append(significantWordPositions, wordPos)
		}
	}

	// If no significant words found, return early
	if len(significantWords) == 0 {
		return
	}

	// For single words (unigrams)
	if p.minNGram == 1 {
		for i, word := range significantWords {
			if len(word) >= minWordLen {
				absPos := frontmatterOffset + currentPosition + significantWordPositions[i]
				context := p.extractContext(content, currentPosition+significantWordPositions[i], len(words[i]))
				*occurrences = append(*occurrences, WordOccurrence{
					Word:     word,
					Position: absPos,
					Context:  context,
				})
			}
		}
		return
	}

	// For n-grams
	if len(significantWords) >= p.minNGram {
		// Generate n-grams for each length between minNGram and maxNGram
		for n := p.minNGram; n <= p.maxNGram && n <= len(significantWords); n++ {
			for i := 0; i <= len(significantWords)-n; i++ {
				ngramWords := significantWords[i : i+n]
				ngram := strings.Join(ngramWords, " ")

				startPos := significantWordPositions[i]
				endWordIdx := i + n - 1
				endPos := significantWordPositions[endWordIdx] + len(words[endWordIdx])
				absPos := frontmatterOffset + currentPosition + startPos

				context := p.extractContext(content, currentPosition+startPos, endPos-startPos)
				*occurrences = append(*occurrences, WordOccurrence{
					Word:     ngram,
					Position: absPos,
					Context:  context,
				})
			}
		}
	}
}

// walkNodesWithPosition walks through nodes recursively and processes text nodes with position tracking
func (p *Parser) walkNodesWithPosition(n ast.Node, content []byte, currentPosition *int, frontmatterOffset int, minWordLen int, occurrences *[]WordOccurrence) ast.WalkStatus {
	// Process text nodes
	if text, ok := n.(*ast.Text); ok {
		// Use the original segment position to maintain correct offsets
		segmentStart := text.Segment.Start
		p.processTextNodeWithPosition(text, content, segmentStart, frontmatterOffset, minWordLen, occurrences)
		*currentPosition = text.Segment.Stop
	}

	// Recurse through all children
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		p.walkNodesWithPosition(child, content, currentPosition, frontmatterOffset, minWordLen, occurrences)
	}

	return ast.WalkContinue
}

// FindWordOccurrences finds all occurrences of words and n-grams in the document
func (p *Parser) FindWordOccurrences(content []byte, minWordLen int) ([]WordOccurrence, error) {
	content, frontmatterOffset := p.skipFrontmatter(content)
	reader := text.NewReader(content)
	doc := p.md.Parser().Parse(reader)

	var occurrences []WordOccurrence
	currentPosition := 0

	// Process the entire document tree
	p.walkNodesWithPosition(doc, content, &currentPosition, frontmatterOffset, minWordLen, &occurrences)

	// Sort occurrences by position to ensure consistent order
	sort.Slice(occurrences, func(i, j int) bool {
		return occurrences[i].Position < occurrences[j].Position
	})

	return occurrences, nil
}

// extractContext extracts surrounding context for a word
func (p *Parser) extractContext(content []byte, position, wordLen int) string {
	// Define context window size (characters before and after the word)
	const contextSize = 50

	start := position - contextSize
	if start < 0 {
		start = 0
	}

	end := position + wordLen + contextSize
	if end > len(content) {
		end = len(content)
	}

	// Extract context and clean it up
	context := content[start:end]

	// Replace newlines and multiple spaces with single spaces
	context = bytes.ReplaceAll(context, []byte{'\n'}, []byte{' '})
	context = bytes.Join(bytes.Fields(context), []byte{' '})

	// Add ellipsis if context is truncated
	var result bytes.Buffer
	if start > 0 {
		result.WriteString("...")
	}
	result.Write(context)
	if end < len(content) {
		result.WriteString("...")
	}

	return result.String()
}

// InsertLink inserts a markdown link at the specified position
func (p *Parser) InsertLink(content []byte, word string, target string, position int) ([]byte, error) {
	if position < 0 || position >= len(content) {
		return nil, fmt.Errorf("position %d is out of range for content length %d", position, len(content))
	}

	// For multi-word phrases, we need to match the exact phrase
	if position+len(word) > len(content) {
		return nil, fmt.Errorf("word '%s' at position %d would exceed content length %d", word, position, len(content))
	}

	// Verify the word matches at the position
	actualWord := string(content[position : position+len(word)])
	if actualWord != word {
		return nil, fmt.Errorf("word at position %d is '%s', not '%s'", position, actualWord, word)
	}

	// Create the link
	link := []byte(fmt.Sprintf("[%s](%s)", word, target))

	// Construct the result
	result := make([]byte, 0, len(content)+len(link)-len(word))
	result = append(result, content[:position]...)
	result = append(result, link...)
	result = append(result, content[position+len(word):]...)

	return result, nil
}
