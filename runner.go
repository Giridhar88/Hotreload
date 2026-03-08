package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Runner manages the build and exec lifecycle.
// It handles killing old processes, rebuilding, and restarting.
type Runner struct {
	buildCmd string
	execCmd  string
	cmd      *exec.Cmd    // currently running server process
	done     chan struct{} // closed when the server process exits
	mu       sync.Mutex   // protects cmd from concurrent access
}

func newRunner(buildCmd, execCmd string) *Runner {
	return &Runner{
		buildCmd: buildCmd,
		execCmd:  execCmd,
	}
}

// restart kills the old server, runs the build command, and starts the server again.
// Returns true if the build and exec succeeded.
func (r *Runner) restart(ctx context.Context) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()

	// Step 1: Kill the old server process (if running)
	r.stopLocked()

	// Step 2: Run the build command
	slog.Info("building", "cmd", r.buildCmd)
	buildCmd := exec.CommandContext(ctx, "sh", "-c", r.buildCmd)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		slog.Error("build failed", "error", err)
		return false
	}
	slog.Info("build succeeded")

	// Step 3: Start the server
	slog.Info("starting server", "cmd", r.execCmd)
	r.cmd = exec.CommandContext(ctx, "sh", "-c", r.execCmd)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	// Create a new process group so we can kill all child processes
	r.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := r.cmd.Start(); err != nil {
		slog.Error("failed to start server", "error", err)
		r.cmd = nil
		return false
	}

	slog.Info("server started", "pid", r.cmd.Process.Pid)
	fmt.Printf("\033[32m✓ restarted in %s\033[0m\n", time.Since(start).Round(time.Millisecond))

	// Wait for the process in a separate goroutine so we don't block.
	// When the process exits, close the done channel so stopLocked knows.
	r.done = make(chan struct{})
	go func() {
		r.cmd.Wait()
		close(r.done)
	}()

	return true
}

// stopLocked kills the current server process and all its children.
// Must be called with r.mu held.
func (r *Runner) stopLocked() {
	if r.cmd == nil || r.cmd.Process == nil {
		return
	}

	pid := r.cmd.Process.Pid
	slog.Info("stopping server", "pid", pid)

	// Kill the entire process group (negative PID)
	// This ensures child processes are killed too
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	// Wait for the process to exit (using the done channel from restart)
	select {
	case <-r.done:
		slog.Info("server stopped gracefully")
	case <-time.After(3 * time.Second):
		// Process didn't stop nicely — force kill
		slog.Warn("server did not stop, force killing", "pid", pid)
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}

	r.cmd = nil
}

// stop is the public version of stopLocked that acquires the mutex.
func (r *Runner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

// loop listens on the rebuild channel and restarts the server each time.
// It returns when ctx is cancelled.
func (r *Runner) loop(ctx context.Context, rebuild <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			r.stop()
			slog.Info("runner stopping")
			return
		case <-rebuild:
			r.restart(ctx)
		}
	}
}
