package versionControlManager

import (
	"os/exec"
)

type IssueTracker interface {
	GetIssueText(issueNumber string) (string, error)
}

type GithubAdapter struct{}

func (gha *GithubAdapter) GetIssueText(issueNumber string) (string, error) {
	cmdObj := exec.Command("gh", "issue", "view", issueNumber, "--json", "body", "--jq", ".body")
	outputBytes, err := cmdObj.Output()

	if err != nil {
		return "", err
	}

	return string(outputBytes), nil
}
