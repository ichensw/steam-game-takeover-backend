package httpapi

import (
	"strings"
	"testing"
)

func TestNormalizeReportImageURLsUsesImageURLsFirst(t *testing.T) {
	got, err := normalizeReportImageURLs([]string{
		" https://example.com/1.png ",
		"https://example.com/2.png",
	})
	if err != nil {
		t.Fatalf("normalizeReportImageURLs() error = %v", err)
	}

	want := []string{"https://example.com/1.png", "https://example.com/2.png"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normalizeReportImageURLs() = %#v, want %#v", got, want)
	}
}

func TestNormalizeReportImageURLsAllowsNoImages(t *testing.T) {
	got, err := normalizeReportImageURLs(nil)
	if err != nil {
		t.Fatalf("normalizeReportImageURLs() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("normalizeReportImageURLs() = %#v, want empty", got)
	}
}

func TestNormalizeReportImageURLsRejectsInvalidImages(t *testing.T) {
	cases := []struct {
		name      string
		imageURLs []string
	}{
		{
			name:      "too many",
			imageURLs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
		},
		{
			name:      "empty in array",
			imageURLs: []string{"https://example.com/1.png", " "},
		},
		{
			name:      "too long",
			imageURLs: []string{strings.Repeat("a", 513)},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeReportImageURLs(tt.imageURLs); err == nil {
				t.Fatal("normalizeReportImageURLs() error = nil, want error")
			}
		})
	}
}

func TestReportImageURLsUsesJSON(t *testing.T) {
	imageURLsJSON := `["https://example.com/1.png","https://example.com/2.png"]`

	got := reportImageURLs(&imageURLsJSON)
	want := []string{"https://example.com/1.png", "https://example.com/2.png"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("reportImageURLs() = %#v, want %#v", got, want)
	}
}

func TestReportImageURLsReturnsEmptyForBadJSON(t *testing.T) {
	badJSON := `not json`

	got := reportImageURLs(&badJSON)
	if len(got) != 0 {
		t.Fatalf("reportImageURLs() = %#v, want empty", got)
	}
}
