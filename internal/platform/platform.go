package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "Context Baggage"

type Info struct {
	Name string
	OS   string
	Arch string
}

func AppHome() (string, error) {
	// Tests and manual dry-runs need an isolated state root so they do not
	// touch a developer's real Context Baggage home.
	if override := os.Getenv("CONTEXT_BAGGAGE_HOME"); override != "" {
		return filepath.Abs(override)
	}
	// Application state belongs in the per-user app-data location for each OS.
	// The target source repository must stay untouched.
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, AppName), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", AppName), nil
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, "context-baggage"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if home == "" {
			return "", errors.New("user home directory is unavailable")
		}
		return filepath.Join(home, ".local", "share", "context-baggage"), nil
	}
	return "", errors.New("application data directory is unavailable")
}

func CurrentInfo() Info {
	name, _ := os.Hostname()
	return Info{Name: name, OS: runtime.GOOS, Arch: runtime.GOARCH}
}
