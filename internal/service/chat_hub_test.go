package service

import (
	"context"
	"testing"
	"time"
)

func TestHubPublishesOnlyToMatchingChannel(t *testing.T) {
	hub := NewChatHub()
	global, cancelGlobal := hub.Subscribe(context.Background(), GlobalChatChannel)
	defer cancelGlobal()
	other, cancelOther := hub.Subscribe(context.Background(), "development")
	defer cancelOther()

	message := ChatMessage{ID: 1, ChannelSlug: GlobalChatChannel, Text: "hello"}
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
	hub := NewChatHub()
	ctx, cancelContext := context.WithCancel(context.Background())
	events, cancel := hub.Subscribe(ctx, GlobalChatChannel)
	cancel()
	cancel()
	if _, ok := <-events; ok {
		t.Fatal("cancelled subscription remained open")
	}

	events, _ = hub.Subscribe(ctx, GlobalChatChannel)
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
