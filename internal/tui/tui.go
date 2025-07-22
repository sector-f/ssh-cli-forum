package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
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
	screenHome   screen = "home"
	screenBoards screen = "boards"
	//screenChat   screen = "chat"
)

type rootModel struct {
	screens map[screen]tea.Model
	current screen

	width  int
	height int
}

var _ tea.Model = rootModel{}

func New(sess ssh.Session, boardService forum.BoardService, user forum.User, chatService forum.ChatroomService) tea.Model {
	screens := map[screen]tea.Model{
		screenHome:   newHomeModel(user.Username),
		screenBoards: newBoardsModel(boardService, user),
	}

	m := rootModel{
		screens: screens,
		current: screenHome,
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

	return current.View()
}

func NewProgram(sess ssh.Session, userService forum.UserService, boardService forum.BoardService, chatService forum.ChatroomService) *tea.Program {
	user, err := userService.GetUserByUsername(sess.User())
	if err != nil {
		// TODO: Log this? Return err somehow?
		return nil
	}

	m := New(sess, boardService, user, chatService)
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(sess), tea.WithOutput(sess))
	return program
}

func Handler(userService forum.UserService, boardService forum.BoardService, chatService forum.ChatroomService) bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		return NewProgram(s, userService, boardService, chatService)
	}
}
