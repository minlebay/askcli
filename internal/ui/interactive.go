package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// ShowAnswer displays the answer and token information, then waits for user action
func ShowAnswer(answer string, showTokens bool, inputTokens, outputTokens, totalTokens int, copyHint string) error {
	// Display answer
	fmt.Println()
	fmt.Println("Answer:")
	fmt.Println(answer)

	// Display token information if needed
	if showTokens {
		fmt.Println()
		fmt.Printf("Tokens: input %d / output %d / total %d\n", inputTokens, outputTokens, totalTokens)
	}

	// Show hint
	fmt.Println()
	fmt.Print(copyHint + " ")

	// Switch terminal to raw mode for reading a single character
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// If failed to switch to raw mode, use regular input
		return readInputFallback(answer, showTokens, inputTokens, outputTokens, totalTokens, copyHint)
	}
	defer term.Restore(fd, oldState)

	// Read one byte (character) - in raw mode character won't be displayed automatically
	var b [1]byte
	_, err = os.Stdin.Read(b[:])
	if err != nil {
		term.Restore(fd, oldState)
		return fmt.Errorf("error reading input: %w", err)
	}

	// Restore terminal before output
	term.Restore(fd, oldState)

	// Clear hint line and move to new line
	fmt.Print("\r")                                 // Return to beginning of line
	fmt.Print(strings.Repeat(" ", len(copyHint)+1)) // Clear line with spaces
	fmt.Print("\r")                                 // Back to beginning
	fmt.Println()                                   // New line

	// Process command
	char := strings.ToUpper(string(b[0]))
	switch char {
	case "C":
		// Copy to clipboard using system utilities
		// Ensure we copy only clean answer without extra characters
		cleanAnswer := strings.TrimSpace(answer)
		if err := copyToClipboard(cleanAnswer); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✓ Command copied to clipboard")
	case "Q":
		// Exit (do nothing, just finish)
		// Don't output anything, just exit
	default:
		// Any other key - just exit without message
	}

	return nil
}

// copyToClipboard copies text to clipboard using system utilities
func copyToClipboard(text string) error {
	// Try to use xclip (X11)
	if cmd := exec.Command("xclip", "-selection", "clipboard"); cmd != nil {
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Try to use wl-copy (Wayland)
	if cmd := exec.Command("wl-copy"); cmd != nil {
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Try to use xsel (alternative for X11)
	if cmd := exec.Command("xsel", "--clipboard", "--input"); cmd != nil {
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("clipboard utility not found (xclip, wl-copy or xsel). Please install one of them")
}

// readInputFallback is used if failed to switch terminal to raw mode
func readInputFallback(answer string, showTokens bool, inputTokens, outputTokens, totalTokens int, copyHint string) error {
	// Use regular input via fmt.Scanln
	var input string
	fmt.Scanln(&input)

	input = strings.TrimSpace(strings.ToUpper(input))

	// Process command
	switch input {
	case "C":
		// Copy to clipboard using system utilities
		// Ensure we copy only clean answer without extra characters
		cleanAnswer := strings.TrimSpace(answer)
		if err := copyToClipboard(cleanAnswer); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✓ Command copied to clipboard")
	case "Q":
		// Exit
	default:
		// Any other key - just exit
	}

	return nil
}
