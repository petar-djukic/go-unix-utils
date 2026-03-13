// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-ts R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R6.1, R6.2, R6.3, R6.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultStrftimeFormat is the strftime format for ts default: "%b %d %H:%M:%S".
// R1.2: The default timestamp format evaluated at the time each line is received.
const defaultStrftimeFormat = "%b %d %H:%M:%S"

// incrementalStrftimeFormat is the default format for -i mode.
// R3.2: Default format in -i mode is "%H:%M:%S".
const incrementalStrftimeFormat = "%H:%M:%S"

func main() {
	// R1.6 (via SIGPIPE): install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Parse flags and positional format argument.
	incremental := false
	sinceStart := false
	monotonic := false
	relativeMode := false
	var format string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-i":
			incremental = true
		case "-s":
			sinceStart = true
		case "-m":
			// R5.1: Use monotonic clock instead of wall clock for timestamp sampling.
			monotonic = true
		case "-r":
			// R6.1: Enable relative-time conversion mode.
			relativeMode = true
		default:
			format = arg
		}
	}

	// Track whether user provided a custom format before applying defaults.
	// R6.3: When -r and a format string are both given, reformat timestamps
	// via strftime rather than producing a relative age string.
	hasCustomFormat := format != ""

	// R3.4: -i and -s are mutually exclusive. When both are given, -i takes
	// precedence, matching the reference binary behavior (Perl checks $opt{i} first).
	if incremental && sinceStart {
		sinceStart = false
	}

	// R2.1, R3.2, R3.3, R4.2, R4.3: Set default format based on mode.
	// R3.3: A custom format argument overrides the -i default; TZ=GMT still applies.
	// R4.2: Default format in -s mode is "%H:%M:%S" with TZ=GMT, same as -i.
	// R4.3: A custom format argument overrides the -s default format.
	// In -r mode, no default format is needed (relative age is the default output).
	if !relativeMode && format == "" {
		if incremental || sinceStart {
			format = incrementalStrftimeFormat
		} else {
			format = defaultStrftimeFormat
		}
	}


	w := bufio.NewWriter(os.Stdout)
	reader := bufio.NewReader(os.Stdin)

	// R3.1: Track start time and previous line time for incremental mode.
	startTime := time.Now()
	prevTime := startTime

	for {
		// R1.5: ReadBytes preserves the delimiter when present; partial lines
		// at EOF are returned without a trailing newline.
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if relativeMode {
				// R6.1-R6.4: Scan line for timestamp patterns and replace.
				if writeErr := processRelativeLine(w, line, format, hasCustomFormat); writeErr != nil {
					fmt.Fprintf(os.Stderr, "ts: write error: %v\n", writeErr)
					os.Exit(1)
				}
			} else {
				// R2.4: Obtain a single time sample per line for atomic second+microsecond.
				now := time.Now()
				var ts string
				if incremental {
					// R3.1: Time elapsed since previous line (first line = since start).
					// R5.2: -m compatible; time.Sub uses monotonic reading when available.
					delta := now.Sub(prevTime)
					prevTime = now
					// R3.2, R3.3: Convert delta to a time value at epoch 0 in UTC so that
					// strftime on the result produces correct elapsed strings (TZ=GMT).
					deltaTime := time.Unix(0, 0).UTC().Add(delta)
					ts = strftime(format, deltaTime)
				} else if sinceStart {
					// R4.1: Time elapsed since ts started, monotonically increasing.
					// R4.3: Custom format argument overrides the -s default format.
					// R5.2: -m compatible; time.Sub uses monotonic reading when available.
					delta := now.Sub(startTime)
					// R4.2: Convert delta to a time value at epoch 0 in UTC (TZ=GMT).
					deltaTime := time.Unix(0, 0).UTC().Add(delta)
					ts = strftime(format, deltaTime)
				} else if monotonic {
					// R5.1: Use monotonic clock for timestamp sampling. Anchor to the
					// wall-clock start time and advance only by monotonic elapsed, so
					// the timestamp never jumps if the wall clock is adjusted (R5.3).
					elapsed := now.Sub(startTime)
					monoTime := startTime.Add(elapsed)
					ts = strftime(format, monoTime)
				} else {
					// R1.2: Evaluate timestamp at the time each line is received.
					ts = strftime(format, now)
				}
				// R1.1: Prefix with timestamp and single space.
				// R1.4: Original newline preserved (included in line from ReadBytes).
				// R1.5: Partial lines passed through without added newline.
				if writeErr := writeTimestampedLine(w, ts, line); writeErr != nil {
					fmt.Fprintf(os.Stderr, "ts: write error: %v\n", writeErr)
					os.Exit(1)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "ts: read error: %v\n", err)
			os.Exit(1)
		}
	}
	// R1.6: Exit 0 on clean EOF.
}

