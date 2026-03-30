// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ts implements moreutils ts: prepend timestamps to stdin lines.
// Implements prd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.2.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the strftime format used when no format argument is given.
// R1.2: "%b %d %H:%M:%S" (e.g., "Mar 30 14:05:32").
const defaultFormat = "%b %d %H:%M:%S"

// defaultElapsedFormat is the default format for -i and -s modes.
// R3.2, R4.2: "%H:%M:%S" with TZ=GMT.
const defaultElapsedFormat = "%H:%M:%S"

// tsMode represents the timestamp source mode.
type tsMode int

const (
	modeDefault     tsMode = iota
	modeIncremental        // -i: elapsed since previous line
	modeElapsed            // -s: elapsed since start
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
	format string
	mode   tsMode
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg := parseArgs(os.Args[1:])

	switch cfg.mode {
	case modeIncremental:
		runIncremental(cfg.format)
	case modeElapsed:
		runElapsed(cfg.format)
	default:
		runDefault(cfg.format)
	}
}

// parseArgs parses ts flags and an optional positional format string.
// R3.4: -i and -s; last flag wins (matches reference binary behavior).
func parseArgs(args []string) tsConfig {
	cfg := tsConfig{format: defaultFormat}
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "-i":
			cfg.mode = modeIncremental
		case "-s":
			cfg.mode = modeElapsed
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
func printUsageAndExit() {
	fmt.Fprintf(os.Stderr, "usage: ts [-i] [-s] [-m] [-r] [format]\n")
	os.Exit(1)
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
