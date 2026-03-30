// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ts implements moreutils ts: prepend timestamps to stdin lines.
// Implements prd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3,
// R6.1, R6.2, R7.1-R7.3, R8.1, R8.2, R9.1.
//
// R7.3: The Go implementation compiles timestamp parsing (time.Parse) into the
// binary unconditionally. Unlike the Perl reference (which requires Date::Parse),
// the parsing dependency is always available and this error path cannot be reached.
//
// R8.1: Wall-clock timestamps respect the TZ environment variable via Go's
// time.Now(), which uses time.Local (initialized from TZ at startup).
//
// R8.2: In -i and -s modes, elapsed time is formatted using UTC (TZ=GMT equivalent)
// regardless of the user's TZ setting, matching the Perl source behavior.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the strftime format used when no format argument is given.
// R1.2: "%b %d %H:%M:%S" (e.g., "Mar 30 14:05:32").
const defaultFormat = "%b %d %H:%M:%S"

// defaultElapsedFormat is the default format for -i and -s modes.
// R3.2, R4.2: "%H:%M:%S" with TZ=GMT.
const defaultElapsedFormat = "%H:%M:%S"

// secondsPerYear matches Time::Duration's year definition (365.25 days).
const secondsPerYear = 31557600

// tsMode represents the timestamp source mode.
type tsMode int

const (
	modeDefault     tsMode = iota
	modeIncremental        // -i: elapsed since previous line
	modeElapsed            // -s: elapsed since start
	modeRelative           // -r: convert timestamps to relative age
)

// simpleDirectives maps strftime single-character directives to Go time layouts.
var simpleDirectives = map[byte]string{
	'a': "Mon",
	'A': "Monday",
	'b': "Jan",
	'B': "January",
	'c': "Mon Jan _2 15:04:05 2006",
	'd': "02",
	'D': "01/02/06",
	'e': "_2",
	'F': "2006-01-02",
	'h': "Jan",
	'H': "15",
	'I': "03",
	'm': "01",
	'M': "04",
	'p': "PM",
	'P': "pm",
	'r': "03:04:05 PM",
	'R': "15:04",
	'S': "05",
	'T': "15:04:05",
	'x': "01/02/06",
	'X': "15:04:05",
	'y': "06",
	'Y': "2006",
	'z': "-0700",
	'Z': "MST",
}

// tsConfig holds parsed command-line configuration.
type tsConfig struct {
	format    string
	mode      tsMode
	monotonic bool // R5.1: -m flag for monotonic clock
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg := parseArgs(os.Args[1:])

	switch cfg.mode {
	case modeIncremental:
		runIncremental(cfg.format)
	case modeElapsed:
		runElapsed(cfg.format)
	case modeRelative:
		// R6.1: scan lines for timestamps, replace with relative age.
		runRelative()
	default:
		if cfg.monotonic {
			// R5.1: use CLOCK_MONOTONIC for timestamp sampling.
			processLines(cfg.format, monotonicNow)
		} else {
			runDefault(cfg.format)
		}
	}
}

// parseArgs parses ts flags and an optional positional format string.
// R3.4: -i and -s; last flag wins (matches reference binary behavior).
// R5.2: -m is compatible with all modes.
// R6.1: -r enables relative-time conversion mode.
// R7.2: unrecognized flags print usage and exit non-zero.
func parseArgs(args []string) tsConfig {
	cfg := tsConfig{format: defaultFormat}
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "-i":
			cfg.mode = modeIncremental
		case "-s":
			cfg.mode = modeElapsed
		case "-m":
			// R5.1: monotonic clock mode.
			cfg.monotonic = true
		case "-r":
			// R6.1: relative-time conversion mode.
			cfg.mode = modeRelative
		default:
			if strings.HasPrefix(arg, "-") && len(arg) > 1 {
				printUsageAndExit()
			}
			remaining = append(remaining, arg)
		}
	}
	if len(remaining) > 0 {
		// R3.3, R4.3: custom format overrides mode default.
		cfg.format = remaining[0]
	} else if cfg.mode == modeIncremental || cfg.mode == modeElapsed {
		// R3.2, R4.2: default format for -i/-s is "%H:%M:%S".
		cfg.format = defaultElapsedFormat
	}
	return cfg
}

