package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
)

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

type ChangeSceenMsg struct {
	Screen screen
	ID     int // Board or Thread ID
}

func ChangeScreenCmd(s screen, id int) tea.Cmd {
	return func() tea.Msg {
		return ChangeSceenMsg{
			Screen: s,
			ID:     id,
		}
	}
}

type screen string

const (
	screenHome      screen = "SSH Bulletin Board"
	screenBoardList screen = "Boards"
	screenBoard     screen = "Threads"
	screenChat      screen = "SSH Chatroom"
)

type rootModel struct {
	screens map[screen]tea.Model
	current screen

	// Used by chat service to send messages to this Program
	callback func(msg forum.Message)

	width  int
	height int
}

var _ tea.Model = rootModel{}

func New(sess ssh.Session, boardService forum.BoardService, user forum.User, chatService forum.ChatroomService, callback func(msg forum.Message)) tea.Model {
	screens := map[screen]tea.Model{
		screenHome:      newHomeModel(user.Username),
		screenBoardList: newBoardListModel(boardService),
		screenChat:      newChatModel(sess, chatService, callback),
		screenBoard:     newBoardModel(boardService),
	}

	m := rootModel{
		screens:  screens,
		current:  screenHome,
		callback: callback,
	}

	return m
}

func (m rootModel) Init() tea.Cmd {
	return nil
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case forum.Message:
		m.screens[screenChat], cmd = m.screens[screenChat].Update(msg)
		if m.current != screenChat {
			return m, nil
		}
		return m, cmd
	case ChangeSceenMsg:
		newScreen := msg.Screen
		m.current = newScreen

		wsm := tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		}

		m.screens[m.current], cmd = m.screens[m.current].Update(wsm)
		cmds = append(cmds, cmd)
	}

	m.screens[m.current], cmd = m.screens[m.current].Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m rootModel) View() string {
	current, ok := m.screens[m.current]
	if !ok {
		// This should never happen
		return "Invalid screen"
	}

	return fmt.Sprintf("%s\n%s", m.headerView(), current.View())
}

func NewProgram(sess ssh.Session, userService forum.UserService, boardService forum.BoardService, chatService forum.ChatroomService) *tea.Program {
	user, err := userService.GetUserByUsername(sess.User())
	if err != nil {
		// TODO: Log this? Return err somehow?
		return nil
	}

	var program *tea.Program
	callback := func(msg forum.Message) {
		program.Send(msg)
	}

	m := New(sess, boardService, user, chatService, callback)
	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(sess), tea.WithOutput(sess))

	go func() {
		program.Wait()
		chatService.DisconnectUser(sess.Context().SessionID())
	}()

	return program
}

func Handler(userService forum.UserService, boardService forum.BoardService, chatService forum.ChatroomService) bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		return NewProgram(s, userService, boardService, chatService)
	}
}

func (m rootModel) headerView() string {
	text := string(m.current)

	title := titleStyle.Render(text)
	line := strings.Repeat(
		"─",
		max(0, (m.width-lipgloss.Width(title))/2),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, line, title, line)
}
