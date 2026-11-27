package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
	"github.com/sector-f/ssh-cli-forum/internal/service"
	"github.com/sector-f/ssh-cli-forum/internal/storage/sqlite"
	"github.com/sector-f/ssh-cli-forum/internal/tui"
	"go.uber.org/zap"
)

const (
	defaultAddress = "127.0.0.1:2222"
	defaultDBPath  = "./forum.db"
)

// Run initializes the application and serves SSH sessions until the server
// stops. Runtime construction stays here so the command entry point remains
// small and storage implementations can be replaced without changing the TUI.
func Run() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer logger.Sync()

	store, err := sqlite.Open(defaultDBPath)
	if err != nil {
		return fmt.Errorf("open SQLite storage: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Warn("Failed to close storage", zap.Error(err))
		}
	}()

	chat := service.NewChatService(store)

	server, err := wish.NewServer(
		wish.WithAddress(defaultAddress),
		wish.WithPublicKeyAuth(authHandler(logger, store)),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(tui.Handler(store, store, chat), termenv.Ascii),
			signupMiddleware(logger, store),
			loggingMiddleware(logger),
			activeterm.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("create SSH server: %w", err)
	}

	logger.Info("Starting server", zap.String("address", defaultAddress))
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Failed to run server", zap.Error(err))
		return fmt.Errorf("run SSH server: %w", err)
	}
	return nil
}

func authHandler(logger *zap.Logger, users service.UserService) ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		username := ctx.User()
		logger.With(
			zap.String("username", username),
			zap.String("session_id", ctx.SessionID()),
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("client_version", ctx.ClientVersion()),
		).Info("Authenticating user")
		if username == service.SignupUsername {
			return true
		}
		return users.AuthenticateUser(username, key)
	}
}

func loggingMiddleware(logger *zap.Logger) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			started := time.Now()
			pty, _, _ := sess.Pty()
			logger.Info("Connected",
				zap.String("user", sess.User()),
				zap.String("remote-addr", sess.RemoteAddr().String()),
				zap.String("term", pty.Term),
				zap.Int("width", pty.Window.Width),
				zap.Int("height", pty.Window.Height),
				zap.String("client-version", sess.Context().ClientVersion()),
			)
			next(sess)
			logger.Info("Disconnected",
				zap.String("user", sess.User()),
				zap.String("remote-addr", sess.RemoteAddr().String()),
				zap.Duration("duration", time.Since(started)),
			)
		}
	}
}

func signupMiddleware(logger *zap.Logger, users service.UserService) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			if sess.User() != service.SignupUsername {
				next(sess)
				return
			}
			if err := handleSignup(logger, sess, users); err != nil {
				fmt.Fprintf(sess, "Error: %s\r\n", err)
				_ = sess.Exit(1)
				return
			}
			fmt.Fprintln(sess, "User created successfully")
		}
	}
}
