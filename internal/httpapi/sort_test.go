package httpapi

import "testing"

func TestSortClause(t *testing.T) {
	allowed := map[string]string{"createdAt": "gmt_create", "nickname": "nickname"}
	if got := sortClause("nickname", "asc", allowed, "gmt_create"); got != "nickname ASC" {
		t.Fatalf("sortClause() = %q, want nickname ASC", got)
	}
	if got := sortClause("bad", "desc", allowed, "gmt_create"); got != "gmt_create DESC" {
		t.Fatalf("sortClause() = %q, want fallback DESC", got)
	}
}
