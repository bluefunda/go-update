package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"tag_name": "v1.5.0",
			"html_url": "https://github.com/owner/repo/releases/tag/v1.5.0",
		})
	}))
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	rel, err := fetchLatest("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.5.0" {
		t.Errorf("got tag %q, want %q", rel.TagName, "v1.5.0")
	}
}

func TestFetchLatest_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	_, err := fetchLatest("owner", "repo")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestFetchLatest_EmptyTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": ""})
	}))
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	_, err := fetchLatest("owner", "repo")
	if err == nil {
		t.Fatal("expected error for empty tag_name, got nil")
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"dev", "dev"},
		{"", ""},
		{"  v1.0.0  ", "v1.0.0"},
	}
	for _, tc := range tests {
		if got := normalizeVersion(tc.input); got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNewerThan(t *testing.T) {
	tests := []struct {
		candidate string
		base      string
		want      bool
	}{
		{"v1.5.0", "v1.4.2", true},
		{"v1.4.2", "v1.5.0", false},
		{"v1.4.2", "v1.4.2", false},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.1", "v1.0.0", true},
		{"v1.0.0", "v1.0.1", false},
		{"v1.10.0", "v1.9.0", true},
	}
	for _, tc := range tests {
		got := newerThan(tc.candidate, tc.base)
		if got != tc.want {
			t.Errorf("newerThan(%q, %q) = %v, want %v", tc.candidate, tc.base, got, tc.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"v1.2.3", [3]int{1, 2, 3}},
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.0.0-rc.1", [3]int{1, 0, 0}},
		{"dev", [3]int{0, 0, 0}},
		{"v10.20.30", [3]int{10, 20, 30}},
	}
	for _, tc := range tests {
		got := parseSemver(tc.input)
		if got != tc.want {
			t.Errorf("parseSemver(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
