//go:build !windows

package platform

import (
	"io"
	"os"
	"os/exec"
	"strconv"
)

func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func ConsoleWidth() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	return 120
}

func ConsoleHeight() int {
	if n, err := strconv.Atoi(os.Getenv("LINES")); err == nil && n > 0 {
		return n
	}
	return 30
}

// Keyboard lets a Mac drive the tabs against fakeriot - stty fails when stdin isn't a terminal and leaves Ctrl+C alone
func Keyboard() (io.Reader, func(), bool) {
	stty := func(args ...string) error {
		cmd := exec.Command("stty", args...)
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	if !IsTerminal() || stty("-icanon", "-echo", "min", "1", "time", "0") != nil {
		return nil, nil, false
	}
	return os.Stdin, func() { stty("icanon", "echo") }, true
}
