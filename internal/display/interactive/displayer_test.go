package interactive

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bluehoodie/whoseport/internal/display/format"
	"github.com/bluehoodie/whoseport/internal/model"
)

func captureOutput(fn func()) string {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(output)
}

func TestPrintModernSectionAlignment(t *testing.T) {
	d := NewDisplayer()

	output := captureOutput(func() {
		d.printModernSection("⚙️  PROCESS IDENTITY")
	})

	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}

	// Left accent bar style should produce exactly 1 line
	if len(lines) != 1 {
		t.Fatalf("expected 1 line for section, got %d", len(lines))
	}

	// Verify the line contains the left accent bar character
	stripped := format.StripAnsiCodes(lines[0])
	if !strings.Contains(stripped, "▌") {
		t.Fatalf("expected section to contain left accent bar '▌', got: %s", stripped)
	}

	// Verify the title appears in the output
	if !strings.Contains(stripped, "PROCESS IDENTITY") {
		t.Fatalf("expected section to contain title text, got: %s", stripped)
	}
}

// TestDisplayHidesEnvCountWhenZero guards against presenting an unmeasured
// value as fact: on macOS EnvCount is always 0 (SIP blocks reading another
// process's environment), so printing "Environment Vars: 0" would falsely
// claim the process has no environment. It should be omitted, matching how
// MemoryLimit and ChildCount are already conditionally rendered.
func TestDisplayHidesEnvCountWhenZero(t *testing.T) {
	d := NewDisplayer()
	info := &model.ProcessInfo{ExePath: "/usr/bin/node", EnvCount: 0}

	output := captureOutput(func() {
		d.Display(info, 8080)
	})

	if strings.Contains(output, "Environment Vars") {
		t.Fatalf("expected Environment Vars to be omitted when EnvCount is 0, got:\n%s", output)
	}
}

func TestDisplayShowsEnvCountWhenPositive(t *testing.T) {
	d := NewDisplayer()
	info := &model.ProcessInfo{ExePath: "/usr/bin/node", EnvCount: 15}

	output := captureOutput(func() {
		d.Display(info, 8080)
	})

	if !strings.Contains(output, "Environment Vars") {
		t.Fatalf("expected Environment Vars to be shown when EnvCount > 0, got:\n%s", output)
	}
}

// TestPrintGradientBannerNarrowWidthDoesNotPanic guards the same class of
// bug fixed in the Docker banner: padding math must never go negative when
// the terminal width is smaller than the banner text. Beyond not panicking,
// it also verifies the actual property the width fix is supposed to
// guarantee: a well-formed box (all three lines the same visual width, no
// garbling) with the banner text intact, rather than just an absence of a
// crash.
func TestPrintGradientBannerNarrowWidthDoesNotPanic(t *testing.T) {
	d := NewDisplayer()
	d.width = 20
	bannerText := "PORT 12345 ANALYSIS"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("printGradientBanner panicked at narrow width: %v", r)
		}
	}()

	output := captureOutput(func() {
		d.printGradientBanner(bannerText)
	})

	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(format.StripAnsiCodes(line)) == "" {
			continue
		}
		lines = append(lines, line)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 banner lines (top border, text, bottom border), got %d: %q", len(lines), lines)
	}

	width := format.VisualWidth(lines[0])
	for i, line := range lines {
		if w := format.VisualWidth(line); w != width {
			t.Fatalf("banner line %d has visual width %d, want %d (matching line 0) - box is misaligned:\n%s", i, w, width, output)
		}
	}

	if !strings.Contains(format.StripAnsiCodes(lines[1]), bannerText) {
		t.Fatalf("expected banner text %q intact in output, got: %s", bannerText, format.StripAnsiCodes(lines[1]))
	}
}
