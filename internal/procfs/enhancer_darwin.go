//go:build darwin

package procfs

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bluehoodie/whoseport/internal/model"
)

// ProcessEnhancer enhances ProcessInfo with macOS-specific process data.
type ProcessEnhancer struct{}

// NewProcessEnhancer creates a new ProcessEnhancer.
func NewProcessEnhancer() *ProcessEnhancer {
	return &ProcessEnhancer{}
}

// Enhance populates ProcessInfo with data from macOS system calls and ps command.
func (e *ProcessEnhancer) Enhance(info *model.ProcessInfo) error {
	pid := info.ID

	// Use ps command to get detailed process information
	// Format: rss,vsz,time,etime,%cpu,state,pri,nice,uid,gid,comm,command
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "rss=,vsz=,time=,etime=,%cpu=,state=,pri=,nice=,uid=,gid=,comm=,command=")
	output, err := cmd.Output()
	if err == nil {
		parsePsOutput(string(output), info)
	}

	// Get parent process info
	cmd = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid=")
	if output, err := cmd.Output(); err == nil {
		info.PPid, _ = strconv.Atoi(strings.TrimSpace(string(output)))
	}

	// Get parent command if we have PPid
	if info.PPid > 0 {
		cmd = exec.Command("ps", "-p", strconv.Itoa(info.PPid), "-o", "comm=")
		if output, err := cmd.Output(); err == nil {
			info.ParentCommand = strings.TrimSpace(string(output))
		}
	}

	// Get working directory (this works on macOS)
	cmd = exec.Command("lsof", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "n") {
				info.WorkingDir = strings.TrimPrefix(line, "n")
				break
			}
		}
	}

	// Get full command line
	if info.FullCommand == "" {
		cmd = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
		if output, err := cmd.Output(); err == nil {
			info.FullCommand = strings.TrimSpace(string(output))
		}
	}

	// Get executable path
	cmd = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	if output, err := cmd.Output(); err == nil {
		comm := strings.TrimSpace(string(output))
		// ps -o comm= on macOS normally already returns an absolute path.
		// If it doesn't, leave ExePath empty rather than guessing at a
		// path via the caller's own $PATH, which can resolve to a
		// completely different binary than the one actually running.
		if strings.HasPrefix(comm, "/") {
			info.ExePath = comm
		}

		// Get executable size if we have a path
		if info.ExePath != "" {
			if stat, err := os.Stat(info.ExePath); err == nil {
				info.ExeSize = stat.Size()
			}
		}
	}

	// Count open file descriptors using lsof
	cmd = exec.Command("lsof", "-p", strconv.Itoa(pid))
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
		// Subtract 1 for header line
		if len(lines) > 1 {
			info.OpenFDs = len(lines) - 1
		}
	}

	// Get file descriptor limit
	cmd = exec.Command("launchctl", "limit", "maxfiles")
	if output, err := cmd.Output(); err == nil {
		fields := strings.Fields(string(output))
		// Output format: "maxfiles 256 unlimited" or similar
		if len(fields) >= 2 {
			if softLimit, err := strconv.Atoi(fields[1]); err == nil {
				info.MaxFDs = softLimit
			}
		}
	}

	// ponytail: EnvCount is left at 0 on macOS. `ps -E`/`ps e` puts the
	// environment on the process's own single line (not one var per line
	// as this code used to assume), and on modern macOS (verified on
	// 26.2/Tahoe) it prints no environment at all, even for the caller's
	// own child processes - SIP blocks it. If a real need for this shows
	// up, look at proc_pidpath-style APIs via cgo; not worth it for a
	// decorative field.

	// Count child processes
	cmd = exec.Command("pgrep", "-P", strconv.Itoa(pid))
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 && lines[0] != "" {
			info.ChildCount = len(lines)
		}
	}

	// Get network connections using lsof (single call, split by protocol)
	info.TCPConns, info.UDPConns = getNetworkConnectionsLsof(pid)
	info.NetworkConns = len(info.TCPConns) + len(info.UDPConns)

	// Note: macOS doesn't easily provide IO statistics without specialized APIs
	// Leaving IOReadBytes, IOWriteBytes, etc. as 0

	return nil
}

func parsePsOutput(output string, info *model.ProcessInfo) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return
	}

	// Parse the single line of output (no header with = format)
	fields := strings.Fields(lines[0])
	if len(fields) < 10 {
		return
	}

	// RSS (Resident Set Size) in KB
	if rss, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
		info.MemoryRSS = rss
	}

	// VSZ (Virtual Size) in KB
	if vsz, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
		info.MemoryVMS = vsz
	}

	// CPU time (format: MM:SS.SS or HH:MM:SS)
	if cpuTime := fields[2]; cpuTime != "" {
		info.CPUTime = parseCPUTime(cpuTime)
	}

	// Elapsed time (format: [[DD-]HH:]MM:SS)
	if etime := fields[3]; etime != "" {
		info.Uptime = parseElapsedTime(etime)
		// Also calculate start time
		if uptime := parseElapsedDuration(etime); uptime > 0 {
			startTime := time.Now().Add(-uptime)
			info.StartTime = startTime.Format("2006-01-02 15:04:05")
		}
	}

	// CPU percentage
	if cpu, err := strconv.ParseFloat(fields[4], 64); err == nil {
		info.CPUPercent = cpu
	}

	// State
	if len(fields[5]) > 0 {
		info.State = ExpandState(fields[5])
	}

	// Priority
	if pri, err := strconv.Atoi(fields[6]); err == nil {
		info.Priority = pri
	}

	// Nice value
	if nice, err := strconv.Atoi(fields[7]); err == nil {
		info.NiceValue = nice
	}

	// UID
	if uid, err := strconv.Atoi(fields[8]); err == nil {
		info.UID = uid
	}

	// GID
	if gid, err := strconv.Atoi(fields[9]); err == nil {
		info.GID = gid
	}

	// Number of threads - use a separate ps call
	pid := info.ID
	cmd := exec.Command("ps", "-M", "-p", strconv.Itoa(pid))
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
		// Count lines minus header
		if len(lines) > 1 {
			info.Threads = len(lines) - 1
		}
	}
}

