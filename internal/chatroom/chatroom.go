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
	sessions map[string]ssh.Session
	chans    map[string]chan forum.Message
	logger   *zap.Logger

	mu sync.Mutex
}

func NewServer(logger *zap.Logger) *Server {
	s := &Server{
		sessions: make(map[string]ssh.Session),
		chans:    make(map[string]chan forum.Message),
		logger:   logger,
	}

	return s
}

func (s *Server) Connect(session ssh.Session) chan forum.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := session.Context().SessionID()

	s.logger.Info("Client connected", zap.String("session_id", id))

	for _, ch := range s.chans {
		msg := forum.SystemMsg{
			Timestamp: time.Now(),
			Text:      fmt.Sprintf("%s has joined", session.User()),
		}
		ch <- msg
	}

	ch := make(chan forum.Message)
	s.chans[id] = ch
	s.sessions[id] = session
	return ch
}

func (s *Server) Disconnect(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := s.sessions[id].User()

	s.logger.Info("Client disconnected", zap.String("session_id", id))
	delete(s.chans, id)
	delete(s.sessions, id)

	for _, ch := range s.chans {
		msg := forum.SystemMsg{
			Timestamp: time.Now(),
			Text:      fmt.Sprintf("%s has quit", username),
		}
		ch <- msg
	}
}

func (s *Server) SendMsg(msg forum.ChatMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("Chat msg sent", zap.String("msg", msg.Body()))
	for _, ch := range s.chans {
		ch <- msg
	}
}
