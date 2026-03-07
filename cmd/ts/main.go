// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ts: timestamp stdin lines.
// Implements prd004-ts R1-R5.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// timestampMode represents the timestamp calculation mode.
type timestampMode int

const (
	modeWallClock   timestampMode = iota // default: wall clock time
	modeElapsed                          // -s: elapsed since start
	modeIncremental                      // -i: time since previous line
)

const (
	defaultFormat = "%b %d %H:%M:%S"
	elapsedFormat = "%H:%M:%S"
)

func main() {
	sys.InstallSIGPIPEHandler()

	mode, format := parseArgs(os.Args[1:])

	startTime := time.Now()
	prevTime := startTime

	reader := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}

		now := time.Now()

		var ts string
		switch mode {
		case modeElapsed:
			// R4.1, R4.2: elapsed since start, formatted via TZ=GMT.
			elapsed := now.Sub(startTime)
			ts = formatElapsed(format, elapsed)
		case modeIncremental:
			// R3.1, R3.2: time since previous line, formatted via TZ=GMT.
			elapsed := now.Sub(prevTime)
			prevTime = now
			ts = formatElapsed(format, elapsed)
		default:
			// R1.2: wall clock timestamp evaluated per line.
			ts = strftime(format, now)
		}

		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		content := line
		if hasNewline {
			content = line[:len(line)-1]
		}

		// R1.1: prepend timestamp and a single space.
		fmt.Fprintf(w, "%s %s", ts, string(content))
		if hasNewline {
			// R1.4: preserve original newline.
			w.WriteByte('\n')
		}
		// R1.3: flush after each line for real-time output.
		w.Flush() // best-effort flush

		if err != nil {
			break
		}
	}
}

// parseArgs parses ts command-line flags and returns the mode and format string.
// Supports combined short flags (e.g., -im for incremental + monotonic).
func parseArgs(args []string) (timestampMode, string) {
	iFlag := false
	sFlag := false
	var positional []string

	endFlags := false
	for _, arg := range args {
		if endFlags {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'i':
					iFlag = true
				case 's':
					sFlag = true
				case 'm':
					// R5.1: -m accepted. Go time.Now() already uses monotonic
					// readings for elapsed calculations via time.Since().
				default:
					fmt.Fprintf(os.Stderr, "ts: usage: ts [-i | -s] [-m] [format]\n")
					os.Exit(1)
				}
			}
			continue
		}
		positional = append(positional, arg)
	}

	// R3.4: -i and -s are mutually exclusive.
	if iFlag && sFlag {
		fmt.Fprintf(os.Stderr, "ts: usage: ts [-i | -s] [-m] [format]\n")
		os.Exit(1)
	}

	mode := modeWallClock
	if iFlag {
		mode = modeIncremental
	} else if sFlag {
		mode = modeElapsed
	}

	// R2.1: optional positional argument overrides default format.
	var format string
	if len(positional) > 0 {
		format = positional[0]
	} else if mode == modeIncremental || mode == modeElapsed {
		format = elapsedFormat
	} else {
		format = defaultFormat
	}

	return mode, format
}

// formatElapsed formats a duration as a timestamp using TZ=GMT strftime.
// R3.2, R4.2: elapsed time formatted by creating a UTC time from epoch + duration.
func formatElapsed(format string, d time.Duration) string {
	t := time.Unix(0, 0).UTC().Add(d)
	return strftime(format, t)
}

// strftime formats a time.Time using a strftime-style format string.
// R2.2: supports standard strftime(3) conversion specifications.
// R2.3: supports ts-specific extensions %.S, %.s, %.T.
func strftime(format string, t time.Time) string {
	var buf strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}
		// R2.3: ts-specific subsecond extensions.
		if format[i] == '.' && i+1 < len(format) {
			usec := t.Nanosecond() / 1000
			switch format[i+1] {
			case 'S':
				// %.S: seconds with microsecond suffix (e.g., "32.001234").
				fmt.Fprintf(&buf, "%02d.%06d", t.Second(), usec)
				i += 2
				continue
			case 's':
				// %.s: Unix epoch with microsecond suffix (e.g., "1708358732.001234").
				fmt.Fprintf(&buf, "%d.%06d", t.Unix(), usec)
				i += 2
				continue
			case 'T':
				// %.T: HH:MM:SS with microsecond suffix.
				fmt.Fprintf(&buf, "%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), usec)
				i += 2
				continue
			}
		}
		switch format[i] {
		case 'Y':
			fmt.Fprintf(&buf, "%04d", t.Year())
		case 'y':
			fmt.Fprintf(&buf, "%02d", t.Year()%100)
		case 'm':
			fmt.Fprintf(&buf, "%02d", int(t.Month()))
		case 'd':
			fmt.Fprintf(&buf, "%02d", t.Day())
		case 'e':
			fmt.Fprintf(&buf, "%2d", t.Day())
		case 'H':
			fmt.Fprintf(&buf, "%02d", t.Hour())
		case 'M':
			fmt.Fprintf(&buf, "%02d", t.Minute())
		case 'S':
			fmt.Fprintf(&buf, "%02d", t.Second())
		case 'b', 'h':
			buf.WriteString(t.Format("Jan"))
		case 'B':
			buf.WriteString(t.Format("January"))
		case 'a':
			buf.WriteString(t.Format("Mon"))
		case 'A':
			buf.WriteString(t.Format("Monday"))
		case 'T':
			fmt.Fprintf(&buf, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'R':
			fmt.Fprintf(&buf, "%02d:%02d", t.Hour(), t.Minute())
		case 'r':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			ampm := "AM"
			if t.Hour() >= 12 {
				ampm = "PM"
			}
			fmt.Fprintf(&buf, "%02d:%02d:%02d %s", h, t.Minute(), t.Second(), ampm)
		case 's':
			fmt.Fprintf(&buf, "%d", t.Unix())
		case 'j':
			fmt.Fprintf(&buf, "%03d", t.YearDay())
		case 'p':
			if t.Hour() < 12 {
				buf.WriteString("AM")
			} else {
				buf.WriteString("PM")
			}
		case 'Z':
			buf.WriteString(t.Format("MST"))
		case 'z':
			buf.WriteString(t.Format("-0700"))
		case 'n':
			buf.WriteByte('\n')
		case 't':
			buf.WriteByte('\t')
		case '%':
			buf.WriteByte('%')
		default:
			buf.WriteByte('%')
			buf.WriteByte(format[i])
		}
		i++
	}
	return buf.String()
}
