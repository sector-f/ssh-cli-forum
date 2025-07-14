package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gliderlabs/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/sector-f/ssh-cli-forum/internal/signup"
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

	server := ssh.Server{
		Addr:             address,
		PublicKeyHandler: authHandler(logger, service),
		Handler:          handler(logger, service),
	}

	// TODO: Add flag for this
	server.SetOption(ssh.HostKeyFile("./id_ed25519"))

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

func handler(logger *zap.Logger, service forum.Service) ssh.Handler {
	return func(s ssh.Session) {
		username := s.User()

		logger := logger.With(
			zap.String("username", username),
			zap.String("remote_addr", s.RemoteAddr().String()),
		)
		defer logger.Info("Disconnected")

		logger.Info("Got login")

		if username == signupUsername {
			err := signup.Handler(logger, s, service)
			if err != nil {
				fmt.Fprintf(s, "Error: %s\n", err)
				s.Exit(1)
				return
			}

			fmt.Fprintln(s, "User created successfully")
			s.Exit(0)
			return
		}

		fmt.Fprintln(s, "Error: unimplemented function")
		s.Exit(1)
		return
	}
}
