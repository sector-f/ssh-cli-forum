package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type mainModel struct {
	user string
}

func (mainModel) Init() tea.Cmd {
	return tea.Quit
}
func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}
func (m mainModel) View() string {
	return fmt.Sprintf("Successfully authenticated as %s\n", m.user)
}
