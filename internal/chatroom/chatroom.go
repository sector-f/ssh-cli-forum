package chatroom

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type Server struct {
	sessions     map[string]ssh.Session
	userFuncs    map[string]func(forum.Message)
	serviceFuncs map[string]func(forum.Message)

	limit      int
	scrollback []forum.Message

	logger *zap.Logger
	mu     sync.Mutex
}

func NewServer(logger *zap.Logger) *Server {
	sbLimit := 100

	s := &Server{
		sessions:     make(map[string]ssh.Session),
		userFuncs:    make(map[string]func(forum.Message)),
		serviceFuncs: make(map[string]func(forum.Message)),
		limit:        sbLimit,
		scrollback:   make([]forum.Message, 0, sbLimit),
		logger:       logger,
	}

	return s
}

func (s *Server) ConnectUser(session ssh.Session, callback func(forum.Message)) []forum.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := session.Context().SessionID()

	s.logger.Info("Client connected", zap.String("session_id", id))

	msg := forum.SystemMsg{
		Timestamp: time.Now(),
		Text:      fmt.Sprintf("%s has joined", session.User()),
	}

	for _, f := range s.serviceFuncs {
		f(msg)
	}
	for _, f := range s.userFuncs {
		f(msg)
	}
	s.storeMsg(msg)

	s.userFuncs[id] = callback
	s.sessions[id] = session
	scrollback := s.scrollback
	return scrollback
}

func (s *Server) ConnectService(id string, callback func(forum.Message)) []forum.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Service connected", zap.String("id", id))

	s.serviceFuncs[id] = callback
	scrollback := s.scrollback
	return scrollback
}

func (s *Server) DisconnectUser(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return
	}
	username := session.User()

	s.logger.Info("Client disconnected", zap.String("session_id", id))
	delete(s.userFuncs, id)
	delete(s.sessions, id)

	msg := forum.SystemMsg{
		Timestamp: time.Now(),
		Text:      fmt.Sprintf("%s has quit", username),
	}

	for _, f := range s.serviceFuncs {
		f(msg)
	}
	for _, f := range s.userFuncs {
		f(msg)
	}

	s.storeMsg(msg)
}

func (s *Server) DisconnectService(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.serviceFuncs, id)
}

func (s *Server) SendMsg(msg forum.ChatMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, f := range s.serviceFuncs {
		f(msg)
	}
	for _, f := range s.userFuncs {
		f(msg)
	}

	s.logger.Debug("Chat msg sent", zap.String("msg", msg.Body()))
	s.storeMsg(msg)
}

// Caller must hold mutex
func (s *Server) storeMsg(msg forum.Message) {
	if len(s.scrollback) < s.limit {
		s.scrollback = append(s.scrollback, msg)
		return
	}

	// Reallocating on every message once we hit the limit is pretty ugly.
	// Replace with ring buffer?
	s.scrollback = append(s.scrollback[1:], msg)
}
