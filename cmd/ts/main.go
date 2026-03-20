// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1–R1.6, R2.1–R2.4, R3.1–R3.4, R4.1–R4.3, R5.1–R5.3,
// R6.1–R6.5: timestamp stdin lines with default and custom strftime format support,
// subsecond extensions, incremental mode, elapsed-since-start mode, monotonic clock
// mode, and relative-time conversion mode.
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

// defaultStrftimeFormat is the strftime format per R1.2: "%b %d %H:%M:%S".
const defaultStrftimeFormat = "%b %d %H:%M:%S"

// defaultIncrementalFormat is the strftime format for -i mode per R3.2.
const defaultIncrementalFormat = "%H:%M:%S"

// defaultElapsedFormat is the strftime format for -s mode per R4.2.
const defaultElapsedFormat = "%H:%M:%S"

// syslogLayout is the time.Parse layout for syslog timestamps (no year).
const syslogLayout = "Jan _2 15:04:05"

// tsConfig holds parsed command-line configuration.
type tsConfig struct {
	format      string
	incremental bool
	elapsed     bool
	monotonic   bool // R5.1: use monotonic clock
	relative    bool // R6.1: relative-time conversion mode
}

func main() {
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ts: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into tsConfig.
// R2.1: optional positional argument for custom format.
// R3.1: -i flag. R4.1: -s flag. R5.1: -m flag. R6.1: -r flag.
func parseArgs(args []string) (tsConfig, error) {
	cfg := tsConfig{}
	for _, arg := range args {
		switch arg {
		case "-i":
			cfg.incremental = true
		case "-s":
			cfg.elapsed = true
		case "-m":
			cfg.monotonic = true
		case "-r":
			cfg.relative = true
		default:
			cfg.format = arg
		}
	}
	if err := validateFlags(cfg); err != nil {
		return tsConfig{}, err
	}
	if cfg.format == "" && !cfg.relative {
		cfg.format = selectDefaultFormat(cfg)
	}
	return cfg, nil
}

// validateFlags checks for mutually exclusive flag combinations.
// R3.4: -i and -s are mutually exclusive.
// R6.5: -r is mutually exclusive with -i and -s.
func validateFlags(cfg tsConfig) error {
	if cfg.incremental && cfg.elapsed {
		return fmt.Errorf("-i and -s are mutually exclusive")
	}
	if cfg.relative && cfg.incremental {
		return fmt.Errorf("-r and -i are mutually exclusive")
	}
	if cfg.relative && cfg.elapsed {
		return fmt.Errorf("-r and -s are mutually exclusive")
	}
	return nil
}

// selectDefaultFormat returns the appropriate default format based on mode.
// R1.2: default "%b %d %H:%M:%S". R3.2: -i default "%H:%M:%S".
// R4.2: -s default "%H:%M:%S".
func selectDefaultFormat(cfg tsConfig) string {
	if cfg.incremental {
		return defaultIncrementalFormat
	}
	if cfg.elapsed {
		return defaultElapsedFormat
	}
	return defaultStrftimeFormat
}

// run parses arguments and processes stdin.
func run(args []string) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}
	if cfg.relative {
		return processRelativeStdin(cfg)
	}
	return processStdin(cfg)
}

// processStdin reads stdin and writes timestamped lines to stdout.
// R1.1: line-by-line. R1.5: partial lines at EOF. R1.6: exit 0 on EOF.
// R3.1: incremental mode tracks time between lines.
// R4.1: elapsed mode tracks time since start.
func processStdin(cfg tsConfig) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	lastTime := time.Now()
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			now := time.Now()
			ts := formatTimestamp(cfg, now, &lastTime)
			if writeErr := writeTimestampedLine(writer, ts, line); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// formatTimestamp produces the timestamp string for a line.
