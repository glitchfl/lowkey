//go:build !windows

package platform

import "errors"

func Prepare() Setup { return Setup{} }

func Priority() (string, error) { return "", nil }

var ErrCantReadBack = errors.New("not Windows")

func EfficiencyMode() (bool, error) { return false, nil }

func EnableANSI() bool { return IsTerminal() }

const Windows = false
