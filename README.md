# whoseport

A comprehensive CLI utility to identify and manage processes listening on network ports. Written in Go with rich terminal UI, detailed process information, and minimal external dependencies.

## Features

- **Rich Interactive Display** - Colorized, emoji-enhanced UI with detailed process metrics
- **Docker Container Detection** - Automatically detects and displays Docker containers using ports
- **Docker-Specific Actions** - Stop or remove containers instead of killing processes
- **Multiple Output Modes** - Interactive (default), JSON, or information-only modes
- **Comprehensive Process Data** - Combines `lsof` output with Linux `/proc` filesystem for deep insights
- **Process Management** - Kill processes with interactive prompt or direct signal support
- **Cross-Platform** - Runs on Linux and macOS with platform-specific optimizations
- **Minimal Dependencies** - Standard library plus two small Unicode width libraries

## Prerequisites

- **Unix/Linux system** with `lsof` command
- **Go 1.24+** (for building from source)
- On **Linux**: Enhanced features via `/proc` filesystem
- On **macOS**: Basic features via `lsof` (graceful degradation without `/proc`)

## Installation

### Using Go Install (Recommended)

If you have Go installed:

```bash
go install github.com/bluehoodie/whoseport/cmd/whoseport@latest
```

This installs the binary to `$GOBIN` (or `$GOPATH/bin`, or `$HOME/go/bin` by default).

### From Source

Clone the repository and install:

```bash
git clone https://github.com/bluehoodie/whoseport.git
cd whoseport
make install
```

Or build locally for testing:

```bash
make build
./whoseport 8080
```

## Usage

### Basic Usage

```bash
whoseport 8080
```

### Display Modes

**Default: Interactive Mode**
Shows detailed process information with an interactive prompt to kill the process.

```bash
whoseport 8080
```

**Information-Only Mode** (no prompt)
```bash
whoseport -n 8080
whoseport --no-interactive 8080
```

**JSON Output** (for scripting)
`--json` always implies `-n`, so the output is nothing but parseable JSON on stdout:
```bash
whoseport --json 8080 | jq .command
```

### Process Management

**Kill with Interactive Prompt** (default behavior)
```bash
whoseport 8080
# Prompts: Select action: [1] SIGTERM  [2] SIGKILL  [3] Cancel
```

**Terminate Gracefully** (SIGTERM - allows cleanup)
```bash
whoseport -t 8080
whoseport --term 8080
```

**Force Kill** (SIGKILL - immediate termination)
```bash
whoseport -k 8080
whoseport --kill 8080
```

### Command-Line Options

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--kill` | `-k` | Force kill the process immediately (SIGKILL) without prompting |
| `--term` | `-t` | Gracefully terminate the process (SIGTERM) without prompting |
| `--no-interactive` | `-n` | Show process info only, no interactive prompt |
| `--json` | | Output in JSON format for scripting |

**Note**: SIGTERM allows processes to clean up resources and exit gracefully, while SIGKILL forcefully terminates the process immediately without cleanup.

## Output Examples

### Interactive Mode (Default)

```
╔════════════════════════════════════════════════════════════════════════════════════════╗
║                              🔍 PORT 8080 ANALYSIS                                     ║
╚════════════════════════════════════════════════════════════════════════════════════════╝

  ▌ ⚙️  PROCESS IDENTITY
  Command:             node
  Full Command:        node server.js --port 8080
  Process ID:          42573
  Parent PID:          41234
  Parent Process:      bash
  User:                colin
  UID / GID:           501 / 20
  Child Processes:     3

  ▌ 📦 BINARY INFORMATION
  Executable Path:     /usr/local/bin/node
  Binary Size:         47.3 MB
  Environment Vars:    42
  Working Directory:   /home/colin/projects

  ▌ 📊 PROCESS STATE
  State:               🟢 S (Sleeping)
  Threads:             12
  Started:             2025-11-01 10:23:15
  Uptime:              ⏱ 2h 34m
  CPU Time:            45.23s (2.5%)

  ▌ 💾 MEMORY USAGE
  Resident Set (RSS):  124.5 MB
  Virtual Memory:      2.3 GB

  ▌ 📁 FILE DESCRIPTORS
  Open FDs:            23 / 1024 (2.2%)
  [██░░░░░░░░░░░░░░░░░░░░░░░░░░░░]

  ▌ 🌐 NETWORK
  Protocol:            IPV4
  Listening On:        🎧 *:8080 (LISTEN)
  Node Type:           TCP
  Total Connections:   5
  ▸ TCP Connections
    ┃ 192.168.1.100:45678 → 192.168.1.50:8080 ESTABLISHED
    ┃ 192.168.1.101:45679 → 192.168.1.50:8080 ESTABLISHED
