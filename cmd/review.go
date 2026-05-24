package cmd

import (
	"fmt"
	"handshake-cli/internal/promptManager"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"handshake-cli/internal/tuiManager"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Perform an AI code review on your staged changes",
	RunE:  runReview,
}

var reviewProviderFlag string

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringVarP(&reviewProviderFlag, "provider", "p", "ollama", "AI provider to use (openrouter, ollama)")
}

func runReview(_ *cobra.Command, _ []string) error {
	prompts, err := promptManager.LoadPrompts()
	if err != nil {
		return fmt.Errorf("failed to load AI prompts: %w", err)
	}

	diffBytes, err := exec.Command("git", "diff", "--staged").Output()
	if err != nil || len(diffBytes) == 0 {
		return fmt.Errorf("no staged changes found — run 'git add' first")
	}

	model := tuiManager.NewReviewModel(string(diffBytes), reviewProviderFlag, prompts)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("UI error: %w", err)
	}

	return nil
}
