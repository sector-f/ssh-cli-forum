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

func (s *Session) actor() (User, error) {
	if s.UserService == nil || s.actorID <= 0 {
		return User{}, ErrPermissionDenied
	}
	user, err := s.UserService.GetUserByID(s.actorID)
	if err != nil {
		return User{}, fmt.Errorf("load acting user: %w", err)
	}
	if user.Status == StatusSuspended {
		return User{}, ErrUserSuspended
	}
	return user, nil
}

func (s *Session) requireModerator() (User, error) {
	user, err := s.actor()
	if err != nil {
		return User{}, err
	}
	if !user.IsAdmin && user.Role != RoleModerator && user.Role != RoleAdmin {
		return User{}, ErrPermissionDenied
	}
	return user, nil
}

func (s *Session) requireAdmin() (User, error) {
	user, err := s.actor()
	if err != nil {
		return User{}, err
	}
	if !user.IsAdmin && user.Role != RoleAdmin {
		return User{}, ErrPermissionDenied
	}
	return user, nil
}

func (s *Session) requireActorID(actorID int) (User, error) {
	user, err := s.actor()
	if err != nil {
		return User{}, err
	}
	if actorID != user.ID {
		return User{}, ErrPermissionDenied
	}
	return user, nil
}

func (s *Session) CreateInvite(code string, expiresAt time.Time, isAdmin bool) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	return s.UserService.CreateInvite(code, expiresAt, isAdmin)
}

func (s *Session) ListInvites() ([]Invite, error) {
	if _, err := s.requireAdmin(); err != nil {
		return nil, err
	}
	return s.UserService.ListInvites()
}

func (s *Session) RevokeInvite(code string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	return s.UserService.RevokeInvite(code)
}

func (s *Session) ListUsers() ([]User, error) {
	if _, err := s.requireAdmin(); err != nil {
		return nil, err
	}
	return s.UserService.ListUsers()
}

func (s *Session) SetUserRole(username, role string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	target, err := s.UserService.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin || target.IsAdmin) && role != RoleAdmin {
		users, err := s.UserService.ListUsers()
		if err != nil {
			return err
		}
		admins := 0
		for _, user := range users {
			if user.Role == RoleAdmin || user.IsAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return errors.New("cannot demote the last administrator")
		}
	}
	return s.UserService.SetUserRole(username, role)
}

func (s *Session) SetUserStatus(username, status string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	target, err := s.UserService.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin || target.IsAdmin) && status == StatusSuspended {
		users, err := s.UserService.ListUsers()
		if err != nil {
			return err
		}
		activeAdmins := 0
		for _, user := range users {
			if (user.Role == RoleAdmin || user.IsAdmin) && user.Status != StatusSuspended {
				activeAdmins++
			}
		}
		if activeAdmins <= 1 {
			return errors.New("cannot suspend the last active administrator")
		}
	}
	return s.UserService.SetUserStatus(username, status)
}

func (s *Session) CreateBoard(name string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	return s.BoardService.CreateBoard(name)
}

func (s *Session) CreateBoardWithDetails(slug, name, description string, sortOrder, createdBy int) error {
	user, err := s.requireAdmin()
	if err != nil {
		return err
	}
	if createdBy != 0 && createdBy != user.ID {
		return ErrPermissionDenied
	}
	return s.BoardService.CreateBoardWithDetails(slug, name, description, sortOrder, user.ID)
}

func (s *Session) UpdateBoard(slug, name, description string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	return s.BoardService.UpdateBoard(slug, name, description)
}

func (s *Session) SetBoardStatus(slug, status string) error {
	if _, err := s.requireAdmin(); err != nil {
		return err
	}
	return s.BoardService.SetBoardStatus(slug, status)
}

func (s *Session) CreateThread(title string, boardID, authorID int) error {
	_, err := s.CreateThreadWithBody(title, "", boardID, authorID)
	return err
}

func (s *Session) CreateThreadWithBody(title, body string, boardID, authorID int) (int, error) {
	user, err := s.requireActorID(authorID)
	if err != nil {
		return 0, err
	}
	board, err := s.BoardService.GetBoardByID(boardID)
	if err != nil {
		return 0, err
	}
	if board.Status != BoardActive {
		return 0, ErrReadOnly
	}
	if strings.TrimSpace(title) == "" {
		return 0, errors.New("topic title cannot be empty")
	}
	return s.BoardService.CreateThreadWithBody(title, body, boardID, user.ID)
}

