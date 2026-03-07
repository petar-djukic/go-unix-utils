// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ts utility for timestamping stdin lines.
//
// Implements prd004-ts: default timestamping (R1), custom format strings (R2),
// incremental mode (R3), elapsed mode (R4), monotonic clock (R5),
// relative-time conversion (R6), exit codes (R7), environment (R8).
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// tsMode selects the timestamping mode.
type tsMode int

const (
	modeDefault     tsMode = iota // wall-clock timestamp per line
	modeIncremental               // -i: time since previous line
	modeElapsed                   // -s: time since program start
	modeRelative                  // -r: convert existing timestamps to relative age
)

const (
	defaultFormat = "%b %d %H:%M:%S"
	elapsedFormat = "%H:%M:%S"
	usageMessage  = "usage: ts [-r] [-i | -s] [-m] [format]\n"
)

// config holds the parsed command-line options.
type config struct {
	mode   tsMode
	mono   bool
	format string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%s", err, usageMessage)
		os.Exit(255)
	}

	switch cfg.mode {
	case modeRelative:
		processRelative(cfg.format)
	default:
		processLines(cfg)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	var hasI, hasS, hasR bool

	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'i':
					hasI = true
				case 's':
					hasS = true
				case 'm':
					cfg.mono = true
				case 'r':
					hasR = true
				default:
					return config{}, fmt.Errorf("Unknown option: %c\n", ch)
				}
			}
		} else {
			cfg.format = arg
		}
	}

	// Precedence matches reference binary: -r > -i > -s > default.
	// The reference binary does not enforce mutual exclusivity.
	switch {
	case hasR:
		cfg.mode = modeRelative
	case hasI:
		cfg.mode = modeIncremental
	case hasS:
		cfg.mode = modeElapsed
	}

	// R1.2, R3.2, R4.2: Set default format based on mode.
	if cfg.format == "" {
		switch cfg.mode {
		case modeIncremental, modeElapsed:
			cfg.format = elapsedFormat
		default:
			cfg.format = defaultFormat
		}
	}

	return cfg, nil
}

// processLines reads stdin line by line and prepends timestamps.
// Handles default, incremental, and elapsed modes.
func processLines(cfg config) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	startTime := time.Now()
	prevTime := startTime

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			now := time.Now()
			var ts string

			switch cfg.mode {
			case modeIncremental:
				// R3.1: Time elapsed since previous line.
				elapsed := now.Sub(prevTime)
				ts = formatElapsed(cfg.format, elapsed)
				prevTime = now
			case modeElapsed:
				// R4.1: Time elapsed since program start.
				elapsed := now.Sub(startTime)
				ts = formatElapsed(cfg.format, elapsed)
			default:
				// R1.2: Wall-clock timestamp.
				ts = strftime(cfg.format, now)
			}

			writer.WriteString(ts)
			writer.WriteByte(' ')
			writer.Write(line)
			// R1.3: Flush after each line.
			writer.Flush()
		}
		if err != nil {
			break
		}
	}
}

// formatElapsed formats a duration as a timestamp using TZ=GMT (UTC).
// R3.2, R4.2: Create a time from epoch + elapsed, format in UTC.
func formatElapsed(format string, d time.Duration) string {
	t := time.Unix(0, 0).UTC().Add(d)
	return strftime(format, t)
}

// processRelative reads stdin and converts recognized timestamps to relative
// age strings (R6.1) or reformats them with a custom format (R6.3).
func processRelative(format string) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			result := convertRelativeLine(string(line), format)
			writer.WriteString(result)
			writer.Flush()
		}
		if err != nil {
			break
		}
	}
}

// convertRelativeLine processes a single line in -r mode.
func convertRelativeLine(line, format string) string {
	parsed, start, end, ok := parseTimestampInLine(line)
	if !ok {
		// R6.4: No recognized timestamp, pass through unchanged.
		return line
	}

	now := time.Now()
	var replacement string
	if format == defaultFormat {
		// R6.1: Convert to relative age string.
		replacement = formatRelative(now.Sub(parsed))
	} else {
		// R6.3: Reformat with the given format.
		replacement = strftime(format, parsed)
	}

	return line[:start] + replacement + line[end:]
}

