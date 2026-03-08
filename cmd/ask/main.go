package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/askcli/internal/cli"
	"github.com/askcli/internal/config"
	"github.com/askcli/internal/gemini"
	"github.com/askcli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showTokens bool
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "CLI utility for working with terminal via Google Gemini API",
	Long: `Ask - a utility that helps work with Linux terminal,
		using Google Gemini API capabilities to generate commands.
		Example usage:
  		ask "how to show last 10 executed commands?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAsk,
}

func init() {
	rootCmd.Flags().BoolVar(&showTokens, "show-tokens", false, "Show number of tokens used")
	rootCmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: ~/.config/ask/config.yaml)")
}

func runAsk(cmd *cobra.Command, args []string) error {
	// Determine alias
	aliasName := cli.GetAliasName()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Join arguments into one question
	question := strings.Join(args, " ")

	// Create Gemini client
	geminiClient := gemini.NewClient(cfg)

	// Send request
	answer, tokenUsage, err := geminiClient.Ask(question, cfg.Prompt.SystemMessage)
	if err != nil {
		return fmt.Errorf("error requesting Gemini API: %w", err)
	}

	// Determine whether to show tokens (from flag or config)
	displayTokens := showTokens || cfg.UI.ShowTokens

	// Display answer and wait for user action
	if err := ui.ShowAnswer(
		answer,
		displayTokens,
		tokenUsage.InputTokens,
		tokenUsage.OutputTokens,
		tokenUsage.TotalTokens,
		cfg.UI.CopyHint,
	); err != nil {
		return fmt.Errorf("error displaying answer: %w", err)
	}

	// Display alias information if it differs from "ask"
	if aliasName != "ask" && cfg.Alias.Enabled {
		fmt.Printf("\n(Run as: %s)\n", aliasName)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