// printUsageAndExit prints a usage error to stderr and exits non-zero.
// R7.2: must exit non-zero with usage message for unrecognized flags.
func printUsageAndExit() {
	fmt.Fprintf(os.Stderr, "usage: ts [-i] [-s] [-m] [-r] [format]\n")
	os.Exit(1)
}

// monotonicNow returns the current CLOCK_MONOTONIC value as a time.Time
// in the local timezone.
// R5.1: monotonic clock instead of wall clock for timestamp sampling.
// R5.3: CLOCK_MONOTONIC does not jump on NTP or wall-clock adjustments.
func monotonicNow() time.Time {
	var ts unix.Timespec
	// CLOCK_MONOTONIC is guaranteed on Darwin and Linux; error ignored.
	_ = unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	return time.Unix(ts.Sec, ts.Nsec).In(time.Local)
}

// runDefault reads stdin and prepends wall-clock timestamps.
func runDefault(format string) {
	processLines(format, func() time.Time { return time.Now() })
}

// runIncremental reads stdin and prepends elapsed-since-previous-line
// timestamps formatted in UTC.
// R3.1: elapsed since previous line; first line since start.
// R3.2: TZ=GMT via UTC epoch offset formatting.
// R3.3: custom format still uses TZ=GMT behavior.
// R5.2: -m compatible; Go's time.Sub already uses monotonic readings.
func runIncremental(format string) {
	lastTime := time.Now()
	processLines(format, func() time.Time {
		now := time.Now()
		elapsed := now.Sub(lastTime)
		lastTime = now
		return time.Unix(0, 0).UTC().Add(elapsed)
	})
}

// runElapsed reads stdin and prepends elapsed-since-start timestamps
// formatted in UTC.
// R4.1: elapsed since ts started, monotonically increasing.
// R4.2: default format "%H:%M:%S" with TZ=GMT.
// R4.3: custom format overrides the -s default format.
// R5.2: -m compatible; Go's time.Since already uses monotonic readings.
func runElapsed(format string) {
	startTime := time.Now()
	processLines(format, func() time.Time {
		elapsed := time.Since(startTime)
		return time.Unix(0, 0).UTC().Add(elapsed)
	})
}

// processLines reads stdin line by line and writes each line to stdout
// prefixed by a timestamp obtained from timeFn.
// R1.1: reads stdin line by line.
// R1.3: flushes stdout after each line.
// R1.4: preserves original newline.
// R1.5: passes through partial lines at EOF.
func processLines(format string, timeFn func() time.Time) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			t := timeFn()
			ts := formatTime(t, format)
			fmt.Fprintf(writer, "%s %s", ts, line)
			writer.Flush()
		}
		if err != nil {
			break
		}
	}
}

// formatTime converts a time value to a string using a strftime format.
// R1.2: evaluates format at the time each line is received.
// R2.1: supports custom strftime format strings.
// R2.2: supports all standard strftime(3) conversion specifications.
// R2.3: supports ts-specific subsecond extensions %.S, %.s, %.T.
// R2.4: single time sample ensures atomic second+microsecond.
func formatTime(t time.Time, format string) string {
	var buf strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			continue
		}
		i++
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}
		if format[i] == '.' && i+1 < len(format) {
			i++
			writeSubsecond(&buf, t, format[i])
			continue
		}
		writeDirective(&buf, t, format[i])
	}
	return buf.String()
}

// writeSubsecond handles ts-specific subsecond extensions.
// R2.3: %.S (seconds.usec), %.s (epoch.usec), %.T (HH:MM:SS.usec).
// R2.4: uses a single time sample for second and microsecond components.
func writeSubsecond(buf *strings.Builder, t time.Time, spec byte) {
	usec := t.Nanosecond() / 1000
	switch spec {
	case 'S':
		fmt.Fprintf(buf, "%s.%06d", t.Format("05"), usec)
	case 's':
		fmt.Fprintf(buf, "%d.%06d", t.Unix(), usec)
	case 'T':
		fmt.Fprintf(buf, "%s.%06d", t.Format("15:04:05"), usec)
	default:
		fmt.Fprintf(buf, "%%.%c", spec)
	}
}

