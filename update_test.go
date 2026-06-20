package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRun_AlreadyLatest(t *testing.T) {
	ts := latestVersionServer("v1.5.0")
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	cfg := Config{
		BinaryName:     "bai",
		CurrentVersion: "v1.5.0",
		GitHubOwner:    "bluefunda",
		GitHubRepo:     "bluefunda-ai",
		HomebrewCask:   "bai",
	}
	if err := Run(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_UpdateAvailable_Decline(t *testing.T) {
	ts := latestVersionServer("v1.6.0")
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	// Simulate user typing "n"
	origStdin := stdin
	stdin = strings.NewReader("n\n")
	defer func() { stdin = origStdin }()

	cfg := Config{
		BinaryName:     "bai",
		CurrentVersion: "v1.5.0",
		GitHubOwner:    "bluefunda",
		GitHubRepo:     "bluefunda-ai",
		HomebrewCask:   "bai",
	}
	if err := Run(cfg); err != nil {
		t.Errorf("unexpected error on decline: %v", err)
	}
}

func TestRun_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	cfg := Config{
		BinaryName:     "bai",
		CurrentVersion: "v1.5.0",
		GitHubOwner:    "bluefunda",
		GitHubRepo:     "bluefunda-ai",
	}
	if err := Run(cfg); err == nil {
		t.Error("expected error from API failure, got nil")
	}
}

func TestRun_DevVersion_AlwaysOffersUpdate(t *testing.T) {
	ts := latestVersionServer("v1.0.0")
	defer ts.Close()

	origAPI := githubAPI
	githubAPI = ts.URL
	defer func() { githubAPI = origAPI }()

	// dev < v1.0.0, so update should be offered; decline it
	origStdin := stdin
	stdin = strings.NewReader("n\n")
	defer func() { stdin = origStdin }()

	cfg := Config{
		BinaryName:     "bai",
		CurrentVersion: "dev",
		GitHubOwner:    "bluefunda",
		GitHubRepo:     "bluefunda-ai",
	}
	if err := Run(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// -- helpers --

func latestVersionServer(tag string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"tag_name": tag,
			"html_url": "https://github.com/example/repo/releases/tag/" + tag,
		})
	}))
}
