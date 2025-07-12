package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type signupModel struct {
	sess    ssh.Session
	service forum.Service
	logger  *zap.Logger

	invite   textinput.Model
	username textinput.Model

	width  int
	height int
}

func newSignupModel(s ssh.Session, service forum.Service, logger *zap.Logger) signupModel {
	m := signupModel{
		sess:    s,
		service: service,
		logger:  logger,
	}

	m.invite = textinput.New()
	m.invite.Focus()
	m.invite.Prompt = "Invite code: "

	m.username = textinput.New()
	m.username.Blur()
	m.username.Prompt = "Username: "

	return m
}

func (m signupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m signupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	if m.invite.Focused() {
		m.invite, cmd = m.invite.Update(msg)
	} else if m.username.Focused() {
		m.username, cmd = m.username.Update(msg)
	}

	return m, cmd
}
func (m signupModel) View() string {
	var sb strings.Builder
	sb.WriteString(m.invite.View() + "\n")
	sb.WriteString(m.username.View() + "\n")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}