// writeDirective writes a single strftime directive to buf.
func writeDirective(buf *strings.Builder, t time.Time, spec byte) {
	if layout, ok := simpleDirectives[spec]; ok {
		buf.WriteString(t.Format(layout))
		return
	}
	writeComputedDirective(buf, t, spec)
}

// writeComputedDirective handles strftime directives that require computation
// beyond a simple Go time layout string.
func writeComputedDirective(buf *strings.Builder, t time.Time, spec byte) {
	switch spec {
	case 'C':
		fmt.Fprintf(buf, "%02d", t.Year()/100)
	case 'G':
		year, _ := t.ISOWeek()
		fmt.Fprintf(buf, "%04d", year)
	case 'g':
		year, _ := t.ISOWeek()
		fmt.Fprintf(buf, "%02d", year%100)
	case 'j':
		fmt.Fprintf(buf, "%03d", t.YearDay())
	case 'k':
		fmt.Fprintf(buf, "%2d", t.Hour())
	case 'l':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		fmt.Fprintf(buf, "%2d", h)
	case 'n':
		buf.WriteByte('\n')
	case 'N':
		fmt.Fprintf(buf, "%09d", t.Nanosecond())
	case 's':
		fmt.Fprintf(buf, "%d", t.Unix())
	case 't':
		buf.WriteByte('\t')
	case 'u':
		d := int(t.Weekday())
		if d == 0 {
			d = 7
		}
		fmt.Fprintf(buf, "%d", d)
	case 'U':
		// R2.2: week number (Sunday as first day, 00-53).
		fmt.Fprintf(buf, "%02d", weekNumberSunday(t))
	case 'V':
		_, week := t.ISOWeek()
		fmt.Fprintf(buf, "%02d", week)
	case 'w':
		fmt.Fprintf(buf, "%d", int(t.Weekday()))
	case 'W':
		// R2.2: week number (Monday as first day, 00-53).
		fmt.Fprintf(buf, "%02d", weekNumberMonday(t))
	case '%':
		buf.WriteByte('%')
	default:
		buf.WriteByte('%')
		buf.WriteByte(spec)
	}
}

// weekNumberSunday returns the week number of the year with Sunday as
// the first day of the week (strftime %U). Days before the first Sunday
// are in week 00.
func weekNumberSunday(t time.Time) int {
	return (t.YearDay() + 6 - int(t.Weekday())) / 7
}

// weekNumberMonday returns the week number of the year with Monday as
// the first day of the week (strftime %W). Days before the first Monday
// are in week 00.
func weekNumberMonday(t time.Time) int {
	return (t.YearDay() + 6 - (int(t.Weekday())+6)%7) / 7
}

// --- Relative-time mode (-r) ---

// tsPattern defines a recognizable timestamp format for -r mode.
// R6.2: patterns for syslog, ISO-8601, RFC 2822, lastlog formats.
type tsPattern struct {
	re    *regexp.Regexp
	parse func(string) (time.Time, bool)
}

// monthRE matches abbreviated English month names for timestamp patterns.
const monthRE = `(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)`

// dayRE matches abbreviated English day names for timestamp patterns.
const dayRE = `(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)`

// buildTimestampPatterns returns the ordered list of timestamp patterns
// for -r mode. Order matters: more specific patterns first.
// R6.2: syslog, ISO-8601, RFC 2822, lastlog.
func buildTimestampPatterns() []tsPattern {
	return []tsPattern{
		{
			re:    regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2})?(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`),
			parse: parseISO8601,
		},
		{
			re:    regexp.MustCompile(`(?:\w{3},?\s+)?\d{1,2}\s+` + monthRE + `\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}\s+\w+`),
			parse: parseRFC2822,
		},
		{
			re:    regexp.MustCompile(monthRE + `\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
			parse: parseSyslog,
		},
		{
			re:    regexp.MustCompile(dayRE + `\s+` + monthRE + `\s+\d{1,2}\s+\d{2}:\d{2}`),
			parse: parseLastlog,
		},
	}
}

