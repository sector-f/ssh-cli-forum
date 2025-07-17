package teashell

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

type screen string

var (
	// Home screen
	screenHome screen = "home"
	// Chatroom screen
	screenChat screen = "chatroom"
)

type model struct {
	session ssh.Session
	screen  screen
	user    string

	chat      forum.ChatroomService
	msgs      []forum.Message
	callback  func(forum.Message)
	connected bool

	viewport   viewport.Model
	input      textinput.Model
	buffer     string
	headerText string
	width      int

	ready bool
}

func NewProgram(sess ssh.Session, chat forum.ChatroomService) *tea.Program {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	id := sess.Context().SessionID()

	var program *tea.Program
	callback := func(msg forum.Message) {
		program.Send(msg)
	}

	m := model{
		session:    sess,
		screen:     screenHome,
		user:       sess.User(),
		chat:       chat,
		input:      ti,
		headerText: "SSH Chatroom",
		callback:   callback,
	}

	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(sess), tea.WithOutput(sess))

	go func() {
		program.Wait()
		chat.Disconnect(id)
	}()

	return program
}

func Handler(chat forum.ChatroomService) bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		return NewProgram(s, chat)
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) homeBuffer() string {
	s := fmt.Sprintf("Available commands:\n\n\t* chat - use the real-time chat feature\n\t* exit - exit the program\n")
	return s
}

func (m model) chatBuffer() string {
	var sb strings.Builder

	// '12:34:56 '
	prefixWidth := 9

	for _, msg := range m.msgs {
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case forum.Message:
		m.msgs = append(m.msgs, msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			switch m.screen {
			case screenChat:
				m.screen = screenHome
			default:
				// Do nothing
			}
		case "ctrl+c":
			switch m.screen {
			case screenChat:
				m.screen = screenHome
			default:
				return m, tea.Quit
			}
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			switch m.screen {
			case screenHome:
				// TODO: is there a way to automate this?
				switch command {
				case "chat":
					if !m.connected {
						msgs := m.chat.Connect(m.session, m.callback)
						m.msgs = msgs
						m.connected = true
					}

					m.screen = screenChat
				case "exit", "quit":
					return m, tea.Quit
				default:
					// Display error message, somehow
				}
				m.input.Reset()
			case screenChat:
				chatMsg := forum.ChatMsg{
					Timestamp: time.Now(),
					Username:  m.user,
					Text:      command,
				}

				cmds = append(cmds, func() tea.Msg { m.chat.SendMsg(chatMsg); return nil })
				m.input.Reset()
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

	switch m.screen {
	case screenHome:
		m.viewport.SetContent(m.homeBuffer())
	case screenChat:
		m.viewport.SetContent(m.chatBuffer())
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
