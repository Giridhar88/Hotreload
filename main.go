package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/fsnotify/fsnotify"
)

func main() {
	root := flag.String("root", ".", "directory to watch for file changes")
	build := flag.String("build", "", "command to build the project")
	exec := flag.String("exec", "", "command to run the built server")

	flag.Parse()

	if *build == "" || *exec == "" {
		fmt.Fprintf(os.Stderr, "Usage: hotreload --root <dir> --build <cmd> --exec <cmd>\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// slog is Go's structured logging package (added in Go 1.21).
	// The assignment requires using log/slog specifically.
	// Structured logging means each log entry has key-value pairs,
	// which makes logs searchable and parseable (vs plain fmt.Println).
	slog.Info("hotreload starting",
		"root", *root,
		"build", *build,
		"exec", *exec,
	)

	watcher, err := fsnotify.NewWatcher()
	defer watcher.close()
	if err != nil {
		log.Error("failed to create watcher", "error", err)
		os.Exit(1)
	}

	// TODO: Step 3 — debounce events

	// TODO: Step 4 — run build command
	// TODO: Step 5 — run exec command
}
