package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type homeModel struct {
	user string

	viewport viewport.Model
	input    textinput.Model
	ready    bool
}

var _ tea.Model = homeModel{}

func newHomeModel(user string) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := homeModel{
		user:  user,
		input: ti,
	}

	return m
}

func (m homeModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}

			switch command {
			// case "boards":

			case "exit", "quit":
				return m, tea.Quit
			default:
				// Display error message, somehow
			}
		}
	case tea.WindowSizeMsg:
		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
		verticalMarginHeight := lipgloss.Height(m.input.View()) + 1

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

func (m homeModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n",
		m.viewport.View(),
		m.input.View(),
	)
}

func (m homeModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\nLogged in as %s\n\n", m.user)
	fmt.Fprintln(&sb, "Available commands:")
	fmt.Fprintln(&sb, "\t* boards - list boards")
	fmt.Fprintln(&sb, "\t* chat - real-time chat")
	fmt.Fprintln(&sb, "\t* exit - exit back to your terminal")
	return sb.String()
}
