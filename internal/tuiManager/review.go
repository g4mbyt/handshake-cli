package tuiManager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"handshake-cli/internal/aiManager"
	"handshake-cli/internal/promptManager"
)

const (
	appBoxHorizontalCost = 2 + 8 + 2
	appBoxVerticalCost   = 2 + 2 + 1
	panelHorizontalCost  = 2 + 4
	panelVerticalCost    = 2 + 2
	viewChatFixedRows    = 1 + 1 + 1 + 1 + 1 + panelVerticalCost*2
)

type reviewState int

const (
	reviewStateLoading reviewState = iota
	reviewStateReady
	reviewStateFollowUp
	reviewStateError
)

type reviewReadyMsg struct {
	review string
	err    error
}

type followUpReadyMsg struct {
	answer string
	err    error
}

type reviewFocus int

const (
	focusReview reviewFocus = iota
	focusChat
	focusInput
)

type ReviewModel struct {
	state    reviewState
	focus    reviewFocus
	spinner  spinner.Model
	provider string
	prompts  *promptManager.Prompts

	termW int
	termH int

	cancel context.CancelFunc

	gitDiff  string
	aiReview string

	reviewVP viewport.Model
	chatVP   viewport.Model
	input    textinput.Model
	chatLog  []string

	errMsg string
}

func NewReviewModel(gitDiff, provider string, prompts *promptManager.Prompts) ReviewModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accentBlue)

	ti := textinput.New()
	ti.Placeholder = "Ask the AI to explain a specific line…"
	ti.CharLimit = 300

	return ReviewModel{
		state:    reviewStateLoading,
		focus:    focusInput,
		spinner:  sp,
		provider: provider,
		prompts:  prompts,
		gitDiff:  gitDiff,
		input:    ti,
		chatLog:  []string{"System: AI reviewer is active. Ask anything about the diff."},
		cancel:   func() {},
	}
}

func (m ReviewModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			return startReviewMsg{ctx: ctx, cancel: cancel}
		},
	)
}

type startReviewMsg struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
		m.termH = msg.Height
		m = m.resizePanels()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.state == reviewStateLoading || m.state == reviewStateFollowUp {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case startReviewMsg:
		m.cancel = msg.cancel
		return m, fetchReviewCmd(msg.ctx, m.gitDiff, m.provider, m.prompts)

	case reviewReadyMsg:
		if msg.err != nil {
			m.state = reviewStateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.state = reviewStateReady
		m.aiReview = msg.review
		m.reviewVP.SetContent(msg.review)
		m.input.Focus()
		return m, nil

	case followUpReadyMsg:
		m.state = reviewStateReady
		m.input.Focus()
		if msg.err != nil {
			m.chatLog = append(m.chatLog, "AI: ❌ "+msg.err.Error())
		} else {
			m.chatLog = append(m.chatLog, "AI: "+msg.answer)
		}
		m.chatVP.SetContent(m.renderChatLog())
		m.chatVP.GotoBottom()
		return m, nil
	}

	switch m.state {
	case reviewStateReady:
		return m.updateChat(msg)
	case reviewStateError:
		return m.updateError(msg)
	}

	return m, nil
}

func (m ReviewModel) resizePanels() ReviewModel {
	if m.termW == 0 || m.termH == 0 {
		return m
	}

	panelW := m.termW - appBoxHorizontalCost - panelHorizontalCost

	available := m.termH - appBoxVerticalCost - viewChatFixedRows
	if available < 4 {
		available = 4
	}
	reviewH := (available * 45) / 100
	chatH := (available * 40) / 100
	if reviewH < 2 {
		reviewH = 2
	}
	if chatH < 2 {
		chatH = 2
	}

	m.reviewVP.Width = panelW
	m.reviewVP.Height = reviewH
	m.chatVP.Width = panelW
	m.chatVP.Height = chatH

	m.reviewVP.SetContent(m.aiReview)
	m.chatVP.SetContent(m.renderChatLog())
	m.input.Width = panelW

	return m
}

func (m ReviewModel) updateChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	var reviewCmd, chatCmd, inputCmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.cancel()
			return m, tea.Quit

		case "tab":
			m.focus = (m.focus + 1) % 3
			if m.focus == focusInput {
				return m, m.input.Focus()
			}
			m.input.Blur()
			return m, nil

		case "enter":
			if m.focus != focusInput {
				return m, nil
			}
			question := strings.TrimSpace(m.input.Value())
			if question == "" {
				return m, nil
			}
			m.chatLog = append(m.chatLog, "You: "+question)
			m.chatVP.SetContent(m.renderChatLog())
			m.chatVP.GotoBottom()
			m.input.SetValue("")
			m.input.Blur()
			m.state = reviewStateFollowUp

			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel

			return m, tea.Batch(
				m.spinner.Tick,
				fetchFollowUpCmd(ctx, m.gitDiff, m.aiReview, m.chatLog, question, m.provider, m.prompts),
			)
		}
	}

	switch m.focus {
	case focusReview:
		m.reviewVP, reviewCmd = m.reviewVP.Update(msg)
	case focusChat:
		m.chatVP, chatCmd = m.chatVP.Update(msg)
	case focusInput:
		m.input, inputCmd = m.input.Update(msg)
	}

	return m, tea.Batch(reviewCmd, chatCmd, inputCmd)
}

