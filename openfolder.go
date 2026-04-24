package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openInFileManager opens the given directory in the platform's native
// file manager (Explorer on Windows, Finder on macOS, xdg-open on Linux).
// Mirrors the dispatch in internal/updater/updater.go for opening URLs.
func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	// `explorer` returns exit code 1 even on success, so we start-and-forget
	// rather than waiting. The other tools behave sanely but we don't care
	// about their exit codes either — the user can see whether it opened.
	return cmd.Start()
}
