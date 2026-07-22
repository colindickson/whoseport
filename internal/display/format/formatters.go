package format

import (
	"fmt"
	"strings"
	"time"

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

// FormatDuration formats a duration to human-readable string, e.g.
// "2h 30m 15s" or "1d 0h 0m 0s".
func FormatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))

	return strings.Join(parts, " ")
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
func GetStateColor(state string, theme terminal.Theme) string {
	if strings.Contains(state, "Running") {
		return theme.StateRunning
	} else if strings.Contains(state, "Sleeping") {
		return theme.StateSleeping
	} else if strings.Contains(state, "Zombie") {
		return theme.StateZombie
	} else if strings.Contains(state, "Stopped") {
		return theme.StateStopped
	}
	return theme.StateDefault
}
