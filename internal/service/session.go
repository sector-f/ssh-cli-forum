package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Session scopes application operations to the authenticated SSH actor.
// The underlying interfaces are persistence-facing contracts; callers never
// supply a trusted role or ownership flag.
type Session struct {
	UserService
	BoardService
	ChatService

	actorID int
}

func NewSession(actorID int, users UserService, boards BoardService, chat ChatService) *Session {
	return &Session{
		UserService: users,
		BoardService: boards,
		ChatService: chat,
		actorID: actorID,
	}
}

func (s *Session) actor(ctx context.Context) (User, error) {
	if s.UserService == nil || s.actorID <= 0 {
		return User{}, ErrPermissionDenied
	}
	user, err := s.UserService.GetUserByID(ctx, s.actorID)
	if err != nil {
		return User{}, fmt.Errorf("load acting user: %w", err)
	}
	if user.Status == StatusSuspended {
		return User{}, ErrUserSuspended
	}
	return user, nil
}

func (s *Session) requireAdmin(ctx context.Context) (User, error) {
	user, err := s.actor(ctx)
	if err != nil {
		return User{}, err
	}
	if user.Role != RoleAdmin {
		return User{}, ErrPermissionDenied
	}
	return user, nil
}

func (s *Session) requireActorID(ctx context.Context, actorID int) (User, error) {
	user, err := s.actor(ctx)
	if err != nil {
		return User{}, err
	}
	if actorID != user.ID {
		return User{}, ErrPermissionDenied
	}
	return user, nil
}

func (s *Session) CreateInvite(ctx context.Context, code string, expiresAt time.Time) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return s.UserService.CreateInvite(ctx, code, expiresAt)
}

func (s *Session) ListInvites(ctx context.Context) ([]Invite, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.UserService.ListInvites(ctx)
}

func (s *Session) RevokeInvite(ctx context.Context, code string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return s.UserService.RevokeInvite(ctx, code)
}

func (s *Session) ListUsers(ctx context.Context) ([]User, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.UserService.ListUsers(ctx)
}

func (s *Session) SetUserStatus(ctx context.Context, username, status string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return NewAccountService(s.UserService).SetUserStatus(ctx, username, status)
}

func (s *Session) CreateBoard(ctx context.Context, name string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return NewBoardManager(s.BoardService).CreateBoard(ctx, name)
}

func (s *Session) CreateBoardWithDetails(ctx context.Context, slug, name, description string, sortOrder, createdBy int) error {
	user, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	if createdBy != 0 && createdBy != user.ID {
		return ErrPermissionDenied
	}
	return NewBoardManager(s.BoardService).CreateBoardWithDetails(ctx, slug, name, description, sortOrder, user.ID)
}

func (s *Session) SetBoardStatus(ctx context.Context, slug, status string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return NewBoardManager(s.BoardService).SetBoardStatus(ctx, slug, status)
}

func (s *Session) CreateThread(ctx context.Context, title string, boardID, authorID int) error {
	_, err := s.CreateThreadWithBody(ctx, title, "", boardID, authorID)
	return err
}

func (s *Session) CreateThreadWithBody(ctx context.Context, title, body string, boardID, authorID int) (int, error) {
	user, err := s.requireActorID(ctx, authorID)
	if err != nil {
		return 0, err
	}
	board, err := s.BoardService.GetBoardByID(ctx, boardID)
	if err != nil {
		return 0, err
	}
	if board.Status != BoardActive {
		return 0, ErrReadOnly
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, ErrEmptyThreadTitle
	}
	return s.BoardService.CreateThreadWithBody(ctx, title, body, boardID, user.ID)
}

func (s *Session) GetThreadByID(ctx context.Context, id int) (Thread, error) {
	return s.BoardService.GetThreadByID(ctx, id)
}

func (s *Session) CreateComment(ctx context.Context, threadID, authorID int, body string) error {
	user, err := s.requireActorID(ctx, authorID)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(ctx, threadID)
	if err != nil {
		return err
	}
	board, err := s.BoardService.GetBoardByID(ctx, thread.BoardID)
	if err != nil {
		return err
	}
	if board.Status != BoardActive {
		return ErrReadOnly
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrEmptyCommentBody
	}
	return s.BoardService.CreateComment(ctx, threadID, user.ID, body)
}

func (s *Session) DeleteThread(ctx context.Context, id int) error {
	user, err := s.actor(ctx)
	if err != nil {
		return err
	}
	item, err := s.BoardService.GetThreadByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Author.ID != user.ID {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteThread(ctx, id)
}
func (s *Session) DeleteComment(ctx context.Context, id int) error {
	user, err := s.actor(ctx)
	if err != nil {
		return err
	}
	item, err := s.BoardService.GetCommentByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Author.ID != user.ID {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteComment(ctx, id)
}
func (s *Session) RecentMessages(ctx context.Context, channelSlug string, limit int) ([]ChatMessage, error) {
	if s.ChatService == nil {
		return nil, errors.New("chat is unavailable")
	}
	if _, err := s.actor(ctx); err != nil {
		return nil, err
	}
	return s.ChatService.RecentMessages(ctx, channelSlug, limit)
}

func (s *Session) PostMessage(ctx context.Context, channelSlug string, userID int, kind ChatMessageKind, body string) (ChatMessage, error) {
	if s.ChatService == nil {
		return ChatMessage{}, errors.New("chat is unavailable")
	}
	user, err := s.requireActorID(ctx, userID)
	if err != nil {
		return ChatMessage{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return ChatMessage{}, ErrChatMessageEmpty
	}
	if len([]rune(body)) > ChatMessageMaxLength {
		return ChatMessage{}, ErrChatMessageTooLong
	}
	for _, r := range body {
		if unicode.IsControl(r) {
			return ChatMessage{}, ErrChatMessageInvalid
		}
	}
	if kind != ChatMessageText && kind != ChatMessageAction {
		return ChatMessage{}, ErrChatMessageInvalid
	}
	return s.ChatService.PostMessage(ctx, channelSlug, user.ID, kind, body)
}

func (s *Session) Subscribe(ctx context.Context, channelSlug string) (<-chan ChatMessage, func()) {
	if s.ChatService == nil {
		ch := make(chan ChatMessage)
		close(ch)
		return ch, func() {}
	}
	if _, err := s.actor(ctx); err != nil {
		ch := make(chan ChatMessage)
		close(ch)
		return ch, func() {}
	}
	return s.ChatService.Subscribe(ctx, channelSlug)
}

// Compile-time checks keep the session facade aligned with its public contracts.
var (
	_ UserSession  = (*Session)(nil)
	_ BoardSession = (*Session)(nil)
	_ ChatService  = (*Session)(nil)
)