func (m ReviewModel) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ReviewModel) View() string {
	switch m.state {
	case reviewStateLoading:
		label := spinnerLabelStyle.Render(
			fmt.Sprintf("%s Fetching context & AI review via %s…", m.spinner.View(), m.provider),
		)
		return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
			logoStyle.Render("HANDSHAKE"),
			"\n",
			label,
		))

	case reviewStateError:
		return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
			summaryErrStyle.Render("❌  Review failed"),
			"\n",
			logLineStyle.Render(m.errMsg),
			"\n",
			helpStyle.Render("enter / esc: quit"),
		))

	case reviewStateReady, reviewStateFollowUp:
		return m.viewChat()
	}

	return ""
}

func (m ReviewModel) viewChat() string {
	panelW := m.termW - appBoxHorizontalCost - panelHorizontalCost

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(panelW)

	reviewBorder := darkBlue
	if m.focus == focusReview {
		reviewBorder = accentBlue
	}

	reviewPanel := panelStyle.
		BorderForeground(reviewBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			subtitleStyle.Foreground(reviewBorder).Render("AI Review  ↑/↓ to scroll"),
			m.reviewVP.View(),
		))

	var inputArea string
	if m.state == reviewStateFollowUp {
		inputArea = spinnerLabelStyle.Render(
			fmt.Sprintf("%s AI is thinking…", m.spinner.View()),
		)
	} else {
		inputArea = m.input.View()
	}

	chatBorder := primaryBlue
	if m.focus == focusChat {
		chatBorder = accentBlue
	} else if m.focus == focusInput {
		chatBorder = primaryBlue
	} else {
		chatBorder = darkBlue
	}

	chatPanel := panelStyle.
		BorderForeground(chatBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			subtitleStyle.Foreground(chatBorder).Render("Chat  ↑/↓ to scroll"),
			m.chatVP.View(),
			inputArea,
			helpStyle.Render("tab: switch focus  •  enter: send  •  esc: quit"),
		))

	return appBox.Render(lipgloss.JoinVertical(lipgloss.Left,
		logoStyle.Render("HANDSHAKE"),
		reviewPanel,
		chatPanel,
	))
}

func (m ReviewModel) renderChatLog() string {
	var sb strings.Builder
	for _, line := range m.chatLog {
		sb.WriteString(logLineStyle.Render(line) + "\n")
	}
	return sb.String()
}

func fetchReviewCmd(ctx context.Context, gitDiff, provider string, prompts *promptManager.Prompts) tea.Cmd {
	return func() tea.Msg {
		ai, err := buildProvider(provider, prompts)
		if err != nil {
			return reviewReadyMsg{err: fmt.Errorf("failed to initialise %q: %w", provider, err)}
		}

		emb, err := aiManager.GetEmbedding(gitDiff)
		if err != nil {
			review, err := ai.ReviewCode(ctx, gitDiff, "No historical context available (embedding failed).")
			if err != nil {
				return reviewReadyMsg{err: err}
			}
			return reviewReadyMsg{review: review}
		}

		cwd, _ := os.Getwd()
		projectName := filepath.Base(cwd)
		history, err := aiManager.SearchSimilarHistory(emb, projectName, 3)
		if err != nil {
			history = "No historical context found or search failed."
		}

		review, err := ai.ReviewCode(ctx, gitDiff, history)
		if err != nil {
			if ctx.Err() != nil {
				return reviewReadyMsg{err: fmt.Errorf("cancelled")}
			}
			return reviewReadyMsg{err: fmt.Errorf("review failed: %w", err)}
		}

		return reviewReadyMsg{review: review}
	}
}

func fetchFollowUpCmd(ctx context.Context, gitDiff, aiReview string, chatLog []string, question, provider string, prompts *promptManager.Prompts) tea.Cmd {
	return func() tea.Msg {
		ai, err := buildProvider(provider, prompts)
		if err != nil {
			return followUpReadyMsg{err: fmt.Errorf("failed to initialise %q: %w", provider, err)}
		}

		var sb strings.Builder
		sb.WriteString("You are an AI code reviewer. Here is the git diff under review:\n\n```\n")
		sb.WriteString(gitDiff)
		sb.WriteString("```\n\nYour initial review was:\n\n")
		sb.WriteString(aiReview)
		sb.WriteString("\n\nConversation so far:\n")
		for _, line := range chatLog {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\nAnswer the developer's latest question concisely and precisely.")

		answer, err := ai.ReviewCode(ctx, sb.String(), "Follow-up question context.")
		if err != nil {
			if ctx.Err() != nil {
				return followUpReadyMsg{err: fmt.Errorf("cancelled")}
			}
			return followUpReadyMsg{err: fmt.Errorf("follow-up failed: %w", err)}
		}

		return followUpReadyMsg{answer: answer}
	}
}

func buildProvider(provider string, prompts *promptManager.Prompts) (aiManager.AIProvider, error) {
	switch provider {
	case "ollama":
		return aiManager.NewOllamaAdapter("qwen2.5-coder:1.5b", "", prompts)
	default:
		return aiManager.NewOpenRouterAdapter("google/gemini-2.5-pro", prompts)
	}
}
