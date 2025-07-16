package sqlite

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func (s *Service) SendMessage(msg forum.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, program := range s.programs {
		program.Send(msg)
	}
}

func (s *Service) Register(id string, program *tea.Program) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("Registering *tea.Program", zap.String("session_id", id))
	s.programs[id] = program
}

func (s *Service) Unregister(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("Unregistering *tea.Program", zap.String("session_id", id))
	delete(s.programs, id)
}
