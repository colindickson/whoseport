# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**whoseport** is a lightweight CLI utility written in Go that identifies which process is using a specific network port on Unix systems. It provides both interactive (colorized UI) and JSON output modes, with the ability to kill processes directly or manage Docker containers.

- **Language**: Go 1.24.4
- **Module**: github.com/bluehoodie/whoseport
- **Type**: Standalone single-binary CLI application
- **Architecture**: Modern Go project layout with SOLID principles
- **Dependencies**: Minimal — stdlib plus two small Unicode width libraries (`go-runewidth`, `uniseg`)
- **Platform**: Unix/Linux systems with `lsof` and `/proc` filesystem
- **Docker Support**: Automatic detection and management of Docker containers using ports

## Common Commands

```bash
# Build the binary locally
make build

# Install to $GOBIN (or $GOPATH/bin or $HOME/go/bin)
make install

# Run tests
make test

# Clean build artifacts
make clean

# Local testing after build
./whoseport 8080
```

## Code Architecture

The project follows a modern Go layout with clear separation of concerns:

### Directory Structure
```
cmd/whoseport/         # Main application entry point
internal/
  action/              # Process actions (kill, prompt)
  display/             # Output formatters
    format/            # Formatting utilities (bytes, memory, visual width, ANSI)
    interactive/       # Interactive UI displayer (box-drawn with colors/emoji)
    docker/            # Docker container UI displayer
    json/              # JSON displayer
  docker/              # Docker container detection, retrieval, and actions
  model/               # Core data structures (ProcessInfo)
  process/             # Process retrieval (lsof executor, parser, retriever)
  procfs/              # /proc filesystem parsing and enhancement
  terminal/            # Terminal theme and color constants
  testutil/            # Test fixtures and mocks
```

### Package Responsibilities

**cmd/whoseport** - Thin orchestration layer that:
- Parses CLI flags
- Retrieves process info via `process.Retriever`
- Enhances with `/proc` data via `procfs.Enhancer`
- Detects Docker containers via `docker.Detector`
- Displays using `display.Displayer` implementations (interactive, docker, or json)
- Handles actions via `action.Killer`/`action.Prompter` or `docker.ActionHandler`

**internal/model** - Core data structure:
- `ProcessInfo`: 40-field struct with JSON serialization
- Basic fields: Command, PID, User, FD, Type, Device, SizeOffset, Node, Name
- Enhanced fields: State, Memory, CPU, Uptime, Network connections, I/O stats

**internal/process** - Process retrieval:
- `Executor` interface: Runs lsof command
- `Parser` interface: Parses lsof output
- `Retriever` interface: Orchestrates executor + parser
- Fine-grained interfaces for easy testing

**internal/procfs** - /proc filesystem parsing:
- `Enhancer` interface: Enriches ProcessInfo with Linux-specific data
- Reads `/proc/[pid]/{status,stat,cmdline,io,limits}`
- Parses network connections from `/proc/net/{tcp,udp}`
- 350+ lines of /proc parsing logic

**internal/docker** - Docker container management:
- `Detector` interface: Identifies Docker-related processes
- `Retriever` interface: Retrieves container information via `docker inspect` and `docker stats`
- `ActionHandler`: Prompts for and executes Docker actions (stop, remove)
- `ContainerInfo`: Container-specific data structure with ports, image, stats, etc.
- Detects containers via: docker-proxy processes, cgroup analysis

**internal/display** - Output formatting:
- `format/`: Utility functions (FormatBytes, FormatMemory, VisualWidth, StripAnsiCodes)
- `interactive/`: Rich colorized UI with boxes, emoji, progress bars
- `docker/`: Docker-specific UI with container details, ports, resources
- `json/`: Structured JSON output for scripting

**internal/action** - Process actions:
- `Killer`: Sends a given signal (SIGKILL/SIGTERM/etc.) to a process by PID
- `Prompter`: Interactive prompt to pick a signal (SIGTERM/SIGKILL) or cancel

**internal/terminal** - Theme system:
- Color constants (basic, bright, 256-color palette)
- Theme struct for consistent color schemes

### CLI Interface
- Entry point: `cmd/whoseport/main.go`
- Flag parsing: Go standard `flag` package
- Flags: `-k/--kill`, `-t/--term`, `-n/--no-interactive`, `--json` (implies `-n`)
- Required argument: Port number (integer)

## Key Implementation Details

### External Dependencies
- **`lsof` command**: Used via subprocess to find processes listening on ports
- **`/proc` filesystem**: Linux-specific, provides process and network state
- **`docker` CLI**: Optional, used for Docker container detection and management
- Graceful fallback when /proc data or Docker unavailable

### Data Flow

**Regular Process Flow:**
```
Port Number
    ↓
lsof -sTCP:LISTEN -F (field-mode output, no grep subprocess)
    ↓
Parse lsof output → ProcessInfo struct
    ↓
Enhance with /proc filesystem data
    ↓
Docker detection (check cgroup, docker-proxy process name)
    ↓
Display (JSON or Interactive UI)
    ↓
Optional: Kill process with SIGTERM/SIGKILL
```

**Docker Container Flow:**
```
Port Number
    ↓
lsof -sTCP:LISTEN -F (field-mode output, no grep subprocess)
    ↓
Parse lsof output → ProcessInfo struct
    ↓
Enhance with /proc filesystem data
    ↓
Docker detection → Container ID found
    ↓
Retrieve container info (docker inspect + docker stats)
    ↓
Display Docker-specific UI with container details
    ↓
Optional: Stop/Remove container (docker stop/rm)
```

### Color & Terminal Output
- 15 predefined ANSI color constants
- UTF-8 box-drawing characters for UI (╔╗╚╝═║─)
- Emoji support with proper width calculation for alignment
- Handles terminal width constraints gracefully

## Testing

Comprehensive test coverage following TDD principles:
- Run with: `make test` or `go test ./...`
- Test files: `cmd/whoseport/main_test.go`, `internal/model/process_test.go`, `internal/process/*_test.go`
- Test utilities: `internal/testutil/` with fixtures and mocks
- Integration tests cover full pipeline from lsof parsing to /proc enhancement
- All public interfaces have corresponding tests

## Code Style and Patterns

**SOLID Principles:**
- **Single Responsibility**: Each package has one clear purpose
- **Open/Closed**: Interfaces allow extension without modification
- **Liskov Substitution**: Implementations are interchangeable
- **Interface Segregation**: Fine-grained interfaces (Executor, Parser, Enhancer)
- **Dependency Inversion**: Depends on abstractions, not concrete types

**Architecture:**
- Interface-driven design for testability
- Dependency injection via constructors
- No global state (flags are local to main)
- Clear separation between data, logic, and presentation
- Table-driven tests throughout

**Error Handling:**
- Errors bubble up through return values
- Main() calls os.Exit() for user-facing errors
- Graceful degradation when /proc data unavailable

## Development Workflow

1. **Make changes** to relevant package in `internal/` or `cmd/`
2. **Write/update tests** following TDD
3. **Test locally**: `make test && make build && ./whoseport [port]`
4. **Verify all modes**: Test with `--json`, interactive, and `-k` flags
5. **Consider cross-platform**: Code runs on Unix systems only; test assumptions about `/proc` and `lsof`
6. **Test edge cases**: Non-existent ports, processes without /proc data, permission issues
