package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/sector-f/ssh-cli-forum/internal/models/signup"
	"github.com/sector-f/ssh-cli-forum/sqlite"
	"go.uber.org/zap"
)

var signupUsername = "signup"

func main() {
	// TODO: add flag
	address := "127.0.0.1:2222"

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.Sync()

	service, err := sqlite.Open(logger, "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	w, err := wish.NewServer(
		wish.WithAddress(address),
		wish.WithPublicKeyAuth(authHandler(logger, service)),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler(logger, service)),
			activeterm.Middleware(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	logger.Info("Starting server", zap.String("address", address))
	log.Fatal(w.ListenAndServe())
}

func authHandler(logger *zap.Logger, service forum.Service) ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		username := ctx.User()

		logger := logger.With(
			zap.String("username", username),
			zap.String("session_id", ctx.SessionID()),
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("client_version", ctx.ClientVersion()),
		)

		logger.Info("Authenticating user")

		// Allow any key when signing up
		if username == signupUsername {
			return true
		}

		user, err := service.GetUserByUsername(ctx.User())
		if err != nil {
			logger.Debug("Failed to get user by username", zap.Error(err))
			return false
		}

		return ssh.KeysEqual(user.PubKey, key)
	}
}

func teaHandler(logger *zap.Logger, service forum.Service) bubbletea.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		user := s.User()

		if user == signupUsername {
			m := signup.New(s, service, logger)
			return m, []tea.ProgramOption{}
		}

		m := mainModel{
			user: user,
		}

		return m, []tea.ProgramOption{}
	}
}
