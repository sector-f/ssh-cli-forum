package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type boardModel struct {
	service forum.BoardService
	board   forum.Board

	viewport viewport.Model
	input    textinput.Model
	ready    bool
}

var _ tea.Model = boardModel{}

func newBoardModel(service forum.BoardService) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := boardModel{
		service: service,
		input:   ti,
	}

	return m
}

func (m boardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case ChangeSceenMsg:
		if msg.Screen != screenBoard {
			break
		}

		board, err := m.service.GetBoardByID(msg.ID)
		if err != nil {
			break
		}

		m.board = board
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ChangeScreenCmd(screenBoardList, 0)
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}
		}
	case tea.WindowSizeMsg:
		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
		verticalMarginHeight := lipgloss.Height(m.input.View()) + 1 + 3

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	m.viewport.SetContent(m.bufferContents())

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m boardModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m boardModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "%s:\n", m.board.Name)
	for i, thread := range m.board.Threads {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, thread.Title)
	}

	return sb.String()
}
