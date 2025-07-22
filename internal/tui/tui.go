package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/internal/tui/home"
)

type screen string

const (
	screenHome screen = "home"
)

type rootModel struct {
	screens map[screen]tea.Model
	current screen
}

var _ tea.Model = rootModel{}

func New(username string) tea.Model {
	screens := map[screen]tea.Model{
		screenHome: home.New(username),
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
	return m.screens[m.current].View()
}

func NewProgram(sess ssh.Session) *tea.Program {
	username := sess.User()
	m := New(username)
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(sess), tea.WithOutput(sess))
	return program
}

func Handler() bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		return NewProgram(s)
	}
}
