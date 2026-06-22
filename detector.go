package update

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Method identifies how the CLI was installed.
type Method int

const (
	MethodBrew   Method = iota // Homebrew formula or cask
	MethodDpkg                 // dpkg / apt-get (Debian/Ubuntu)
	MethodRpm                  // rpm (RHEL/Fedora)
	MethodBinary               // standalone binary (fallback)
)

// execCommand is a variable so tests can replace exec.Command.
var execCommand = exec.Command

// detectMethod infers the installation method from the binary's resolved path.
// binaryName is the formula/package name used to verify brew tracking.
func detectMethod(exePath, binaryName string) Method {
	real, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		real = exePath
	}

	lower := strings.ToLower(real)
	if strings.Contains(lower, "homebrew") ||
		strings.Contains(lower, "linuxbrew") ||
		strings.Contains(lower, "/cellar/") {
		// Verify brew actually tracks this binary. install.sh may place
		// the binary under a Homebrew-managed prefix without brew knowing,
		// causing `brew upgrade` to fail with "not installed".
		if isBrewManaged(binaryName) {
			return MethodBrew
		}
		return MethodBinary
	}

	if runtime.GOOS == "linux" {
		if out, err := execCommand("dpkg", "-S", real).Output(); err == nil {
			if strings.TrimSpace(string(out)) != "" {
				return MethodDpkg
			}
		}
		if out, err := execCommand("rpm", "-qf", real).Output(); err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" && !strings.Contains(s, "not owned") && !strings.Contains(s, "not installed") {
				return MethodRpm
			}
		}
	}

	return MethodBinary
}

// isBrewManaged returns true when brew tracks the named formula.
func isBrewManaged(name string) bool {
	out, err := execCommand("brew", "list", "--formula", name).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
