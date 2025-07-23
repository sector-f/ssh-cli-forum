package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type boardListModel struct {
	service forum.BoardService
	list    []forum.BoardListing

	viewport viewport.Model
	input    textinput.Model
	ready    bool
}

var _ tea.Model = boardListModel{}

func newBoardListModel(service forum.BoardService) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	// TODO: error handling.
	// Error field on struct that gets displayed instead of list?
	boards, _ := service.ListBoards()

	m := boardListModel{
		service: service,
		list:    boards,
		input:   ti,
	}

	return m
}

func (m boardListModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m boardListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ChangeScreenCmd(screenHome, 0)
		case "ctrl+c":
			// TODO: change screen message
			return m, tea.Quit
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}

			args := strings.Fields(command)
			if len(args) != 1 {
				break
			}

			index, err := strconv.Atoi(args[0])
			if err != nil {
				break
			}

			if index <= 0 || index > len(m.list) {
				break
			}

			selection := m.list[index-1]
			return m, ChangeScreenCmd(screenBoard, selection.ID)
		}
	case tea.WindowSizeMsg:
		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
		verticalMarginHeight := lipgloss.Height(m.input.View()) + 1 + 3

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.KeyMap = km
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

func (m boardListModel) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m boardListModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	var sb strings.Builder

	for i, board := range m.list {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, board.Name)
	}

	return sb.String()
}
