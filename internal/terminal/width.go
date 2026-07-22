package terminal

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// MaxWidth is the design width the UI was built for; detected/configured
// widths are never allowed to exceed it.
const MaxWidth = 90

// MinWidth is the narrowest width the UI will render at.
const MinWidth = 40

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

// Width returns a usable terminal width for the interactive UI, clamped to
// [MinWidth, MaxWidth]. It prefers the real terminal column count (queried
// via ioctl), falls back to the COLUMNS environment variable, and defaults
// to MaxWidth when stdout is not a TTY (e.g. piped to a file or pager) so
// redirected output stays stable.
func Width() int {
	if cols, ok := ttyWidth(); ok {
		return clampWidth(cols)
	}
	if cols, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil {
		return clampWidth(cols)
	}
	return MaxWidth
}

func clampWidth(w int) int {
	if w > MaxWidth {
		return MaxWidth
	}
	if w < MinWidth {
		return MinWidth
	}
	return w
}

func ttyWidth() (int, bool) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 {
		return 0, false
	}
	return int(ws.Col), true
}