// runRelative reads stdin and replaces recognized timestamps with
// relative age strings.
// R6.1: replace matched timestamps with human-readable relative age.
func runRelative() {
	patterns := buildTimestampPatterns()
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			result := replaceTimestamp(line, patterns, time.Now())
			fmt.Fprint(writer, result)
			writer.Flush()
		}
		if err != nil {
			break
		}
	}
}

// replaceTimestamp scans line for the first matching timestamp pattern
// and replaces it with a relative age string.
// R6.1: replaces matched timestamp with human-readable relative age.
func replaceTimestamp(line string, patterns []tsPattern, now time.Time) string {
	for _, p := range patterns {
		loc := p.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		t, ok := p.parse(line[loc[0]:loc[1]])
		if !ok {
			continue
		}
		age := now.Sub(t)
		return line[:loc[0]] + formatRelativeAge(age) + line[loc[1]:]
	}
	return line
}

// isoLayouts lists Go time layouts for ISO-8601 parsing.
var isoLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// rfc2822Layouts lists Go time layouts for RFC 2822 parsing.
var rfc2822Layouts = []string{
	time.RFC1123,
	"Mon, 02 Jan 06 15:04:05 MST",
	"02 Jan 2006 15:04:05 MST",
	"02 Jan 06 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 06 15:04:05 MST",
	"2 Jan 2006 15:04:05 MST",
	"2 Jan 06 15:04:05 MST",
}

// syslogLayouts lists Go time layouts for syslog format parsing.
var syslogLayouts = []string{"Jan _2 15:04:05", "Jan 02 15:04:05"}

// lastlogLayouts lists Go time layouts for lastlog format parsing.
var lastlogLayouts = []string{"Mon Jan _2 15:04", "Mon Jan 02 15:04"}

// tryParseLayouts attempts to parse s using each layout in order.
func tryParseLayouts(s string, layouts []string) (time.Time, bool) {
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseISO8601 parses an ISO-8601 timestamp string.
func parseISO8601(s string) (time.Time, bool) {
	return tryParseLayouts(s, isoLayouts)
}

// parseRFC2822 parses an RFC 2822 timestamp string.
func parseRFC2822(s string) (time.Time, bool) {
	return tryParseLayouts(s, rfc2822Layouts)
}

// parseSyslog parses a syslog-format timestamp and assigns the current year.
func parseSyslog(s string) (time.Time, bool) {
	t, ok := tryParseLayouts(s, syslogLayouts)
	if !ok {
		return time.Time{}, false
	}
	return fixYear(t), true
}

// parseLastlog parses a lastlog-format timestamp and assigns the current year.
func parseLastlog(s string) (time.Time, bool) {
	t, ok := tryParseLayouts(s, lastlogLayouts)
	if !ok {
		return time.Time{}, false
	}
	return fixYear(t), true
}

// fixYear assigns the current year to a timestamp parsed without a year.
// If the resulting date is in the future, uses the previous year.
func fixYear(t time.Time) time.Time {
	now := time.Now()
	fixed := time.Date(now.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	if fixed.After(now) {
		fixed = fixed.AddDate(-1, 0, 0)
	}
	return fixed
}

// formatRelativeAge formats a duration as a concise relative age string
// matching Time::Duration::concise(ago()) output (e.g., "5m30s ago").
// R6.1: human-readable relative age string.
func formatRelativeAge(d time.Duration) string {
	total := int64(d.Seconds())
	if total == 0 {
		return "right now"
	}
	suffix := " ago"
	if total < 0 {
		total = -total
		suffix = " from now"
	}
	return buildAgeUnits(total) + suffix
}

// buildAgeUnits formats an absolute number of seconds as up to two
// concise duration units (e.g., "5m30s", "2d3h").
func buildAgeUnits(total int64) string {
	years := total / secondsPerYear
	total %= secondsPerYear
	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	mins := total / 60
	secs := total % 60
	units := []struct {
		v int64
		s string
	}{
		{years, "y"}, {days, "d"}, {hours, "h"}, {mins, "m"}, {secs, "s"},
	}
	var buf strings.Builder
	n := 0
	for _, u := range units {
		if u.v > 0 && n < 2 {
			fmt.Fprintf(&buf, "%d%s", u.v, u.s)
			n++
		}
	}
	if buf.Len() == 0 {
		return "0s"
	}
	return buf.String()
}
