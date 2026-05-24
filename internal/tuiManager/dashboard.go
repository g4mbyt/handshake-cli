package tuiManager

import (
	"fmt"
	"handshake-cli/internal/aiManager"
	"handshake-cli/internal/promptManager"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const handshakeLogo = `
██╗  ██╗ █████╗ ███╗   ██╗██████╗ ███████╗██╗  ██╗ █████╗ ██╗  ██╗███████╗
██║  ██║██╔══██╗████╗  ██║██╔══██╗██╔════╝██║  ██║██╔══██╗██║ ██╔╝██╔════╝
███████║███████║██╔██╗ ██║██║  ██║███████╗███████║███████║█████╔╝ █████╗
██╔══██║██╔══██║██║╚██╗██║██║  ██║╚════██║██╔══██║██╔══██║██╔═██╗ ██╔══╝
██║  ██║██║  ██║██║ ╚████║██████╔╝███████║██║  ██║██║  ██║██║  ██╗███████╗
╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═════╝ ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝
`

const handshakeLogoNarrow = `HANDSHAKE`
const logoMinWidth = 76

const (
	primaryBlue  = lipgloss.Color("#2563EB")
	accentBlue   = lipgloss.Color("#60A5FA")
	darkBlue     = lipgloss.Color("#1E3A8A")
	textMuted    = lipgloss.Color("#94A3B8")
	textWhite    = lipgloss.Color("#F8FAFC")
	successGreen = lipgloss.Color("#22C55E")
	errorRed     = lipgloss.Color("#EF4444")
)

var (
	appBox            = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(darkBlue).Padding(1, 4).MarginTop(1).MarginLeft(2)
	logoStyle         = lipgloss.NewStyle().Foreground(primaryBlue).Bold(true).MarginBottom(1)
	subtitleStyle     = lipgloss.NewStyle().Foreground(accentBlue).MarginBottom(2)
	itemStyle         = lipgloss.NewStyle().Foreground(textMuted).PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().Foreground(textWhite).Background(primaryBlue).Bold(true).PaddingLeft(2).PaddingRight(4).MarginLeft(-2)
	helpStyle         = lipgloss.NewStyle().Foreground(darkBlue).MarginTop(2)
	logLineStyle      = lipgloss.NewStyle().Foreground(textMuted)
	spinnerLabelStyle = lipgloss.NewStyle().Foreground(accentBlue).Bold(true)
	summaryGoodStyle  = lipgloss.NewStyle().Foreground(successGreen).Bold(true)
	summaryErrStyle   = lipgloss.NewStyle().Foreground(errorRed).Bold(true)
	resultStyle       = lipgloss.NewStyle().Foreground(textWhite)
)

const maxLogLines = 8

type appState int

const (
	stateMenu appState = iota
	stateLoading
	stateResult
)

type jobFinishedMsg struct {
	result aiManager.VectorizeResult
	kind   string // "vectorize" | "clear-cache"
	err    error
}

type progressMsg struct{ line string }

type ActionType string

const (
	ActionNone     ActionType = ""
	ActionCoverage ActionType = "coverage"
	ActionReview   ActionType = "review"
	ActionSSH      ActionType = "ssh"
)

type DashboardModel struct {
	state          appState
	cursor         int
	choices        []string
	spinner        spinner.Model
	logLines       []string
	resultMsg      string
	termW          int
	termH          int
	prompts        *promptManager.Prompts
	SelectedAction ActionType
}

func NewDashboardModel(choices []string, prompts *promptManager.Prompts) DashboardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accentBlue)

	return DashboardModel{
		state:          stateMenu,
		choices:        choices,
		spinner:        sp,
		prompts:        prompts,
		SelectedAction: ActionNone,
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return nil
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.termW = msg.Width
		m.termH = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case progressMsg:
		m.logLines = append(m.logLines, msg.line)
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		return m, nil

	case jobFinishedMsg:
		m.state = stateResult
		if msg.err != nil {
			m.resultMsg = summaryErrStyle.Render(fmt.Sprintf("❌  Error\n\n%v", msg.err))
		} else if msg.kind == "clear-cache" {
			m.resultMsg = summaryGoodStyle.Render("🗑️  Cache cleared successfully.")
		} else {
			r := msg.result
			skippedLine := fmt.Sprintf("   • %d already cached (skipped)", r.Skipped)
			errLine := ""
			if r.Errors > 0 {
				errLine = "\n" + summaryErrStyle.Render(fmt.Sprintf("   • %d failed", r.Errors))
			}
			m.resultMsg = fmt.Sprintf(
				"%s\n\n%s\n%s%s\n   • %d total scanned",
				summaryGoodStyle.Render("✅  Sync complete"),
				summaryGoodStyle.Render(fmt.Sprintf("   • %d new memories vectorized", r.Embedded)),
				logLineStyle.Render(skippedLine),
				errLine,
				r.Total,
			)
		}
		return m, nil
	}

	switch m.state {
	case stateMenu:
		return m.updateMenu(msg)
	case stateResult:
		return m.updateResult(msg)
	default:
		tea.Quit()
	}

	return m, nil
}

