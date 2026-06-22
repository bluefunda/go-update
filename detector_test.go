package update

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

func TestDetectMethod_HomebrewPath_Managed(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 && args[0] == "list" {
			return fakeExecSuccess("bai 1.0.0")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	tests := []struct {
		path string
		name string
	}{
		{"/usr/local/Cellar/bai/1.0.0/bin/bai", "bai"},
		{"/opt/homebrew/Cellar/abaper/1.9.0/bin/abaper", "abaper"},
		{"/home/linuxbrew/.linuxbrew/Cellar/breq/0.3.0/bin/breq", "breq"},
	}
	for _, tc := range tests {
		if got := detectMethod(tc.path, tc.name); got != MethodBrew {
			t.Errorf("detectMethod(%q, %q) = %v, want MethodBrew", tc.path, tc.name, got)
		}
	}
}

func TestDetectMethod_HomebrewPath_NotManaged(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "brew" && len(args) > 0 && args[0] == "list" {
			return fakeExecFail() // brew list returns non-zero: not managed
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	// Binary is under /opt/homebrew but was placed there by install.sh,
	// not by brew. Should fall back to MethodBinary.
	got := detectMethod("/opt/homebrew/bin/bai", "bai")
	if got != MethodBinary {
		t.Errorf("detectMethod = %v, want MethodBinary (binary in Homebrew path but not brew-managed)", got)
	}
}

func TestDetectMethod_Dpkg(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("dpkg detection only runs on Linux")
	}

	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "dpkg" {
			return fakeExecSuccess("abaper: /usr/local/bin/abaper")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	if got := detectMethod("/usr/local/bin/abaper", "abaper"); got != MethodDpkg {
		t.Errorf("detectMethod = %v, want MethodDpkg", got)
	}
}

func TestDetectMethod_Rpm(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("rpm detection only runs on Linux")
	}

	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		switch name {
		case "dpkg":
			return fakeExecFail()
		case "rpm":
			return fakeExecSuccess("abaper-1.9.0-1.x86_64")
		}
		return exec.Command(name, args...)
	}
	defer func() { execCommand = origExec }()

	if got := detectMethod("/usr/local/bin/abaper", "abaper"); got != MethodRpm {
		t.Errorf("detectMethod = %v, want MethodRpm", got)
	}
}

func TestDetectMethod_Binary_Fallback(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return fakeExecFail()
	}
	defer func() { execCommand = origExec }()

	if got := detectMethod("/usr/local/bin/bai", "bai"); got != MethodBinary {
		t.Errorf("detectMethod = %v, want MethodBinary", got)
	}
}

// fakeExecSuccess returns a command that succeeds and prints output.
func fakeExecSuccess(output string) *exec.Cmd {
	return exec.Command("echo", output)
}

// fakeExecFail returns a command that fails.
func fakeExecFail() *exec.Cmd {
	cmd := exec.Command("false")
	// On systems without "false", use a clearly invalid command.
	_ = errors.New("cmd will fail")
	return cmd
}
