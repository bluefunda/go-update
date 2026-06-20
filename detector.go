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
	MethodBrew   Method = iota // Homebrew cask
	MethodDpkg                 // dpkg / apt-get (Debian/Ubuntu)
	MethodRpm                  // rpm (RHEL/Fedora)
	MethodBinary               // standalone binary (fallback)
)

// execCommand is a variable so tests can replace exec.Command.
var execCommand = exec.Command

// detectMethod infers the installation method from the binary's resolved path.
func detectMethod(exePath string) Method {
	real, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		real = exePath
	}

	lower := strings.ToLower(real)
	if strings.Contains(lower, "homebrew") ||
		strings.Contains(lower, "linuxbrew") ||
		strings.Contains(lower, "/cellar/") {
		return MethodBrew
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
