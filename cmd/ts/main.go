// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1–R1.6, R2.1–R2.4, R3.1–R3.2: timestamp stdin lines
// with default and custom strftime format support, subsecond extensions, and
// incremental mode.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultStrftimeFormat is the strftime format per R1.2: "%b %d %H:%M:%S".
const defaultStrftimeFormat = "%b %d %H:%M:%S"

// defaultIncrementalFormat is the strftime format for -i mode per R3.2.
const defaultIncrementalFormat = "%H:%M:%S"

// tsConfig holds parsed command-line configuration.
type tsConfig struct {
	format      string
	incremental bool
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
// R3.1: -i flag for incremental mode.
func parseArgs(args []string) tsConfig {
	cfg := tsConfig{}
	for _, arg := range args {
		if arg == "-i" {
			cfg.incremental = true
		} else {
			cfg.format = arg
		}
	}
	if cfg.format == "" {
		cfg.format = defaultStrftimeFormat
		if cfg.incremental {
			cfg.format = defaultIncrementalFormat
		}
	}
	return cfg
}

// run parses arguments and processes stdin.
func run(args []string) error {
	cfg := parseArgs(args)
	return processStdin(cfg)
}

// processStdin reads stdin and writes timestamped lines to stdout.
// R1.1: line-by-line. R1.5: partial lines at EOF. R1.6: exit 0 on EOF.
// R3.1: incremental mode tracks time between lines.
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
// R3.2: in incremental mode, uses UTC (GMT) for delta formatting.
func formatTimestamp(cfg tsConfig, now time.Time, lastTime *time.Time) string {
	if !cfg.incremental {
		return strftime(cfg.format, now)
	}
	delta := now.Sub(*lastTime)
	*lastTime = now
	// R3.2: Convert delta to time at Unix epoch in UTC for strftime formatting.
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