// Timestamp patterns for -r mode (R6.2).
var (
	// ISO-8601: "2024-01-05T14:30:00.000Z" or with timezone offset.
	reISO = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`)

	// RFC 2822 with optional day name: "16 Jun 94 07:29:35 GMT".
	reRFC2822 = regexp.MustCompile(`(?:\w{3},?\s+)?\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}(?:\s+\w+)?`)

	// Syslog: "Jan  5 14:30:00".
	reSyslog = regexp.MustCompile(`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`)

	// Lastlog: "Mon Jan  5 14:30".
	reLastlog = regexp.MustCompile(`(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}`)
)

// parseTimestampInLine attempts to find and parse a timestamp in the line.
// Returns the parsed time, the start and end indices of the match, and success.
func parseTimestampInLine(line string) (time.Time, int, int, bool) {
	now := time.Now()

	// Try ISO-8601 first (most specific).
	if loc := reISO.FindStringIndex(line); loc != nil {
		match := line[loc[0]:loc[1]]
		layouts := []string{
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000-07:00",
			"2006-01-02T15:04:05-07:00",
			"2006-01-02T15:04:05.000-0700",
			"2006-01-02T15:04:05-0700",
			"2006-01-02T15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, match); err == nil {
				return t, loc[0], loc[1], true
			}
		}
	}

	// Try RFC 2822.
	if loc := reRFC2822.FindStringIndex(line); loc != nil {
		match := line[loc[0]:loc[1]]
		layouts := []string{
			"Mon, 02 Jan 2006 15:04:05 MST",
			"02 Jan 2006 15:04:05 MST",
			"Mon, 02 Jan 06 15:04:05 MST",
			"02 Jan 06 15:04:05 MST",
			"Mon, 2 Jan 2006 15:04:05 MST",
			"2 Jan 2006 15:04:05 MST",
			"Mon, 2 Jan 06 15:04:05 MST",
			"2 Jan 06 15:04:05 MST",
			"02 Jan 06 15:04:05",
			"2 Jan 06 15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, match); err == nil {
				return t, loc[0], loc[1], true
			}
		}
	}

	// Try syslog format (assumes current year).
	if loc := reSyslog.FindStringIndex(line); loc != nil {
		match := line[loc[0]:loc[1]]
		if t, err := time.Parse("Jan  2 15:04:05", match); err == nil {
			t = t.AddDate(now.Year(), 0, 0)
			return t, loc[0], loc[1], true
		}
		if t, err := time.Parse("Jan 2 15:04:05", match); err == nil {
			t = t.AddDate(now.Year(), 0, 0)
			return t, loc[0], loc[1], true
		}
	}

	// Try lastlog format (assumes current year).
	if loc := reLastlog.FindStringIndex(line); loc != nil {
		match := line[loc[0]:loc[1]]
		if t, err := time.Parse("Mon Jan  2 15:04", match); err == nil {
			t = t.AddDate(now.Year(), 0, 0)
			return t, loc[0], loc[1], true
		}
		if t, err := time.Parse("Mon Jan 2 15:04", match); err == nil {
			t = t.AddDate(now.Year(), 0, 0)
			return t, loc[0], loc[1], true
		}
	}

	return time.Time{}, 0, 0, false
}

// formatRelative converts a duration to a human-readable relative age string.
// Matches the Perl ts output format: "Xs ago", "XmXs ago", "XhXm ago", etc.
func formatRelative(d time.Duration) string {
	if d < 0 {
		return fmt.Sprintf("%.0fs in the future", math.Abs(d.Seconds()))
	}

	totalSec := int64(d.Seconds())

	switch {
	case totalSec < 60:
		return fmt.Sprintf("%ds ago", totalSec)
	case totalSec < 3600:
		m := totalSec / 60
		s := totalSec % 60
		return fmt.Sprintf("%dm%ds ago", m, s)
	case totalSec < 86400:
		h := totalSec / 3600
		m := (totalSec % 3600) / 60
		return fmt.Sprintf("%dh%dm ago", h, m)
	case totalSec < 86400*365:
		d := totalSec / 86400
		h := (totalSec % 86400) / 3600
		return fmt.Sprintf("%dd%dh ago", d, h)
	default:
		y := totalSec / (86400 * 365)
		d := (totalSec % (86400 * 365)) / 86400
		return fmt.Sprintf("%dy%dd ago", y, d)
	}
}

// strftime formats a time.Time using a strftime-compatible format string.
// Supports standard strftime specifiers and ts-specific extensions:
// %.S (seconds with microseconds), %.s (epoch with microseconds),
// %.T (HH:MM:SS with microseconds). R2.2, R2.3.
func strftime(format string, t time.Time) string {
	var buf strings.Builder
	runes := []rune(format)

	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' {
			buf.WriteRune(runes[i])
			continue
		}
		i++
		if i >= len(runes) {
			buf.WriteByte('%')
			break
		}

		switch runes[i] {
		case '%':
			buf.WriteByte('%')
		case 'a':
			buf.WriteString(t.Format("Mon"))
		case 'A':
			buf.WriteString(t.Format("Monday"))
		case 'b', 'h':
			buf.WriteString(t.Format("Jan"))
		case 'B':
			buf.WriteString(t.Format("January"))
		case 'c':
			buf.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
		case 'C':
			fmt.Fprintf(&buf, "%02d", t.Year()/100)
		case 'd':
			fmt.Fprintf(&buf, "%02d", t.Day())
		case 'D':
			fmt.Fprintf(&buf, "%02d/%02d/%02d", t.Month(), t.Day(), t.Year()%100)
		case 'e':
			fmt.Fprintf(&buf, "%2d", t.Day())
		case 'F':
			fmt.Fprintf(&buf, "%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
		case 'H':
			fmt.Fprintf(&buf, "%02d", t.Hour())
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&buf, "%02d", h)
		case 'j':
			fmt.Fprintf(&buf, "%03d", t.YearDay())
		case 'k':
			fmt.Fprintf(&buf, "%2d", t.Hour())
		case 'l':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&buf, "%2d", h)
		case 'm':
			fmt.Fprintf(&buf, "%02d", int(t.Month()))
		case 'M':
			fmt.Fprintf(&buf, "%02d", t.Minute())
		case 'n':
			buf.WriteByte('\n')
		case 'p':
			buf.WriteString(t.Format("PM"))
		case 'P':
			buf.WriteString(strings.ToLower(t.Format("PM")))
		case 'r':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			ampm := "AM"
			if t.Hour() >= 12 {
				ampm = "PM"
			}
			fmt.Fprintf(&buf, "%02d:%02d:%02d %s", h, t.Minute(), t.Second(), ampm)
		case 'R':
			fmt.Fprintf(&buf, "%02d:%02d", t.Hour(), t.Minute())
		case 's':
			fmt.Fprintf(&buf, "%d", t.Unix())
		case 'S':
			fmt.Fprintf(&buf, "%02d", t.Second())
		case 't':
			buf.WriteByte('\t')
		case 'T':
			fmt.Fprintf(&buf, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'u':
			d := int(t.Weekday())
			if d == 0 {
				d = 7
			}
			fmt.Fprintf(&buf, "%d", d)
		case 'w':
			fmt.Fprintf(&buf, "%d", int(t.Weekday()))
		case 'x':
			fmt.Fprintf(&buf, "%02d/%02d/%02d", int(t.Month()), t.Day(), t.Year()%100)
		case 'X':
			fmt.Fprintf(&buf, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'y':
			fmt.Fprintf(&buf, "%02d", t.Year()%100)
		case 'Y':
			fmt.Fprintf(&buf, "%04d", t.Year())
		case 'z':
			buf.WriteString(t.Format("-0700"))
		case 'Z':
			buf.WriteString(t.Format("MST"))
		case '.':
			// R2.3: ts-specific subsecond extensions.
			i++
			if i >= len(runes) {
				buf.WriteString("%.")
				break
			}
			usec := t.Nanosecond() / 1000
			switch runes[i] {
			case 'S':
				// R2.3: seconds with microsecond suffix.
				fmt.Fprintf(&buf, "%02d.%06d", t.Second(), usec)
			case 's':
				// R2.3: epoch with microsecond suffix.
				fmt.Fprintf(&buf, "%d.%06d", t.Unix(), usec)
			case 'T':
				// R2.3: HH:MM:SS with microsecond suffix.
				fmt.Fprintf(&buf, "%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), usec)
			default:
				buf.WriteString("%.")
				buf.WriteRune(runes[i])
			}
		default:
			buf.WriteByte('%')
			buf.WriteRune(runes[i])
		}
	}

	return buf.String()
}
