package service

import "context"

// chatService composes durable chat storage with in-process live delivery.
// Persisted messages are published only after the storage transaction commits.
type chatService struct {
	store ChatStore
	hub   *ChatHub
}

func NewChatService(store ChatStore) ChatService {
	return &chatService{store: store, hub: NewChatHub()}
}

func (s *chatService) RecentMessages(ctx context.Context, channelSlug string, limit int) ([]ChatMessage, error) {
	return s.store.RecentMessages(ctx, channelSlug, limit)
}

func (s *chatService) PostMessage(ctx context.Context, channelSlug string, userID int, kind ChatMessageKind, body string) (ChatMessage, error) {
	message, err := s.store.PostMessage(ctx, channelSlug, userID, kind, body)
	if err != nil {
		return ChatMessage{}, err
	}
	s.hub.Publish(message)
	return message, nil
}

func (s *chatService) Subscribe(ctx context.Context, channelSlug string) (<-chan ChatMessage, func()) {
	return s.hub.Subscribe(ctx, channelSlug)
}
