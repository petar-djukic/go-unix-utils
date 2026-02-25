// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ts utility: prepend strftime timestamps to each
// stdin line with incremental, elapsed, monotonic, and relative modes.
//
// Implements: prd004-ts (R1-R8)
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Default strftime format matching the Perl ts reference binary.
// Per prd004-ts R1.2.
const defaultFormat = "%b %d %H:%M:%S"

// Default format for -i and -s elapsed modes.
// Per prd004-ts R3.2, R4.2.
const defaultElapsedFormat = "%H:%M:%S"

// Sentinel placeholders for ts-specific subsecond extensions.
// These are substituted before strftime-to-Go conversion and replaced
// with actual microsecond values after time.Format returns.
// Per design decision D1.
const (
	sentinelDotS    = "\x00SUBSEC_DOT_S\x00"
	sentinelDots    = "\x00SUBSEC_DOT_SMALL_S\x00"
	sentinelDotT    = "\x00SUBSEC_DOT_T\x00"
	sentinelEpoch   = "\x00EPOCH_S\x00"
)

// gmtZone is a fixed GMT timezone for elapsed-time formatting.
// Per prd004-ts R3.2, R4.2, R8.2 and design decision D2.
var gmtZone = time.FixedZone("GMT", 0)

func main() {
	// SIGPIPE handling: exit 0 silently on broken pipe.
	// Per prd004-ts R1.6 and design decision D5.
	sigpipeCh := make(chan os.Signal, 1)
	signal.Notify(sigpipeCh, syscall.SIGPIPE)
	go func() {
		<-sigpipeCh
		os.Exit(0)
	}()

	flagI, flagS, flagM, flagR, formatStr, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ts: %s\nusage: ts [-r] [-i | -s] [-m] [format]\n", err)
		os.Exit(1)
	}

	if flagR {
		runRelativeMode(formatStr)
	} else {
		runTimestampMode(flagI, flagS, flagM, formatStr)
	}
}

// parseFlags parses ts command-line arguments and validates mutual exclusivity.
// Per design decision D3 and prd004-ts R3.4, R6.5, R7.2.
func parseFlags(args []string) (flagI, flagS, flagM, flagR bool, formatStr string, err error) {
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// Parse short flags, potentially combined (e.g., -im)
			for _, ch := range arg[1:] {
				switch ch {
				case 'i':
					flagI = true
				case 's':
					flagS = true
				case 'm':
					flagM = true
				case 'r':
					flagR = true
				default:
					return false, false, false, false, "", fmt.Errorf("unrecognized option '-%c'", ch)
				}
			}
		} else if len(arg) > 2 && arg[:2] == "--" {
			return false, false, false, false, "", fmt.Errorf("unrecognized option '%s'", arg)
		} else {
			positional = append(positional, arg)
		}
	}

	// Validate mutual exclusivity. Per prd004-ts R3.4, R6.5.
	if flagI && flagS {
		return false, false, false, false, "", fmt.Errorf("-i and -s are mutually exclusive")
	}
	if flagR && flagI {
		return false, false, false, false, "", fmt.Errorf("-r and -i are mutually exclusive")
	}
	if flagR && flagS {
		return false, false, false, false, "", fmt.Errorf("-r and -s are mutually exclusive")
	}

	if len(positional) > 0 {
		formatStr = positional[0]
	}

	return flagI, flagS, flagM, flagR, formatStr, nil
}

// runTimestampMode handles default, -i, -s, and -m timestamp modes.
// Per prd004-ts R1, R2, R3, R4, R5.
func runTimestampMode(flagI, flagS, flagM bool, formatStr string) {
	if formatStr == "" {
		if flagI || flagS {
			formatStr = defaultElapsedFormat
		} else {
			formatStr = defaultFormat
		}
	}

	isElapsed := flagI || flagS
	goLayout, hasDotS, hasDots, hasDotT := convertFormat(formatStr)

	startTime := time.Now()
	prevTime := startTime

	writer := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		now := time.Now()

		var ts string
		if isElapsed {
			var delta time.Duration
			if flagS {
				// Elapsed since start. Per prd004-ts R4.1.
				if flagM {
					delta = now.Sub(startTime)
				} else {
					delta = now.Sub(startTime)
				}
			} else {
				// Incremental since previous line. Per prd004-ts R3.1.
				if flagM {
					delta = now.Sub(prevTime)
				} else {
					delta = now.Sub(prevTime)
				}
			}
			ts = formatElapsed(delta, goLayout, hasDotS, hasDots, hasDotT)
		} else {
			ts = formatTimestamp(now, goLayout, hasDotS, hasDots, hasDotT)
		}

		prevTime = now
		// Per prd004-ts R1.1: prefix with timestamp and single space.
		if _, err := fmt.Fprintf(writer, "%s %s\n", ts, line); err != nil {
			os.Exit(0) // Write error on stdout; exit silently (SIGPIPE or broken pipe).
		}
		// Per prd004-ts R1.3: flush after each line.
		if err := writer.Flush(); err != nil {
			os.Exit(0) // Flush error on stdout; exit silently.
		}
	}

	// Handle partial last line (no trailing newline). Per prd004-ts R1.5.
	// bufio.Scanner does not distinguish between lines ending with \n and partial
	// lines. The Scan loop above handles both cases since Scanner strips the newline
	// and we always write \n. For a truly partial line at EOF (no trailing newline),
	// Scanner still returns the content via Scan()/Text().

	// Per prd004-ts R1.6, R7.1: exit 0 on clean EOF.
}

