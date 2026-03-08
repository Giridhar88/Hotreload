# <span style="color:blue"> hotreload </span>

`hotreload` is a Go CLI tool that watches a project directory, rebuilds on change, and restarts the server automatically.

It is designed for backend development workflows where repeatedly stopping, rebuilding, and starting the server manually slows iteration.

## What it does

- Watches a root directory recursively (including nested folders)
- Detects file changes and batches rapid events with debouncing
- Triggers an initial build immediately on startup
- Rebuilds and restarts the server after successful builds
- Streams server logs directly to terminal in real time
- Kills process groups (not just parent PID) during restarts
- Cancels an in-progress rebuild if newer changes arrive, then rebuilds latest state
- Handles directory create/remove/rename events while running
- Ignores common noise (`.git`, `node_modules`, editor temp files, binary artifacts)
- Supports extra project-specific ignore patterns through `--ignore`

## Architecture

The tool is split into three core components:

1. Watcher (`watcher.go`)
- Uses `fsnotify` to subscribe to directories under `--root`
- Adds newly created directories dynamically
- Cleans up watcher state for removed/renamed paths
- Forwards relevant file events to the change channel

2. Debouncer (`debounce.go`)
- Receives raw change signals from watcher
- Resets a timer while events keep arriving
- Emits one rebuild signal once changes settle
- Also emits an initial signal shortly after startup to force first build

3. Runner (`runner.go`)
- Receives rebuild signals
- Stops existing server process group (SIGTERM, then SIGKILL fallback)
- Executes build command
- Starts exec command only when build succeeds
- Cancels current restart/build if a newer change arrives

`main.go` wires these pieces together and handles graceful shutdown via `SIGINT`/`SIGTERM`.

## Requirements

- Go 1.24+
- Unix-like environment recommended (process-group signaling uses `syscall` semantics common on Linux/macOS)

## Build

```bash
make build
```

Binary output:

```text
./bin/hotreload
```

## Usage

```bash
hotreload --root <project-folder> --build "<build-command>" --exec "<run-command>" [--ignore "<patterns>"]
```

Example:

```bash
./bin/hotreload \
  --root ./myproject \
  --build "go build -o ./bin/server ./cmd/server" \
  --exec "./bin/server"
```

Example with custom ignores:

```bash
./bin/hotreload \
  --root ./myproject \
  --build "go build -o ./bin/server ./cmd/server" \
  --exec "./bin/server" \
  --ignore "tmp,generated,cmd/internal,.env"
```

### Flags

- `--root`: directory to watch recursively
- `--build`: shell command used to build project
- `--exec`: shell command used to run built server
- `--ignore`: optional comma-separated file or directory names/paths to ignore; default is empty (no additional ignores)

## Demo with included test server

Run end-to-end demo:

```bash
make demo
```

This runs:

- watcher root: `./testserver`
- build command: `go build -o ./bin/testserver ./testserver`
- exec command: `./bin/testserver`

Open `http://localhost:8080` and edit `testserver/main.go`.  
On save, `hotreload` rebuilds and restarts automatically.

## Development workflow

Run tests:

```bash
make test
```

Or:

```bash
go test ./...
```

Clean generated binaries:

```bash
make clean
```

## Behavior details

- If build fails, server is not started/restarted until a successful future build
- If multiple file events happen quickly, they are coalesced into one rebuild
- If a build/restart is already running and new changes arrive:
  - current attempt is canceled
  - newest state is built next
- On restart, child processes are terminated with the parent process group
- Watcher add failures caused by OS limits are logged with actionable hints

## Project layout

```text
.
├── main.go           # CLI entrypoint and orchestration
├── watcher.go        # fsnotify integration and filtering
├── debounce.go       # event coalescing
├── runner.go         # build/exec lifecycle and process management
├── testserver/       # sample HTTP server for demo
├── *_test.go         # unit tests
└── Makefile          # build/test/demo helpers
```

## Troubleshooting

### Linux inotify limits

If you see watcher limit errors (`ENOSPC`, `too many open files`), increase inotify limits:

```bash
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=1024
```

Persist these values via your distro's sysctl config if needed.

### No rebuild on change

- Ensure edited files are under `--root`
- Check file path is not filtered by ignore rules
- Confirm build command works independently

## Loom video
Demonstration video of the tool
[Video Link](https://www.loom.com/share/b0486c05c9514e3097bb33dac865799b)
