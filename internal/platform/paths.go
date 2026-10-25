package platform

import (
	"os"
	"path/filepath"
)

// ExeDir is where lowkey keeps lowkey.toml, its error log and recordings
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}
