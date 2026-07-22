//go:build linux

package procfs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bluehoodie/whoseport/internal/model"
)

// ProcessEnhancer enhances ProcessInfo with /proc filesystem data.
type ProcessEnhancer struct{}

// NewProcessEnhancer creates a new ProcessEnhancer.
func NewProcessEnhancer() *ProcessEnhancer {
	return &ProcessEnhancer{}
}

// Enhance populates ProcessInfo with data from /proc filesystem.
func (e *ProcessEnhancer) Enhance(info *model.ProcessInfo) error {
	pid := info.ID

	// Read /proc/[pid]/cmdline for full command
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		info.FullCommand = strings.ReplaceAll(string(bytes.TrimSpace(cmdline)), "\x00", " ")
		if info.FullCommand == "" {
			info.FullCommand = info.Command
		}
	}

	// Read /proc/[pid]/status
	if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		parseStatus(string(status), info)
	}

	// Read /proc/[pid]/stat
	if stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		parseStat(string(stat), info)
	}

	// Get working directory
	if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
		info.WorkingDir = cwd
	}

	// Get executable path and size
	if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
		info.ExePath = exe
		if stat, err := os.Stat(exe); err == nil {
			info.ExeSize = stat.Size()
		}
	}

	// Count open file descriptors
	if fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid)); err == nil {
		info.OpenFDs = len(fds)
	}

	// Count environment variables
	if environ, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid)); err == nil {
		envVars := bytes.Split(environ, []byte{0})
		info.EnvCount = len(envVars) - 1
		if info.EnvCount < 0 {
			info.EnvCount = 0
		}
	}

	// Count child processes
	if tasks, err := os.ReadDir("/proc"); err == nil {
		for _, task := range tasks {
			if !task.IsDir() {
				continue
			}
			childPid, err := strconv.Atoi(task.Name())
			if err != nil {
				continue
			}
			if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", childPid)); err == nil {
				lines := strings.Split(string(status), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "PPid:") {
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							ppid, _ := strconv.Atoi(fields[1])
							if ppid == pid {
								info.ChildCount++
							}
						}
						break
					}
				}
			}
		}
	}

	// Get IO statistics
	if io, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid)); err == nil {
		parseIO(string(io), info)
	}

	// Get limits
	if limits, err := os.ReadFile(fmt.Sprintf("/proc/%d/limits", pid)); err == nil {
		parseLimits(string(limits), info)
	}

	// Get parent process info
	if info.PPid > 0 {
		if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", info.PPid)); err == nil {
			parentCmd := strings.ReplaceAll(string(bytes.TrimSpace(cmdline)), "\x00", " ")
			if parentCmd != "" {
				parts := strings.Fields(parentCmd)
				if len(parts) > 0 {
					info.ParentCommand = filepath.Base(parts[0])
				}
			}
		}
		if info.ParentCommand == "" {
			if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", info.PPid)); err == nil {
				info.ParentCommand = strings.TrimSpace(string(comm))
			}
		}
	}

	// Get network connections
	info.TCPConns = getNetworkConnections(pid, "tcp")
	info.UDPConns = getNetworkConnections(pid, "udp")
	info.NetworkConns = len(info.TCPConns) + len(info.UDPConns)

	return nil
}

func parseStatus(status string, info *model.ProcessInfo) {
	lines := strings.Split(status, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		switch key {
		case "PPid":
			info.PPid, _ = strconv.Atoi(fields[1])
		case "State":
			info.State = ExpandState(fields[1])
		case "Threads":
			info.Threads, _ = strconv.Atoi(fields[1])
		case "VmRSS":
			info.MemoryRSS, _ = strconv.ParseInt(fields[1], 10, 64)
		case "VmSize":
			info.MemoryVMS, _ = strconv.ParseInt(fields[1], 10, 64)
		case "Uid":
			if len(fields) > 1 {
				info.UID, _ = strconv.Atoi(fields[1])
			}
		case "Gid":
			if len(fields) > 1 {
				info.GID, _ = strconv.Atoi(fields[1])
			}
		case "Groups":
			if len(fields) > 1 {
				info.Groups = strings.Join(fields[1:], " ")
			}
		case "FDSize":
			info.MaxFDs, _ = strconv.Atoi(fields[1])
		}
	}
}

// parseStat parses /proc/[pid]/stat. Field 2 (comm) is wrapped in
// parentheses and can itself contain spaces and parentheses (e.g. Firefox's
// "Web Content" processes), so every field after it must be located relative
// to the LAST ')' rather than by naive whitespace splitting of the whole
// line (see man 5 proc).
func parseStat(stat string, info *model.ProcessInfo) {
	lastParen := strings.LastIndex(stat, ")")
	if lastParen == -1 || lastParen+1 >= len(stat) {
		return
	}

	// fields[0] is "state" (proc stat field 3); everything below is indexed
	// relative to that.
	fields := strings.Fields(stat[lastParen+1:])
	const (
		idxUtime     = 11 // field 14
		idxStime     = 12 // field 15
		idxPriority  = 15 // field 18
		idxNice      = 16 // field 19
		idxStarttime = 19 // field 22
	)
	if len(fields) <= idxStarttime {
		return
	}

	info.Priority, _ = strconv.Atoi(fields[idxPriority])
	info.NiceValue, _ = strconv.Atoi(fields[idxNice])

	utime, _ := strconv.ParseInt(fields[idxUtime], 10, 64)
	stime, _ := strconv.ParseInt(fields[idxStime], 10, 64)
	clockTicks := float64(utime + stime)
	// ponytail: assumes USER_HZ (sysconf(_SC_CLK_TCK)) == 100, true on
	// virtually all Linux builds; add a sysconf lookup if a target ever uses
	// a different tick rate.
	info.CPUTime = clockTicks / 100.0

	starttime, _ := strconv.ParseInt(fields[idxStarttime], 10, 64)
	bootTime := getBootTime()
	startTimeSec := bootTime + (starttime / 100)
	startTimeObj := time.Unix(startTimeSec, 0)
	info.StartTime = startTimeObj.Format("2006-01-02 15:04:05")

	uptime := time.Since(startTimeObj)
	info.Uptime = FormatDuration(uptime)

	if uptime.Seconds() > 0 {
		info.CPUPercent = (info.CPUTime / uptime.Seconds()) * 100.0
	}
}