func parseCPUTime(timeStr string) float64 {
	// Format can be MM:SS.SS or HH:MM:SS or HH:MM:SS.SS
	parts := strings.Split(timeStr, ":")
	var hours, minutes, seconds float64

	switch len(parts) {
	case 2: // MM:SS
		minutes, _ = strconv.ParseFloat(parts[0], 64)
		seconds, _ = strconv.ParseFloat(parts[1], 64)
	case 3: // HH:MM:SS
		hours, _ = strconv.ParseFloat(parts[0], 64)
		minutes, _ = strconv.ParseFloat(parts[1], 64)
		seconds, _ = strconv.ParseFloat(parts[2], 64)
	}

	return hours*3600 + minutes*60 + seconds
}

func parseElapsedTime(etime string) string {
	// Format: [[DD-]HH:]MM:SS
	duration := parseElapsedDuration(etime)
	return FormatDuration(duration)
}

func parseElapsedDuration(etime string) time.Duration {
	var days, hours, minutes, seconds int

	// Check for days (DD-)
	if strings.Contains(etime, "-") {
		parts := strings.Split(etime, "-")
		days, _ = strconv.Atoi(parts[0])
		etime = parts[1]
	}

	// Parse time components
	timeParts := strings.Split(etime, ":")
	switch len(timeParts) {
	case 2: // MM:SS
		minutes, _ = strconv.Atoi(timeParts[0])
		seconds, _ = strconv.Atoi(timeParts[1])
	case 3: // HH:MM:SS
		hours, _ = strconv.Atoi(timeParts[0])
		minutes, _ = strconv.Atoi(timeParts[1])
		seconds, _ = strconv.Atoi(timeParts[2])
	}

	totalSeconds := days*86400 + hours*3600 + minutes*60 + seconds
	return time.Duration(totalSeconds) * time.Second
}

func getNetworkConnectionsLsof(pid int) (tcpConns, udpConns []string) {
	// -a: AND the selection criteria (internet files AND this PID)
	// -i: internet files
	// -n: no hostname resolution
	// -P: no port name resolution
	// -p: filter by PID
	cmd := exec.Command("lsof", "-a", "-i", "-n", "-P", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	return parseNetworkConnections(string(output))
}

// parseNetworkConnections splits a single `lsof -a -i -n -P -p PID` output
// into TCP and UDP connection strings, so both protocols come from one
// subprocess call instead of two identical ones.
func parseNetworkConnections(output string) (tcpConns, udpConns []string) {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// fields[8] contains the connection info (e.g., "localhost:8080->localhost:52242 (ESTABLISHED)")
		connInfo := fields[8]
		if len(fields) > 9 {
			// Sometimes state is in a separate field
			connInfo += " " + strings.Join(fields[9:], " ")
		}

		switch {
		case strings.HasPrefix(fields[7], "TCP"):
			tcpConns = append(tcpConns, connInfo)
		case strings.HasPrefix(fields[7], "UDP"):
			udpConns = append(udpConns, connInfo)
		}
	}

	return tcpConns, udpConns
}

// ExpandState expands process state code to human-readable string (macOS version).
func ExpandState(state string) string {
	states := map[string]string{
		"R": "Running",
		"S": "Sleeping",
		"I": "Idle",
		"T": "Stopped",
		"U": "Uninterruptible wait",
		"Z": "Zombie",
	}

	if len(state) > 0 {
		mainState := string(state[0])
		expanded := states[mainState]
		if expanded == "" {
			expanded = state
		}

		if len(state) > 1 {
			modifiers := []string{}
			for _, c := range state[1:] {
				switch c {
				case '+':
					modifiers = append(modifiers, "foreground")
				case '<':
					modifiers = append(modifiers, "high-priority")
				case 'N':
					modifiers = append(modifiers, "low-priority")
				case 'E':
					modifiers = append(modifiers, "trying to exit")
				case 'L':
					modifiers = append(modifiers, "pages locked")
				case 's':
					modifiers = append(modifiers, "session leader")
				case 'l':
					modifiers = append(modifiers, "multi-threaded")
				case 'W':
					modifiers = append(modifiers, "swapped out")
				case 'X':
					modifiers = append(modifiers, "traced")
				}
			}
			if len(modifiers) > 0 {
				expanded += " (" + strings.Join(modifiers, ", ") + ")"
			}
		}
		return expanded
	}
	return state
}

// FormatDuration formats a duration to human-readable string.
func FormatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))

	return strings.Join(parts, " ")
}
