package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/internal/tui/home"
)

type rootModel struct {
	home tea.Model
}

var _ tea.Model = rootModel{}

func New(username string) tea.Model {
	m := rootModel{
		home: home.New(username),
	}

	return m
}

func (m rootModel) Init() tea.Cmd {
	return nil
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.home, cmd = m.home.Update(msg)
	return m, cmd
}

func (m rootModel) View() string {
	return m.home.View()
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
