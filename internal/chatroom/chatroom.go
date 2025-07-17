package chatroom

import (
	"sync"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type Server struct {
	chans  map[string]chan forum.Message
	logger *zap.Logger

	mu sync.Mutex
}

func NewServer(logger *zap.Logger) *Server {
	s := &Server{
		chans:  make(map[string]chan forum.Message),
		logger: logger,
	}

	return s
}

func (s *Server) Connect(id string) chan forum.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Client connected", zap.String("session_id", id))

	for _, ch := range s.chans {
		msg := forum.SystemMsg{Timestamp: time.Now(), Text: "User has joined"}
		ch <- msg
	}

	ch := make(chan forum.Message)
	s.chans[id] = ch
	return ch
}

func (s *Server) Disconnect(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Client disconnected", zap.String("session_id", id))
	delete(s.chans, id)

	for _, ch := range s.chans {
		msg := forum.SystemMsg{Timestamp: time.Now(), Text: "User has quit"}
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
