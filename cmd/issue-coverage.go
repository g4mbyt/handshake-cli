package cmd

import (
	"fmt"
	"handshake-cli/internal/promptManager"
	"handshake-cli/internal/tuiManager"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var coverageCmd = &cobra.Command{
	Use: "coverage",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkDependencies(); err != nil {
			return err
		}

		issueBody, err := getIssueBody()
		if err != nil {
			return err
		}

		gitDiff, err := getGitDiff()
		if err != nil {
			return err
		}

		prompts, err := promptManager.LoadPrompts()
		if err != nil {
			return fmt.Errorf("error loading prompts: %v", err)
		}

		model := tuiManager.NewCoverageModel(issueBody, gitDiff, coverageProviderFlag, prompts)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("UI error: %w", err)
		}

		return nil
	},
}

var coverageProviderFlag string
var issueFlag string

func checkDependencies() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("GitHub CLI ('gh') is not installed or not found in PATH")
	}

	_, err = exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("Git ('git') is not installed or not found in PATH")
	}

	return nil
}

func extractIssueNumber(branchName string) string {
	reNamedIssue := regexp.MustCompile(`(?i)issue-(\d+)`)
	if matches := reNamedIssue.FindStringSubmatch(branchName); len(matches) > 1 {
		return matches[1]
	}

	reJira := regexp.MustCompile(`(?i)[a-z]+-(\d+)`)
	if matches := reJira.FindStringSubmatch(branchName); len(matches) > 1 {
		return matches[1]
	}

	reSlash := regexp.MustCompile(`\/(\d+)`)
	if matches := reSlash.FindStringSubmatch(branchName); len(matches) > 1 {
		return matches[1]
	}

	reLead := regexp.MustCompile(`^(\d+)-`)
	if matches := reLead.FindStringSubmatch(branchName); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func getIssueBody() (string, error) {
	var issueNumber string
	if issueFlag != "" {
		issueNumber = issueFlag
	} else {
		currentBranch := exec.Command("git", "branch", "--show-current")
		outputBytes, err := currentBranch.Output()

		if err != nil {
			return "", fmt.Errorf("failed to get git branch")
		}

		rawString := string(outputBytes)
		branchName := strings.TrimSpace(rawString)
		issueNumber = extractIssueNumber(branchName)
	}

	if issueNumber == "" {
		return "", fmt.Errorf("no issue number found in branch name")
	}

	outputBytes, err := exec.Command("gh", "issue", "view", issueNumber, "--json", "body", "--jq", ".body").Output()
	if err != nil {
		return "", fmt.Errorf("error fetching issue text from GitHub")
	}

	return string(outputBytes), nil
}

func getGitDiff() (string, error) {
	diffCmd := exec.Command("git", "diff", "--staged")
	diffBytes, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("error reading git diff")
	}
	gitDiff := string(diffBytes)

	if gitDiff == "" {
		diffCmd = exec.Command("git", "diff")
		diffBytes, err = diffCmd.Output()
		if err != nil {
			return "", fmt.Errorf("error reading git diff")
		}
		gitDiff = string(diffBytes)
	}

	if gitDiff == "" {
		return "", fmt.Errorf("no code changes found")
	}

	return gitDiff, nil
}

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.Flags().StringVarP(&coverageProviderFlag, "provider", "p", "ollama", "AI provider to use (openrouter, ollama)")
	coverageCmd.Flags().StringVar(&issueFlag, "issue", "", "Issue number to analyze against (e.g. 123)")
}
