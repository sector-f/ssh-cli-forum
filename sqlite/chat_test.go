package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func createChatTestUser(t *testing.T, service *Service) int {
	t.Helper()
	if _, err := service.db.Exec("INSERT INTO invites(code, created_at, expires_at) VALUES('chat-invite', ?, ?)",
		nowText(), "9999-12-31T23:59:59Z"); err != nil {
		t.Fatal(err)
	}
	result, err := service.db.Exec("INSERT INTO users(username, pubkey, invite_code, status, created_at) VALUES('alice', x'01', 'chat-invite', 'active', ?)", nowText())
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}

func TestChatMessagesPersistAndPublish(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	service, err := Open(zap.NewNop(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	userID := createChatTestUser(t, service)

	events, cancel := service.Subscribe(context.Background(), forum.GlobalChatChannel)
	defer cancel()

	for i := 0; i < 105; i++ {
		kind := forum.ChatMessageText
		body := fmt.Sprintf("message %03d", i)
		if i == 104 {
			kind = forum.ChatMessageAction
			body = "waves"
		}
		if _, err := service.PostMessage(forum.GlobalChatChannel, userID, kind, body); err != nil {
			t.Fatalf("post message %d: %v", i, err)
		}
	}

	select {
	case event := <-events:
		if event.ID != 1 || event.Text != "message 000" {
			t.Fatalf("unexpected live event: %+v", event)
		}
	default:
		t.Fatal("subscriber did not receive persisted message")
	}

	history, err := service.RecentMessages(forum.GlobalChatChannel, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 100 {
		t.Fatalf("history length = %d, want 100", len(history))
	}
	if history[0].Text != "message 005" || history[99].Text != "waves" {
		t.Fatalf("history is not the latest 100 messages in chronological order: first=%+v last=%+v", history[0], history[99])
	}
	if history[99].Kind != forum.ChatMessageAction || history[99].Username != "alice" {
		t.Fatalf("action message was not stored correctly: %+v", history[99])
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	service, err = Open(zap.NewNop(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	history, err = service.RecentMessages(forum.GlobalChatChannel, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 100 || history[0].Text != "message 005" {
		t.Fatalf("chat history did not survive reopen: %+v", history)
	}
}

func TestChatRejectsInvalidMessagesAndUnknownChannels(t *testing.T) {
	service, err := Open(zap.NewNop(), filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	userID := createChatTestUser(t, service)

	if _, err := service.PostMessage(forum.GlobalChatChannel, userID, forum.ChatMessageText, "   "); !errors.Is(err, forum.ErrChatMessageEmpty) {
		t.Fatalf("empty message error = %v", err)
	}
	if _, err := service.PostMessage(forum.GlobalChatChannel, userID, forum.ChatMessageText, "line one\nline two"); !errors.Is(err, forum.ErrChatMessageInvalid) {
		t.Fatalf("multiline message error = %v", err)
	}
	if _, err := service.PostMessage(forum.GlobalChatChannel, userID, forum.ChatMessageText, strings.Repeat("x", forum.ChatMessageMaxLength+1)); !errors.Is(err, forum.ErrChatMessageTooLong) {
		t.Fatalf("overlong message error = %v", err)
	}
	if _, err := service.PostMessage("missing-channel", userID, forum.ChatMessageText, "hello"); !errors.Is(err, forum.ErrChatChannelNotFound) {
		t.Fatalf("unknown channel error = %v", err)
	}
}

func TestChatSubscriptionIsContextBound(t *testing.T) {
	service, err := Open(zap.NewNop(), filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	ctx, cancelContext := context.WithCancel(context.Background())
	events, _ := service.Subscribe(ctx, forum.GlobalChatChannel)
	cancelContext()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("subscription remained open after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not close after context cancellation")
	}
}
