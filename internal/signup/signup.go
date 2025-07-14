package signup

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	"github.com/gliderlabs/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func Handler(logger *zap.Logger, s ssh.Session, service forum.Service) error {
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

	return service.CreateUser(newUsername, s.PublicKey(), inviteCode)
}
