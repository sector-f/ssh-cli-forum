package teashell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	"github.com/sector-f/ssh-cli-forum/forum"
)

var _ tea.Model = model{}

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.NormalBorder()
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

type model struct {
	user     string
	chatroom forum.ChatroomService
	lines    []string
	buffer   string

	viewport   viewport.Model
	input      textinput.Model
	headerText string
	width      int
	ready      bool
}

func New(s forum.ChatroomService, username string) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := model{
		user:       username,
		chatroom:   s,
		buffer:     "Welcome to this stupid chatroom that probably won't work!\n\n",
		input:      ti,
		headerText: "SSH Bulletin Board",
	}

	return m
}

func Handler(service forum.ChatroomService) bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		id := s.Context().SessionID()
		program := tea.NewProgram(New(service, s.User()), tea.WithAltScreen(), tea.WithInput(s), tea.WithOutput(s))

		go func() {
			program.Wait()
			service.Unregister(id)
		}()

		service.Register(id, program)
		return program
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case forum.Message:
		line := strings.TrimSpace(fmt.Sprintf("%s: %s", msg.Username, msg.Text))
		m.lines = append(m.lines, line)

		lines := strings.Join(m.lines, "\n")
		text := wordwrap.String(lines, m.width)
		m.viewport.SetContent(text)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			text := m.input.Value()

			// Send on separate goroutine so we don't deadlock:
			// SendMessage blocks trying to send to this session,
			// but this session is blocked waiting for SendMessage to return.
			go m.chatroom.SendMessage(m.user, strings.TrimSpace(text))
			m.input.Reset()
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		verticalMarginHeight := headerHeight + lipgloss.Height(m.input.View()) + 1
		m.width = msg.Width

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		lines := strings.Join(m.lines, "\n")
		text := wordwrap.String(lines, m.width)
		m.viewport.SetContent(text)
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		m.headerView(),
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m model) headerView() string {
	title := titleStyle.Render(m.headerText)
	line := strings.Repeat(
		"─",
		max(0, (m.viewport.Width-lipgloss.Width(title))/2),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, line, title, line)
}
