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
	"github.com/sector-f/ssh-cli-forum/internal/signup"
	"github.com/sector-f/ssh-cli-forum/internal/table"
	"github.com/sector-f/ssh-cli-forum/sqlite"
	"go.uber.org/zap"
)

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

	// TODO: figure out some way to convert *zap.Logger into whatever type
	// the wish structured logging middleware expects.
	server, err := wish.NewServer(
		wish.WithAddress(address),
		wish.WithPublicKeyAuth(authHandler(logger, service)),
		wish.WithMiddleware(
			bubbletea.Middleware(tableHandler(logger, service)),
			signupMiddleware(logger, service),
			activeterm.Middleware(),
		),
	)

	// TODO: Add flag for this
	// server.SetOption(ssh.HostKeyFile("./id_ed25519"))

	logger.Info("Starting server", zap.String("address", address))

	err = server.ListenAndServe()
	if err != nil {
		logger.Error("Failed to run server", zap.Error(err))
	}
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
		if username == forum.SignupUsername {
			return true
		}

		return service.AuthenticateUser(username, key)
	}
}

func tableHandler(logger *zap.Logger, service forum.Service) bubbletea.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		m := table.New(logger, service)
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}
}

func signupMiddleware(logger *zap.Logger, service forum.Service) func(next ssh.Handler) ssh.Handler {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if s.User() != forum.SignupUsername {
				next(s)
				return
			}

			err := signup.Handler(logger, s, service)
			if err != nil {
				fmt.Fprintf(s, "Error: %s\n", err)
				s.Exit(1)
			}

			fmt.Fprintln(s, "User created successfully")
		}
	}
}
