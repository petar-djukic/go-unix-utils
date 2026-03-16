// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3, R6.1-R6.5, R7.1-R7.3:
// cmd/ts reads stdin line by line and prepends a strftime-formatted timestamp to
// each line, writing to stdout. Supports a default format ("%b %d %H:%M:%S"), an
// optional positional format argument with ts-specific subsecond extensions
// (%.S, %.s, %.T), incremental mode (-i), elapsed-since-start mode (-s), and
// relative-time conversion mode (-r). R6.1-R6.4: -r scans input lines for known
// timestamp patterns and replaces them with human-readable relative age strings or
// reformats them via a custom strftime format.
// R6.5: -r is mutually exclusive with -i and -s.
// R7.1: exits 0 on clean EOF from stdin.
// R7.2: exits non-zero with usage message on unrecognized flags.
// R7.3: the Go implementation compiles the timestamp parsing library in statically;
// the -r dependency-unavailable condition cannot arise at runtime.
// R5.1-R5.3: uses bufio.Writer with explicit Flush() after each output line for
// line-buffered output across all modes (default, custom, incremental, elapsed).
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the strftime format used when no format argument is provided.
// R1.2: "%b %d %H:%M:%S" (e.g., "Feb 19 14:05:32").
const defaultFormat = "%b %d %H:%M:%S"

// elapsedDefaultFormat is the strftime format used in -i and -s modes when no
// custom format argument is provided. R3.2, R4.2: "%H:%M:%S".
const elapsedDefaultFormat = "%H:%M:%S"

func main() {
	// R1.4: install SIGPIPE handler for clean exit on broken pipe.
	sys.InstallSIGPIPEHandler()

	// Parse flags: -i (incremental), -s (elapsed since start), -r (relative),
	// -m (monotonic clock).
	var incremental, elapsed, relative, monotonic bool
	var positionalArgs []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-i":
			incremental = true
		case "-s":
			elapsed = true
		case "-r":
			relative = true
		case "-m":
			monotonic = true
		default:
			// R7.2: reject unrecognized flags (anything starting with "-"
			// that is not a known flag).
			if len(arg) > 0 && arg[0] == '-' {
				fmt.Fprintf(os.Stderr, "ts: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			positionalArgs = append(positionalArgs, arg)
		}
	}
	// Suppress unused variable warning for -m; it is parsed and accepted but
	// the monotonic behavior is inherent in Go's time.Now() (R5.1).
	_ = monotonic

	// R3.4: -i and -s are mutually exclusive.
	if incremental && elapsed {
		fmt.Fprintf(os.Stderr, "ts: -i and -s are mutually exclusive\n")
		os.Exit(1)
	}

	// R6.5: -r is mutually exclusive with -i and -s.
	if relative && (incremental || elapsed) {
		fmt.Fprintf(os.Stderr, "ts: -r is mutually exclusive with -i and -s\n")
		os.Exit(1)
	}

	// Determine format: -i/-s default to "%H:%M:%S" (R3.2, R4.2),
	// otherwise "%b %d %H:%M:%S" (R1.2). Custom format overrides (R3.3, R4.3).
	format := defaultFormat
	if incremental || elapsed {
		format = elapsedDefaultFormat
	}
	if len(positionalArgs) > 0 {
		format = positionalArgs[0]
	}

	// R3.1, R4.1: record start time for elapsed calculations.
	startTime := time.Now()
	lastTime := startTime

	// R6.3: when -r and a format string are both given, use the format for
	// reformatting rather than producing relative age strings.
	hasCustomFormat := len(positionalArgs) > 0

	// R1.1: read stdin line by line and prepend a timestamp to each line.
	scanner := bufio.NewScanner(os.Stdin)
	w := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		now := time.Now()

		var output string
		if relative {
			// R6.1-R6.4: scan line for timestamp patterns and replace.
			output = processRelativeLine(line, now, format, hasCustomFormat)
		} else {
			var ts string
			if incremental {
				// R3.1: time elapsed since the previous line was received.
				delta := now.Sub(lastTime)
				lastTime = now
				ts = strftime(format, deltaToTime(delta))
			} else if elapsed {
				// R4.1: time elapsed since ts started.
				delta := now.Sub(startTime)
				ts = strftime(format, deltaToTime(delta))
			} else {
				// R1.2: evaluate timestamp at the time each line is received.
				ts = strftime(format, now)
			}
			// R1.1: prefix with timestamp and a single space.
			output = ts + " " + line
		}

		// R1.4: preserve the original newline; do not add extra.
		if _, err := fmt.Fprintf(w, "%s\n", output); err != nil {
			break
		}
		// R1.3: flush stdout after each line for real-time output.
		if err := w.Flush(); err != nil {
			break
		}
	}

	// Best-effort final flush.
	w.Flush() //nolint:errcheck // best-effort flush

	// R1.6: exit 0 after stdin is closed with EOF.
	os.Exit(0)
}

