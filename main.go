package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/sector-f/ssh-cli-forum/internal/signup"
	"github.com/sector-f/ssh-cli-forum/internal/tui"
	"github.com/sector-f/ssh-cli-forum/sqlite"
	"go.uber.org/zap"
)

func main() {
	address := "127.0.0.1:2222"
	logger, err := zap.NewDevelopment()
	if err != nil { fmt.Fprint(os.Stderr, err); os.Exit(1) }
	defer logger.Sync()

	service, err := sqlite.Open(logger, "./forum.db")
	if err != nil { log.Fatal(err) }
	defer service.Close()

	server, err := wish.NewServer(
		wish.WithAddress(address),
		wish.WithPublicKeyAuth(authHandler(logger, service)),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(tui.Handler(service, service, service), termenv.Ascii),
			signupMiddleware(logger, service),
			loggingMiddleware(logger),
			activeterm.Middleware(),
		),
	)
	if err != nil { log.Fatal(err) }

	logger.Info("Starting server", zap.String("address", address))
	if err := server.ListenAndServe(); err != nil { logger.Error("Failed to run server", zap.Error(err)) }
}

func authHandler(logger *zap.Logger, service forum.UserService) ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		username := ctx.User()
		logger.With(
			zap.String("username", username),
			zap.String("session_id", ctx.SessionID()),
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("client_version", ctx.ClientVersion()),
		).Info("Authenticating user")
		if username == forum.SignupUsername { return true }
		return service.AuthenticateUser(username, key)
	}
}

func loggingMiddleware(logger *zap.Logger) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			started := time.Now()
			pty, _, _ := sess.Pty()
			logger.Info("Connected", zap.String("user", sess.User()), zap.String("remote-addr", sess.RemoteAddr().String()),
				zap.String("term", pty.Term), zap.Int("width", pty.Window.Width), zap.Int("height", pty.Window.Height),
				zap.String("client-version", sess.Context().ClientVersion()))
			next(sess)
			logger.Info("Disconnected", zap.String("user", sess.User()), zap.String("remote-addr", sess.RemoteAddr().String()),
				zap.Duration("duration", time.Since(started)))
		}
	}
}

func signupMiddleware(logger *zap.Logger, service forum.UserService) func(next ssh.Handler) ssh.Handler {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			if sess.User() != forum.SignupUsername { next(sess); return }
			if err := signup.Handler(logger, sess, service); err != nil {
				fmt.Fprintf(sess, "Error: %s\r\n", err)
				_ = sess.Exit(1)
				return
			}
			fmt.Fprintln(sess, "User created successfully")
		}
	}
}
