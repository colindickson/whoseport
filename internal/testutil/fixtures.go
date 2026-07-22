package testutil

import (
	"time"

	"github.com/bluehoodie/whoseport/internal/model"
)

// SampleLsofOutput returns a typical lsof output for testing
func SampleLsofOutput() string {
	return `node    12345 testuser   21u  IPv4 0x123456      0t0  TCP *:8080 (LISTEN)`
}

// SampleLsofOutputMultipleFields returns lsof output with more complex formatting
func SampleLsofOutputMultipleFields() string {
	return `python3    9876 www-data   3u  IPv6 0xabcdef      0t0  TCP *:3000 (LISTEN)`
}

// SampleProcStatus returns sample content from /proc/[pid]/status
func SampleProcStatus() string {
	return `Name:	node
Umask:	0022
State:	S (sleeping)
Tgid:	12345
Ngid:	0
Pid:	12345
PPid:	1000
TracerPid:	0
Uid:	1001	1001	1001	1001
Gid:	1001	1001	1001	1001
FDSize:	256
Groups:	1001
NStgid:	12345
NSpid:	12345
NSpgid:	12345
NSsid:	12345
VmPeak:	  123456 kB
VmSize:	  120000 kB
VmLck:	       0 kB
VmPin:	       0 kB
VmHWM:	   50000 kB
VmRSS:	   48000 kB
RssAnon:   40000 kB
RssFile:    8000 kB
RssShmem:      0 kB
VmData:	   20000 kB
VmStk:	     132 kB
VmExe:	    4096 kB
VmLib:	   10000 kB
VmPTE:	     256 kB
VmSwap:	       0 kB
HugetlbPages:          0 kB
Threads:	4
SigQ:	0/127282
SigPnd:	0000000000000000
ShdPnd:	0000000000000000
SigBlk:	0000000000000000
SigIgn:	0000000000001000
SigCgt:	0000000180014002
CapInh:	0000000000000000
CapPrm:	0000000000000000
CapEff:	0000000000000000
CapBnd:	0000003fffffffff
CapAmb:	0000000000000000
NoNewPrivs:	0
Seccomp:	0
Speculation_Store_Bypass:	thread vulnerable
Cpus_allowed:	ff
Cpus_allowed_list:	0-7
Mems_allowed:	00000001
Mems_allowed_list:	0
voluntary_ctxt_switches:	1234
nonvoluntary_ctxt_switches:	567`
}

// SampleProcStat returns sample content from /proc/[pid]/stat
func SampleProcStat() string {
	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime priority nice num_threads itrealvalue starttime vsize rss rsslim ...
	return `12345 (node) S 1000 12345 12345 0 -1 4194304 12345 0 10 0 1234 567 0 0 20 0 4 0 1234567890 122880000 12000 18446744073709551615 4194304 5242880 140737488345216 0 0 0 0 0 65536 1 0 0 17 2 0 0 0 0 0 7340032 7864320 24576000 140737488348160 140737488348240 140737488348240 140737488350839 0`
}

// SampleProcIO returns sample content from /proc/[pid]/io
func SampleProcIO() string {
	return `rchar: 1234567890
wchar: 987654321
syscr: 123456
syscw: 98765
read_bytes: 1048576
write_bytes: 2097152
cancelled_write_bytes: 0`
}

// SampleProcLimits returns sample content from /proc/[pid]/limits
func SampleProcLimits() string {
	return `Limit                     Soft Limit           Hard Limit           Units
Max cpu time              unlimited            unlimited            seconds
Max file size             unlimited            unlimited            bytes
Max data size             unlimited            unlimited            bytes
Max stack size            8388608              unlimited            bytes
Max core file size        0                    unlimited            bytes
Max resident set          unlimited            unlimited            bytes
Max processes             127282               127282               processes
Max open files            1024                 1048576              files
Max locked memory         67108864             67108864             bytes
Max address space         unlimited            unlimited            bytes
Max file locks            unlimited            unlimited            locks
Max pending signals       127282               127282               signals
Max msgqueue size         819200               819200               bytes
Max nice priority         0                    0
Max realtime priority     0                    0
Max realtime timeout      unlimited            unlimited            us`
}

// SampleProcNetTCP returns sample content from /proc/net/tcp
func SampleProcNetTCP() string {
	return `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1001        0 123456 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0CEA 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1001        0 789012 1 0000000000000000 100 0 0 10 0`
}

// SampleProcNetUDP returns sample content from /proc/net/udp
func SampleProcNetUDP() string {
	return `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
   0: 00000000:1388 00000000:0000 07 00000000:00000000 00:00000000 00000000  1001        0 456789 2 0000000000000000 0`
}

// SampleProcNetTCP6 returns sample content from /proc/net/tcp6, listening on
// the IPv6 wildcard address.
func SampleProcNetTCP6() string {
	return `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1001        0 999999 1 0000000000000000 100 0 0 10 0`
}

// SampleProcStat returns sample content from /proc/stat for boot time
func SampleProcBootStat() string {
	return `cpu  1234567 890 2345678 9012345 67890 0 123456 0 0 0
cpu0 308641 222 586427 2253112 16972 0 30864 0 0 0
intr 123456789 10 20 30 40
ctxt 123456789012
btime 1609459200
processes 123456
procs_running 2
procs_blocked 0`
}

// MockProcessInfo creates a fully populated ProcessInfo for testing
func MockProcessInfo() *model.ProcessInfo {
	return &model.ProcessInfo{
		// Basic lsof fields
		Command:    "node",
		ID:         12345,
		User:       "testuser",
		FD:         "21u",
		Type:       "IPv4",
		Device:     "0x123456",
		SizeOffset: "0t0",
		Node:       "TCP",
		Name:       "*:8080 (LISTEN)",

		// Enhanced fields
		FullCommand:     "/usr/local/bin/node /app/server.js",
		PPid:            1000,
		ParentCommand:   "bash",
		State:           "S",
		Threads:         4,
		WorkingDir:      "/app",
		MemoryRSS:       48000,
		MemoryVMS:       120000,
		CPUTime:         17.0,
		StartTime:       time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Uptime:          "1d 0h 0m 0s",
		OpenFDs:         256,
		MaxFDs:          1024,
		UID:             1001,
		GID:             1001,
		Groups:          "1001",
		NetworkConns:    2,
		TCPConns:        []string{"127.0.0.1:8080 -> *:* (LISTEN)"},
		UDPConns:        []string{},
		ExePath:         "/usr/local/bin/node",
		ExeSize:         4096,
		NiceValue:       0,
		Priority:        20,
		EnvCount:        15,
		ChildCount:      0,
		IOReadBytes:     1048576,
		IOWriteBytes:    2097152,
		IOReadSyscalls:  123456,
		IOWriteSyscalls: 98765,
		CPUPercent:      5.5,
		MemoryLimit:     -1,
	}
}
