package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"internal-link/pkg/analyzer"
	"internal-link/pkg/markdown"
)

var (
	cfgFile    string
	dryRun     bool
	minScore   float64
	singleFile string
	cacheDir   string
	minNGram   int
	maxNGram   int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "internal-link [directory]",
	Short: "A tool for suggesting internal links in markdown files",
	Long: `internal-link analyzes markdown files in a directory and suggests
potential internal links based on content similarity using the BM25 algorithm.
It can analyze all files in a directory or focus on a single file.

The tool supports n-gram analysis, allowing you to find matches based on phrases
rather than just single words. Use --min-ngram to set the minimum n-gram length.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := args[0]

		// Set default cache directory if not specified
		if cacheDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			cacheDir = filepath.Join(home, ".cache", "internal-link")
		}

		// Create parser config
		parserConfig := markdown.ParserConfig{
			MinNGram: minNGram,
			MaxNGram: maxNGram,
		}

		config := analyzer.Config{
			MinScore:     minScore,
			DryRun:       dryRun,
			SingleFile:   singleFile,
			TargetDir:    targetDir,
			CacheDir:     cacheDir,
			ParserConfig: parserConfig,
		}

		a, err := analyzer.NewAnalyzer(config)
		if err != nil {
			return fmt.Errorf("failed to create analyzer: %w", err)
		}

		suggestions, err := a.Analyze()
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		// Print suggestions
		for _, s := range suggestions {
			fmt.Printf("File: %s\n", s.SourcePath)
			fmt.Printf("  Suggested link to: %s\n", s.TargetPath)
			fmt.Printf("  Score: %.4f\n", s.Score)
			if dryRun {
				fmt.Printf("  Context: %s\n", s.Context)
				fmt.Printf("  Phrase to link: %s\n", s.WordToLink)
			}
			fmt.Println()
		}

		if !dryRun {
			if err := a.ApplyChanges(suggestions); err != nil {
				return fmt.Errorf("failed to apply changes: %w", err)
			}
			fmt.Println("Successfully applied all suggested links")
		}

		return nil
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.internal-link.yaml)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show suggestions without making changes")
	rootCmd.Flags().Float64Var(&minScore, "min-score", 0.3, "minimum similarity score threshold")
	rootCmd.Flags().StringVar(&singleFile, "file", "", "analyze a single file against all others")
	rootCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "directory for caching analysis results")
	rootCmd.Flags().IntVar(&minNGram, "min-ngram", 2, "minimum number of words in phrases to match (e.g., 2 for bigrams)")
	rootCmd.Flags().IntVar(&maxNGram, "max-ngram", 3, "maximum number of words in phrases to match (e.g., 3 for trigrams)")

	viper.BindPFlag("dry-run", rootCmd.Flags().Lookup("dry-run"))
	viper.BindPFlag("min-score", rootCmd.Flags().Lookup("min-score"))
	viper.BindPFlag("file", rootCmd.Flags().Lookup("file"))
	viper.BindPFlag("cache-dir", rootCmd.Flags().Lookup("cache-dir"))
	viper.BindPFlag("min-ngram", rootCmd.Flags().Lookup("min-ngram"))
	viper.BindPFlag("max-ngram", rootCmd.Flags().Lookup("max-ngram"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".internal-link")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