// R2.4: uses a single time sample (now) for both second and subsecond.
// R3.1: in incremental mode, formats delta since previous line.
// R3.2, R3.3: in incremental mode, uses UTC (GMT) for delta formatting.
// R4.1: in elapsed mode, formats delta since start (lastTime is not updated).
// R4.2: in elapsed mode, uses UTC (GMT) for delta formatting.
func formatTimestamp(cfg tsConfig, now time.Time, lastTime *time.Time) string {
	if !cfg.incremental && !cfg.elapsed {
		return strftime(cfg.format, now)
	}
	delta := now.Sub(*lastTime)
	// R3.1: incremental updates lastTime; R4.1: elapsed does not.
	if cfg.incremental {
		*lastTime = now
	}
	// R3.2, R4.2: Convert delta to time at Unix epoch in UTC for strftime.
	deltaTime := time.Unix(0, 0).Add(delta).UTC()
	return strftime(cfg.format, deltaTime)
}

// writeTimestampedLine writes a timestamp-prefixed line and flushes.
// R1.3: flush after each line. R1.4: preserves original newline.
func writeTimestampedLine(w *bufio.Writer, ts string, line []byte) error {
	w.WriteString(ts)
	w.WriteByte(' ')
	w.Write(line)
	return w.Flush()
}

// --- Relative-time conversion mode (R6.1–R6.5) ---

// relativeTimestampRE matches timestamp patterns for -r mode (R6.2).
// Ordered from most specific to least specific via alternation.
var relativeTimestampRE = regexp.MustCompile(
	// ISO-8601: 2024-01-05T14:30:00.000Z or 2024-01-05T14:30:00+05:30
	`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?` +
		// RFC 2822 with day name: Thu, 16 Jun 94 07:29:35 GMT
		`|[A-Z][a-z]{2}, \d{1,2} [A-Z][a-z]{2} \d{2,4} \d{2}:\d{2}:\d{2} [A-Z]+` +
		// RFC 2822 without day name: 16 Jun 94 07:29:35 GMT
		`|\d{1,2} [A-Z][a-z]{2} \d{2,4} \d{2}:\d{2}:\d{2} [A-Z]+` +
		// Lastlog: Mon Jan  5 14:30
		`|[A-Z][a-z]{2} [A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}` +
		// Syslog: Jan  5 14:30:00
		`|[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`,
)

// parseLayouts are time.Parse layouts for -r mode, most specific first.
var parseLayouts = []string{
	time.RFC3339Nano,                    // 2006-01-02T15:04:05.999999999Z07:00
	time.RFC3339,                        // 2006-01-02T15:04:05Z07:00
	"2006-01-02T15:04:05",              // ISO-8601 without timezone
	time.RFC1123,                        // Mon, 02 Jan 2006 15:04:05 MST
	"Mon, 2 Jan 2006 15:04:05 MST",     // RFC 1123 non-padded day
	"Mon, 2 Jan 06 15:04:05 MST",       // 2-digit year
	"Mon, 02 Jan 06 15:04:05 MST",      // 2-digit year, padded day
	"2 Jan 2006 15:04:05 MST",          // no day name, 4-digit year
	"2 Jan 06 15:04:05 MST",            // no day name, 2-digit year
	"02 Jan 2006 15:04:05 MST",         // padded day, 4-digit year
	"02 Jan 06 15:04:05 MST",           // padded day, 2-digit year
	syslogLayout,                        // Jan _2 15:04:05
}

