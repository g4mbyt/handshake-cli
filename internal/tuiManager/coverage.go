package tuiManager

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"handshake-cli/internal/promptManager"
)

type coverageState int

const (
	coverageStateLoading coverageState = iota
	coverageStateReady
	coverageStateError
)

type coverageReadyMsg struct {
	report string
	err    error
}

type CoverageModel struct {
	state    coverageState
	spinner  spinner.Model
	provider string
	prompts  *promptManager.Prompts

	termW int
	termH int

	cancel context.CancelFunc

	issueBody string
	gitDiff   string
	report    string

	viewport viewport.Model
	errMsg   string
}

func NewCoverageModel(issueBody, gitDiff, provider string, prompts *promptManager.Prompts) CoverageModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accentBlue)

	return CoverageModel{
		state:    coverageStateLoading,
		spinner:  sp,
		provider: provider,
		prompts:  prompts,
		issueBody: issueBody,
		gitDiff:  gitDiff,
		cancel:   func() {},
	}
}

func (m CoverageModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			return startCoverageMsg{ctx: ctx, cancel: cancel}
		},
	)
}

type startCoverageMsg struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m CoverageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
		m.termH = msg.Height
		m = m.resize()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			m.cancel()
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.state == coverageStateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case startCoverageMsg:
		m.cancel = msg.cancel
		return m, fetchCoverageCmd(msg.ctx, m.issueBody, m.gitDiff, m.provider, m.prompts)

	case coverageReadyMsg:
		if msg.err != nil {
			m.state = coverageStateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.state = coverageStateReady
		m.report = msg.report
		m.viewport.SetContent(msg.report)
		return m, nil
	}

	if m.state == coverageStateReady {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m CoverageModel) resize() CoverageModel {
	if m.termW == 0 || m.termH == 0 {
		return m
	}

	m.viewport.Width = m.termW - appBoxHorizontalCost - panelHorizontalCost
	m.viewport.Height = m.termH - appBoxVerticalCost - 6
	m.viewport.SetContent(m.report)

	return m
}

func (m CoverageModel) View() string {
	switch m.state {
	case coverageStateLoading:
		label := spinnerLabelStyle.Render(
			fmt.Sprintf("%s Analyzing requirements coverage via %s…", m.spinner.View(), m.provider),
		)
		return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
			logoStyle.Render("HANDSHAKE"),
			"\n",
			label,
		))

	case coverageStateError:
		return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
			summaryErrStyle.Render("❌  Analysis failed"),
			"\n",
			logLineStyle.Render(m.errMsg),
			"\n",
			helpStyle.Render("q / esc: quit"),
		))

	case coverageStateReady:
		panelW := m.termW - appBoxHorizontalCost - panelHorizontalCost
		panelStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryBlue).
			Padding(1, 2).
			Width(panelW)

		reportPanel := panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			subtitleStyle.Render("Requirements Coverage Report"),
			m.viewport.View(),
			helpStyle.Render("↑/↓: scroll  •  q: quit"),
		))

		return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
			logoStyle.Render("HANDSHAKE"),
			reportPanel,
		))
	}
	return ""
}

func fetchCoverageCmd(ctx context.Context, issueBody, gitDiff, provider string, prompts *promptManager.Prompts) tea.Cmd {
	return func() tea.Msg {
		ai, err := buildProvider(provider, prompts)
		if err != nil {
			return coverageReadyMsg{err: fmt.Errorf("failed to initialise %q: %w", provider, err)}
		}

		report, err := ai.AnalyzeCoverage(ctx, issueBody, gitDiff)
		if err != nil {
			if ctx.Err() != nil {
				return coverageReadyMsg{err: fmt.Errorf("cancelled")}
			}
			return coverageReadyMsg{err: fmt.Errorf("analysis failed: %w", err)}
		}

		return coverageReadyMsg{report: report}
	}
}