// runRelativeMode handles -r mode: scan lines for timestamps and replace them
// with relative age strings.
// Per prd004-ts R6.1-R6.5.
func runRelativeMode(formatStr string) {
	var goLayout string
	var hasDotS, hasDots, hasDotT bool
	hasCustomFormat := formatStr != ""
	if hasCustomFormat {
		goLayout, hasDotS, hasDots, hasDotT = convertFormat(formatStr)
	}

	writer := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		result := processRelativeLine(line, hasCustomFormat, goLayout, hasDotS, hasDots, hasDotT)
		if _, err := fmt.Fprintf(writer, "%s\n", result); err != nil {
			os.Exit(0)
		}
		if err := writer.Flush(); err != nil {
			os.Exit(0)
		}
	}
}

// relativePattern represents a compiled regex and its time parsing function for -r mode.
type relativePattern struct {
	re    *regexp.Regexp
	parse func(match string) (time.Time, bool)
}

// relativePatterns are the timestamp formats recognized by -r mode.
// Per prd004-ts R6.2.
var relativePatterns = []relativePattern{
	{
		// ISO-8601: "2024-01-05T14:30:00.000Z" or "2024-01-05T14:30:00"
		re: regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`),
		parse: func(match string) (time.Time, bool) {
			layouts := []string{
				"2006-01-02T15:04:05.000Z",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05.000-07:00",
				"2006-01-02T15:04:05-07:00",
				"2006-01-02T15:04:05.000",
				"2006-01-02T15:04:05",
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, match); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	{
		// RFC 2822 with optional day: "16 Jun 94 07:29:35 GMT" or "Thu, 16 Jun 94 07:29:35 GMT"
		re: regexp.MustCompile(`(?:[A-Z][a-z]{2},?\s+)?\d{1,2}\s+[A-Z][a-z]{2}\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}\s+[A-Z]{2,4}`),
		parse: func(match string) (time.Time, bool) {
			layouts := []string{
				"02 Jan 2006 15:04:05 MST",
				"02 Jan 06 15:04:05 MST",
				"2 Jan 2006 15:04:05 MST",
				"2 Jan 06 15:04:05 MST",
				"Mon, 02 Jan 2006 15:04:05 MST",
				"Mon, 02 Jan 06 15:04:05 MST",
				"Mon, 2 Jan 2006 15:04:05 MST",
				"Mon, 2 Jan 06 15:04:05 MST",
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, match); err == nil {
					return t, true
				}
			}
			return time.Time{}, false
		},
	},
	{
		// Syslog format: "Jan  5 14:30:00" or "Feb 19 12:34:56"
		re: regexp.MustCompile(`[A-Z][a-z]{2}\s{1,2}\d{1,2}\s\d{2}:\d{2}:\d{2}`),
		parse: func(match string) (time.Time, bool) {
			t, err := time.Parse("Jan  2 15:04:05", match)
			if err != nil {
				t, err = time.Parse("Jan 2 15:04:05", match)
			}
			if err != nil {
				return time.Time{}, false
			}
			// Syslog has no year; assume current year.
			now := time.Now()
			t = t.AddDate(now.Year()-t.Year(), 0, 0)
			return t, true
		},
	},
	{
		// Lastlog format: "Mon Jan  5 14:30"
		re: regexp.MustCompile(`[A-Z][a-z]{2}\s[A-Z][a-z]{2}\s{1,2}\d{1,2}\s\d{2}:\d{2}`),
		parse: func(match string) (time.Time, bool) {
			t, err := time.Parse("Mon Jan  2 15:04", match)
			if err != nil {
				t, err = time.Parse("Mon Jan 2 15:04", match)
			}
			if err != nil {
				return time.Time{}, false
			}
			// Lastlog has no year; assume current year.
			now := time.Now()
			t = t.AddDate(now.Year()-t.Year(), 0, 0)
			return t, true
		},
	},
}

// processRelativeLine processes a single line in -r mode.
func processRelativeLine(line string, hasCustomFormat bool, goLayout string, hasDotS, hasDots, hasDotT bool) string {
	for _, pat := range relativePatterns {
		loc := pat.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		match := line[loc[0]:loc[1]]
		t, ok := pat.parse(match)
		if !ok {
			continue
		}

		var replacement string
		if hasCustomFormat {
			// Per prd004-ts R6.3: convert matched timestamps to the specified format.
			replacement = formatTimestamp(t, goLayout, hasDotS, hasDots, hasDotT)
		} else {
			// Per prd004-ts R6.1: replace with relative age string.
			replacement = formatRelativeAge(time.Since(t))
		}
		return line[:loc[0]] + replacement + line[loc[1]:]
	}
	// Per prd004-ts R6.4: no recognized timestamp, pass through unchanged.
	return line
}

// formatRelativeAge converts a duration to a human-readable relative age string.
// Per design decision D4: uses the largest two non-zero units.
func formatRelativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	totalSeconds := int64(math.Round(d.Seconds()))

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

	// Use at most two units for readability.
	if len(parts) > 2 {
		parts = parts[:2]
	}

	return strings.Join(parts, "") + " ago"
}

// convertFormat pre-processes a strftime format string: extracts ts-specific
// subsecond extensions (%.S, %.s, %.T) as sentinel placeholders, then converts
// the remaining strftime directives to Go time.Format layout tokens.
// Per design decision D1 and prd004-ts R2.1-R2.3.
func convertFormat(strftimeFmt string) (goLayout string, hasDotS, hasDots, hasDotT bool) {
	// Pre-process ts-specific extensions before general strftime conversion.
	processed := strftimeFmt
	if strings.Contains(processed, "%.S") {
		hasDotS = true
		processed = strings.ReplaceAll(processed, "%.S", sentinelDotS)
	}
	if strings.Contains(processed, "%.s") {
		hasDots = true
		processed = strings.ReplaceAll(processed, "%.s", sentinelDots)
	}
	if strings.Contains(processed, "%.T") {
		hasDotT = true
		processed = strings.ReplaceAll(processed, "%.T", sentinelDotT)
	}

	goLayout = strftimeToGo(processed)
	return goLayout, hasDotS, hasDots, hasDotT
}

// strftimeToGo converts a strftime format string to a Go time.Format layout string.
// Per prd004-ts R2.2 and design decision D1.
func strftimeToGo(format string) string {
	var result strings.Builder
	runes := []rune(format)

	for i := 0; i < len(runes); i++ {
		if runes[i] == '%' && i+1 < len(runes) {
			i++
			switch runes[i] {
			case 'Y': // Four-digit year
				result.WriteString("2006")
			case 'y': // Two-digit year
				result.WriteString("06")
			case 'm': // Month as zero-padded decimal
				result.WriteString("01")
			case 'd': // Day of month as zero-padded decimal
				result.WriteString("02")
			case 'e': // Day of month, space-padded
				result.WriteString("_2")
			case 'H': // Hour (24-hour) as zero-padded decimal
				result.WriteString("15")
			case 'I': // Hour (12-hour) as zero-padded decimal
				result.WriteString("03")
			case 'M': // Minute as zero-padded decimal
				result.WriteString("04")
			case 'S': // Second as zero-padded decimal
				result.WriteString("05")
			case 'b', 'h': // Abbreviated month name
				result.WriteString("Jan")
			case 'B': // Full month name
				result.WriteString("January")
			case 'a': // Abbreviated weekday name
				result.WriteString("Mon")
			case 'A': // Full weekday name
				result.WriteString("Monday")
			case 'p': // AM/PM
				result.WriteString("PM")
			case 'P': // am/pm
				result.WriteString("pm")
			case 'Z': // Timezone abbreviation
				result.WriteString("MST")
			case 'z': // Timezone offset
				result.WriteString("-0700")
			case 'j': // Day of year (001-366) - Go doesn't have a direct equivalent
				result.WriteString("002") // Go's yearday placeholder, but non-standard
			case 'u': // Weekday as number (1=Monday)
				// Go has no direct equivalent; we use a placeholder
				result.WriteString("Mon")
			case 'w': // Weekday as number (0=Sunday)
				result.WriteString("Mon")
			case 's': // Unix epoch seconds
				result.WriteString(sentinelEpoch)
			case 'T': // Equivalent to %H:%M:%S
				result.WriteString("15:04:05")
			case 'F': // Equivalent to %Y-%m-%d
				result.WriteString("2006-01-02")
			case 'R': // Equivalent to %H:%M
				result.WriteString("15:04")
			case 'D': // Equivalent to %m/%d/%y
				result.WriteString("01/02/06")
			case 'n': // Newline
				result.WriteRune('\n')
			case 't': // Tab
				result.WriteRune('\t')
			case '%': // Literal percent
				result.WriteRune('%')
			case 'r': // 12-hour time (e.g., "11:11:04 PM")
				result.WriteString("03:04:05 PM")
			case 'c': // Date and time representation
				result.WriteString("Mon Jan _2 15:04:05 2006")
			case 'x': // Date representation
				result.WriteString("01/02/06")
			case 'X': // Time representation
				result.WriteString("15:04:05")
			default:
				// Unknown directive: pass through literally.
				result.WriteRune('%')
				result.WriteRune(runes[i])
			}
		} else {
			result.WriteRune(runes[i])
		}
	}

	return result.String()
}

// formatTimestamp formats a wall-clock time using the converted Go layout,
// substituting subsecond extension sentinels with actual values.
// Per prd004-ts R2.3-R2.4.
func formatTimestamp(t time.Time, goLayout string, hasDotS, hasDots, hasDotT bool) string {
	result := t.Format(goLayout)

	usec := t.Nanosecond() / 1000 // Microseconds from a single time sample.

	if hasDotS {
		// %.S: seconds with microsecond suffix, e.g. "32.001234"
		secStr := fmt.Sprintf("%02d.%06d", t.Second(), usec)
		result = strings.ReplaceAll(result, sentinelDotS, secStr)
	}
	if hasDots {
		// %.s: Unix epoch with microsecond suffix, e.g. "1708358732.001234"
		epochStr := fmt.Sprintf("%d.%06d", t.Unix(), usec)
		result = strings.ReplaceAll(result, sentinelDots, epochStr)
	}
	if hasDotT {
		// %.T: HH:MM:SS with microsecond suffix, e.g. "14:05:32.001234"
		timeStr := fmt.Sprintf("%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), usec)
		result = strings.ReplaceAll(result, sentinelDotT, timeStr)
	}

	// %s: standard Unix epoch seconds (no microseconds).
	if strings.Contains(result, sentinelEpoch) {
		result = strings.ReplaceAll(result, sentinelEpoch, fmt.Sprintf("%d", t.Unix()))
	}

	return result
}

// formatElapsed formats an elapsed duration using the converted Go layout with
// TZ=GMT. The delta is converted to a time.Time at epoch + delta for strftime
// compatibility.
// Per design decision D2 and prd004-ts R3.2, R4.2, R8.2.
func formatElapsed(d time.Duration, goLayout string, hasDotS, hasDots, hasDotT bool) string {
	// Construct a time.Time from epoch + delta seconds in GMT.
	epochGMT := time.Unix(0, 0).In(gmtZone)
	t := epochGMT.Add(d)

	result := t.Format(goLayout)

	usec := int64(d/time.Microsecond) % 1000000

	if hasDotS {
		sec := int64(d / time.Second)
		secStr := fmt.Sprintf("%02d.%06d", sec%60, usec)
		result = strings.ReplaceAll(result, sentinelDotS, secStr)
	}
	if hasDots {
		totalSec := int64(d / time.Second)
		epochStr := fmt.Sprintf("%d.%06d", totalSec, usec)
		result = strings.ReplaceAll(result, sentinelDots, epochStr)
	}
	if hasDotT {
		totalSec := int64(d / time.Second)
		hours := totalSec / 3600
		minutes := (totalSec % 3600) / 60
		seconds := totalSec % 60
		timeStr := fmt.Sprintf("%02d:%02d:%02d.%06d", hours, minutes, seconds, usec)
		result = strings.ReplaceAll(result, sentinelDotT, timeStr)
	}

	// %s: standard Unix epoch seconds for elapsed mode.
	if strings.Contains(result, sentinelEpoch) {
		totalSec := int64(d / time.Second)
		result = strings.ReplaceAll(result, sentinelEpoch, fmt.Sprintf("%d", totalSec))
	}

	return result
}
