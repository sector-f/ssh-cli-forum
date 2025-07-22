package teashell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ tea.Model = homeModel{}

type homeModel struct {
	parent *tea.Model

	user     string
	viewport viewport.Model
	input    textinput.Model
	width    int
	ready    bool
	header   string
}

func newHomeModel() homeModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	return homeModel{
		input:  ti,
		header: "SSH Bulletin Board",
	}
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
		case "esc":
			return *m.parent, nil
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}

			// TODO: is there a way to automate this?
			switch command {
			case "boards":

			case "exit", "quit":
				return m, tea.Quit
			default:
				// Display error message, somehow
			}
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		verticalMarginHeight := headerHeight + lipgloss.Height(m.input.View()) + 1
		m.width = msg.Width
		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m homeModel) View() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Logged in as %s\n\n", m.user)
	fmt.Fprintln(&sb, "Available commands:")
	fmt.Fprintln(&sb, "\t* boards - list boards")
	fmt.Fprintln(&sb, "\t* chat - real-time chat")
	fmt.Fprintln(&sb, "\t* exit - exit back to your terminal")

	return sb.String()
}
