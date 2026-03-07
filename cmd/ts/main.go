// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ts command that reads stdin line by line and
// prepends a timestamp to each line on stdout, or converts existing timestamps
// to relative age strings.
// Implements prd004-ts R1-R8.
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

// Default strftime format strings matching moreutils ts behavior.
const (
	defaultAbsoluteFormat = "%b %d %H:%M:%S"
	defaultElapsedFormat  = "%H:%M:%S"
)

// Placeholders for strftime specifiers that require runtime value substitution.
// Null-byte delimiters ensure they cannot collide with Go time.Format reference
// patterns or user-provided text.
const (
	placeholderEpoch = "\x00EPOCH\x00"
	placeholderNano  = "\x00NANO\x00"
	placeholderDotS  = "\x00DS\x00"
	placeholderDotSm = "\x00Ds\x00"
	placeholderDotT  = "\x00DT\x00"
)

// relativePattern holds a compiled regex and Go time layouts for parsing
// timestamps in -r mode (R6.2).
type relativePattern struct {
	re      *regexp.Regexp
	layouts []string
	hasYear bool
}

// relativePatterns defines the timestamp formats recognized in -r mode,
// ordered by specificity (R6.2).
var relativePatterns = []relativePattern{
	// ISO-8601: "2024-01-05T14:30:00" with optional subseconds and Z/offset.
	{
		re:      regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`),
		layouts: []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"},
		hasYear: true,
	},
	// RFC 2822 with optional day name: "Thu, 16 Jun 94 07:29:35 GMT" or
	// "16 Jun 94 07:29:35 GMT".
	{
		re:      regexp.MustCompile(`([A-Z][a-z]{2},\s+)?\d{1,2}\s[A-Z][a-z]{2}\s\d{2,4}\s\d{2}:\d{2}:\d{2}\s[A-Z]+`),
		layouts: []string{"Mon, 02 Jan 2006 15:04:05 MST", "02 Jan 2006 15:04:05 MST", "Mon, 02 Jan 06 15:04:05 MST", "02 Jan 06 15:04:05 MST"},
		hasYear: true,
	},
	// Lastlog: "Mon Jan  5 14:30" (checked before syslog to avoid partial match).
	{
		re:      regexp.MustCompile(`[A-Z][a-z]{2}\s[A-Z][a-z]{2}\s{1,2}\d{1,2}\s\d{2}:\d{2}`),
		layouts: []string{"Mon Jan  2 15:04", "Mon Jan 2 15:04"},
		hasYear: false,
	},
	// Syslog: "Jan  5 14:30:00" or "Jan 05 14:30:00".
	{
		re:      regexp.MustCompile(`[A-Z][a-z]{2}\s{1,2}\d{1,2}\s\d{2}:\d{2}:\d{2}`),
		layouts: []string{"Jan  2 15:04:05", "Jan 2 15:04:05"},
		hasYear: false,
	},
}

func main() {
	// R1.6: exit 0 on SIGPIPE per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	flagI := flag.Bool("i", false, "incremental per-line elapsed timestamps")
	flagS := flag.Bool("s", false, "elapsed since start timestamps")
	flagM := flag.Bool("m", false, "use monotonic clock")
	flagR := flag.Bool("r", false, "convert existing timestamps to relative age")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ts [-i | -s] [-m] [-r] [format]\n")
	}
	flag.Parse()

	// R3.4: -i and -s are mutually exclusive.
	if *flagI && *flagS {
		fmt.Fprintf(os.Stderr, "ts: usage error: -i and -s are mutually exclusive\n")
		flag.Usage()
		os.Exit(1)
	}

	// R6.5: -r is mutually exclusive with -i and -s.
	if *flagR && (*flagI || *flagS) {
		fmt.Fprintf(os.Stderr, "ts: usage error: -r is mutually exclusive with -i and -s\n")
		flag.Usage()
		os.Exit(1)
	}

	// R5.1: -m is accepted for compatibility. Go's time.Now() includes a
	// monotonic reading used automatically by Sub() for elapsed calculations.
	_ = *flagM

	// R3.2, R4.2: default format for elapsed modes is HH:MM:SS.
	format := defaultAbsoluteFormat
	if *flagI || *flagS {
		format = defaultElapsedFormat
	}
	// R2.1: optional positional argument overrides the default format.
	hasCustomFormat := flag.NArg() > 0
	if hasCustomFormat {
		format = flag.Arg(0)
	}

	// R6: relative-time conversion mode.
	if *flagR {
		runRelativeMode(format, hasCustomFormat)
		return
	}

	goLayout := strftimeToGo(format)

	startTime := time.Now()
	lastTime := startTime

	reader := bufio.NewReader(os.Stdin)
	// R1.3: flush stdout after each line so downstream consumers receive
	// timestamps in real time.
	writer := bufio.NewWriter(os.Stdout)
	for {
		// R1.1: read stdin line by line (newline-delimited).
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			// R1.2, R2.4: obtain time at the moment each line is received.
			now := time.Now()
			var ts string
			if *flagI {
				// R3.1: elapsed since previous line.
				elapsed := now.Sub(lastTime)
				ts = formatTime(elapsedToTime(elapsed), goLayout)
				lastTime = now
			} else if *flagS {
				// R4.1: elapsed since start, monotonically increasing.
				elapsed := now.Sub(startTime)
				ts = formatTime(elapsedToTime(elapsed), goLayout)
			} else {
				// R1.2: absolute wall-clock timestamp.
				ts = formatTime(now, goLayout)
			}
			// R1.4: preserve the original newline; do not add an extra one.
			// R1.5: partial lines (no trailing newline) are output without one.
			if _, wErr := fmt.Fprintf(writer, "%s %s", ts, line); wErr != nil {
				break
			}
			if fErr := writer.Flush(); fErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	// R7.1: exit 0 on clean EOF.
}

// runRelativeMode implements -r mode (R6). It reads stdin line by line,
// scans each line for known timestamp patterns, and replaces them with
// human-readable relative age strings or reformatted timestamps.
func runRelativeMode(format string, hasCustomFormat bool) {
	goLayout := ""
	if hasCustomFormat {
		goLayout = strftimeToGo(format)
	}

	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	now := time.Now()

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			output := processRelativeLine(line, now, goLayout, hasCustomFormat)
			if _, wErr := writer.WriteString(output); wErr != nil {
				break
			}
			if fErr := writer.Flush(); fErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
}

// processRelativeLine scans a single line for timestamp patterns and replaces
// the first match with a relative age string (R6.1) or a reformatted timestamp
// (R6.3). Lines with no match are returned unchanged (R6.4).
func processRelativeLine(line string, now time.Time, goLayout string, hasCustomFormat bool) string {
	for _, pat := range relativePatterns {
		loc := pat.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		matched := line[loc[0]:loc[1]]
		var parsed time.Time
		var parseErr error
		for _, layout := range pat.layouts {
			if pat.hasYear {
				parsed, parseErr = time.Parse(layout, matched)
			} else {
				parsed, parseErr = time.ParseInLocation(layout, matched, time.Local)
			}
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			continue
		}
		// If the format has no year, assume current year.
		if !pat.hasYear {
			parsed = parsed.AddDate(now.Year(), 0, 0)
		}

		var replacement string
		if hasCustomFormat {
			// R6.3: reformat matched timestamp to the specified format.
			replacement = formatTime(parsed, goLayout)
		} else {
			// R6.1: replace with relative age string.
			replacement = formatRelativeAge(now.Sub(parsed))
		}
		return line[:loc[0]] + replacement + line[loc[1]:]
	}
	// R6.4: no recognizable timestamp, pass through unchanged.
	return line
}

// formatRelativeAge converts a duration to a human-readable relative age
// string such as "5s ago", "3m5s ago", or "1d2h3m5s ago".
func formatRelativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	totalSeconds := int64(d.Seconds())
	days := totalSeconds / 86400
	totalSeconds %= 86400
	hours := totalSeconds / 3600
	totalSeconds %= 3600
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	fmt.Fprintf(&b, "%ds", seconds)
	b.WriteString(" ago")
	return b.String()
}

// elapsedToTime converts a duration to a time.Time anchored at the Unix epoch
// in UTC, so that strftime-style formatting of hours, minutes, and seconds
// produces correct elapsed values (matching Perl's gmtime($elapsed) behavior
// per R3.2 and R4.2).
func elapsedToTime(d time.Duration) time.Time {
	nsec := d.Nanoseconds()
	sec := nsec / 1e9
	rem := nsec % 1e9
	return time.Unix(sec, rem).In(time.UTC)
}

// strftimeToGo converts a strftime format string to a Go time layout.
// Handles the common specifiers used by moreutils ts per R2.2:
// %Y, %m, %d, %H, %M, %S, %b, %T, %s (epoch), %N (nanoseconds),
// and subsecond extensions %.S, %.s, %.T (R2.3).
// Unrecognized specifiers pass through literally.
func strftimeToGo(format string) string {
	var b strings.Builder
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
		// R2.3: check for %.<spec> subsecond extensions.
		if format[i+1] == '.' && i+2 < len(format) {
			switch format[i+2] {
			case 'S':
				b.WriteString(placeholderDotS)
				i += 3
				continue
			case 's':
				b.WriteString(placeholderDotSm)
				i += 3
				continue
			case 'T':
				b.WriteString(placeholderDotT)
				i += 3
				continue
			}
		}
		switch format[i+1] {
		case 'Y':
			b.WriteString("2006")
		case 'm':
			b.WriteString("01")
		case 'd':
			b.WriteString("02")
		case 'e':
			b.WriteString("_2")
		case 'H':
			b.WriteString("15")
		case 'M':
			b.WriteString("04")
		case 'S':
			b.WriteString("05")
		case 'b', 'h':
			b.WriteString("Jan")
		case 'B':
			b.WriteString("January")
		case 'a':
			b.WriteString("Mon")
		case 'A':
			b.WriteString("Monday")
		case 'p':
			b.WriteString("PM")
		case 'T':
			b.WriteString("15:04:05")
		case 'F':
			b.WriteString("2006-01-02")
		case 'D':
			b.WriteString("01/02/06")
		case 'Z':
			b.WriteString("MST")
		case 'z':
			b.WriteString("-0700")
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '%':
			b.WriteByte('%')
		case 's':
			b.WriteString(placeholderEpoch)
		case 'N':
			b.WriteString(placeholderNano)
		default:
			// Unrecognized: pass through literally.
			b.WriteByte('%')
			b.WriteByte(format[i+1])
		}
		i += 2
	}
	return b.String()
}

// formatTime formats a time using the pre-converted Go layout and substitutes
// runtime-dependent placeholders for epoch seconds, nanoseconds, and subsecond
// extensions (R2.3, R2.4).
func formatTime(t time.Time, goLayout string) string {
	result := t.Format(goLayout)

	if strings.Contains(result, placeholderEpoch) {
		epoch := fmt.Sprintf("%d", t.Unix())
		result = strings.ReplaceAll(result, placeholderEpoch, epoch)
	}
	if strings.Contains(result, placeholderNano) {
		result = strings.ReplaceAll(result, placeholderNano,
			fmt.Sprintf("%09d", t.Nanosecond()))
	}
	if strings.Contains(result, placeholderDotS) {
		// %.S: seconds with microsecond suffix, e.g. "32.001234".
		result = strings.ReplaceAll(result, placeholderDotS,
			fmt.Sprintf("%02d.%06d", t.Second(), t.Nanosecond()/1000))
	}
	if strings.Contains(result, placeholderDotSm) {
		// %.s: Unix epoch with microsecond suffix, e.g. "1708358732.001234".
		result = strings.ReplaceAll(result, placeholderDotSm,
			fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000))
	}
	if strings.Contains(result, placeholderDotT) {
		// %.T: HH:MM:SS with microsecond suffix, e.g. "14:05:32.001234".
		result = strings.ReplaceAll(result, placeholderDotT,
			fmt.Sprintf("%02d:%02d:%02d.%06d",
				t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000))
	}

	return result
}