func (m DashboardModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			choice := m.choices[m.cursor]

			if choice == "Requirements Coverage" {
				m.SelectedAction = ActionCoverage
				return m, tea.Quit
			}

			if choice == "AI Code Review" {
				m.SelectedAction = ActionReview
				return m, tea.Quit
			}

			if choice == "Host Multiplayer SSH Session" {
				m.SelectedAction = ActionSSH
				return m, tea.Quit
			}

			if choice == "Vectorize AI Session History" {
				m.state = stateLoading
				m.logLines = nil
				return m, tea.Batch(
					m.spinner.Tick,
					runVectorizeCmd(),
				)
			}

			if choice == "Clear Vectorize Cache" {
				m.state = stateLoading
				m.logLines = nil
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						time.Sleep(500 * time.Millisecond)
						err := aiManager.ClearSyncCache()
						if err != nil {
							return jobFinishedMsg{err: err, kind: "clear-cache"}
						}
						return jobFinishedMsg{result: aiManager.VectorizeResult{}, kind: "clear-cache"}
					},
				)
			}
		}
	}
	return m, nil
}

func (m DashboardModel) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", "esc", "q":
			m.state = stateMenu
			m.logLines = nil
			return m, nil
		}
	}
	return m, nil
}

func runVectorizeCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := aiManager.RunVectorizeJob()
		return jobFinishedMsg{result: result, kind: "vectorize", err: err}
	}
}

func (m DashboardModel) View() string {
	innerW := m.termW - appBoxHorizontalCost
	if innerW < 20 {
		innerW = 20
	}

	var header string
	if m.termW >= logoMinWidth {
		header = logoStyle.Render(strings.TrimSpace(handshakeLogo))
	} else {
		header = logoStyle.Render(handshakeLogoNarrow)
	}

	subtitle := subtitleStyle.Render("AI-Powered Code Reviewer")

	var content string

	switch m.state {

	case stateMenu:
		var b strings.Builder
		for i, choice := range m.choices {
			if m.cursor == i {
				b.WriteString(selectedItemStyle.Width(innerW).Render(choice) + "\n")
			} else {
				b.WriteString(itemStyle.Render(choice) + "\n")
			}
		}
		content = b.String() + "\n" + helpStyle.Render("↑/↓: navigate  •  enter: select  •  q: quit")

	case stateLoading:
		spinnerLine := spinnerLabelStyle.
			Width(innerW).
			Render(fmt.Sprintf("%s Vectorizing session history…", m.spinner.View()))

		var logBuilder strings.Builder
		for _, line := range m.logLines {
			if len(line) > innerW-2 {
				line = line[:innerW-5] + "…"
			}
			logBuilder.WriteString(logLineStyle.Render("  "+line) + "\n")
		}

		content = fmt.Sprintf("\n%s\n\n%s", spinnerLine, logBuilder.String())

	case stateResult:
		content = resultStyle.Width(innerW).Render("\n"+m.resultMsg+"\n") +
			helpStyle.Render("\n\nenter: return to menu")
	}

	ui := lipgloss.JoinVertical(lipgloss.Left, header, subtitle, "\n", content)
	return appBox.Render(ui)
}
