// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ts implements moreutils ts: prepend timestamps to stdin lines.
// Implements prd004-ts R1.1-R1.4, R2.1.
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

func main() {
	sys.InstallSIGPIPEHandler()

	format := defaultFormat
	if len(os.Args) > 1 {
		format = os.Args[1]
	}

	processStdin(format)
}

// processStdin reads stdin line by line and writes each line to stdout
// prefixed by a timestamp formatted with the given strftime format.
// R1.1: reads stdin line by line (newline-delimited).
// R1.3: flushes stdout after each line.
// R1.4: preserves the original newline; does not add an extra one.
func processStdin(format string) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			now := time.Now()
			ts := formatTime(now, format)
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
		// R2.3: ts-specific subsecond extensions %.S, %.s, %.T.
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
	case 'w':
		fmt.Fprintf(buf, "%d", int(t.Weekday()))
	case '%':
		buf.WriteByte('%')
	default:
		buf.WriteByte('%')
		buf.WriteByte(spec)
	}
}
