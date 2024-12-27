# internal-link

A CLI tool that analyzes markdown files and suggests internal linking opportunities using BM25 algorithm.

## Features

- Recursive markdown file scanning
- Smart internal link suggestions using BM25 algorithm
- Lexical and semantic analysis for high-quality linking
- Fast performance with caching
- Dry-run mode with scoring insights
- Configurable scoring threshold

## Installation

```bash
go install github.com/yourusername/internal-link/cmd/internal-link@latest
```

## Usage

```bash
# Analyze all markdown files in a directory
internal-link analyze /path/to/markdown/folder

# Analyze a single file against all others
internal-link analyze --file single.md /path/to/markdown/folder

# Dry run mode
internal-link analyze --dry-run /path/to/markdown/folder

# Set custom threshold
internal-link analyze --threshold 0.5 /path/to/markdown/folder
```

## Development

Requirements:
- Go 1.21 or higher

Setup:
```bash
git clone https://github.com/yourusername/internal-link
cd internal-link
go mod download
```

Run tests:
```bash
go test ./...
```

## License

MIT 
