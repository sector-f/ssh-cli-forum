package service

import (
	"context"
	"sync"

)

const subscriberBuffer = 128

// Hub distributes newly persisted chat messages to interested consumers.
// SQLite remains the source of truth; this in-memory hub is only a live-event
// path. Subscribers are isolated so a slow terminal cannot block message writes.
type ChatHub struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[string]map[uint64]chan ChatMessage
}

func NewChatHub() *ChatHub {
	return &ChatHub{subscribers: make(map[string]map[uint64]chan ChatMessage)}
}

// Subscribe registers for one channel. The subscription ends when cancel is
// called or ctx is cancelled. Consumers can subscribe independently of any
// particular UI view, which also permits future background notifications.
func (h *ChatHub) Subscribe(ctx context.Context, channel string) (<-chan ChatMessage, func()) {
	ch := make(chan ChatMessage, subscriberBuffer)

	h.mu.Lock()
	h.nextID++
	id := h.nextID
	if h.subscribers[channel] == nil {
		h.subscribers[channel] = make(map[uint64]chan ChatMessage)
	}
	h.subscribers[channel][id] = ch
	h.mu.Unlock()

	done := make(chan struct{})
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if channelSubscribers := h.subscribers[channel]; channelSubscribers != nil {
				if subscriber, ok := channelSubscribers[id]; ok {
					delete(channelSubscribers, id)
					close(subscriber)
				}
				if len(channelSubscribers) == 0 {
					delete(h.subscribers, channel)
				}
			}
			h.mu.Unlock()
			close(done)
		})
	}

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-done:
			}
		}()
	}

	return ch, cancel
}

// Publish is intentionally non-blocking. If a consumer falls behind, its
// oldest queued event is discarded; it can recover from durable history.
func (h *ChatHub) Publish(message ChatMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, subscriber := range h.subscribers[message.ChannelSlug] {
		select {
		case subscriber <- message:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- message:
			default:
			}
		}
	}
}
