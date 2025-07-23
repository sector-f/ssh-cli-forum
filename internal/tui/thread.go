package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type threadModel struct {
	service forum.BoardService
	thread  forum.Thread

	viewport viewport.Model
	input    textinput.Model
	ready    bool
	width    int
}

var _ tea.Model = boardModel{}

func newThreadModel(service forum.BoardService) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := threadModel{
		service: service,
		input:   ti,
	}

	return m
}

func (m threadModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m threadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case ChangeSceenMsg:
		if msg.Screen != screenThread {
			break
		}

		thread, err := m.service.GetThreadByID(msg.ID)
		if err != nil {
			break
		}

		m.thread = thread
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ChangeScreenCmd(screenBoard, 0)
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
		m.width = msg.Width

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

func (m threadModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m threadModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	thread := m.thread

	var sb strings.Builder

	fmt.Fprintf(&sb, "%s\nPosted by %s\n\n", thread.Title, thread.Author.Username)
	fmt.Fprintf(&sb, "%s\n", wordwrap.String(thread.Body, m.width))

	for i, comment := range thread.Comments {
		fmt.Fprintf(&sb, "%s responded:\n%s", comment.Author.Username, comment.Body)

		if i < len(thread.Comments)-1 {
			fmt.Fprintf(&sb, "\n")
		}
	}

	return sb.String()
}
