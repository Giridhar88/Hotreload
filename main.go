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

	slog.Info("hotreload starting",
		"root", *root,
		"build", *build,
		"exec", *exec,
	)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create watcher", "error", err)
		os.Exit(1)
	}
	defer watcher.Close()
	if err := watcher.Add(*root); err != nil {
		slog.Error("failed to watch direcotry", "path", *root, "error", err)
		os.Exit(1)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				slog.Error("failed to listen event", "error", err)
				os.Exit(1)
			}
			slog.Info("event received", "event", event.String())
		case err, ok := <-watcher.Errors:
			if !ok {
				slog.Error("failed to listen error", "error", err)
				os.Exit(1)
			}
			slog.Error("watcher error", "error", err)
		}

	}
	// TODO: Step 3 -  debounce events
	// TODO: Step 4 - run build command
	// TODO: Step 5 - run exec command
}
