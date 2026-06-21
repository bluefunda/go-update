package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// githubAPI is the GitHub REST API base URL. Tests override this.
var githubAPI = "https://api.github.com"

// httpClient is the HTTP client used for all requests. Tests override this.
var httpClient = &http.Client{}

// githubRelease is the subset of the GitHub releases API response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// fetchLatest returns the latest release for the given GitHub repo.
func fetchLatest(owner, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPI, owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-update/0.1 (+github.com/bluefunda/go-update)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d for %s/%s releases/latest", resp.StatusCode, owner, repo)
	}

	var r githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parsing GitHub API response: %w", err)
	}
	if r.TagName == "" {
		return nil, fmt.Errorf("GitHub API returned empty tag_name for %s/%s", owner, repo)
	}
	return &r, nil
}

// normalizeVersion ensures a version string has a "v" prefix.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// newerThan returns true if candidate is strictly newer than base.
// Both should be normalised semver strings (with or without "v" prefix).
func newerThan(candidate, base string) bool {
	if candidate == base {
		return false
	}
	c := parseSemver(candidate)
	b := parseSemver(base)
	for i := range c {
		if c[i] != b[i] {
			return c[i] > b[i]
		}
	}
	return false
}

// parseSemver parses "vMAJOR.MINOR.PATCH[-pre]" into [MAJOR, MINOR, PATCH].
// Returns [0,0,0] for unparseable input (e.g. "dev").
func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		// Strip pre-release suffix e.g. "-rc.1"
		p, _, _ = strings.Cut(p, "-")
		n := 0
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}