// processRelativeStdin reads stdin and replaces timestamps in each line.
// R6.1: replace timestamps with relative age strings.
// R6.4: lines with no recognizable timestamp pass through unchanged.
func processRelativeStdin(cfg tsConfig) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			modified := replaceTimestampInLine(cfg, line, time.Now())
			if _, writeErr := writer.Write(modified); writeErr != nil {
				return writeErr
			}
			if flushErr := writer.Flush(); flushErr != nil {
				return flushErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// replaceTimestampInLine finds a timestamp in line and replaces it.
// R6.1: replace with relative age. R6.3: replace with reformatted time.
// R6.4: return line unchanged if no timestamp found.
func replaceTimestampInLine(cfg tsConfig, line []byte, now time.Time) []byte {
	loc := relativeTimestampRE.FindIndex(line)
	if loc == nil {
		return line
	}
	matched := string(line[loc[0]:loc[1]])
	parsed, ok := tryParseTimestamp(matched, now)
	if !ok {
		return line
	}
	replacement := buildReplacement(cfg, parsed, now)
	var buf []byte
	buf = append(buf, line[:loc[0]]...)
	buf = append(buf, []byte(replacement)...)
	buf = append(buf, line[loc[1]:]...)
	return buf
}

// buildReplacement formats the parsed timestamp for -r mode output.
// R6.1: relative age string. R6.3: strftime with custom format.
func buildReplacement(cfg tsConfig, parsed, now time.Time) string {
	if cfg.format != "" {
		return strftime(cfg.format, parsed)
	}
	return formatRelativeAge(now.Sub(parsed))
}

// tryParseTimestamp attempts to parse a matched timestamp string.
// Uses time.ParseInLocation with local timezone for formats without
// explicit timezone information, matching Perl Date::Parse behavior.
func tryParseTimestamp(s string, now time.Time) (time.Time, bool) {
	for _, layout := range parseLayouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err != nil {
			continue
		}
		if layout == syslogLayout {
			t = t.AddDate(now.Year(), 0, 0)
		}
		return t, true
	}
	// R6.2: lastlog format — strip weekday prefix to avoid Go's
	// weekday-date consistency validation.
	return tryParseLastlog(s, now)
}

// tryParseLastlog parses "Mon Jan  5 14:30" lastlog format.
// Strips the 3-letter weekday prefix before parsing to avoid
// Go's weekday validation against year 0.
func tryParseLastlog(s string, now time.Time) (time.Time, bool) {
	if len(s) < 4 || s[3] != ' ' {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("Jan _2 15:04", s[4:], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t.AddDate(now.Year(), 0, 0), true
}

// formatRelativeAge converts a duration to a human-readable age string.
// R6.1: format like "5d12h3m2s ago" with only non-zero components.
func formatRelativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	total := int(d.Seconds())
	days := total / 86400
	hours := (total % 86400) / 3600
	mins := (total % 3600) / 60
	secs := total % 60
	var buf strings.Builder
	writeAgeComponent(&buf, days, "d")
	writeAgeComponent(&buf, hours, "h")
	writeAgeComponent(&buf, mins, "m")
	if secs > 0 || buf.Len() == 0 {
		fmt.Fprintf(&buf, "%ds", secs)
	}
	buf.WriteString(" ago")
	return buf.String()
}

// writeAgeComponent appends a non-zero age component to the builder.
func writeAgeComponent(buf *strings.Builder, value int, unit string) {
	if value > 0 {
		fmt.Fprintf(buf, "%d%s", value, unit)
	}
}

// --- strftime formatting ---

// strftime formats a time.Time using a strftime(3) format string.
// R2.2: supports all standard strftime conversion specifications.
// R2.3: supports %.S, %.s, %.T subsecond extensions.
func strftime(format string, t time.Time) string {
	var buf strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}
		if i+1 >= len(format) {
			buf.WriteByte('%')
			break
		}
		// R2.3: check for %.S, %.s, %.T subsecond extensions.
		if format[i+1] == '.' && i+2 < len(format) {
			if s, ok := fmtSubsecondSpec(format[i+2], t); ok {
				buf.WriteString(s)
				i += 3
				continue
			}
		}
		buf.WriteString(fmtSpec(format[i+1], t))
		i += 2
	}
	return buf.String()
}

// fmtSubsecondSpec handles ts-specific subsecond extensions.
// R2.3: %.S, %.s, %.T with microsecond precision (6 decimal places).
// R2.4: uses the same time.Time for both second and microsecond.
func fmtSubsecondSpec(spec byte, t time.Time) (string, bool) {
	usec := t.Nanosecond() / 1000
	switch spec {
	case 'S':
		return fmt.Sprintf("%02d.%06d", t.Second(), usec), true
	case 's':
		return fmt.Sprintf("%d.%06d", t.Unix(), usec), true
	case 'T':
		return fmt.Sprintf("%02d:%02d:%02d.%06d",
			t.Hour(), t.Minute(), t.Second(), usec), true
	default:
		return "", false
	}
}

