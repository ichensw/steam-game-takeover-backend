package httpapi

import (
	"testing"
	"time"
)

func TestKookDanmakuMessageFromPayload(t *testing.T) {
	payload := map[string]interface{}{
		"d": map[string]interface{}{
			"type":          float64(1),
			"target_id":     "voice-1",
			"content":       "hello",
			"msg_id":        "message-1",
			"msg_timestamp": float64(1710000000000),
			"extra": map[string]interface{}{
				"author": map[string]interface{}{"username": "Alice"},
			},
		},
	}

	message, ok := kookDanmakuMessageFromPayload(payload)
	if !ok {
		t.Fatal("message should be accepted")
	}
	if message.ChannelID != "voice-1" || message.AuthorName != "Alice" || message.Content != "hello" || message.Timestamp != 1710000000000 {
		t.Fatalf("message = %+v", message)
	}
}

func TestKookDanmakuMessageFromPayloadRejectsBotAndUnsupportedTypes(t *testing.T) {
	for _, payload := range []map[string]interface{}{
		{"type": float64(10), "target_id": "voice-1", "content": "card"},
		{"type": float64(1), "target_id": "voice-1", "content": "bot", "extra": map[string]interface{}{"author": map[string]interface{}{"bot": true}}},
	} {
		if _, ok := kookDanmakuMessageFromPayload(payload); ok {
			t.Fatalf("payload should be rejected: %#v", payload)
		}
	}
}

func TestKookDanmakuHubPublishesOnlyToTheTargetChannel(t *testing.T) {
	hub := newKookDanmakuHub()
	listener, unsubscribe := hub.subscribe("voice-1")
	defer unsubscribe()
	hub.publish(kookDanmakuMessage{ChannelID: "voice-2", Content: "ignored"})
	hub.publish(kookDanmakuMessage{ChannelID: "voice-1", Content: "shown"})

	select {
	case message := <-listener:
		if message.Content != "shown" {
			t.Fatalf("message = %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("expected target-channel message")
	}
}
