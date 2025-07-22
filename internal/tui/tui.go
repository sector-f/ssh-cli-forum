package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type screen string

const (
	screenHome   screen = "home"
	screenBoards screen = "boards"
	screenChat   screen = "chat"
)

type rootModel struct {
	screens map[screen]tea.Model
	current screen
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
	var cmd tea.Cmd

	m.screens[m.current], cmd = m.screens[m.current].Update(msg)
	return m, cmd
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
