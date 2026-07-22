//go:build linux

package procfs

import (
	"bytes"
	"fmt"
	"net"
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
	info.ChildCount = countChildren(pid)

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

	// sawMemoryLimit tracks whether a relevant limit line was found at all;
	// haveFiniteMemoryLimit tracks whether any of those lines gave a real
	// (non-unlimited) number. A -1 sentinel must not poison the running-min
	// comparison, so it's tracked separately rather than compared directly.
	sawMemoryLimit := false
	haveFiniteMemoryLimit := false

	for _, line := range lines {
		if strings.Contains(line, "Max address space") || strings.Contains(line, "Max data size") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				sawMemoryLimit = true
				limit := fields[3]
				if limit != "unlimited" {
					if limitVal, err := strconv.ParseInt(limit, 10, 64); err == nil {
						limitVal /= 1024
						if !haveFiniteMemoryLimit || limitVal < info.MemoryLimit {
							info.MemoryLimit = limitVal
							haveFiniteMemoryLimit = true
						}
					}
				}
			}
		}
		if strings.Contains(line, "Max open files") {
			fields := strings.Fields(line)
			// "Max" "open" "files" <soft> <hard> "files"
			if len(fields) >= 4 && fields[3] != "unlimited" {
				info.MaxFDs, _ = strconv.Atoi(fields[3])
			}
		}
	}

	if sawMemoryLimit && !haveFiniteMemoryLimit {
		info.MemoryLimit = -1
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

	// Both the IPv4 (e.g. "tcp") and IPv6 (e.g. "tcp6") variants need to be
	// read; the kernel keeps them in separate files.
	for _, file := range []string{
		fmt.Sprintf("/proc/net/%s", protocol),
		fmt.Sprintf("/proc/net/%s6", protocol),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
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
	}

	return connections
}

// countChildren sums the child PIDs listed in /proc/[pid]/task/[tid]/children
// across all of the process's threads, instead of scanning every entry in
// /proc and reading each one's status file.
func countChildren(pid int) int {
	taskDir := fmt.Sprintf("/proc/%d/task", pid)
	tasks, err := os.ReadDir(taskDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, task := range tasks {
		data, err := os.ReadFile(filepath.Join(taskDir, task.Name(), "children"))
		if err != nil {
			// ponytail: /proc/[pid]/task/[tid]/children requires
			// CONFIG_PROC_CHILDREN; if it's unavailable, ChildCount just
			// stays 0 for this decorative field rather than falling back to
			// a full /proc sweep.
			continue
		}
		count += len(strings.Fields(string(data)))
	}
	return count
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

	if ip == "0.0.0.0" || ip == "::" {
		return fmt.Sprintf("*:%d", port)
	}

	return fmt.Sprintf("%s:%d", ip, port)
}

// HexToIP converts a /proc/net/{tcp,udp}[6] hex address to an IP string.
// IPv4 addresses are 4 bytes; IPv6 addresses are 16 bytes stored as 4
// little-endian 32-bit words. In both cases the raw bytes are byte-reversed
// per 32-bit word relative to a plain big-endian hex decode.
func HexToIP(hex string) string {
	switch len(hex) {
	case 8:
		return net.IP(reverseHexWords(hex)).String()
	case 32:
		b := make([]byte, 0, 16)
		for w := 0; w < 4; w++ {
			b = append(b, reverseHexWords(hex[w*8:w*8+8])...)
		}
		return net.IP(b).String()
	}
	return hex
}

// reverseHexWords decodes an 8-character (4-byte) hex string and reverses
// the byte order, matching the little-endian word encoding /proc/net uses
// for addresses.
func reverseHexWords(hex string) []byte {
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		val, _ := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		b[3-i] = byte(val)
	}
	return b
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
