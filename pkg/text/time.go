package text

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Formats a unix timestamp to ISO 8601 date (yyyy-mm-dd).
func FormatTime(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("2006-01-02")
}

// Formats a unix timestamp to ISO 8601 date (Mon 02 Jan 2006 03:04:05 PM MST).
func FormatTimeQuery(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("Mon 02 Jan 2006 03:04:05 PM MST")
}

// ParseDuration extends time.ParseDuration with support for "d" (days).
// An empty string returns (0, nil), meaning disabled.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	if rest, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}

		return time.Duration(n) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}

// FormatDuration formats a duration as at most two significant units ("2d", "3h", "38d8h", "1h30m", "45m").
// Sub-minute precision is discarded.
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)

	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)

	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
