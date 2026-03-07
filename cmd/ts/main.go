// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ts command that reads stdin line by line and
// prepends a timestamp to each line on stdout.
// Implements prd004-ts R1-R5, R7-R8.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
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

func main() {
	// R1.6: exit 0 on SIGPIPE per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	flagI := flag.Bool("i", false, "incremental per-line elapsed timestamps")
	flagS := flag.Bool("s", false, "elapsed since start timestamps")
	flagM := flag.Bool("m", false, "use monotonic clock")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ts [-i | -s] [-m] [format]\n")
	}
	flag.Parse()

	// R5.1: -m is accepted for compatibility. Go's time.Now() includes a
	// monotonic reading used automatically by Sub() for elapsed calculations.
	_ = *flagM

	// R3.2, R4.2: default format for elapsed modes is HH:MM:SS.
	format := defaultAbsoluteFormat
	if *flagI || *flagS {
		format = defaultElapsedFormat
	}
	// R2.1: optional positional argument overrides the default format.
	if flag.NArg() > 0 {
		format = flag.Arg(0)
	}

	goLayout := strftimeToGo(format)
	elapsedMode := *flagI || *flagS

	startTime := time.Now()
	lastTime := startTime

	reader := bufio.NewReader(os.Stdin)
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
				ts = formatTime(elapsedToTime(elapsed), goLayout, elapsedMode)
				lastTime = now
			} else if *flagS {
				// R4.1: elapsed since start, monotonically increasing.
				elapsed := now.Sub(startTime)
				ts = formatTime(elapsedToTime(elapsed), goLayout, elapsedMode)
			} else {
				// R1.2: absolute wall-clock timestamp.
				ts = formatTime(now, goLayout, elapsedMode)
			}
			// R1.4: preserve the original newline; do not add an extra one.
			// R1.5: partial lines (no trailing newline) are output without one.
			fmt.Fprintf(os.Stdout, "%s %s", ts, line)
		}
		if err != nil {
			break
		}
	}
	// R7.1: exit 0 on clean EOF.
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
// Handles the common specifiers used by moreutils ts per D2:
// %Y, %m, %d, %H, %M, %S, %b, %T, %s (epoch), %N (nanoseconds),
// and subsecond extensions %.S, %.s, %.T.
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
			// Unrecognized: pass through literally per D2.
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
func formatTime(t time.Time, goLayout string, elapsedMode bool) string {
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
