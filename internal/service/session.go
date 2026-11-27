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

func (s *Session) requireModerator(ctx context.Context) (User, error) {
	user, err := s.actor(ctx)
	if err != nil {
		return User{}, err
	}
	if user.Role != RoleModerator && user.Role != RoleAdmin {
		return User{}, ErrPermissionDenied
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

func (s *Session) CreateInvite(ctx context.Context, code string, expiresAt time.Time, isAdmin bool) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return s.UserService.CreateInvite(ctx, code, expiresAt, isAdmin)
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

func (s *Session) SetUserRole(ctx context.Context, username, role string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return NewAccountService(s.UserService).SetUserRole(ctx, username, role)
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

func (s *Session) UpdateBoard(ctx context.Context, slug, name, description string) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	return NewBoardManager(s.BoardService).UpdateBoard(ctx, slug, name, description)
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
	thread, err := s.BoardService.GetThreadByID(ctx, id)
	if err != nil {
		return Thread{}, err
	}
	if thread.Status == ThreadHidden {
		user, err := s.actor(ctx)
		if err != nil {
			return Thread{}, err
		}
		if user.Role != RoleAdmin && user.Role != RoleModerator {
			return Thread{}, ErrThreadNotFound
		}
	}
	return thread, nil
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
	if thread.Status == ThreadLocked {
		return ErrThreadLocked
	}
	if thread.Status == ThreadHidden {
		return ErrThreadNotFound
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrEmptyCommentBody
	}
	return s.BoardService.CreateComment(ctx, threadID, user.ID, body)
}

func (s *Session) UpdateThread(ctx context.Context, id int, title, body string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrEmptyThreadTitle
	}
	user, err := s.actor(ctx)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(ctx, id)
	if err != nil {
		return err
	}
	moderator := user.Role == RoleAdmin || user.Role == RoleModerator
	if thread.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.UpdateThread(ctx, id, title, body)
}

func (s *Session) DeleteThread(ctx context.Context, id, actorID int, reason string) error {
	user, err := s.requireActorID(ctx, actorID)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(ctx, id)
	if err != nil {
		return err
	}
	moderator := user.Role == RoleAdmin || user.Role == RoleModerator
	if thread.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteThread(ctx, id, user.ID, reason)
}

func (s *Session) SetThreadPinned(ctx context.Context, id, actorID int, pinned bool, reason string) error {
	user, err := s.requireActorID(ctx, actorID)
	if err != nil {
		return err
	}
	if user.Role != RoleAdmin && user.Role != RoleModerator {
		return ErrPermissionDenied
	}
	return s.BoardService.SetThreadPinned(ctx, id, user.ID, pinned, reason)
}

func (s *Session) SetThreadStatus(ctx context.Context, id, actorID int, status, reason string) error {
	if status != ThreadOpen && status != ThreadLocked && status != ThreadHidden {
		return ErrInvalidThreadStatus
	}
	user, err := s.requireActorID(ctx, actorID)
	if err != nil {
		return err
	}
	if user.Role != RoleAdmin && user.Role != RoleModerator {
		return ErrPermissionDenied
	}
	return s.BoardService.SetThreadStatus(ctx, id, user.ID, status, reason)
}

func (s *Session) UpdateComment(ctx context.Context, id int, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrEmptyCommentBody
	}
	user, err := s.actor(ctx)
	if err != nil {
		return err
	}
	comment, err := s.BoardService.GetCommentByID(ctx, id)
	if err != nil {
		return err
	}
	moderator := user.Role == RoleAdmin || user.Role == RoleModerator
	if comment.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.UpdateComment(ctx, id, body)
}

func (s *Session) DeleteComment(ctx context.Context, id, actorID int, reason string) error {
	user, err := s.requireActorID(ctx, actorID)
	if err != nil {
		return err
	}
	comment, err := s.BoardService.GetCommentByID(ctx, id)
	if err != nil {
		return err
	}
	moderator := user.Role == RoleAdmin || user.Role == RoleModerator
	if comment.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteComment(ctx, id, user.ID, reason)
}

func (s *Session) ListModerationLog(ctx context.Context, limit int) ([]ModerationEntry, error) {
	if _, err := s.requireModerator(ctx); err != nil {
		return nil, err
	}
	return s.BoardService.ListModerationLog(ctx, limit)
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