func (s *Session) CreateComment(threadID, authorID int, body string) error {
	user, err := s.requireActorID(authorID)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(threadID)
	if err != nil {
		return err
	}
	board, err := s.BoardService.GetBoardByID(thread.BoardID)
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
	if strings.TrimSpace(body) == "" {
		return errors.New("reply cannot be empty")
	}
	return s.BoardService.CreateComment(threadID, user.ID, body)
}

func (s *Session) UpdateThread(id, actorID int, title, body string, _ bool) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(id)
	if err != nil {
		return err
	}
	moderator := user.IsAdmin || user.Role == RoleAdmin || user.Role == RoleModerator
	if thread.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.UpdateThread(id, user.ID, title, body, moderator)
}

func (s *Session) DeleteThread(id, actorID int, _ bool, reason string) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	thread, err := s.BoardService.GetThreadByID(id)
	if err != nil {
		return err
	}
	moderator := user.IsAdmin || user.Role == RoleAdmin || user.Role == RoleModerator
	if thread.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteThread(id, user.ID, moderator, reason)
}

func (s *Session) SetThreadPinned(id, actorID int, pinned bool, reason string) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	if !user.IsAdmin && user.Role != RoleAdmin && user.Role != RoleModerator {
		return ErrPermissionDenied
	}
	return s.BoardService.SetThreadPinned(id, user.ID, pinned, reason)
}

func (s *Session) SetThreadStatus(id, actorID int, status, reason string) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	if !user.IsAdmin && user.Role != RoleAdmin && user.Role != RoleModerator {
		return ErrPermissionDenied
	}
	return s.BoardService.SetThreadStatus(id, user.ID, status, reason)
}

func (s *Session) UpdateComment(id, actorID int, body string, _ bool) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	comment, err := s.BoardService.GetCommentByID(id)
	if err != nil {
		return err
	}
	moderator := user.IsAdmin || user.Role == RoleAdmin || user.Role == RoleModerator
	if comment.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.UpdateComment(id, user.ID, body, moderator)
}

func (s *Session) DeleteComment(id, actorID int, _ bool, reason string) error {
	user, err := s.requireActorID(actorID)
	if err != nil {
		return err
	}
	comment, err := s.BoardService.GetCommentByID(id)
	if err != nil {
		return err
	}
	moderator := user.IsAdmin || user.Role == RoleAdmin || user.Role == RoleModerator
	if comment.Author.ID != user.ID && !moderator {
		return ErrPermissionDenied
	}
	return s.BoardService.DeleteComment(id, user.ID, moderator, reason)
}

func (s *Session) ListModerationLog(limit int) ([]ModerationEntry, error) {
	if _, err := s.requireModerator(); err != nil {
		return nil, err
	}
	return s.BoardService.ListModerationLog(limit)
}

func (s *Session) RecentMessages(channelSlug string, limit int) ([]ChatMessage, error) {
	if s.ChatService == nil {
		return nil, errors.New("chat is unavailable")
	}
	if _, err := s.actor(); err != nil {
		return nil, err
	}
	return s.ChatService.RecentMessages(channelSlug, limit)
}

func (s *Session) PostMessage(channelSlug string, userID int, kind ChatMessageKind, body string) (ChatMessage, error) {
	if s.ChatService == nil {
		return ChatMessage{}, errors.New("chat is unavailable")
	}
	user, err := s.requireActorID(userID)
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
	return s.ChatService.PostMessage(channelSlug, user.ID, kind, body)
}

func (s *Session) Subscribe(ctx context.Context, channelSlug string) (<-chan ChatMessage, func()) {
	if s.ChatService == nil {
		ch := make(chan ChatMessage)
		close(ch)
		return ch, func() {}
	}
	if _, err := s.actor(); err != nil {
		ch := make(chan ChatMessage)
		close(ch)
		return ch, func() {}
	}
	return s.ChatService.Subscribe(ctx, channelSlug)
}
