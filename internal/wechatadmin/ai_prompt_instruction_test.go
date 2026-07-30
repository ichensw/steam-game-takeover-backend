package wechatadmin

import "testing"

func TestValidAIPromptInstructionKey(t *testing.T) {
	for _, key := range aiPromptInstructionKeys {
		if !validAIPromptInstructionKey(key) {
			t.Fatalf("expected key %q to be allowed", key)
		}
	}
	if validAIPromptInstructionKey("reply_system") {
		t.Fatal("unexpected prompt instruction key allowed")
	}
}
