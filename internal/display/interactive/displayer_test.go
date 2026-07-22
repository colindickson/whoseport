package interactive

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bluehoodie/whoseport/internal/display/format"
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

// TestPrintGradientBannerNarrowWidthDoesNotPanic guards the same class of
// bug fixed in the Docker banner: padding math must never go negative when
// the terminal width is smaller than the banner text.
func TestPrintGradientBannerNarrowWidthDoesNotPanic(t *testing.T) {
	d := NewDisplayer()
	d.width = 20

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("printGradientBanner panicked at narrow width: %v", r)
		}
	}()

	captureOutput(func() {
		d.printGradientBanner("PORT 12345 ANALYSIS")
	})
}