// writeTimestampedLine writes the timestamp, a space, and the line data to w,
// then flushes for real-time downstream consumption (R1.3).
func writeTimestampedLine(w *bufio.Writer, ts string, line []byte) error {
	if _, err := w.WriteString(ts); err != nil {
		return err
	}
	if err := w.WriteByte(' '); err != nil {
		return err
	}
	if _, err := w.Write(line); err != nil {
		return err
	}
	return w.Flush()
}

// timestampRE matches known timestamp patterns in input lines for -r mode (R6.2).
// Translated from the Perl ts source regex. Uses \b word boundaries and matches
// all occurrences per line (via ReplaceAllStringFunc).
var timestampRE = regexp.MustCompile(
	`\b(?:` +
		// Pattern 1: "21 dec 17:05" / "21 dec/93 17:05:30 +0000"
		`\d\d[\s/\-]\w{3}(?:/\d{2,})?[\s:]\d\d:\d\d(?::\d\d)?(?:\s+[+\-]\d{4})?` +
		`|` +
		// Pattern 2: Syslog "Jan  5 14:30:00"
		`\w{3}\s+\d{1,2}\s+\d\d:\d\d:\d\d` +
		`|` +
		// Pattern 3: ISO-8601 "2024-01-05T14:30:00.000Z" (requires fractional seconds)
		`\d{4}[\-:]\d\d[\-:]\d\dT\d\d:\d\d:\d\d.\d+Z?` +
		`|` +
		// Pattern 4: RFC 2822 "16 Jun 94 07:29:35 GMT"
		`(?:\w{3},?\s+)?\d+\s+\w{3}\s+\d{2,}\s+\d\d:\d\d:\d\d(?:\s+\w{3}|\s[+\-]\d{4})?` +
		`|` +
		// Pattern 5: Lastlog "Mon Jan 05 14:30"
		`\w{3}\s+\w{3}\s+\d\d\s+\d\d:\d\d` +
		`)\b`,
)

// timestampLayouts lists Go time.Parse layouts tried in order when parsing a
// matched timestamp substring. Layouts without a year component produce year 0;
// the caller adjusts to the current year.
var timestampLayouts = []string{
	// ISO-8601 variants (with fractional seconds)
	time.RFC3339Nano,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05.000",
	// RFC 2822 variants
	time.RFC1123,
	time.RFC1123Z,
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 06 15:04:05 MST",
	"2 Jan 06 15:04:05 -0700",
	// Generic "DD Mon HH:MM" and "DD Mon HH:MM:SS"
	"02-Jan 15:04:05",
	"02-Jan 15:04",
	// Lastlog: "Mon Jan 05 14:30"
	"Mon Jan 02 15:04",
	// Syslog: "Jan  5 14:30:00"
	"Jan 2 15:04:05",
}

