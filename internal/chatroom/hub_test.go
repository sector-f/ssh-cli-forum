package chatroom

import (
	"context"
	"testing"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func TestHubPublishesOnlyToMatchingChannel(t *testing.T) {
	hub := NewHub()
	global, cancelGlobal := hub.Subscribe(context.Background(), forum.GlobalChatChannel)
	defer cancelGlobal()
	other, cancelOther := hub.Subscribe(context.Background(), "development")
	defer cancelOther()

	message := forum.ChatMessage{ID: 1, ChannelSlug: forum.GlobalChatChannel, Text: "hello"}
	hub.Publish(message)

	select {
	case got := <-global:
		if got.ID != message.ID || got.Text != message.Text {
			t.Fatalf("got %+v, want %+v", got, message)
		}
	case <-time.After(time.Second):
		t.Fatal("global subscriber did not receive message")
	}

	select {
	case got := <-other:
		t.Fatalf("unrelated channel received message: %+v", got)
	default:
	}
}

func TestHubCancelClosesSubscriptionAndContextCancels(t *testing.T) {
	hub := NewHub()
	ctx, cancelContext := context.WithCancel(context.Background())
	events, cancel := hub.Subscribe(ctx, forum.GlobalChatChannel)
	cancel()
	cancel()
	if _, ok := <-events; ok {
		t.Fatal("cancelled subscription remained open")
	}

	events, _ = hub.Subscribe(ctx, forum.GlobalChatChannel)
	cancelContext()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("context-cancelled subscription remained open")
		}
	case <-time.After(time.Second):
		t.Fatal("context did not cancel subscription")
	}
}
