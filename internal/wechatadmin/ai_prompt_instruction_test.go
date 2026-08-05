package wechatadmin

import (
	"strings"
	"testing"
)

func TestValidAIPromptInstructionKey(t *testing.T) {
	for _, key := range aiPromptInstructionKeys {
		if !validAIPromptInstructionKey(key) {
			t.Fatalf("expected key %q to be allowed", key)
		}
	}
	if validAIPromptInstructionKey("not_a_template") {
		t.Fatal("unexpected prompt instruction key allowed")
	}
}

func TestDefaultAIPromptInstructionsCoverEveryAllowedKey(t *testing.T) {
	for _, key := range aiPromptInstructionKeys {
		if strings.TrimSpace(defaultAIPromptInstructions[key]) == "" {
			t.Fatalf("missing default instruction for %q", key)
		}
	}
}

func TestObsoleteAIPromptInstructionsAreNotExposed(t *testing.T) {
	for _, key := range obsoleteAIPromptInstructionKeys {
		if validAIPromptInstructionKey(key) {
			t.Fatalf("obsolete instruction %q must not be editable", key)
		}
		if _, ok := defaultAIPromptInstructions[key]; ok {
			t.Fatalf("obsolete instruction %q must not have a default", key)
		}
	}
}