func parseIO(io string, info *model.ProcessInfo) {
	lines := strings.Split(io, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, _ := strconv.ParseInt(fields[1], 10, 64)

		switch key {
		case "read_bytes":
			info.IOReadBytes = value
		case "write_bytes":
			info.IOWriteBytes = value
		case "syscr":
			info.IOReadSyscalls = value
		case "syscw":
			info.IOWriteSyscalls = value
		}
	}
}

func parseLimits(limits string, info *model.ProcessInfo) {
	lines := strings.Split(limits, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Max address space") || strings.Contains(line, "Max data size") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				limit := fields[3]
				if limit != "unlimited" {
					limitVal, _ := strconv.ParseInt(limit, 10, 64)
					if info.MemoryLimit == 0 || limitVal < info.MemoryLimit {
						info.MemoryLimit = limitVal / 1024
					}
				}
			}
		}
	}
}

func getBootTime() int64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Now().Unix()
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "btime") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				btime, _ := strconv.ParseInt(fields[1], 10, 64)
				return btime
			}
		}
	}
	return time.Now().Unix()
}

func getNetworkConnections(pid int, protocol string) []string {
	var connections []string

	inodes := getProcessInodes(pid)
	if len(inodes) == 0 {
		return connections
	}

	file := fmt.Sprintf("/proc/net/%s", protocol)
	data, err := os.ReadFile(file)
	if err != nil {
		return connections
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		inode := fields[9]
		if _, exists := inodes[inode]; exists {
			localAddr := ParseAddress(fields[1])
			remoteAddr := ParseAddress(fields[2])
			state := ""
			if protocol == "tcp" && len(fields) > 3 {
				state = ParseTCPState(fields[3])
			}

			connStr := fmt.Sprintf("%s -> %s", localAddr, remoteAddr)
			if state != "" {
				connStr += fmt.Sprintf(" [%s]", state)
			}
			connections = append(connections, connStr)
		}
	}

	return connections
}

func getProcessInodes(pid int) map[string]bool {
	inodes := make(map[string]bool)

	fdPath := fmt.Sprintf("/proc/%d/fd", pid)
	fds, err := os.ReadDir(fdPath)
	if err != nil {
		return inodes
	}

	for _, fd := range fds {
		link, err := os.Readlink(filepath.Join(fdPath, fd.Name()))
		if err != nil {
			continue
		}

		if strings.HasPrefix(link, "socket:[") {
			inode := strings.TrimPrefix(link, "socket:[")
			inode = strings.TrimSuffix(inode, "]")
			inodes[inode] = true
		}
	}

	return inodes
}

// ExpandState expands process state code to human-readable string.
func ExpandState(state string) string {
	states := map[string]string{
		"R": "Running",
		"S": "Sleeping (interruptible)",
		"D": "Waiting (uninterruptible)",
		"Z": "Zombie",
		"T": "Stopped",
		"t": "Tracing stop",
		"X": "Dead",
		"I": "Idle",
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
				case '<':
					modifiers = append(modifiers, "high-priority")
				case 'N':
					modifiers = append(modifiers, "low-priority")
				case 'L':
					modifiers = append(modifiers, "pages locked")
				case 's':
					modifiers = append(modifiers, "session leader")
				case 'l':
					modifiers = append(modifiers, "multi-threaded")
				case '+':
					modifiers = append(modifiers, "foreground")
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

// ParseAddress parses hex address to IP:port format.
func ParseAddress(hex string) string {
	parts := strings.Split(hex, ":")
	if len(parts) != 2 {
		return hex
	}

	ip := HexToIP(parts[0])
	port, _ := strconv.ParseInt(parts[1], 16, 64)

	if ip == "0.0.0.0" || ip == "0:0:0:0:0:0:0:0" {
		return fmt.Sprintf("*:%d", port)
	}

	return fmt.Sprintf("%s:%d", ip, port)
}

// HexToIP converts hex string to IP address.
func HexToIP(hex string) string {
	if len(hex) == 8 {
		var parts []string
		for i := 6; i >= 0; i -= 2 {
			val, _ := strconv.ParseInt(hex[i:i+2], 16, 64)
			parts = append(parts, fmt.Sprintf("%d", val))
		}
		return strings.Join(parts, ".")
	}
	return "IPv6"
}

// ParseTCPState parses hex TCP state to string.
func ParseTCPState(hex string) string {
	states := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}

	state, _ := strconv.ParseInt(hex, 16, 64)
	hexStr := fmt.Sprintf("%02X", state)
	if s, ok := states[hexStr]; ok {
		return s
	}
	return hex
}
