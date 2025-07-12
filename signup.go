package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type signupModel struct {
	sess    ssh.Session
	service forum.Service
	logger  *zap.Logger

	form *huh.Form

	invite   string
	username string

	width  int
	height int
}

func newSignupModel(s ssh.Session, service forum.Service, logger *zap.Logger) signupModel {
	m := signupModel{
		sess:    s,
		service: service,
		logger:  logger,
	}

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Enter invite code").Prompt("> ").Value(&m.invite),
			huh.NewInput().Title("Enter username").Prompt("> ").Value(&m.username),
		),
	)

	return m
}

func (m signupModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m signupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
}

func (m signupModel) View() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.form.View())
}
