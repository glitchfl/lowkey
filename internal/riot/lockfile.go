package riot

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Lockfile struct {
	Name     string
	PID      int
	Port     int
	Password string
	Protocol string
}

func ReadLockfile(path string) (Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Lockfile{}, err
	}
	return ParseLockfile(data)
}

// ParseLockfile reads name:pid:port:password:protocol and never puts the password in an error
func ParseLockfile(data []byte) (Lockfile, error) {
	parts := strings.Split(strings.TrimSpace(string(data)), ":")
	if len(parts) != 5 {
		return Lockfile{}, fmt.Errorf("lockfile has %d fields, want name:pid:port:password:protocol", len(parts))
	}
	pid, err := strconv.Atoi(parts[1])
	if err != nil {
		return Lockfile{}, fmt.Errorf("lockfile pid %q isn't a number", parts[1])
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil || port < 1 || port > 65535 {
		return Lockfile{}, fmt.Errorf("lockfile port %q isn't valid", parts[2])
	}
	if parts[3] == "" {
		return Lockfile{}, errors.New("lockfile has no password")
	}
	return Lockfile{Name: parts[0], PID: pid, Port: port, Password: parts[3], Protocol: parts[4]}, nil
}
