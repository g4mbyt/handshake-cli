package cmd

import (
	"fmt"
	"handshake-cli/internal/aiManager"
	"handshake-cli/internal/promptManager"
	"handshake-cli/internal/versionControlManager"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var coverageCmd = &cobra.Command{
	Use: "coverage",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !checkDependencies() {
			return nil
		}

		issueBody := getIssueBody()
		if issueBody == "" {
			return nil
		}

		gitDiff := getGitDiff()
		if gitDiff == "" {
			return nil
		}

		prompts, err := promptManager.LoadPrompts()
		if err != nil {
			return fmt.Errorf("error loading prompts: %v", err)
		}

		var provider aiManager.AIProvider
		switch coverageProviderFlag {
		case "ollama":
			provider, err = aiManager.NewOllamaAdapter("qwen2.5-coder:1.5b", "", prompts)
		default:
			provider, err = aiManager.NewOpenRouterAdapter("google/gemini-2.5-pro", prompts)
		}

		if err != nil {
			return fmt.Errorf("error initializing AI provider: %v", err)
		}

		fmt.Println("Analyzing requirements coverage...")
		coverage, err := provider.AnalyzeCoverage(cmd.Context(), issueBody, gitDiff)
		if err != nil {
			return fmt.Errorf("error analyzing coverage: %v", err)
		}

		fmt.Println("\n=== Requirements Coverage Report ===")
		fmt.Println(coverage)
		return nil
	},
}

var coverageProviderFlag string
var issueFlag string

func checkDependencies() bool {
	_, err := exec.LookPath("gh")
	if err != nil {
		fmt.Println("⚠️  Warning: GitHub CLI ('gh') is not installed or is not found in your PATH.")
		fmt.Println("Skipping Requirements Coverage analysis. Install 'gh' to enable this feature.")
		return false
	}

	_, err = exec.LookPath("git")
	if err != nil {
		fmt.Println("⚠️  Warning: Git('git') is not installed or is not found in your PATH.")
		fmt.Println("Skipping Requirements Coverage analysis. Install 'git' to enable this feature.")
		return false
	}

	return true
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

func getIssueBody() string {
	var issueNumber string
	if issueFlag != "" {
		issueNumber = issueFlag
	} else {
		currentBranch := exec.Command("git", "branch", "--show-current")
		outputBytes, err := currentBranch.Output()

		if err != nil {
			fmt.Println("Error: Failed to get git branch. Are you in a git repository?")
			return ""
		}

		rawString := string(outputBytes)
		branchName := strings.TrimSpace(rawString)
		issueNumber = extractIssueNumber(branchName)
	}

	if issueNumber == "" {
		fmt.Println("No issue number found in branch name. Please provide it via --issue flag.")
		return ""
	}

	var tracker versionControlManager.IssueTracker = &versionControlManager.GithubAdapter{}
	issuetext, err := tracker.GetIssueText(issueNumber)

	if err != nil {
		fmt.Printf("Error fetching issue text: %v\n", err)
		return ""
	}

	return issuetext
}

func getGitDiff() string {
	diffCmd := exec.Command("git", "diff", "--staged")
	diffBytes, err := diffCmd.Output()
	if err != nil {
		fmt.Println("Error reading git diff")
		return ""
	}
	gitDiff := string(diffBytes)

	if gitDiff == "" {
		diffCmd = exec.Command("git", "diff")
		diffBytes, err = diffCmd.Output()
		if err != nil {
			fmt.Println("Error reading git diff")
			return ""
		}
		gitDiff = string(diffBytes)
	}

	if gitDiff == "" {
		fmt.Println("No code changes found! Write some code before running coverage.")
		return ""
	}

	return gitDiff
}

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.Flags().StringVarP(&coverageProviderFlag, "provider", "p", "openrouter", "AI provider to use (openrouter, ollama)")
	coverageCmd.Flags().StringVar(&issueFlag, "issue", "", "Issue number to analyze against (e.g. 123)")
}
