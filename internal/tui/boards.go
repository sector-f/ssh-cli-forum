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

type boardsModel struct {
	service forum.BoardService
	user    forum.User
	list    []forum.BoardListing

	viewport viewport.Model
	input    textinput.Model
	ready    bool
}

var _ tea.Model = boardsModel{}

func newBoardsModel(service forum.BoardService, user forum.User) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	// TODO: error handling.
	// Error field on struct that gets displayed instead of list?
	boards, _ := service.ListBoards()

	m := boardsModel{
		service: service,
		user:    user,
		list:    boards,
		input:   ti,
	}

	return m
}

func (m boardsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m boardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			// TODO: change screen message
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

func (m boardsModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m boardsModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	var sb strings.Builder

	for i, board := range m.list {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, board.Name)
	}

	return sb.String()
}
