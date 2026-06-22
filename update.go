// Package update provides CLI self-update functionality for BlueFunda CLIs.
// It detects the installation method (Homebrew, dpkg, rpm, or standalone binary)
// and performs the upgrade using the appropriate strategy.
package update

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Config holds per-CLI settings for the update command.
type Config struct {
	// BinaryName is the CLI binary name, e.g. "bai", "abaper", "breq".
	BinaryName string
	// CurrentVersion is the version string injected at build time via ldflags.
	// May be "dev", "v1.2.3", or "1.2.3".
	CurrentVersion string
	// GitHubOwner is the GitHub organisation, e.g. "bluefunda".
	GitHubOwner string
	// GitHubRepo is the GitHub repository name, e.g. "bluefunda-ai".
	GitHubRepo string
	// HomebrewTap is the tap name, e.g. "bluefunda/tap". When set, the tap
	// is trusted before upgrade so Homebrew 4.x loads the formula without
	// requiring the user to run `brew trust` manually.
	HomebrewTap string
	// HomebrewCask is the formula or cask name in the tap, e.g. "bai".
	HomebrewCask string
}

// stdin is the reader used for interactive prompts. Tests override this.
var stdin io.Reader = os.Stdin

// Run executes the full self-update flow: check → diff → prompt → upgrade.
func Run(cfg Config) error {
	fmt.Fprintln(os.Stderr, "Checking for updates...")

	rel, err := fetchLatest(cfg.GitHubOwner, cfg.GitHubRepo)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	current := normalizeVersion(cfg.CurrentVersion)
	latest := normalizeVersion(rel.TagName)

	if !newerThan(latest, current) {
		fmt.Printf("%s is already up to date (%s)\n", cfg.BinaryName, current)
		return nil
	}

	fmt.Printf("\nCurrent: %s %s\n", cfg.BinaryName, current)
	fmt.Printf("Latest:  %s\n\n", latest)
	fmt.Println("Update available.")
	fmt.Println()

	if !confirm("Continue?") {
		fmt.Println("Aborted.")
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determining executable path: %w", err)
	}

	method := detectMethod(exe, cfg.BinaryName)
	upgrader := newUpgrader(method, cfg)

	fmt.Printf("\nUpdating via %s...\n\n", upgrader.Name())

	// version without the "v" prefix for asset filenames
	plain := strings.TrimPrefix(rel.TagName, "v")
	if err := upgrader.Upgrade(cfg, plain); err != nil {
		return err
	}

	fmt.Printf("✓ Successfully updated to %s\n", latest)
	return nil
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	scanner := bufio.NewScanner(stdin)
	if scanner.Scan() {
		return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
	}
	return false
}
