package promptManager

import (
	"embed"
	"fmt"
)

//go:embed *.md
var promptFS embed.FS

type Prompts struct {
	CoverageSystem string
	CoverageUser   string
	ReviewSystem   string
	ReviewUser     string
}

func LoadPrompts() (*Prompts, error) {
	covSys, err := promptFS.ReadFile("coverage-system.md")
	if err != nil {
		return nil, fmt.Errorf("missing coverage-system.md: %w", err)
	}

	covUser, err := promptFS.ReadFile("coverage-user.md")
	if err != nil {
		return nil, fmt.Errorf("missing coverage-user.md: %w", err)
	}

	revSys, err := promptFS.ReadFile("review-system.md")
	if err != nil {
		return nil, fmt.Errorf("missing review_system.md: %w", err)
	}

	revUser, err := promptFS.ReadFile("review-user.md")
	if err != nil {
		return nil, fmt.Errorf("missing review-user.md: %w", err)
	}

	return &Prompts{
		CoverageSystem: string(covSys),
		CoverageUser:   string(covUser),
		ReviewSystem:   string(revSys),
		ReviewUser:     string(revUser),
	}, nil
}
