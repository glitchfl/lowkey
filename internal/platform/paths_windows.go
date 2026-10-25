package platform

import (
	"os"
	"path/filepath"
)

func LockfilePath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Riot Games", "Riot Client", "Config", "lockfile")
}

func GameLogPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "VALORANT", "Saved", "Logs", "ShooterGame.log")
}
