package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// watchLockfile hears about changes through filesystem notifications - it watches the nearest
// folder that exists so a Riot Client installed later is still noticed
func watchLockfile(ctx context.Context, path string, changed func()) error {
	if path == "" {
		return nil
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	armed := ""
	arm := func() {
		dir := filepath.Dir(path)
		for {
			if _, err := os.Stat(dir); err == nil {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				return
			}
			dir = parent
		}
		if dir == armed {
			return
		}
		if armed != "" {
			w.Remove(armed)
		}
		if w.Add(dir) == nil {
			armed = dir
		}
	}
	arm()
	go func() {
		defer w.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-w.Events:
				if !ok {
					return
				}
				arm()
				changed()
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return nil
}
