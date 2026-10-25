//go:build !windows

package platform

// the Riot Client only exists on Windows - other builds are for development against fakeriot
func LockfilePath() string { return "" }

func GameLogPath() string { return "" }
