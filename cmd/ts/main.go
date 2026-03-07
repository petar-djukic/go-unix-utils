// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ts command, which reads lines from stdin and
// prepends a timestamp to each line. Implements prd004-ts R1-R8.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the strftime format used when no format argument is given
// in the default (wall-clock) mode. (R1.2)
const defaultFormat = "%b %d %H:%M:%S"

// elapsedDefaultFormat is the strftime format used in -i and -s modes when
// no custom format is given. (R3.2, R4.2)
const elapsedDefaultFormat = "%H:%M:%S"

func main() {
	// R1.6: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	flagI := flag.Bool("i", false, "incremental time since previous line")
	flagS := flag.Bool("s", false, "elapsed time since start")
	flagM := flag.Bool("m", false, "use monotonic clock")
	flagR := flag.Bool("r", false, "convert existing timestamps to relative age")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ts [-i | -s] [-m] [-r] [format]\n")
	}
	flag.Parse()

	// R3.4: -i and -s are mutually exclusive.
	if *flagI && *flagS {
		fmt.Fprintf(os.Stderr, "usage: ts: -i and -s are mutually exclusive\n")
		os.Exit(1)
	}

	// R6.5: -r is mutually exclusive with -i and -s.
	if *flagR && (*flagI || *flagS) {
		fmt.Fprintf(os.Stderr, "usage: ts: -r cannot be used with -i or -s\n")
		os.Exit(1)
	}

	// R2.1: optional positional argument for custom format.
	format := ""
	if flag.NArg() > 0 {
		format = flag.Arg(0)
	}

	if *flagR {
		processRelative(format)
		return
	}

	if format == "" {
		if *flagI || *flagS {
			format = elapsedDefaultFormat
		} else {
			format = defaultFormat
		}
	}

	switch {
	case *flagI:
		processIncremental(format, *flagM)
	case *flagS:
		processElapsed(format, *flagM)
	default:
		processDefault(format, *flagM)
	}
}

// processDefault reads stdin line by line and prepends a wall-clock
// timestamp using the given strftime format. (R1, R2)
func processDefault(format string, _ bool) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			now := time.Now()
			ts := formatStrftime(format, now)
			// R1.1, R1.4: write timestamp + space + original line (preserving newline).
			fmt.Fprint(os.Stdout, ts, " ", line)
		}
		if err != nil {
			break
		}
	}
}

// processElapsed reads stdin and prepends elapsed time since start. (R4)
func processElapsed(format string, _ bool) {
	startTime := time.Now()
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			now := time.Now()
			elapsed := now.Sub(startTime)
			// R4.2, R8.2: format elapsed duration as a time in GMT.
			t := time.Unix(0, 0).UTC().Add(elapsed)
			ts := formatStrftime(format, t)
			fmt.Fprint(os.Stdout, ts, " ", line)
		}
		if err != nil {
			break
		}
	}
}

// processIncremental reads stdin and prepends time since the previous line. (R3)
func processIncremental(format string, _ bool) {
	lastTime := time.Now()
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			now := time.Now()
			elapsed := now.Sub(lastTime)
			lastTime = now
			// R3.2, R8.2: format elapsed duration as a time in GMT.
			t := time.Unix(0, 0).UTC().Add(elapsed)
			ts := formatStrftime(format, t)
			fmt.Fprint(os.Stdout, ts, " ", line)
		}
		if err != nil {
			break
		}
	}
}