```

The UI adapts to your terminal width (capped at 90 columns, floored at 40) instead of a fixed layout.

### JSON Mode

```bash
$ whoseport --json 8080
```

```json
{
  "command": "node",
  "id": 42573,
  "user": "colin",
  "fd": "7u",
  "type": "IPv4",
  "device": "0x5e4b104643390241",
  "size_offset": "0t0",
  "node": "TCP",
  "name": "*:8080 (LISTEN)",
  "full_command": "node server.js --port 8080",
  "ppid": 41234,
  "parent_command": "bash",
  "state": "S",
  "threads": 12,
  "working_dir": "/home/colin/projects",
  "memory_rss_kb": 127488,
  "memory_vms_kb": 2457600,
  "cpu_time_seconds": 45.23,
  "start_time": "2025-11-01 10:23:15",
  "uptime": "2h 34m",
  "open_fds": 23,
  "max_fds": 1024,
  "uid": 501,
  "gid": 20,
  "network_connections": 5,
  "tcp_connections": [
    "192.168.1.100:45678 → 192.168.1.50:8080 ESTABLISHED",
    "192.168.1.101:45679 → 192.168.1.50:8080 ESTABLISHED"
  ],
  "exe_path": "/usr/local/bin/node",
  "exe_size_bytes": 49643520,
  "io_read_bytes": 1048576,
  "io_write_bytes": 524288
}
```

## Docker Container Support

When **whoseport** detects that a port is being used by a Docker container, it automatically switches to a Docker-specific display and action mode.

### Docker Detection

The tool automatically detects Docker containers in two ways:

1. **docker-proxy processes** - Detects port forwarding proxies created by Docker
2. **Container cgroup** - Checks if a process is running inside a container via `/proc/{pid}/cgroup`

### Docker Display Mode

When a Docker container is detected, you'll see a specialized display with container-specific information:

```
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                           🐳  PORT 8080 → DOCKER CONTAINER                               ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝

  ▌ 🐳 CONTAINER IDENTITY
  Container Name:      📦 my-web-app
  Container ID:        abc123def456
    Full ID:           abc123def456789012345678901234567890123456789012
  State:               ✅ RUNNING
  Running For:         ⏱ 2 hours 15 minutes

  ▌ 📀 IMAGE
  Image:               nginx:latest
  Image ID:            sha256:abcd1234
  Platform:            linux/amd64
  Command:             ⚙️ nginx -g daemon off;

  ▌ 🌐 NETWORK & PORTS
  Port Mappings:
    *:8080:80          👉 TCP    (← matched port)
    *:8443:443         TCP
  IP Address:          🔗 172.17.0.2
  Gateway:             172.17.0.1
  Networks:            bridge

  ▌ 📊 RESOURCE USAGE
  CPU Usage:           ⚡ 2.5%
  Memory Usage:        💾 45.2MiB / 512MiB (8.8%)
  Network I/O:         📡 1.2MB / 856KB
  Block I/O:           💿 4.5MB / 0B
  PIDs:                12

  ▌ ⚙️  CONFIGURATION
  Restart Policy:      always
  Mounts:              📂 2 volume(s)
    ┃ /var/lib/docker/volumes/web-data → /usr/share/nginx/html (volume, rw)
    ┃ /home/user/config/nginx.conf → /etc/nginx/nginx.conf (bind, ro)
  Labels:              🏷 3 label(s)
    ┃ com.docker.compose.project=myapp
    ┃ com.docker.compose.service=web
    ┃ version=1.0.0

  ▌ 🔧 UNDERLYING PROCESS
  Process ID:          12345
  Process Command:     docker-proxy
  Note: The process above is the Docker proxy/container process.
  Actions below will affect the container, not just the process.
```

### Docker Actions

When interacting with a Docker container, you have different action options:

**Interactive Mode** (default)
```bash
whoseport 8080
```

You'll be prompted with Docker-specific options:
```
🐳 Container my-web-app (abc123def456) - Select action:
  [1] Stop container (docker stop)
  [2] Stop and remove container (docker stop + docker rm)
  [3] Force remove running container (docker rm -f)
  [4] Cancel
