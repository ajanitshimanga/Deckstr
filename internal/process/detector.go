package process

import (
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// normalizeProcessName returns a normalized process name for cross-platform comparison
// Strips .exe suffix (Windows) and converts to lowercase
func normalizeProcessName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".exe")
	return name
}

// IsRunning checks if a process with the given name is currently running
// Uses native OS APIs via gopsutil - no subprocess spawning
// Latency: ~3-5ms cross-platform
// Cross-platform: automatically normalizes .exe suffix for Windows/macOS/Linux compatibility
func IsRunning(processName string) bool {
	procs, err := process.Processes()
	if err != nil {
		return false
	}

	targetNormalized := normalizeProcessName(processName)
	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if normalizeProcessName(name) == targetNormalized {
			return true
		}
	}
	return false
}

// AnyRunning checks if any of the given processes are running
// Returns true on first match for efficiency
// Cross-platform: automatically normalizes .exe suffix
func AnyRunning(processNames []string) bool {
	if len(processNames) == 0 {
		return false
	}

	procs, err := process.Processes()
	if err != nil {
		return false
	}

	// Build normalized lookup set for efficiency
	targets := make(map[string]bool, len(processNames))
	for _, name := range processNames {
		targets[normalizeProcessName(name)] = true
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if targets[normalizeProcessName(name)] {
			return true
		}
	}
	return false
}

// GetExePath returns the full executable path of the first matching running process
// Returns empty string if none are running or path cannot be retrieved
// Cross-platform: automatically normalizes .exe suffix
func GetExePath(processNames []string) string {
	if len(processNames) == 0 {
		return ""
	}

	procs, err := process.Processes()
	if err != nil {
		return ""
	}

	targets := make(map[string]bool, len(processNames))
	for _, name := range processNames {
		targets[normalizeProcessName(name)] = true
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if targets[normalizeProcessName(name)] {
			exe, err := p.Exe()
			if err == nil && exe != "" {
				return exe
			}
		}
	}
	return ""
}

// GetRunningProcess returns the name of the first matching process found
// Returns empty string if none are running
// Cross-platform: automatically normalizes .exe suffix
func GetRunningProcess(processNames []string) string {
	if len(processNames) == 0 {
		return ""
	}

	procs, err := process.Processes()
	if err != nil {
		return ""
	}

	// Map normalized name -> original name for return value
	targets := make(map[string]string, len(processNames))
	for _, name := range processNames {
		targets[normalizeProcessName(name)] = name
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if original, found := targets[normalizeProcessName(name)]; found {
			return original
		}
	}
	return ""
}
