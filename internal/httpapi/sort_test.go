package httpapi

import (
	"strings"
	"testing"
)

func TestSortClause(t *testing.T) {
	allowed := map[string]string{"createdAt": "gmt_create", "nickname": "nickname"}
	if got := sortClause("nickname", "asc", allowed, "gmt_create"); got != "nickname ASC" {
		t.Fatalf("sortClause() = %q, want nickname ASC", got)
	}
	if got := sortClause("bad", "desc", allowed, "gmt_create"); got != "gmt_create DESC" {
		t.Fatalf("sortClause() = %q, want fallback DESC", got)
	}
}

func TestWXUserPublishWhitelistSortClause(t *testing.T) {
	got := wxUserPublishWhitelistSortClause("asc")
	if !strings.Contains(got, "ttw_publish_takeover_whitelist") {
		t.Fatalf("sort clause missing whitelist table: %q", got)
	}
	if !strings.HasSuffix(got, "ASC") {
		t.Fatalf("sort clause = %q, want ASC direction", got)
	}
}
