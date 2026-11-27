package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
	"go.uber.org/zap"
)

func createChatTestUser(t *testing.T, store *Service) int {
	t.Helper()
	if _, err := store.db.Exec("INSERT INTO invites(code, created_at, expires_at) VALUES('chat-invite', ?, ?)",
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
	store, err := Open(zap.NewNop(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	userID := createChatTestUser(t, store)

	events, cancel := store.Subscribe(context.Background(), service.GlobalChatChannel)
	defer cancel()

	for i := 0; i < 105; i++ {
		kind := service.ChatMessageText
		body := fmt.Sprintf("message %03d", i)
		if i == 104 {
			kind = service.ChatMessageAction
			body = "waves"
		}
		if _, err := store.PostMessage(service.GlobalChatChannel, userID, kind, body); err != nil {
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

	history, err := store.RecentMessages(service.GlobalChatChannel, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 100 {
		t.Fatalf("history length = %d, want 100", len(history))
	}
	if history[0].Text != "message 005" || history[99].Text != "waves" {
		t.Fatalf("history is not the latest 100 messages in chronological order: first=%+v last=%+v", history[0], history[99])
	}
	if history[99].Kind != service.ChatMessageAction || history[99].Username != "alice" {
		t.Fatalf("action message was not stored correctly: %+v", history[99])
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(zap.NewNop(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	history, err = store.RecentMessages(service.GlobalChatChannel, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 100 || history[0].Text != "message 005" {
		t.Fatalf("chat history did not survive reopen: %+v", history)
	}
}

func TestChatRejectsInvalidMessagesAndUnknownChannels(t *testing.T) {
	store, err := Open(zap.NewNop(), filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := createChatTestUser(t, store)

	if _, err := store.PostMessage(service.GlobalChatChannel, userID, service.ChatMessageText, "   "); !errors.Is(err, service.ErrChatMessageEmpty) {
		t.Fatalf("empty message error = %v", err)
	}
	if _, err := store.PostMessage(service.GlobalChatChannel, userID, service.ChatMessageText, "line one\nline two"); !errors.Is(err, service.ErrChatMessageInvalid) {
		t.Fatalf("multiline message error = %v", err)
	}
	if _, err := store.PostMessage(service.GlobalChatChannel, userID, service.ChatMessageText, strings.Repeat("x", service.ChatMessageMaxLength+1)); !errors.Is(err, service.ErrChatMessageTooLong) {
		t.Fatalf("overlong message error = %v", err)
	}
	if _, err := store.PostMessage("missing-channel", userID, service.ChatMessageText, "hello"); !errors.Is(err, service.ErrChatChannelNotFound) {
		t.Fatalf("unknown channel error = %v", err)
	}
}

func TestChatSubscriptionIsContextBound(t *testing.T) {
	store, err := Open(zap.NewNop(), filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx, cancelContext := context.WithCancel(context.Background())
	events, _ := store.Subscribe(ctx, service.GlobalChatChannel)
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
