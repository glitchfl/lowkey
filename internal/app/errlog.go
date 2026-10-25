package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glitchfl/lowkey/internal/record"
)

// errlog appends errors to lowkey.log next to the exe - created on the first error and kept small
type errlog struct {
	mu      sync.Mutex
	path    string
	now     func() time.Time
	secrets []string
}

const errlogLimit = 256 << 10

func newErrlog(dir string, now func() time.Time) *errlog {
	return &errlog{path: filepath.Join(dir, "lowkey.log"), now: now}
}

// secret registers a value that must never reach the log - the lockfile password for one
func (l *errlog) secret(s string) {
	if l == nil || s == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.secrets = append(l.secrets, s)
}

func (l *errlog) printf(format string, args ...any) {
	if l == nil {
		return
	}
	msg := record.RedactTokens(fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.secrets {
		msg = strings.ReplaceAll(msg, s, "<scrubbed>")
	}
	if fi, err := os.Stat(l.path); err == nil && fi.Size() > errlogLimit {
		os.Rename(l.path, l.path+".old")
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", l.now().Format(time.RFC3339), msg)
}
