//go:build linux

package procfs

import (
	"testing"

	"github.com/bluehoodie/whoseport/internal/model"
)

// TestParseStatCommWithSpacesAndParens verifies that a comm field containing
// spaces and parentheses (e.g. Firefox's "(Web Content)") does not shift the
// numeric field indices that follow it.
func TestParseStatCommWithSpacesAndParens(t *testing.T) {
	// Fields after comm (from state=3 onward, per man 5 proc), all set to
	// easily distinguishable values so a wrong index is caught.
	// state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime priority nice num_threads itrealvalue starttime
	stat := "1234 (Web Content (weird)) S 1 2 3 4 5 6 7 8 9 10 111 222 13 14 17 18 1 0 999900 extra fields here"

	info := &model.ProcessInfo{}
	parseStat(stat, info, "")

	if info.Priority != 17 {
		t.Errorf("Priority = %d, want 17", info.Priority)
	}
	if info.NiceValue != 18 {
		t.Errorf("NiceValue = %d, want 18", info.NiceValue)
	}
	// utime=111, stime=222 clock ticks -> (111+222)/100 = 3.33
	if info.CPUTime != 3.33 {
		t.Errorf("CPUTime = %v, want 3.33", info.CPUTime)
	}
}

// TestParseStatNoSpacesInComm ensures the common case (no spaces in comm)
// still parses correctly.
func TestParseStatNoSpacesInComm(t *testing.T) {
	stat := "1234 (bash) S 1 2 3 4 5 6 7 8 9 10 100 50 13 14 20 0 1 0 500000 extra"

	info := &model.ProcessInfo{}
	parseStat(stat, info, "")

	if info.Priority != 20 {
		t.Errorf("Priority = %d, want 20", info.Priority)
	}
	if info.NiceValue != 0 {
		t.Errorf("NiceValue = %d, want 0", info.NiceValue)
	}
	if info.CPUTime != 1.5 {
		t.Errorf("CPUTime = %v, want 1.5", info.CPUTime)
	}
}

// TestParseLimitsMaxFDs verifies MaxFDs comes from the "Max open files" soft
// limit (an RLIMIT_NOFILE ceiling), not from FDSize (a table-allocation
// count that has nothing to do with the limit).
func TestParseLimitsMaxFDs(t *testing.T) {
	limits := "Limit                     Soft Limit           Hard Limit           Units\n" +
		"Max open files            1024                 4096                 files\n"

	info := &model.ProcessInfo{}
	parseLimits(limits, info)

	if info.MaxFDs != 1024 {
		t.Errorf("MaxFDs = %d, want 1024", info.MaxFDs)
	}
}

// TestParseLimitsMaxFDsUnlimited ensures an unlimited soft limit does not
// get parsed as a bogus numeric value.
func TestParseLimitsMaxFDsUnlimited(t *testing.T) {
	limits := "Limit                     Soft Limit           Hard Limit           Units\n" +
		"Max open files            unlimited            unlimited            files\n"

	info := &model.ProcessInfo{}
	parseLimits(limits, info)

	if info.MaxFDs != 0 {
		t.Errorf("MaxFDs = %d, want 0 (unset)", info.MaxFDs)
	}
}

// TestParseLimitsMemoryLimitUnlimited verifies MemoryLimit is set to the
// documented -1 sentinel when the relevant limits are all "unlimited".
func TestParseLimitsMemoryLimitUnlimited(t *testing.T) {
	limits := "Limit                     Soft Limit           Hard Limit           Units\n" +
		"Max address space         unlimited            unlimited            bytes\n" +
		"Max data size             unlimited            unlimited            bytes\n"

	info := &model.ProcessInfo{}
	parseLimits(limits, info)

	if info.MemoryLimit != -1 {
		t.Errorf("MemoryLimit = %d, want -1", info.MemoryLimit)
	}
}

// TestParseLimitsMemoryLimitFinite verifies a real numeric limit is kept
// (as the min of the two candidate lines) and does not get overridden by
// the -1 "unlimited" sentinel.
func TestParseLimitsMemoryLimitFinite(t *testing.T) {
	limits := "Limit                     Soft Limit           Hard Limit           Units\n" +
		"Max address space         unlimited            unlimited            bytes\n" +
		"Max data size             1048576              2097152              bytes\n"

	info := &model.ProcessInfo{}
	parseLimits(limits, info)

	if info.MemoryLimit != 1024 {
		t.Errorf("MemoryLimit = %d, want 1024", info.MemoryLimit)
	}
}

// TestHexToIPv6 verifies IPv6 addresses from /proc/net/tcp6 (32 hex chars,
// 4 little-endian 32-bit words) are decoded instead of returning the
// literal string "IPv6".
func TestHexToIPv6(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"loopback ::1", "00000000000000000000000001000000", "::1"},
		{"unspecified ::", "00000000000000000000000000000000", "::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HexToIP(tt.hex)
			if got != tt.want {
				t.Errorf("HexToIP(%q) = %v, want %v", tt.hex, got, tt.want)
			}
		})
	}
}

// TestParseAddressIPv6 verifies IPv6 loopback/unspecified addresses format
// correctly through ParseAddress, including the "*" wildcard for unspecified.
func TestParseAddressIPv6(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"loopback:8080", "00000000000000000000000001000000:1F90", "::1:8080"},
		{"unspecified:3000", "00000000000000000000000000000000:0BB8", "*:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAddress(tt.hex)
			if got != tt.want {
				t.Errorf("ParseAddress(%q) = %v, want %v", tt.hex, got, tt.want)
			}
		})
	}
}
