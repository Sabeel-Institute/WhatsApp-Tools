// Package platform holds the small amount of operating-system specific
// behaviour the app needs: where data lives, opening the browser, the
// single-instance lock and keeping the computer awake during a job.
//
// Everything OS-specific lives in files with build constraints
// (browser_linux.go, lock_windows.go, ...). This file is portable.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDirName is the folder created inside the per-user config directory.
const DataDirName = "sabeel-whatsapp-tools"

// DataDir returns the per-user data directory, creating it with owner-only
// permissions. override, when non-empty, replaces the default location.
func DataDir(override string) (string, error) {
	dir := override
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("finding the user config directory: %w", err)
		}
		dir = filepath.Join(base, DataDirName)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating data directory %s: %w", dir, err)
	}
	// MkdirAll does not change the mode of an existing directory.
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("restricting data directory %s: %w", dir, err)
	}
	return dir, nil
}

// OpenBrowser opens url in the user's default browser.
func OpenBrowser(url string) error {
	return openBrowser(url)
}