// deltaToTime converts a duration to a time.Time at the Unix epoch plus the
// duration, in UTC. R3.2, R4.2: TZ=GMT so strftime on a delta-second value
// produces a correct elapsed string (e.g., 5 seconds → "00:00:05").
func deltaToTime(d time.Duration) time.Time {
	return time.Unix(0, 0).UTC().Add(d)
}

// R6.2: timestamp patterns recognized by -r mode. Each entry has a regex and
// a function to parse the matched string into a time.Time.
var tsPatterns = []struct {
	re    *regexp.Regexp
	parse func(match string, now time.Time) (time.Time, bool)
}{
	// ISO-8601: "2024-01-05T14:30:00.000Z" or "2024-01-05T14:30:00" or with offset.
	{
		re: regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`),
		parse: func(match string, now time.Time) (time.Time, bool) {
			for _, layout := range []string{
				"2006-01-02T15:04:05.000Z",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05.000-07:00",
				"2006-01-02T15:04:05-07:00",
				"2006-01-02T15:04:05.000",
				"2006-01-02T15:04:05",
			} {
				if t, err := time.Parse(layout, match); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	// RFC 2822 with optional day: "16 Jun 94 07:29:35 GMT" or "Thu, 16 Jun 94 07:29:35 GMT".
	{
		re: regexp.MustCompile(`(?:[A-Z][a-z]{2},?\s+)?\d{1,2}\s+[A-Z][a-z]{2}\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}\s+[A-Z]{2,4}`),
		parse: func(match string, now time.Time) (time.Time, bool) {
			for _, layout := range []string{
				"Mon, 02 Jan 2006 15:04:05 MST",
				"Mon, 2 Jan 2006 15:04:05 MST",
				"02 Jan 2006 15:04:05 MST",
				"2 Jan 2006 15:04:05 MST",
				"Mon, 02 Jan 06 15:04:05 MST",
				"Mon, 2 Jan 06 15:04:05 MST",
				"02 Jan 06 15:04:05 MST",
				"2 Jan 06 15:04:05 MST",
			} {
				if t, err := time.Parse(layout, match); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	// Syslog format: "Jan  5 14:30:00" (month day time, day may be space-padded).
	{
		re: regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
		parse: func(match string, now time.Time) (time.Time, bool) {
			for _, layout := range []string{
				"Jan  2 15:04:05",
				"Jan 2 15:04:05",
			} {
				if t, err := time.Parse(layout, match); err == nil {
					// Syslog format has no year; assume current year.
					t = t.AddDate(now.Year(), 0, 0)
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	// Lastlog format: "Mon Jan  5 14:30" (weekday month day time without seconds).
	{
		re: regexp.MustCompile(`[A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}`),
		parse: func(match string, now time.Time) (time.Time, bool) {
			for _, layout := range []string{
				"Mon Jan  2 15:04",
				"Mon Jan 2 15:04",
			} {
				if t, err := time.Parse(layout, match); err == nil {
					// Lastlog format has no year; assume current year.
					t = t.AddDate(now.Year(), 0, 0)
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
}

// processRelativeLine scans a line for known timestamp patterns (R6.1-R6.2)
// and replaces the first match with a relative age string or a reformatted
// timestamp (R6.3). Lines with no match pass through unchanged (R6.4).
func processRelativeLine(line string, now time.Time, format string, hasCustomFormat bool) string {
	for _, pat := range tsPatterns {
		loc := pat.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		match := line[loc[0]:loc[1]]
		parsed, ok := pat.parse(match, now)
		if !ok {
			continue
		}

		var replacement string
		if hasCustomFormat {
			// R6.3: reformat to the specified strftime format.
			replacement = strftime(format, parsed)
		} else {
			// R6.1: produce human-readable relative age string.
			replacement = relativeAge(now.Sub(parsed))
		}
		return line[:loc[0]] + replacement + line[loc[1]:]
	}
	// R6.4: no recognizable timestamp — pass through unchanged.
	return line
}

// relativeAge formats a duration as a human-readable relative age string
// matching the Perl ts -r output format (e.g., "15m5s ago", "2d3h ago",
// "right now" for zero).
func relativeAge(d time.Duration) string {
	totalSec := int(d.Seconds())
	if totalSec == 0 && d >= 0 {
		return "right now"
	}
	if totalSec == 0 && d < 0 {
		return "right now"
	}
	if d < 0 {
		return formatDuration(-d) + " from now"
	}
	return formatDuration(d) + " ago"
}

// formatDuration formats a non-negative duration as a compact string using
// all non-zero units: years, days, hours, minutes, seconds. Matches the
// Perl ts -r output (e.g., "31y281d", "69d23h", "1m", "5s").
func formatDuration(d time.Duration) string {
	totalSec := int(d.Seconds())
	if totalSec == 0 {
		return "0s"
	}

	const secsPerYear = 365 * 86400

	years := totalSec / secsPerYear
	totalSec %= secsPerYear
	days := totalSec / 86400
	totalSec %= 86400
	hours := totalSec / 3600
	totalSec %= 3600
	minutes := totalSec / 60
	seconds := totalSec % 60

	var b strings.Builder
	if years > 0 {
		fmt.Fprintf(&b, "%dy", years)
	}
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	if seconds > 0 {
		fmt.Fprintf(&b, "%ds", seconds)
	}
	return b.String()
}

// strftime converts a strftime format string to a formatted time string using
// the given time. Supports standard strftime(3) conversion specifications.
func strftime(format string, t time.Time) string {
	var b strings.Builder
	b.Grow(len(format) * 2)

	i := 0
	for i < len(format) {
		if format[i] != '%' {
			b.WriteByte(format[i])
			i++
			continue
		}

		if i+1 >= len(format) {
			b.WriteByte('%')
			i++
			continue
		}

		i++ // skip '%'
		spec := format[i]
		i++

		// R2.3: ts-specific subsecond extensions: %.S, %.s, %.T.
		if spec == '.' && i < len(format) {
			subSpec := format[i]
			i++
			switch subSpec {
			case 'S': // seconds with microsecond suffix (e.g., "32.001234")
				fmt.Fprintf(&b, "%02d.%06d", t.Second(), t.Nanosecond()/1000)
			case 's': // Unix epoch with microsecond suffix (e.g., "1708358732.001234")
				fmt.Fprintf(&b, "%d.%06d", t.Unix(), t.Nanosecond()/1000)
			case 'T': // HH:MM:SS with microsecond suffix (e.g., "14:05:32.001234")
				fmt.Fprintf(&b, "%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
			default:
				// Unknown %.X — pass through as-is.
				b.WriteByte('%')
				b.WriteByte('.')
				b.WriteByte(subSpec)
			}
			continue
		}

		switch spec {
		case 'Y': // 4-digit year
			fmt.Fprintf(&b, "%04d", t.Year())
		case 'y': // 2-digit year
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'm': // 2-digit month
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'd': // 2-digit day of month
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'e': // day of month, space-padded
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'H': // 2-digit hour (24h)
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I': // 2-digit hour (12h)
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d", h)
		case 'k': // hour (24h), space-padded
			fmt.Fprintf(&b, "%2d", t.Hour())
		case 'l': // hour (12h), space-padded
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%2d", h)
		case 'M': // 2-digit minute
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'S': // 2-digit second
			fmt.Fprintf(&b, "%02d", t.Second())
		case 's': // Unix epoch seconds
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'b', 'h': // abbreviated month name
			b.WriteString(t.Format("Jan"))
		case 'B': // full month name
			b.WriteString(t.Format("January"))
		case 'a': // abbreviated weekday
			b.WriteString(t.Format("Mon"))
		case 'A': // full weekday
			b.WriteString(t.Format("Monday"))
		case 'p': // AM/PM
			b.WriteString(t.Format("PM"))
		case 'P': // am/pm (lowercase)
			b.WriteString(strings.ToLower(t.Format("PM")))
		case 'Z': // timezone name
			b.WriteString(t.Format("MST"))
		case 'z': // timezone offset +HHMM
			b.WriteString(t.Format("-0700"))
		case 'j': // day of year (001-366)
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'w': // weekday number (0=Sun)
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'u': // weekday number (1=Mon, 7=Sun)
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			fmt.Fprintf(&b, "%d", wd)
		case 'U': // week number (Sun start)
			yday := t.YearDay()
			wday := int(t.Weekday())
			fmt.Fprintf(&b, "%02d", (yday+6-wday)/7)
		case 'W': // week number (Mon start)
			yday := t.YearDay()
			wday := int(t.Weekday())
			if wday == 0 {
				wday = 7
			}
			fmt.Fprintf(&b, "%02d", (yday+6-(wday-1))/7)
		case 'T': // equivalent to %H:%M:%S
			fmt.Fprintf(&b, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'D': // equivalent to %m/%d/%y
			fmt.Fprintf(&b, "%02d/%02d/%02d", int(t.Month()), t.Day(), t.Year()%100)
		case 'F': // equivalent to %Y-%m-%d
			fmt.Fprintf(&b, "%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
		case 'R': // equivalent to %H:%M
			fmt.Fprintf(&b, "%02d:%02d", t.Hour(), t.Minute())
		case 'r': // 12-hour time with AM/PM
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d:%02d:%02d %s", h, t.Minute(), t.Second(), t.Format("PM"))
		case 'c': // preferred date and time representation
			b.WriteString(t.Format("Mon Jan  2 15:04:05 2006"))
		case 'x': // preferred date representation
			fmt.Fprintf(&b, "%02d/%02d/%02d", int(t.Month()), t.Day(), t.Year()%100)
		case 'X': // preferred time representation
			fmt.Fprintf(&b, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'n': // newline
			b.WriteByte('\n')
		case 't': // tab
			b.WriteByte('\t')
		case '%': // literal %
			b.WriteByte('%')
		default:
			// Unknown specifier: pass through as-is.
			b.WriteByte('%')
			b.WriteByte(spec)
		}
	}

	return b.String()
}
