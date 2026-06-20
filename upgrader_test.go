package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var testCfg = Config{
	BinaryName:   "bai",
	GitHubOwner:  "bluefunda",
	GitHubRepo:   "bluefunda-ai",
	HomebrewCask: "bai",
}

func TestBrewUpgrader_Success(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" {
			return exec.Command("echo", "==> Upgrading bai cask")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	u := &BrewUpgrader{Cask: "bai"}
	if err := u.Upgrade(testCfg, "1.5.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBrewUpgrader_Failure(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" {
			return exec.Command("false")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	u := &BrewUpgrader{Cask: "bai"}
	if err := u.Upgrade(testCfg, "1.5.0"); err == nil {
		t.Error("expected error from brew failure, got nil")
	}
}

func TestDpkgUpgrader_Success(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("dpkg test only relevant on Linux")
	}

	ts := newSimpleAssetServer(t, fmt.Sprintf("bai_1.5.0_linux_%s.deb", runtime.GOARCH))
	defer ts.Close()

	origDownload := githubDownload
	githubDownload = ts.URL
	defer func() { githubDownload = origDownload }()

	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "sudo" {
			return exec.Command("echo", "dpkg ok")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	u := &DpkgUpgrader{}
	if err := u.Upgrade(testCfg, "1.5.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRpmUpgrader_Success(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("rpm test only relevant on Linux")
	}

	ts := newSimpleAssetServer(t, fmt.Sprintf("bai_1.5.0_linux_%s.rpm", runtime.GOARCH))
	defer ts.Close()

	origDownload := githubDownload
	githubDownload = ts.URL
	defer func() { githubDownload = origDownload }()

	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "sudo" {
			return exec.Command("echo", "rpm ok")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	u := &RpmUpgrader{}
	if err := u.Upgrade(testCfg, "1.5.0"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtractFromTarGz(t *testing.T) {
	archiveData := buildTarGz(t, "bai", []byte("#!/bin/sh\necho hello"))

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmp, "bai")
	if err := extractFromTarGz(archivePath, "bai", dst); err != nil {
		t.Fatalf("extractFromTarGz: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("expected extracted file at %s: %v", dst, err)
	}
}

func TestExtractFromZip(t *testing.T) {
	archiveData := buildZip(t, "bai", []byte("#!/bin/sh\necho hello"))

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.zip")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmp, "bai")
	if err := extractFromZip(archivePath, "bai", dst); err != nil {
		t.Fatalf("extractFromZip: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("expected extracted file at %s: %v", dst, err)
	}
}

func TestVerifyChecksum_Match(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	expected := fmt.Sprintf("%x", h)
	if err := verifyChecksum(path, expected); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(path, "badhash"); err == nil {
		t.Error("expected checksum mismatch error, got nil")
	}
}

func TestBinaryUpgrader_ChecksumMismatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("this test uses a linux tar.gz asset name")
	}

	archiveData := buildTarGz(t, "bai", []byte("fake binary"))
	asset := "bai_1.5.0_linux_amd64.tar.gz"
	// Deliberately wrong checksum
	checksumLine := fmt.Sprintf("%s  %s\n", "0000000000000000000000000000000000000000000000000000000000000000", asset)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case "checksums.txt":
			w.Write([]byte(checksumLine))
		case asset:
			w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	origDownload := githubDownload
	githubDownload = ts.URL
	defer func() { githubDownload = origDownload }()

	u := &BinaryUpgrader{}
	cfg := Config{BinaryName: "bai", GitHubOwner: "bluefunda", GitHubRepo: "bluefunda-ai"}
	if err := u.Upgrade(cfg, "1.5.0"); err == nil {
		t.Error("expected checksum mismatch error, got nil")
	}
}

// -- test helpers --

// newSimpleAssetServer serves a single named asset as dummy bytes.
func newSimpleAssetServer(t *testing.T, assetName string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(r.URL.Path) == assetName {
			w.Write([]byte("fake package bytes"))
			return
		}
		http.NotFound(w, r)
	}))
}

// buildTarGz creates an in-memory tar.gz with one file named binaryName.
func buildTarGz(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{
		Name:     binaryName,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	})
	tw.Write(content)
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// buildZip creates an in-memory zip with one file named binaryName.
func buildZip(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(content)
	zw.Close()
	return buf.Bytes()
}
