package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func newTestRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(".", "itest-")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		_ = os.RemoveAll(root)
		t.Fatalf("failed to get absolute path for temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(abs)
	})
	return abs
}

type flowHarness struct {
	cancel  context.CancelFunc
	watcher *fsnotify.Watcher
}

func newFlowHarness(t *testing.T, root, buildCmd, execCmd string, delay time.Duration) *flowHarness {
	t.Helper()

	setCustomIgnore(nil)
	watchingFiles = make(map[string]bool)

	ctx, cancel := context.WithCancel(context.Background())

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := watchRecursive(root, watcher); err != nil {
		watcher.Close()
		cancel()
		t.Fatalf("failed to watch root: %v", err)
	}

	debouncer := newDebouncer(delay)
	runner := newRunner(buildCmd, execCmd)
	onChange := make(chan string, 32)

	go startWatching(ctx, watcher, onChange)
	go debouncer.run()
	go runner.loop(ctx, debouncer.output)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-onChange:
				debouncer.signal()
			}
		}
	}()

	return &flowHarness{
		cancel:  cancel,
		watcher: watcher,
	}
}

func (h *flowHarness) Close() {
	h.cancel()
	_ = h.watcher.Close()
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func fileContent(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func lineCount(path string) int {
	content := strings.TrimSpace(fileContent(path))
	if content == "" {
		return 0
	}
	return len(strings.Split(content, "\n"))
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool, label string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", label)
}

func TestIntegration_InitialBuildAndRestartOnChange(t *testing.T) {
	root := newTestRoot(t)
	artifacts := t.TempDir()
	versionPath := filepath.Join(root, "version.txt")
	buildLog := filepath.Join(artifacts, "build.log")
	execLog := filepath.Join(artifacts, "exec.log")

	writeFile(t, versionPath, "v1")

	buildCmd := fmt.Sprintf("v=$(cat %s); echo \"$v\" >> %s", shQuote(versionPath), shQuote(buildLog))
	execCmd := fmt.Sprintf(
		"v=$(cat %s); echo \"start-$v\" >> %s; trap 'echo \"stop-$v\" >> %s; exit 0' TERM INT; while :; do sleep 1; done",
		shQuote(versionPath),
		shQuote(execLog),
		shQuote(execLog),
	)

	h := newFlowHarness(t, root, buildCmd, execCmd, 120*time.Millisecond)
	defer h.Close()

	waitForCondition(t, 4*time.Second, func() bool {
		return strings.Contains(fileContent(buildLog), "v1")
	}, "initial build")
	waitForCondition(t, 4*time.Second, func() bool {
		return strings.Contains(fileContent(execLog), "start-v1")
	}, "initial exec start")

	writeFile(t, versionPath, "v2")

	waitForCondition(t, 4*time.Second, func() bool {
		return strings.Contains(fileContent(buildLog), "v2")
	}, "rebuild with updated content")
	waitForCondition(t, 4*time.Second, func() bool {
		return strings.Contains(fileContent(execLog), "start-v2")
	}, "server restart with updated content")
}

func TestIntegration_RapidChangesAreCoalesced(t *testing.T) {
	root := newTestRoot(t)
	artifacts := t.TempDir()
	versionPath := filepath.Join(root, "version.txt")
	buildLog := filepath.Join(artifacts, "build.log")
	execLog := filepath.Join(artifacts, "exec.log")

	writeFile(t, versionPath, "base")

	buildCmd := fmt.Sprintf("v=$(cat %s); echo \"$v\" >> %s", shQuote(versionPath), shQuote(buildLog))
	execCmd := fmt.Sprintf(
		"trap 'exit 0' TERM INT; echo run >> %s; while :; do sleep 1; done",
		shQuote(execLog),
	)

	h := newFlowHarness(t, root, buildCmd, execCmd, 180*time.Millisecond)
	defer h.Close()

	waitForCondition(t, 4*time.Second, func() bool {
		return lineCount(buildLog) >= 1
	}, "initial build")

	writeFile(t, versionPath, "v1")
	time.Sleep(40 * time.Millisecond)
	writeFile(t, versionPath, "v2")
	time.Sleep(40 * time.Millisecond)
	writeFile(t, versionPath, "v3")

	waitForCondition(t, 5*time.Second, func() bool {
		return lineCount(buildLog) >= 2 && strings.Contains(fileContent(buildLog), "v3")
	}, "coalesced rebuild")

	time.Sleep(400 * time.Millisecond)
	if got := lineCount(buildLog); got != 2 {
		t.Fatalf("expected exactly one extra rebuild after rapid changes, got %d total builds", got)
	}
}

func TestIntegration_NewChangeCancelsInProgressBuild(t *testing.T) {
	root := newTestRoot(t)
	artifacts := t.TempDir()
	versionPath := filepath.Join(root, "version.txt")
	buildEvents := filepath.Join(artifacts, "build-events.log")
	execLog := filepath.Join(artifacts, "exec.log")

	writeFile(t, versionPath, "v1")

	buildCmd := fmt.Sprintf(
		"v=$(cat %s); echo \"start-$v\" >> %s; sleep 2; echo \"done-$v\" >> %s",
		shQuote(versionPath),
		shQuote(buildEvents),
		shQuote(buildEvents),
	)
	execCmd := fmt.Sprintf(
		"v=$(cat %s); echo \"start-$v\" >> %s; trap 'exit 0' TERM INT; while :; do sleep 1; done",
		shQuote(versionPath),
		shQuote(execLog),
	)

	h := newFlowHarness(t, root, buildCmd, execCmd, 120*time.Millisecond)
	defer h.Close()

	waitForCondition(t, 4*time.Second, func() bool {
		return strings.Contains(fileContent(buildEvents), "start-v1")
	}, "in-progress initial build start")

	writeFile(t, versionPath, "v2")

	waitForCondition(t, 6*time.Second, func() bool {
		content := fileContent(buildEvents)
		return strings.Contains(content, "start-v2") && strings.Contains(content, "done-v2")
	}, "canceled old build and completed latest build")

	content := fileContent(buildEvents)
	if strings.Contains(content, "done-v1") {
		t.Fatalf("expected old build to be canceled before completion, got events:\n%s", content)
	}
}

func TestIntegration_NewDirectoryIsWatched(t *testing.T) {
	root := newTestRoot(t)
	artifacts := t.TempDir()
	triggerPath := filepath.Join(root, "trigger.txt")
	buildLog := filepath.Join(artifacts, "build.log")
	execLog := filepath.Join(artifacts, "exec.log")

	writeFile(t, triggerPath, "root")

	buildCmd := fmt.Sprintf("echo build >> %s", shQuote(buildLog))
	execCmd := fmt.Sprintf("trap 'exit 0' TERM INT; echo run >> %s; while :; do sleep 1; done", shQuote(execLog))

	h := newFlowHarness(t, root, buildCmd, execCmd, 120*time.Millisecond)
	defer h.Close()

	waitForCondition(t, 4*time.Second, func() bool {
		return lineCount(buildLog) >= 1
	}, "initial build")

	newDir := filepath.Join(root, "newpkg")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("failed to create new directory: %v", err)
	}

	waitForCondition(t, 4*time.Second, func() bool {
		return lineCount(buildLog) >= 2
	}, "rebuild after directory create")
	baseCount := lineCount(buildLog)

	newFile := filepath.Join(newDir, "x.go")
	writeFile(t, newFile, "package newpkg\n")
	time.Sleep(250 * time.Millisecond)
	writeFile(t, newFile, "package newpkg\n// changed\n")

	waitForCondition(t, 5*time.Second, func() bool {
		return lineCount(buildLog) >= baseCount+1
	}, "rebuild after file change in newly created directory")
}
