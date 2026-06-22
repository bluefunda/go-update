package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// githubDownload is the GitHub release download base URL. Tests override this.
var githubDownload = "https://github.com"

// Upgrader performs the actual CLI upgrade.
type Upgrader interface {
	// Name returns the human-readable name of the upgrade strategy.
	Name() string
	// Upgrade downloads and installs the given version.
	// version is the plain semver string without "v" prefix (e.g. "1.5.0").
	Upgrade(cfg Config, version string) error
}

func newUpgrader(method Method, cfg Config) Upgrader {
	switch method {
	case MethodBrew:
		return &BrewUpgrader{Cask: cfg.HomebrewCask}
	case MethodDpkg:
		return &DpkgUpgrader{}
	case MethodRpm:
		return &RpmUpgrader{}
	default:
		return &BinaryUpgrader{}
	}
}

// BrewUpgrader upgrades via `brew upgrade --cask <cask>`.
type BrewUpgrader struct {
	Cask string
}

func (b *BrewUpgrader) Name() string { return "Homebrew" }

func (b *BrewUpgrader) Upgrade(cfg Config, version string) error {
	// Homebrew 4.x requires taps to be explicitly trusted before formulas
	// can be loaded. Trust silently; errors are ignored (already trusted).
	if cfg.HomebrewTap != "" && runtime.GOOS == "darwin" {
		_ = execCommand("brew", "trust", cfg.HomebrewTap).Run()
	}

	// Refresh tap index so the formula knows about the latest release.
	// Ignore errors — if update fails, upgrade may still succeed if the tap is warm.
	upd := execCommand("brew", "update", "--quiet")
	upd.Stdout = os.Stdout
	upd.Stderr = os.Stderr
	_ = upd.Run()

	cmd := execCommand("brew", "upgrade", b.Cask)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew upgrade %s failed: %w\n\nFix: run `brew upgrade %s` manually", b.Cask, err, b.Cask)
	}

	// brew exits 0 even when "already at latest version" — verify the install.
	installed := brewInstalledVersion(b.Cask)
	if installed != "" && installed != version {
		return fmt.Errorf(
			"brew upgrade completed but installed version is %s (wanted %s)\n\n"+
				"The Homebrew tap may still be catching up. Try again in a moment:\n"+
				"  brew update && brew upgrade %s",
			installed, version, b.Cask,
		)
	}
	return nil
}

// brewInstalledVersion returns the currently installed version for a formula/cask,
// or an empty string if it cannot be determined.
func brewInstalledVersion(name string) string {
	out, err := execCommand("brew", "list", "--versions", name).Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	// Output format: "<name> <version>" or "<name> <v1> <v2> ..."
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 2 {
		return fields[len(fields)-1]
	}
	return ""
}

// DpkgUpgrader downloads the .deb from GitHub Releases and installs it with dpkg.
type DpkgUpgrader struct{}

func (d *DpkgUpgrader) Name() string { return "dpkg" }

func (d *DpkgUpgrader) Upgrade(cfg Config, version string) error {
	asset := fmt.Sprintf("%s_%s_linux_%s.deb", cfg.BinaryName, version, normalizeArch(runtime.GOARCH))
	path, err := downloadAsset(cfg.GitHubOwner, cfg.GitHubRepo, version, asset)
	if err != nil {
		return err
	}
	defer os.Remove(path) //nolint:errcheck

	cmd := execCommand("sudo", "dpkg", "-i", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dpkg -i failed: %w\n\nFix: run `sudo dpkg -i %s` manually after re-downloading", err, asset)
	}
	return nil
}

// RpmUpgrader downloads the .rpm from GitHub Releases and installs it with rpm.
type RpmUpgrader struct{}

func (r *RpmUpgrader) Name() string { return "rpm" }