Choice [4]:
```

**Direct Actions**

On a Docker-backed port, both `-t` and `-k` just stop the container (`docker stop`) without
prompting; neither one removes it. Removing a container is only ever done as an explicit
choice ([2] or [3]) in the interactive menu above - it's never a silent default.

```bash
whoseport -t 8080  # Stops the Docker container gracefully
whoseport -k 8080  # Also just stops the container - does not remove it
```

**JSON Output** on a Docker-backed port emits the container's JSON (not the process JSON):
```bash
whoseport --json 8080 | jq .name
```

### Docker Prerequisites

Docker container detection requires:
- Docker daemon running and accessible
- `docker` CLI command available in PATH
- Sufficient permissions to run `docker` commands

If Docker is not available or detection fails, **whoseport** gracefully falls back to showing regular process information.

## Architecture

Built following SOLID principles with clear separation of concerns:

- **`cmd/whoseport`** - Main entry point with CLI flag parsing
- **`internal/process`** - Process retrieval using `lsof` (executor, parser, retriever)
- **`internal/procfs`** - `/proc` filesystem parsing for enhanced data (Linux-specific)
- **`internal/docker`** - Docker container detection, information retrieval, and actions
- **`internal/model`** - Core `ProcessInfo` data structure (40+ fields)
- **`internal/display`** - Output formatters:
  - `interactive` - Rich UI for regular processes
  - `docker` - Docker-specific UI with container details
  - `json` - Structured JSON output
- **`internal/action`** - Process actions (killer, prompter)
- **`internal/terminal`** - Terminal theme and color constants
- **`internal/testutil`** - Test fixtures and mocks

Interface-driven design enables easy testing and extension.

## Process Information Collected

### Basic (from `lsof`)
- Command name, PID, User, File Descriptor
- Protocol type (TCP/UDP), Network type (IPv4/IPv6)
- Port and listening address

If a process listens on the same port over both IPv4 and IPv6 (a common pattern), whoseport
reports the first listener it finds rather than corrupting the output.

### Enhanced (from `/proc` on Linux)
- **Process Details**: Full command line, parent process, state, threads
- **Memory**: RSS, VMS, memory limits
- **CPU**: CPU time, usage percentage, uptime
- **I/O**: Read/write bytes and syscall counts
- **Network**: Active TCP/UDP connections with addresses
- **Files**: Open file descriptor count and limits
- **Binary**: Executable path, size, working directory
- **Identity**: UID, GID, groups, nice value, priority

## Development

### Building

```bash
# Build for local testing
make build

# Install to $GOBIN
make install

# Run tests
make test

# Clean build artifacts
make clean
```

### Testing

Comprehensive test coverage following TDD principles:

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
```

### Cross-Compilation

Build for different platforms:

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o whoseport-linux-amd64 ./cmd/whoseport

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o whoseport-darwin-amd64 ./cmd/whoseport

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o whoseport-darwin-arm64 ./cmd/whoseport
```

### CI/CD

GitHub Actions workflow includes:
- **Testing** on Ubuntu and macOS with Go 1.24
- **Race detection** for concurrency issues
- **Test coverage** reporting
- **Cross-compilation** for multiple platforms, building both the Linux- and Darwin-tagged
  source on every run regardless of which OS the job happens to run on
- **Linting** with `go vet` (native and Darwin-targeted), `go fmt`, and `staticcheck`
- **Security scanning** with `govulncheck`

## Code Style

- **SOLID principles** throughout
- **Interface-driven** design for testability
- **Dependency injection** via constructors
- **Table-driven tests** for comprehensive coverage
- **No global state** - all dependencies explicitly passed
- **Graceful degradation** when `/proc` unavailable

## Platform Support

| Platform | `lsof` | `/proc` Enhancement | Status |
|----------|--------|---------------------|--------|
| Linux    | ✅     | ✅                  | Fully supported |
| macOS    | ✅     | ❌                  | Basic support (graceful degradation) |
| BSD      | ✅     | ❌                  | Untested (should work with basic features) |
| Windows  | ❌     | ❌                  | Not supported |

## Dependencies

**Runtime**: None (just `lsof` command)

**Build Dependencies**:
- Go 1.24+
- `github.com/mattn/go-runewidth` - Unicode width calculation
- `github.com/rivo/uniseg` - Unicode segmentation

## License

See the [LICENSE](LICENSE) file for details.

## Acknowledgements

This project stands on the shoulders of many similar utilities. It was created to combine:
- The simplicity of `lsof`-based port queries
- The richness of `/proc` filesystem data
- A modern, colorful terminal UI
- Clean, testable Go architecture

Inspired by countless command-line utilities and aliases that solve this same problem.
