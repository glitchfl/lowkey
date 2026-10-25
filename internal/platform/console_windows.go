package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

func IsTerminal() bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(os.Stdout.Fd()), &mode) == nil
}

func ConsoleWidth() int {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stdout.Fd()), &info); err != nil {
		return 120
	}
	return int(info.Window.Right-info.Window.Left) + 1
}