func (r *RpmUpgrader) Upgrade(cfg Config, version string) error {
	asset := fmt.Sprintf("%s_%s_linux_%s.rpm", cfg.BinaryName, version, normalizeArch(runtime.GOARCH))
	path, err := downloadAsset(cfg.GitHubOwner, cfg.GitHubRepo, version, asset)
	if err != nil {
		return err
	}
	defer os.Remove(path) //nolint:errcheck

	cmd := execCommand("sudo", "rpm", "-U", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rpm -U failed: %w\n\nFix: run `sudo rpm -U %s` manually after re-downloading", err, asset)
	}
	return nil
}

// BinaryUpgrader downloads the archive from GitHub Releases, verifies SHA256,
// extracts the binary and replaces the current executable in place.
type BinaryUpgrader struct{}

func (b *BinaryUpgrader) Name() string { return "binary" }

func (b *BinaryUpgrader) Upgrade(cfg Config, version string) error {
	goos := runtime.GOOS
	arch := normalizeArch(runtime.GOARCH)

	ext := "tar.gz"
	if goos == "darwin" || goos == "windows" {
		ext = "zip"
	}

	asset := fmt.Sprintf("%s_%s_%s_%s.%s", cfg.BinaryName, version, goos, arch, ext)

	// Fetch and parse the checksums file.
	csURL := assetURL(cfg.GitHubOwner, cfg.GitHubRepo, version, "checksums.txt")
	checksums, err := fetchChecksums(csURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	expected, ok := checksums[asset]
	if !ok {
		return fmt.Errorf("no checksum found for %s — the release may not include this platform", asset)
	}

	// Download archive.
	archivePath, err := downloadAsset(cfg.GitHubOwner, cfg.GitHubRepo, version, asset)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath) //nolint:errcheck

	// Verify integrity.
	if err := verifyChecksum(archivePath, expected); err != nil {
		return err
	}

	// Determine the destination (current executable).
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determining current executable path: %w", err)
	}

	tmpPath := binPath + ".new"
	if err := extractBinary(archivePath, cfg.BinaryName, ext, tmpPath); err != nil {
		return fmt.Errorf("extracting binary from %s: %w", asset, err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions on new binary: %w", err)
	}
	if err := os.Rename(tmpPath, binPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing %s: %w\n\nFix: try running with elevated permissions or install to a user-writable location", binPath, err)
	}
	return nil
}

// -- internal helpers ---------------------------------------------------------

func assetURL(owner, repo, version, asset string) string {
	return fmt.Sprintf("%s/%s/%s/releases/download/v%s/%s", githubDownload, owner, repo, version, asset)
}

func downloadAsset(owner, repo, version, asset string) (path string, err error) {
	url := assetURL(owner, repo, version, asset)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", asset, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: HTTP %d", asset, resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "go-update-*")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = io.Copy(tmp, resp.Body); err != nil {
		return "", fmt.Errorf("writing %s: %w", asset, err)
	}
	return tmp.Name(), nil
}

func fetchChecksums(url string) (map[string]string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching checksums", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Format per line: "<hash>  <filename>"
	m := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 {
			m[parts[1]] = parts[0]
		}
	}
	return m, nil
}

func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	got := fmt.Sprintf("%x", h.Sum(nil))
	if got != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, got)
	}
	return nil
}

func extractBinary(archivePath, binaryName, ext, dst string) error {
	if strings.HasSuffix(ext, "zip") {
		return extractFromZip(archivePath, binaryName, dst)
	}
	return extractFromTarGz(archivePath, binaryName, dst)
}

func extractFromTarGz(archivePath, binaryName, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close() //nolint:errcheck

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			out, err := os.Create(dst)
			if err != nil {
				return err
			}
			_, cpErr := io.Copy(out, tr)
			_ = out.Close()
			return cpErr
		}
	}
	return fmt.Errorf("binary %q not found in tar archive", binaryName)
}

func extractFromZip(archivePath, binaryName, dst string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			out, err := os.Create(dst)
			if err != nil {
				_ = rc.Close()
				return err
			}
			_, cpErr := io.Copy(out, rc)
			_ = rc.Close()
			_ = out.Close()
			return cpErr
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}

// normalizeArch maps Go runtime arch names to the naming used in GoReleaser archives.
func normalizeArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}
