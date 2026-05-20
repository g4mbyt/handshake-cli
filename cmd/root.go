package cmd

import (
	"os"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "handshake",
	Short: "A proactive, AI-powered local code reviewer and SSH collaboration hub",
	Long:
	"\033[0;36m\033[1m    __  __                 __     __          __  \033[0m\n" +
	"\033[0;36m\033[1m   / / / /___ _____  ____/ /____/ /_  ____ _/ /_____\033[0m\n" +
	"\033[0;36m\033[1m  / /_/ / __ `/ __ \\/ __  / ___/ __ \\/ __ `/ //_/ _ \\\033[0m\n" +
	"\033[0;34m\033[1m / __  / /_/ / / / / /_/ (__  ) / / / /_/ / ,< /  __/\033[0m\n" +
	"\033[0;34m\033[1m/_/ /_/\\__,_/_/ /_/\\__,_/____/_/ /_/\\__,_/_/|_|\\___/ \033[0m\n" +
	"\n" +
	"Welcome to Handshake! A terminal-native AI helper that verifies your code before it leaves your machine."+

	"Instead of waiting for CI/CD bots to flag issues on a public pull request, Handshake establishes a \"handshake\" between your business requirements (GitHub issues), your original intent (local AI agent logs), and your actual code (Git diffs)."+

	"Run it locally to catch edge cases and verify requirement coverage, or use the --together flag to spin up a peer-to-peer SSH dashboard for instant, terminal-to-terminal collaboration with your team.",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
