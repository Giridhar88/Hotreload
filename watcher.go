package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// shouldWatch returns true if the file path is a .go file
// and not inside ignored directories like .git, node_modules, etc.
func shouldWatch(path string) bool {
	// Ignored directories — never care about changes inside these
	ignoredDirs := []string{".git", "node_modules", "vendor", ".idea", ".vscode", "bin", "tmp"}
	for _, dir := range ignoredDirs {
		if strings.Contains(path, string(os.PathSeparator)+dir+string(os.PathSeparator)) ||
			strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			return false
		}
	}

	base := filepath.Base(path)

	// Hidden files (dotfiles)
	if strings.HasPrefix(base, ".") {
		return false
	}

	// Editor temp/swap/backup files
	if strings.HasSuffix(base, "~") ||
		strings.HasSuffix(base, ".swp") ||
		strings.HasSuffix(base, ".swo") ||
		strings.HasSuffix(base, ".tmp") ||
		strings.HasPrefix(base, "#") ||
		strings.HasPrefix(base, ".#") {
		return false
	}

	// Binary/build artifacts
	ext := filepath.Ext(path)
	ignoredExts := []string{".exe", ".o", ".a", ".so", ".dylib", ".test"}
	for _, e := range ignoredExts {
		if ext == e {
			return false
		}
	}

	return true
}

// watchRecursive walks the directory tree starting at root
// and adds all subdirectories to the watcher.
var watchingFiles = make(map[string]bool)

func watchRecursive(root string, watcher *fsnotify.Watcher) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			watchingFiles[path] = false
			return nil
		}
		if !shouldWatch(path) {
			watchingFiles[path] = false
			return filepath.SkipDir
		}
		watchingFiles[path] = true
		return watcher.Add(path)
	})
}

// startWatching listens for file system events and sends
// matching file paths to the onChange channel.
// It returns when ctx is cancelled.
func startWatching(ctx context.Context, watcher *fsnotify.Watcher, onChange chan<- string) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("watcher stopping")
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					watchRecursive(event.Name, watcher)
				}
			}
			if event.Has(fsnotify.Remove) {
				slog.Info("path removed", "path", event.Name)
				watchingFiles[event.Name] = false
				watcher.Remove(event.Name)
			}
			if shouldWatch(event.Name) {
				onChange <- event.Name
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)
		}
	}
}
