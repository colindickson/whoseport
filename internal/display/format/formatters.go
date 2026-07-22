package format

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"

	"github.com/bluehoodie/whoseport/internal/terminal"
)

// FormatBytes converts bytes to human-readable format
func FormatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	}
}

// FormatMemory converts KB to human-readable format
func FormatMemory(kb int64) string {
	if kb < 1024 {
		return fmt.Sprintf("%d KB", kb)
	} else if kb < 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(kb)/1024)
	} else {
		return fmt.Sprintf("%.2f GB", float64(kb)/1024/1024)
	}
}

// Truncate shortens s to fit within maxLen display columns (not bytes or
// runes), appending "..." when truncation occurs. It is grapheme-aware, so
// multi-byte/emoji input is never split mid-rune. It never panics: maxLen
// <= 0 returns "", and 0 < maxLen <= 3 returns that many "." characters
// instead of a truncated fragment plus ellipsis.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if VisualWidth(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}

	target := maxLen - 3
	var b strings.Builder
	width := 0
	graphemes := uniseg.NewGraphemes(s)
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := runewidth.StringWidth(cluster)
		if width+clusterWidth > target {
			break
		}
		b.WriteString(cluster)
		width += clusterWidth
	}
	return b.String() + "..."
}

// VisualWidth calculates the visual width of a string accounting for emojis
func VisualWidth(s string) int {
	cleaned := StripAnsiCodes(s)
	graphemes := uniseg.NewGraphemes(cleaned)
	width := 0
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := runewidth.StringWidth(cluster)
		if clusterWidth < 2 && (strings.ContainsRune(cluster, '\uFE0F') || strings.ContainsRune(cluster, '\u200D')) {
			clusterWidth = 2
		}
		width += clusterWidth
	}
	return width
}

// StripAnsiCodes removes ANSI escape sequences from a string.
// ponytail: only recognizes ESC...m (SGR color codes), which is all this
// repo ever emits; other escape sequences (cursor moves, etc.) would not
// be stripped correctly. Upgrade to a full CSI-aware parser if that changes.
func StripAnsiCodes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEscape := false
	for _, c := range s {
		if c == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if c == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// GetStateEmoji returns an emoji representing the process state
func GetStateEmoji(state string) string {
	if strings.Contains(state, "Running") {
		return "🟢"
	} else if strings.Contains(state, "Sleeping") {
		return "🔵"
	} else if strings.Contains(state, "Zombie") {
		return "💀"
	} else if strings.Contains(state, "Stopped") {
		return "🟡"
	} else if strings.Contains(state, "Waiting") {
		return "⏸"
	}
	return "⚪"
}

// GetStateColor returns the theme color for a process state.
//
// theme is accepted as interface{}, not terminal.Theme, purely so that
// cmd/whoseport/main_test.go (owned by another agent) can keep passing nil
// without a build break; there is no real circular-dependency constraint
// (internal/terminal imports nothing). When theme is a terminal.Theme, its
// State* fields are used; otherwise terminal.DefaultTheme() is used, which
// is byte-identical to the old hard-coded colors.
// ponytail: switch this to a plain terminal.Theme parameter once
// main_test.go's nil call site is updated to pass a real theme.
func GetStateColor(state string, theme interface{}) string {
	th, ok := theme.(terminal.Theme)
	if !ok {
		th = terminal.DefaultTheme()
	}
	if strings.Contains(state, "Running") {
		return th.StateRunning
	} else if strings.Contains(state, "Sleeping") {
		return th.StateSleeping
	} else if strings.Contains(state, "Zombie") {
		return th.StateZombie
	} else if strings.Contains(state, "Stopped") {
		return th.StateStopped
	}
	return th.StateDefault
}