// fmtSpec dispatches a single strftime specifier to the appropriate formatter.
func fmtSpec(spec byte, t time.Time) string {
	if s, ok := fmtCalendarSpec(spec, t); ok {
		return s
	}
	if s, ok := fmtClockSpec(spec, t); ok {
		return s
	}
	return "%" + string(spec)
}

// fmtCalendarSpec handles date-related strftime specifiers.
func fmtCalendarSpec(spec byte, t time.Time) (string, bool) {
	switch spec {
	case 'Y':
		return fmt.Sprintf("%04d", t.Year()), true
	case 'y':
		return fmt.Sprintf("%02d", t.Year()%100), true
	case 'C':
		return fmt.Sprintf("%02d", t.Year()/100), true
	case 'm':
		return fmt.Sprintf("%02d", int(t.Month())), true
	case 'd':
		return fmt.Sprintf("%02d", t.Day()), true
	case 'e':
		return fmt.Sprintf("%2d", t.Day()), true
	case 'b', 'h':
		return t.Format("Jan"), true
	case 'B':
		return t.Format("January"), true
	case 'a':
		return t.Format("Mon"), true
	case 'A':
		return t.Format("Monday"), true
	case 'j':
		return fmt.Sprintf("%03d", t.YearDay()), true
	case 'u':
		return fmt.Sprintf("%d", isoWeekday(t.Weekday())), true
	case 'w':
		return fmt.Sprintf("%d", int(t.Weekday())), true
	case 'U':
		return fmt.Sprintf("%02d", weekNumSunday(t)), true
	case 'W':
		return fmt.Sprintf("%02d", weekNumMonday(t)), true
	case 'F':
		return strftime("%Y-%m-%d", t), true
	case 'D', 'x':
		return strftime("%m/%d/%y", t), true
	case 'c':
		return strftime("%a %b %e %H:%M:%S %Y", t), true
	default:
		return "", false
	}
}

// fmtClockSpec handles time-related and miscellaneous strftime specifiers.
func fmtClockSpec(spec byte, t time.Time) (string, bool) {
	switch spec {
	case 'H':
		return fmt.Sprintf("%02d", t.Hour()), true
	case 'I':
		return fmt.Sprintf("%02d", hour12(t.Hour())), true
	case 'k':
		return fmt.Sprintf("%2d", t.Hour()), true
	case 'l':
		return fmt.Sprintf("%2d", hour12(t.Hour())), true
	case 'M':
		return fmt.Sprintf("%02d", t.Minute()), true
	case 'S':
		return fmt.Sprintf("%02d", t.Second()), true
	case 'p':
		return t.Format("PM"), true
	case 'P':
		return strings.ToLower(t.Format("PM")), true
	case 'T', 'X':
		return strftime("%H:%M:%S", t), true
	case 'R':
		return strftime("%H:%M", t), true
	case 'r':
		return strftime("%I:%M:%S %p", t), true
	case 's':
		return fmt.Sprintf("%d", t.Unix()), true
	case 'z':
		return t.Format("-0700"), true
	case 'Z':
		return t.Format("MST"), true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	case '%':
		return "%", true
	default:
		return "", false
	}
}

// hour12 converts a 24-hour value to 12-hour (1–12).
func hour12(h int) int {
	h = h % 12
	if h == 0 {
		h = 12
	}
	return h
}

// isoWeekday returns Monday=1 through Sunday=7.
func isoWeekday(d time.Weekday) int {
	if d == time.Sunday {
		return 7
	}
	return int(d)
}

// weekNumSunday returns the week number with Sunday as the first day.
func weekNumSunday(t time.Time) int {
	yday := t.YearDay() - 1
	wday := int(t.Weekday())
	return (yday + 7 - wday) / 7
}

// weekNumMonday returns the week number with Monday as the first day.
func weekNumMonday(t time.Time) int {
	yday := t.YearDay() - 1
	wday := (int(t.Weekday()) + 6) % 7
	return (yday + 7 - wday) / 7
}
