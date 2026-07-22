package process

import (
	"errors"
	"strings"
	"testing"
)

func TestLsofParser_Parse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCmd      string
		wantPID      int
		wantUser     string
		wantFD       string
		wantType     string
		wantDevice   string
		wantOffset   string
		wantNode     string
		wantName     string
		wantErrAny   bool
		wantSentinel bool // if true, err must satisfy errors.Is(err, ErrNoService)
	}{
		{
			name: "single row",
			input: "p623\n" +
				"crapportd\n" +
				"Lcolindickson\n" +
				"f10\n" +
				"tIPv4\n" +
				"d0xbda6d5072c7491cc\n" +
				"o0t0\n" +
				"PTCP\n" +
				"n*:60970\n",
			wantCmd:    "rapportd",
			wantPID:    623,
			wantUser:   "colindickson",
			wantFD:     "10",
			wantType:   "IPv4",
			wantDevice: "0xbda6d5072c7491cc",
			wantOffset: "0t0",
			wantNode:   "TCP",
			wantName:   "*:60970 (LISTEN)",
		},
		{
			// Real-world case: a process listening on both IPv4 and IPv6 emits two
			// file records under the same process record. Only the first row's
			// fields must appear in the result - nothing from the second row.
			name: "two rows same PID (IPv4+IPv6)",
			input: "p623\n" +
				"crapportd\n" +
				"Lcolindickson\n" +
				"f10\n" +
				"tIPv4\n" +
				"d0xbda6d5072c7491cc\n" +
				"o0t0\n" +
				"PTCP\n" +
				"n*:60970\n" +
				"f11\n" +
				"tIPv6\n" +
				"d0xa9b4680e463066b7\n" +
				"o0t0\n" +
				"PTCP\n" +
				"n*:60970\n",
			wantCmd:    "rapportd",
			wantPID:    623,
			wantUser:   "colindickson",
			wantFD:     "10",
			wantType:   "IPv4",
			wantDevice: "0xbda6d5072c7491cc",
			wantOffset: "0t0",
			wantNode:   "TCP",
			wantName:   "*:60970 (LISTEN)",
		},
		{
			// Two distinct processes both listening on the same port: the first
			// process record's first file row wins, and none of the second
			// process's fields leak into the result.
			name: "two rows different PIDs",
			input: "p100\n" +
				"cnode\n" +
				"Lalice\n" +
				"f5\n" +
				"tIPv4\n" +
				"PTCP\n" +
				"n*:8080\n" +
				"p200\n" +
				"cpython3\n" +
				"Lbob\n" +
				"f6\n" +
				"tIPv6\n" +
				"PTCP\n" +
				"n*:8080\n",
			wantCmd:  "node",
			wantPID:  100,
			wantUser: "alice",
			wantFD:   "5",
			wantType: "IPv4",
			wantNode: "TCP",
			wantName: "*:8080 (LISTEN)",
		},
		{
			name:         "empty input",
			input:        "",
			wantErrAny:   true,
			wantSentinel: true,
		},
		{
			name:         "no file records - process record only",
			input:        "p623\ncrapportd\nLcolindickson\n",
			wantErrAny:   true,
			wantSentinel: true,
		},
		{
			name:       "malformed - non-numeric pid",
			input:      "pabcd\ncnode\nf1\n",
			wantErrAny: true,
		},
	}

	parser := NewLsofParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parser.Parse([]byte(tt.input))

			if tt.wantErrAny {
				if err == nil {
					t.Fatalf("Parse() error = nil, want error")
				}
				if tt.wantSentinel && !errors.Is(err, ErrNoService) {
					t.Errorf("Parse() error = %v, want errors.Is(err, ErrNoService)", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			if info.Command != tt.wantCmd {
				t.Errorf("Command = %q, want %q", info.Command, tt.wantCmd)
			}
			if info.ID != tt.wantPID {
				t.Errorf("ID = %d, want %d", info.ID, tt.wantPID)
			}
			if info.User != tt.wantUser {
				t.Errorf("User = %q, want %q", info.User, tt.wantUser)
			}
			if info.FD != tt.wantFD {
				t.Errorf("FD = %q, want %q", info.FD, tt.wantFD)
			}
			if info.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", info.Type, tt.wantType)
			}
			if tt.wantDevice != "" && info.Device != tt.wantDevice {
				t.Errorf("Device = %q, want %q", info.Device, tt.wantDevice)
			}
			if tt.wantOffset != "" && info.SizeOffset != tt.wantOffset {
				t.Errorf("SizeOffset = %q, want %q", info.SizeOffset, tt.wantOffset)
			}
			if info.Node != tt.wantNode {
				t.Errorf("Node = %q, want %q", info.Node, tt.wantNode)
			}
			if info.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", info.Name, tt.wantName)
			}

			// Regression guard for the bug this rewrite fixes: nothing from a
			// second row may ever end up folded into a field of the first.
			if strings.Contains(info.Name, "\n") {
				t.Errorf("Name contains a newline / fragment of another row: %q", info.Name)
			}
		})
	}
}
