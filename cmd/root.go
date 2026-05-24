package cmd

import (
	"fmt"
	"handshake-cli/internal/promptManager"
	"handshake-cli/internal/tuiManager"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "handshake",
	Short: "AI-powered code review workflows in your terminal",
	Run: func(cmd *cobra.Command, args []string) {

		prompts, err := promptManager.LoadPrompts()
		if err != nil {
			fmt.Printf("Critical error loading AI promptManager: %v\n", err)
			os.Exit(1)
		}

		choices := []string{
			"Requirements Coverage",
			"AI Code Review",
			"Host Multiplayer SSH Session",
			"Vectorize AI Session History",
			"Clear Vectorize Cache",
		}
		dashModel := tuiManager.NewDashboardModel(choices, prompts)

		teaProgram := tea.NewProgram(dashModel, tea.WithAltScreen())
		finalModel, err := teaProgram.Run()
		if err != nil {
			fmt.Println("Error running application:", err)
			os.Exit(1)
		}

		if dm, ok := finalModel.(tuiManager.DashboardModel); ok {
			switch dm.SelectedAction {
			case tuiManager.ActionCoverage:
				if coverageCmd.RunE != nil {
					if err := coverageCmd.RunE(coverageCmd, nil); err != nil {
						fmt.Println("Error:", err)
					}
				} else if coverageCmd.Run != nil {
					coverageCmd.Run(coverageCmd, nil)
				}
			case tuiManager.ActionReview:
				if reviewCmd.RunE != nil {
					if err := reviewCmd.RunE(reviewCmd, nil); err != nil {
						fmt.Println("Error:", err)
					}
				} else if reviewCmd.Run != nil {
					reviewCmd.Run(reviewCmd, nil)
				}
			case tuiManager.ActionSSH:
				fmt.Println("🚧 Host Multiplayer SSH Session feature coming soon!")
			}
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
