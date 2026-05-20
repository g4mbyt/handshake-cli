package cmd

import (
	"fmt"
	"os/exec"
	"github.com/spf13/cobra"
	"strings"
	"regexp"
)

var coverageCmd = &cobra.Command {
	Use: "coverage",
	Run: func(cmd *cobra.Command, args []string) {
		if !checkDependencies() {
			return
		}

		issueBody := getIssueBody()
		fmt.Println(issueBody)
	},
}

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
	currentBranch := exec.Command("git", "branch", "--show-current")
	outputBytes, err := currentBranch.Output()

	if err != nil {
		fmt.Println("Error: Failed to get git branch. Are you in a git repository?")
		return ""
	}

	rawString := string(outputBytes)
	branchName := strings.TrimSpace(rawString)

	issueNumber := extractIssueNumber(branchName)
	if issueNumber == "" {
		fmt.Print("No issue number found in branch name. Enter manually? [y/n]: ")
		var manualInput string
		for {
			fmt.Scanln(&manualInput)
			manualInput = strings.ToLower(strings.TrimSpace(manualInput))

			if manualInput == "y" || manualInput == "n" {
				break
			}

			fmt.Println("⚠️ Invalid input. Please type 'y' or 'n'.")
		}

		if manualInput == "n" {
			fmt.Println("Skipping coverage analysis...")
			return ""
		}

		fmt.Print("Enter the issue number: ")
		fmt.Scanln(&manualInput)
		issueNumber = strings.TrimSpace(manualInput)

	}

	var tracker IssueTracker = &GithubAdapter{}
	issuetext, err := tracker.GetIssueText(issueNumber)

	if err != nil {
		return ""
	}

	return issuetext
}

func init() {
	rootCmd.AddCommand(coverageCmd)
}