// Timestamp patterns for -r mode (R6.2).
var relativePatterns = []struct {
	re     *regexp.Regexp
	layout string
	hasYear bool
}{
	{
		// ISO-8601: "2024-01-05T14:30:00.000Z" or "2024-01-05T14:30:00"
		re:      regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`),
		layout:  "2006-01-02T15:04:05",
		hasYear: true,
	},
	{
		// RFC 2822 with optional day: "16 Jun 94 07:29:35 GMT"
		re:      regexp.MustCompile(`\d{1,2}\s+[A-Z][a-z]{2}\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}\s*[A-Z]*`),
		layout:  "2 Jan 06 15:04:05 MST",
		hasYear: true,
	},
	{
		// Syslog: "Jan  5 14:30:00"
		re:      regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
		layout:  "Jan  2 15:04:05",
		hasYear: false,
	},
	{
		// Lastlog: "Mon Jan  5 14:30"
		re:      regexp.MustCompile(`[A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}`),
		layout:  "Mon Jan  2 15:04",
		hasYear: false,
	},
}

// processRelative reads stdin and replaces recognized timestamps with
// relative age strings (or reformats them if a format is given). (R6)
func processRelative(format string) {
	now := time.Now()
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			result := replaceTimestamp(line, now, format)
			fmt.Fprint(os.Stdout, result)
		}
		if err != nil {
			break
		}
	}
}

// replaceTimestamp finds the first recognized timestamp in a line and
// replaces it with a relative age string or reformatted timestamp. (R6.1-R6.4)
func replaceTimestamp(line string, now time.Time, format string) string {
	for _, pat := range relativePatterns {
		loc := pat.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		matched := line[loc[0]:loc[1]]

		parsed, parseErr := time.Parse(pat.layout, matched)
		if parseErr != nil {
			// Try alternate layouts for RFC 2822 with 4-digit year.
			if pat.hasYear {
				parsed, parseErr = time.Parse("2 Jan 2006 15:04:05 MST", matched)
			}
			if parseErr != nil {
				continue
			}
		}

		// For formats without a year, assume the current year.
		if !pat.hasYear {
			parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(),
				parsed.Hour(), parsed.Minute(), parsed.Second(),
				parsed.Nanosecond(), time.Local)
		}

		var replacement string
		if format != "" {
			// R6.3: reformat the parsed timestamp using the given format.
			replacement = formatStrftime(format, parsed)
		} else {
			// R6.1: show relative age.
			replacement = formatRelativeAge(now.Sub(parsed))
		}

		return line[:loc[0]] + replacement + line[loc[1]:]
	}
	// R6.4: no recognized timestamp; pass through unchanged.
	return line
}

// formatRelativeAge converts a duration to a human-readable relative age
// string such as "5d3h45m12s ago". (R6.1)
func formatRelativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, "") + " ago"
}

// formatStrftime converts a strftime format string to a formatted time
// string using Go's time package. Supports standard strftime(3) directives
// and the ts-specific subsecond extensions %.S, %.s, and %.T. (R2.2, R2.3, R2.4)
func formatStrftime(format string, t time.Time) string {
	var buf strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}

		// Consume the '%'.
		i++
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}

		// Check for ts-specific subsecond extensions (%.S, %.s, %.T).
		if format[i] == '.' && i+1 < len(format) {
			usec := t.Nanosecond() / 1000
			switch format[i+1] {
			case 'S':
				// R2.3: seconds with microsecond suffix (e.g., "32.001234").
				fmt.Fprintf(&buf, "%02d.%06d", t.Second(), usec)
				i += 2
				continue
			case 's':
				// R2.3: Unix epoch with microsecond suffix (e.g., "1708358732.001234").
				fmt.Fprintf(&buf, "%d.%06d", t.Unix(), usec)
				i += 2
				continue
			case 'T':
				// R2.3: HH:MM:SS with microsecond suffix (e.g., "14:05:32.001234").
				fmt.Fprintf(&buf, "%02d:%02d:%02d.%06d",
					t.Hour(), t.Minute(), t.Second(), usec)
				i += 2
				continue
			}
		}

		// Standard strftime directives.
		switch format[i] {
		case 'a':
			buf.WriteString(t.Format("Mon"))
		case 'A':
			buf.WriteString(t.Format("Monday"))
		case 'b', 'h':
			buf.WriteString(t.Format("Jan"))
		case 'B':
			buf.WriteString(t.Format("January"))
		case 'c':
			buf.WriteString(t.Format("Mon Jan  2 15:04:05 2006"))
		case 'C':
			fmt.Fprintf(&buf, "%02d", t.Year()/100)
		case 'd':
			buf.WriteString(t.Format("02"))
		case 'D':
			buf.WriteString(t.Format("01/02/06"))
		case 'e':
			fmt.Fprintf(&buf, "%2d", t.Day())
		case 'F':
			buf.WriteString(t.Format("2006-01-02"))
		case 'g':
			year, _ := t.ISOWeek()
			fmt.Fprintf(&buf, "%02d", year%100)
		case 'G':
			year, _ := t.ISOWeek()
			fmt.Fprintf(&buf, "%04d", year)
		case 'H':
			buf.WriteString(t.Format("15"))
		case 'I':
			buf.WriteString(t.Format("03"))
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
			buf.WriteString(t.Format("01"))
		case 'M':
			buf.WriteString(t.Format("04"))
		case 'n':
			buf.WriteByte('\n')
		case 'p':
			buf.WriteString(t.Format("PM"))
		case 'P':
			buf.WriteString(strings.ToLower(t.Format("PM")))
		case 'r':
			buf.WriteString(t.Format("03:04:05 PM"))
		case 'R':
			buf.WriteString(t.Format("15:04"))
		case 's':
			fmt.Fprintf(&buf, "%d", t.Unix())
		case 'S':
			buf.WriteString(t.Format("05"))
		case 't':
			buf.WriteByte('\t')
		case 'T':
			buf.WriteString(t.Format("15:04:05"))
		case 'u':
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			fmt.Fprintf(&buf, "%d", wd)
		case 'U':
			yday := t.YearDay()
			wday := int(t.Weekday())
			week := (yday + 6 - wday) / 7
			fmt.Fprintf(&buf, "%02d", week)
		case 'V':
			_, week := t.ISOWeek()
			fmt.Fprintf(&buf, "%02d", week)
		case 'w':
			fmt.Fprintf(&buf, "%d", int(t.Weekday()))
		case 'W':
			yday := t.YearDay()
			wday := int(t.Weekday())
			if wday == 0 {
				wday = 7
			}
			week := (yday + 6 - (wday - 1)) / 7
			fmt.Fprintf(&buf, "%02d", week)
		case 'x':
			buf.WriteString(t.Format("01/02/06"))
		case 'X':
			buf.WriteString(t.Format("15:04:05"))
		case 'y':
			buf.WriteString(t.Format("06"))
		case 'Y':
			buf.WriteString(t.Format("2006"))
		case 'z':
			buf.WriteString(t.Format("-0700"))
		case 'Z':
			buf.WriteString(t.Format("MST"))
		case '%':
			buf.WriteByte('%')
		default:
			// Pass through unknown directives.
			buf.WriteByte('%')
			buf.WriteByte(format[i])
		}
		i++
	}
	return buf.String()
}

