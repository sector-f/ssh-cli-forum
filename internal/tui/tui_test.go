package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestSplitCommandQuotedArguments(t *testing.T) {
	got, err := splitCommand(`board create general "General Discussion" "All topics here"`)
	if err != nil { t.Fatal(err) }
	want := []string{"board", "create", "general", "General Discussion", "All topics here"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("splitCommand() = %#v, want %#v", got, want) }
}

func TestSplitCommandRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommand(`board create general "General Discussion`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
}

func TestFormatChatMessage(t *testing.T) {
	message := service.ChatMessage{
		Username: "alice",
		Kind: service.ChatMessageText,
		Text: "hello\x1b[31m",
		Timestamp: time.Date(2026, 11, 27, 12, 34, 0, 0, time.Local),
	}
	got := formatChatMessage(message)
	if !strings.Contains(got, "alice: hello[31m") || strings.ContainsRune(got, '\x1b') {
		t.Fatalf("chat message was not rendered safely: %q", got)
	}

	message.Kind = service.ChatMessageAction
	message.Text = "waves"
	got = formatChatMessage(message)
	if !strings.Contains(got, "* alice waves") {
		t.Fatalf("action message was not formatted correctly: %q", got)
	}
}
