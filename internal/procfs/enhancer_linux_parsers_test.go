//go:build linux

package procfs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/testutil"
)

// TestParseStatus verifies the fields pulled from /proc/[pid]/status,
// against a realistic fixture, land in the right ProcessInfo fields with the
// right types (Uid/Gid/VmRSS/VmSize are columns, not single values).
func TestParseStatus(t *testing.T) {
	info := &model.ProcessInfo{}
	parseStatus(testutil.SampleProcStatus(), info)

	if info.PPid != 1000 {
		t.Errorf("PPid = %d, want 1000", info.PPid)
	}
	if info.Threads != 4 {
		t.Errorf("Threads = %d, want 4", info.Threads)
	}
	if info.MemoryRSS != 48000 {
		t.Errorf("MemoryRSS = %d, want 48000", info.MemoryRSS)
	}
	if info.MemoryVMS != 120000 {
		t.Errorf("MemoryVMS = %d, want 120000", info.MemoryVMS)
	}
	if info.UID != 1001 {
		t.Errorf("UID = %d, want 1001", info.UID)
	}
	if info.GID != 1001 {
		t.Errorf("GID = %d, want 1001", info.GID)
	}
	if info.Groups != "1001" {
		t.Errorf("Groups = %q, want %q", info.Groups, "1001")
	}
}

// TestParseIO verifies /proc/[pid]/io counters land in the right fields;
// read/write byte counts and syscall counts are easy to transpose since they
// share the same "key: value" line shape.
func TestParseIO(t *testing.T) {
	info := &model.ProcessInfo{}
	parseIO(testutil.SampleProcIO(), info)

	if info.IOReadBytes != 1048576 {
		t.Errorf("IOReadBytes = %d, want 1048576", info.IOReadBytes)
	}
	if info.IOWriteBytes != 2097152 {
		t.Errorf("IOWriteBytes = %d, want 2097152", info.IOWriteBytes)
	}
	if info.IOReadSyscalls != 123456 {
		t.Errorf("IOReadSyscalls = %d, want 123456", info.IOReadSyscalls)
	}
	if info.IOWriteSyscalls != 98765 {
		t.Errorf("IOWriteSyscalls = %d, want 98765", info.IOWriteSyscalls)
	}
}

// TestParseTCPState verifies the /proc/net/tcp hex state byte is decoded to
// its named socket state rather than left as a raw hex string, which is what
// a caller actually wants to display.
func TestParseTCPState(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"01", "ESTABLISHED"},
		{"02", "SYN_SENT"},
		{"03", "SYN_RECV"},
		{"04", "FIN_WAIT1"},
		{"05", "FIN_WAIT2"},
		{"06", "TIME_WAIT"},
		{"07", "CLOSE"},
		{"08", "CLOSE_WAIT"},
		{"09", "LAST_ACK"},
		{"0A", "LISTEN"},
		{"0B", "CLOSING"},
		{"FF", "FF"}, // unknown code falls back to the raw hex
		{"", ""},     // empty returns empty
	}

	for _, tt := range tests {
		if got := ParseTCPState(tt.hex); got != tt.want {
			t.Errorf("ParseTCPState(%q) = %q, want %q", tt.hex, got, tt.want)
		}
	}
}

// TestGetBootTime verifies the kernel boot time is read from the "btime"
// line of /proc/stat, not guessed from some other line/field.
func TestGetBootTime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(testutil.SampleProcBootStat()), 0o644); err != nil {
		t.Fatal(err)
	}

	got := getBootTime(dir)
	if got != 1609459200 {
		t.Errorf("getBootTime() = %d, want 1609459200", got)
	}
}

// TestGetBootTimeFallback verifies a missing/unreadable /proc/stat falls
// back to the current time instead of returning a zero epoch (which would
// make every process look decades old).
func TestGetBootTimeFallback(t *testing.T) {
	got := getBootTime(filepath.Join(t.TempDir(), "does-not-exist"))
	if delta := time.Now().Unix() - got; delta < 0 || delta > 5 {
		t.Errorf("getBootTime() fallback = %d, want within 5s of now", got)
	}
}

// TestParseStatStartTimeAndUptime pins down info.StartTime against a fixed
// btime (from the SampleProcBootStat fixture) and a fixed starttime tick
// count (from the SampleProcStat fixture), so a future off-by-one on
// idxStarttime is caught even though this file only runs in CI/Linux. The
// expected value is computed independently here (not by re-deriving the
// production formula) so a real regression in parseStat can't cancel out.
// Uptime is derived from time.Since and is therefore not pinned to an exact
// string; it's only checked for the expected "Nd Nh Nm Ns" shape.
func TestParseStatStartTimeAndUptime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(testutil.SampleProcBootStat()), 0o644); err != nil {
		t.Fatal(err)
	}

	info := &model.ProcessInfo{}
	parseStat(testutil.SampleProcStat(), info, dir)

	// btime = 1609459200 (SampleProcBootStat), starttime = 1234567890 ticks
	// (SampleProcStat, field 22 per man 5 proc) -> 12345678s after boot.
	const btime = 1609459200
	const starttimeTicks = 1234567890
	wantStartTime := time.Unix(btime+starttimeTicks/100, 0).Format("2006-01-02 15:04:05")
	if info.StartTime != wantStartTime {
		t.Errorf("StartTime = %q, want %q", info.StartTime, wantStartTime)
	}

	if !uptimeShape.MatchString(info.Uptime) {
		t.Errorf("Uptime = %q, want to match %s", info.Uptime, uptimeShape)
	}
}

var uptimeShape = regexp.MustCompile(`^\d+d \d+h \d+m \d+s$`)
