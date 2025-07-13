package main

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type signupModel struct {
	sess    ssh.Session
	service forum.Service
	logger  *zap.Logger

	form *huh.Form

	width  int
	height int

	err error
}

func newSignupModel(s ssh.Session, service forum.Service, logger *zap.Logger) signupModel {
	m := signupModel{
		sess:    s,
		service: service,
		logger:  logger,
	}

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("invite").Title("Enter invite code").Prompt("> "),
			huh.NewInput().Key("username").Title("Enter username").Prompt("> "),
			huh.NewConfirm().
				Key("submit").
				Title("Create account?").
				Negative("No").
				Affirmative("Yes"),
		).WithTheme(huh.ThemeBase()),
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
		username := strings.TrimSpace(m.form.GetString("username"))
		invite := strings.TrimSpace(m.form.GetString("invite"))

		if username == "" || invite == "" {
			m.err = errors.New("username and invite code are required")
		} else {
			m.err = m.service.CreateUser(username, m.sess.PublicKey(), invite)
			if m.err == nil {
				cmds = append(cmds, tea.Quit)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m signupModel) View() string {
	s := m.form.View()
	if m.err != nil {
		s = fmt.Sprintf("%s\n\n%s\n", m.err.Error(), s)
	}

	return s
}
