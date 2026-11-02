//go:build !windows

package platform

import "errors"

func ShowDot(x, y int) error { return errors.New("the agent dot needs Windows") }

func HideDot() {}

func CursorPos() (int, int, bool) { return 0, 0, false }
