package signup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/chzyer/readline"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func Handler(logger *zap.Logger, s ssh.Session, service forum.UserService) error {
	rlConfig := &readline.Config{
		HistoryFile:  "",
		HistoryLimit: 0,
		Stdin:        s,
		Stdout:       s,
		Stderr:       s.Stderr(),
	}

	rl, err := readline.NewEx(rlConfig)
	if err != nil {
		return fmt.Errorf("failed to create readline: %w", err)
	}
	defer rl.Close()

	var (
		inviteCode  string
		newUsername string
	)

	rl.SetPrompt("Enter invite code: ")
	for {
		line, err := rl.Readline()
		if err != nil {
			return fmt.Errorf("failed to get line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line != "" {
			inviteCode = line
			break
		}
	}

	logger = logger.With(zap.String("invite_code", inviteCode))

	rl.SetPrompt("Enter desired username: ")
	for {
		line, err := rl.Readline()
		if err != nil {
			return fmt.Errorf("failed to get line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line != "" {
			newUsername = line
			break
		}
	}

	logger = logger.With(zap.String("username", newUsername))

	err = service.CreateUser(newUsername, s.PublicKey(), inviteCode)
	if err != nil {
		logger.Warn("Failed to create user", zap.Error(err))
	}

	return err
}
