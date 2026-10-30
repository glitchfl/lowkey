package platform

import (
	"io"
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

func ConsoleHeight() int {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stdout.Fd()), &info); err != nil {
		return 30
	}
	return int(info.Window.Bottom-info.Window.Top) + 1
}

// Keyboard asks the console for single key presses with the arrows as escape sequences - Ctrl+C still
// closes lowkey and QuickEdit still selects text
func Keyboard() (io.Reader, func(), bool) {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if !IsTerminal() || windows.GetConsoleMode(h, &mode) != nil {
		return nil, nil, false
	}
	raw := mode&^(windows.ENABLE_LINE_INPUT|windows.ENABLE_ECHO_INPUT) | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if windows.SetConsoleMode(h, raw) != nil {
		return nil, nil, false
	}
	return consoleKeys{h}, func() { windows.SetConsoleMode(h, mode) }, true
}

// consoleKeys reads the console itself - os.Stdin calls Ctrl+Z the end of the file and that would end the tabs
type consoleKeys struct{ h windows.Handle }

func (c consoleKeys) Read(p []byte) (int, error) {
	buf := make([]uint16, len(p))
	var n uint32
	if err := windows.ReadConsole(c.h, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return 0, err
	}
	for i, u := range buf[:n] {
		// the tabs only know ascii - cut down to a byte a letter from another alphabet could pass for tab or space
		if u > 0x7f {
			u = '?'
		}
		p[i] = byte(u)
	}
	return int(n), nil
}
