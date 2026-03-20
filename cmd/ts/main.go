// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1–R1.6, R2.1, R2.2: timestamp stdin lines with
// default and custom strftime format support.
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

func main() {
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ts: %v\n", err)
		os.Exit(1)
	}
}

// run parses arguments and processes stdin.
// R2.1: optional positional argument for custom strftime format.
func run(args []string) error {
	format := defaultStrftimeFormat
	if len(args) > 0 {
		format = args[0]
	}
	return processStdin(format)
}

// processStdin reads stdin and writes timestamped lines to stdout.
// R1.1: line-by-line. R1.5: partial lines at EOF. R1.6: exit 0 on EOF.
func processStdin(format string) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if writeErr := writeLine(writer, format, line); writeErr != nil {
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

// writeLine writes a single timestamped line to the writer.
// R1.2: timestamp at receipt time. R1.3: flush after each line.
// R1.4: preserves original newline. R1.5: no added newline for partial lines.
func writeLine(w *bufio.Writer, format string, line []byte) error {
	ts := strftime(format, time.Now())
	w.WriteString(ts)
	w.WriteByte(' ')
	w.Write(line)
	return w.Flush()
}

// strftime formats a time.Time using a strftime(3) format string.
// R2.2: supports all standard strftime conversion specifications.
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
		buf.WriteString(fmtSpec(format[i+1], t))
		i += 2
	}
	return buf.String()
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
