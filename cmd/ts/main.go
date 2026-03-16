// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1-R1.6, R3.1-R3.4, R4.1-R4.3: cmd/ts reads stdin
// line by line and prepends a strftime-formatted timestamp to each line, writing
// to stdout. Supports a default format ("%b %d %H:%M:%S"), an optional positional
// format argument, incremental mode (-i), and elapsed-since-start mode (-s).
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"bufio"
	"fmt"
	"os"
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

	// Parse flags: -i (incremental), -s (elapsed since start).
	var incremental, elapsed bool
	var positionalArgs []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-i":
			incremental = true
		case "-s":
			elapsed = true
		default:
			positionalArgs = append(positionalArgs, arg)
		}
	}

	// R3.4: -i and -s are mutually exclusive.
	if incremental && elapsed {
		fmt.Fprintf(os.Stderr, "ts: -i and -s are mutually exclusive\n")
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

	// R1.1: read stdin line by line and prepend a timestamp to each line.
	scanner := bufio.NewScanner(os.Stdin)
	w := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		now := time.Now()

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
		// R1.4: preserve the original newline; do not add extra.
		if _, err := fmt.Fprintf(w, "%s %s\n", ts, line); err != nil {
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
