//go:build !windows

package platform

import "errors"

func ShowDots(marks []Mark) error { return errors.New("the agent dot needs Windows") }

func HideDots() {}

func CursorPos() (int, int, bool) { return 0, 0, false }
