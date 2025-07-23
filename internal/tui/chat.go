package tui

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/muesli/reflow/wordwrap"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type chatModel struct {
	session    ssh.Session
	user       string // TODO: change to forum.User?
	service    forum.ChatroomService
	callback   func(forum.Message)
	scrollback []forum.Message
	connected  bool

	viewport viewport.Model
	input    textinput.Model
	ready    bool
	width    int
}

var _ tea.Model = chatModel{}

func newChatModel(session ssh.Session, service forum.ChatroomService, callback func(forum.Message)) tea.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	msgs := service.ConnectUser(session, callback)

	m := chatModel{
		session:    session,
		user:       session.User(),
		service:    service,
		input:      ti,
		callback:   callback,
		scrollback: msgs,
	}

	return m
}

func (m chatModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case forum.Message:
		m.scrollback = append(m.scrollback, msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, ChangeScreenCmd(screenHome, 0)
		case "enter":
			message := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if message == "" {
				break
			}

			chatMsg := forum.ChatMsg{
				Timestamp: time.Now(),
				Username:  m.user,
				Text:      message,
			}

			cmds = append(cmds, func() tea.Msg { m.service.SendMsg(chatMsg); return nil })
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

func (m chatModel) View() string {
	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m chatModel) bufferContents() string {
	if !m.ready {
		return "Initializing\n\n"
	}

	var sb strings.Builder

	// '12:34:56 '
	prefixWidth := 9

	for _, msg := range m.scrollback {
		timeStamp := msg.Time().Format(time.TimeOnly)
		wrappedMsg := wordwrap.String(msg.Body(), m.width-prefixWidth)

		sb.WriteString(timeStamp + " ")

		var lineNum int
		scanner := bufio.NewScanner(strings.NewReader(wrappedMsg))
		for scanner.Scan() {
			if lineNum > 0 {
				sb.WriteString(strings.Repeat(" ", prefixWidth))
			}
			sb.WriteString(scanner.Text() + "\n")
			lineNum++
		}
	}

	return sb.String()
}
