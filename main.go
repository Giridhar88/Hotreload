package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	root := flag.String("root", ".", "directory to watch for file changes")
	build := flag.String("build", "", "command to build the project")
	execCmd := flag.String("exec", "", "command to run the built server")

	flag.Parse()

	if *build == "" || *execCmd == "" {
		fmt.Fprintf(os.Stderr, "Usage: hotreload --root <dir> --build <cmd> --exec <cmd>\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	slog.Info("hotreload starting",
		"root", *root,
		"build", *build,
		"exec", *execCmd,
	)

	// Graceful shutdown on Ctrl+C or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create watcher", "error", err)
		os.Exit(1)
	}
	defer watcher.Close()

	// Recursively watch the root directory
	if err := watchRecursive(*root, watcher); err != nil {
		slog.Error("failed to watch directory", "path", *root, "error", err)
		os.Exit(1)
	}

	// Create debouncer (500ms delay) and runner
	debouncer := newDebouncer(500 * time.Millisecond)
	runner := newRunner(*build, *execCmd)

	// Channel to pass file change events from watcher to debouncer
	onChange := make(chan string, 10)

	// Start all components as goroutines
	go startWatching(ctx, watcher, onChange)
	go debouncer.run()
	go runner.loop(ctx, debouncer.output)

	// Bridge: watcher onChange → debouncer signal
	// This runs on the main goroutine
	for {
		select {
		case <-ctx.Done():
			slog.Info("hotreload shutting down")
			return
		case path := <-onChange:
			slog.Info("file changed", "path", path)
			debouncer.signal()
		}
	}
}