// parseTimestamp attempts to parse a timestamp string matched by timestampRE.
// For formats without a year component (syslog, lastlog), the current year is assumed.
// Returns the parsed time in local time zone for zone-free formats.
//
// R6.2: Recognize syslog, ISO-8601, RFC 2822, and lastlog timestamp patterns.
func parseTimestamp(s string) (time.Time, bool) {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			// Formats without a year produce year 0; fix to current year.
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// processRelativeLine handles a single input line in -r mode. It scans for
// known timestamp patterns and replaces all matches with either a relative age
// string (R6.1) or a strftime-formatted string (R6.3). Lines with no recognizable
// timestamp are passed through unchanged (R6.4).
func processRelativeLine(w *bufio.Writer, line []byte, format string, hasCustomFormat bool) error {
	now := time.Now()
	result := timestampRE.ReplaceAllStringFunc(string(line), func(matched string) string {
		parsed, ok := parseTimestamp(matched)
		if !ok {
			// Regex matched but parsing failed; leave unchanged (R6.4).
			return matched
		}
		if hasCustomFormat {
			// R6.3: Convert matched timestamp to the specified strftime format.
			return strftime(format, parsed)
		}
		// R6.1: Replace with human-readable relative age string.
		return formatRelativeAge(now, parsed)
	})
	if _, err := w.WriteString(result); err != nil {
		return err
	}
	return w.Flush()
}

// secondsPerYear is 365 days in seconds, matching Perl Time::Duration's YEAR constant.
const secondsPerYear = 365 * 86400

// formatRelativeAge computes the age of a parsed timestamp relative to now and
// returns a concise human-readable string matching Time::Duration::concise(ago(..., 2)).
// At most 2 non-zero components are shown. Uses "ago" for past, "from now" for
// future, and "right now" for zero difference.
//
// R6.1: Human-readable relative age string.
func formatRelativeAge(now, parsed time.Time) string {
	diff := int64(now.Sub(parsed) / time.Second)
	if diff == 0 {
		return "right now"
	}

	suffix := " ago"
	if diff < 0 {
		suffix = " from now"
		diff = -diff
	}

	years := diff / secondsPerYear
	diff %= secondsPerYear
	days := diff / 86400
	diff %= 86400
	hours := diff / 3600
	diff %= 3600
	mins := diff / 60
	secs := diff % 60

	// Collect non-zero components, limited to precision of 2.
	type component struct {
		value int64
		unit  string
	}
	components := []component{
		{years, "y"},
		{days, "d"},
		{hours, "h"},
		{mins, "m"},
		{secs, "s"},
	}

	var buf strings.Builder
	shown := 0
	for _, c := range components {
		if c.value > 0 && shown < 2 {
			fmt.Fprintf(&buf, "%d%s", c.value, c.unit)
			shown++
		}
	}
	if shown == 0 {
		buf.WriteString("0s")
	}
	buf.WriteString(suffix)
	return buf.String()
}

// strftime formats a time.Time using a strftime(3) format string.
// R2.2: Supports all standard strftime(3) conversion specifications.
func strftime(format string, t time.Time) string {
	var buf strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' || i+1 >= len(format) {
			buf.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip '%'

		// R2.3: Handle ts-specific subsecond extensions %.S, %.s, %.T before
		// other specifiers. These require the full time sample for atomic
		// second+microsecond formatting (R2.4).
		if format[i] == '.' && i+1 < len(format) {
			switch format[i+1] {
			case 'S':
				// %.S: seconds with microsecond suffix, e.g. "32.001234"
				fmt.Fprintf(&buf, "%02d.%06d", t.Second(), t.Nanosecond()/1000)
				i += 2
				continue
			case 's':
				// %.s: Unix epoch with microsecond suffix, e.g. "1708358732.001234"
				fmt.Fprintf(&buf, "%d.%06d", t.Unix(), t.Nanosecond()/1000)
				i += 2
				continue
			case 'T':
				// %.T: HH:MM:SS with microsecond suffix, e.g. "14:05:32.001234"
				fmt.Fprintf(&buf, "%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
				i += 2
				continue
			default:
				// Not a subsecond extension; fall through to normal handling.
			}
		}

		// Handle %E and %O POSIX alternate-representation modifiers.
		// In C locale these produce the same output as without the modifier.
		if (format[i] == 'E' || format[i] == 'O') && i+1 < len(format) {
			i++
		}

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
			buf.WriteString(strftime("%a %b %e %H:%M:%S %Y", t))
		case 'C':
			fmt.Fprintf(&buf, "%02d", t.Year()/100)
		case 'd':
			fmt.Fprintf(&buf, "%02d", t.Day())
		case 'D':
			buf.WriteString(strftime("%m/%d/%y", t))
		case 'e':
			fmt.Fprintf(&buf, "%2d", t.Day())
		case 'F':
			buf.WriteString(strftime("%Y-%m-%d", t))
		case 'g':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&buf, "%02d", y%100)
		case 'G':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&buf, "%04d", y)
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
			if t.Hour() < 12 {
				buf.WriteString("AM")
			} else {
				buf.WriteString("PM")
			}
		case 'P':
			if t.Hour() < 12 {
				buf.WriteString("am")
			} else {
				buf.WriteString("pm")
			}
		case 'r':
			buf.WriteString(strftime("%I:%M:%S %p", t))
		case 'R':
			buf.WriteString(strftime("%H:%M", t))
		case 's':
			fmt.Fprintf(&buf, "%d", t.Unix())
		case 'S':
			fmt.Fprintf(&buf, "%02d", t.Second())
		case 't':
			buf.WriteByte('\t')
		case 'T':
			buf.WriteString(strftime("%H:%M:%S", t))
		case 'u':
			d := int(t.Weekday())
			if d == 0 {
				d = 7
			}
			fmt.Fprintf(&buf, "%d", d)
		case 'U':
			wday := int(t.Weekday())
			fmt.Fprintf(&buf, "%02d", (t.YearDay()+6-wday)/7)
		case 'V':
			_, w := t.ISOWeek()
			fmt.Fprintf(&buf, "%02d", w)
		case 'w':
			fmt.Fprintf(&buf, "%d", int(t.Weekday()))
		case 'W':
			wdayMon0 := (int(t.Weekday()) + 6) % 7
			fmt.Fprintf(&buf, "%02d", (t.YearDay()+6-wdayMon0)/7)
		case 'x':
			buf.WriteString(strftime("%m/%d/%y", t))
		case 'X':
			buf.WriteString(strftime("%H:%M:%S", t))
		case 'y':
			fmt.Fprintf(&buf, "%02d", t.Year()%100)
		case 'Y':
			fmt.Fprintf(&buf, "%04d", t.Year())
		case 'z':
			buf.WriteString(t.Format("-0700"))
		case 'Z':
			buf.WriteString(t.Format("MST"))
		case '%':
			buf.WriteByte('%')
		default:
			// Unknown specifier: pass through as-is.
			buf.WriteByte('%')
			buf.WriteByte(format[i])
		}
		i++
	}
	return buf.String()
}
