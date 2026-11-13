package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/askcli/internal/cli"
	"github.com/askcli/internal/config"
	"github.com/askcli/internal/gemini"
	"github.com/askcli/internal/kwallet"
	"github.com/askcli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage the Gemini API key in KWallet",
}

var keySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Store the Gemini API key in KWallet",
	Args:  cobra.NoArgs,
	RunE:  runKeySet,
}

var keyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the Gemini API key is stored in KWallet",
	Args:  cobra.NoArgs,
	RunE:  runKeyStatus,
}

var keyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove the Gemini API key from KWallet",
	Args:  cobra.NoArgs,
	RunE:  runKeyDelete,
}

func init() {
	rootCmd.Flags().BoolVar(&showTokens, "show-tokens", false, "Show number of tokens used")
	rootCmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: ~/.config/ask/config.yaml)")

	keyCmd.AddCommand(keySetCmd, keyStatusCmd, keyDeleteCmd)
	rootCmd.AddCommand(keyCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	// Determine alias
	aliasName := cli.GetAliasName()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// The key lives in KWallet only; it is not read from the environment.
	apiKey, err := kwallet.New().Get(cmd.Context())
	if err != nil {
		if errors.Is(err, kwallet.ErrNotFound) {
			return errors.New("Gemini API key not found in KWallet. Run 'ask key set' to store it")
		}
		return fmt.Errorf("error reading Gemini API key: %w", err)
	}

	// Join arguments into one question
	question := strings.Join(args, " ")

	// Create Gemini client
	geminiClient := gemini.NewClient(cfg, apiKey)

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

func runKeySet(cmd *cobra.Command, _ []string) error {
	key, err := readKey()
	if err != nil {
		return err
	}

	if err := kwallet.New().Set(cmd.Context(), key); err != nil {
		return err
	}

	fmt.Println("Gemini API key saved to KWallet.")
	return nil
}

func runKeyStatus(cmd *cobra.Command, _ []string) error {
	_, err := kwallet.New().Get(cmd.Context())
	switch {
	case err == nil:
		fmt.Println("Gemini API key: found in KWallet")
	case errors.Is(err, kwallet.ErrNotFound):
		fmt.Println("Gemini API key: not found in KWallet")
		fmt.Println("Run 'ask key set' to store it.")
	default:
		return err
	}
	return nil
}

func runKeyDelete(cmd *cobra.Command, _ []string) error {
	if err := kwallet.New().Delete(cmd.Context()); err != nil {
		return err
	}
	fmt.Println("Gemini API key removed from KWallet.")
	return nil
}

// readKey reads a key without echoing it and without taking it from argv.
func readKey() (string, error) {
	fd := int(os.Stdin.Fd())

	var data []byte
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Gemini API key: ")
		key, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("could not read API key: %w", err)
		}
		data = key
	} else {
		key, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("could not read API key: %w", err)
		}
		data = key
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", errors.New("API key is empty")
	}
	if strings.ContainsAny(key, "\r\n") {
		return "", errors.New("API key must be a single line")
	}
	return key, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
