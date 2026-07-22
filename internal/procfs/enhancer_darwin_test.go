//go:build darwin

package procfs

import "testing"

// TestParseNetworkConnectionsSplitsProtocols verifies that a single lsof
// output (as produced by one `lsof -a -i -n -P -p PID` call) is correctly
// split into TCP and UDP connections, and that non-TCP/UDP rows (ICMP,
// ICMPv6) are excluded. Fixture captured from real `lsof -a -i -n -P -p PID`
// output against a running process.
func TestParseNetworkConnectionsSplitsProtocols(t *testing.T) {
	output := "COMMAND    PID         USER   FD   TYPE             DEVICE SIZE/OFF   NODE NAME\n" +
		"com.docke 3122 colindickson   11u  IPv4 0xc1ea4b65225e5b6c      0t0    TCP 127.0.0.1:61622->127.0.0.1:61796 (ESTABLISHED)\n" +
		"com.docke 3122 colindickson   37u  IPv4 0x1d7cd8240855c14e      0t0   ICMP *:*\n" +
		"com.docke 3122 colindickson   50u  IPv6 0x2625bf8aaa6531af      0t0 ICMPV6 *:*\n" +
		"com.docke 3122 colindickson  131u  IPv4 0xe5028f65e472a458      0t0    TCP 127.0.0.1:12434 (LISTEN)\n" +
		"rapportd   623 colindickson   10u  IPv4 0xbda6d5072c7491cc      0t0    UDP *:60970\n"

	tcp, udp := parseNetworkConnections(output)

	if len(tcp) != 2 {
		t.Fatalf("tcp = %v, want 2 entries", tcp)
	}
	if tcp[0] != "127.0.0.1:61622->127.0.0.1:61796 (ESTABLISHED)" {
		t.Errorf("tcp[0] = %q, want established connection with state", tcp[0])
	}
	if tcp[1] != "127.0.0.1:12434 (LISTEN)" {
		t.Errorf("tcp[1] = %q, want listening entry with state", tcp[1])
	}

	if len(udp) != 1 {
		t.Fatalf("udp = %v, want 1 entry", udp)
	}
	if udp[0] != "*:60970" {
		t.Errorf("udp[0] = %q, want *:60970", udp[0])
	}
}

// TestParseNetworkConnectionsNoConnections ensures an lsof output with only
// a header (no connections found for the PID) yields no TCP or UDP entries
// instead of panicking or misreading the header row as data.
func TestParseNetworkConnectionsNoConnections(t *testing.T) {
	output := "COMMAND    PID         USER   FD   TYPE             DEVICE SIZE/OFF   NODE NAME\n"

	tcp, udp := parseNetworkConnections(output)

	if len(tcp) != 0 || len(udp) != 0 {
		t.Errorf("got tcp=%v udp=%v, want both empty", tcp, udp)
	}
}
