package docker

import (
	"io"
	"os"
	"testing"
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

// TestPrintDockerBannerNarrowWidthDoesNotPanic guards against a regression
// where a narrow width leaves no room for the whale emoji's leading space,
// driving leftPadding to 0 and strings.Repeat(" ", leftPadding-1) to panic.
func TestPrintDockerBannerNarrowWidthDoesNotPanic(t *testing.T) {
	d := NewDisplayer()
	d.width = 20

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("printDockerBanner panicked at narrow width: %v", r)
		}
	}()

	captureOutput(func() {
		d.printDockerBanner("PORT 12345 → DOCKER CONTAINER")
	})
}
