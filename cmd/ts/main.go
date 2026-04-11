// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ts: prepend timestamps to stdin lines.
// Implements srd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ts"

// defaultStrftimeFmt is the default strftime format per R1.2.
const defaultStrftimeFmt = "%b %d %H:%M:%S"

// defaultIncrementalFmt is the default format for -i and -s modes per R3.2, R4.2.
const defaultIncrementalFmt = "%H:%M:%S"

// config holds parsed command-line options.
type config struct {
	format      string
	incremental bool // -i mode (R3.1)
	elapsed     bool // -s mode (R4.1)
	monotonic   bool // -m mode (R5.1)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and timestamps stdin.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return timestampStdin(cfg)
}

// parseArgs extracts flags and the optional format string.
// R2.1: accepts optional positional format string.
// R3.1: -i enables incremental mode.
// R3.3: custom format overrides -i default.
// R3.4: when both -i and -s are given, -s takes precedence (matches reference).
// R4.1: -s enables elapsed-since-start mode.
// R4.3: custom format overrides -s default.
// R5.1: -m enables monotonic clock mode.
// R7.2: unrecognized flags produce an error.
func parseArgs(args []string) (config, error) {
	var cfg config
	hasCustomFormat := false
	var customFormat string
	for _, arg := range args {
		if arg == "-i" {
			cfg.incremental = true
			continue
		}
		if arg == "-s" {
			cfg.elapsed = true
			continue
		}
		// R5.1: -m uses monotonic clock. In Go, time.Now() already
		// includes a monotonic reading used by time.Sub(), so this
		// flag is accepted for compatibility but behavior is inherent.
		if arg == "-m" {
			cfg.monotonic = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return config{}, fmt.Errorf(
				"%s: unrecognized option '%s'", progName, arg)
		}
		customFormat = arg
		hasCustomFormat = true
	}
	// R3.4: when both -i and -s are given, -s takes precedence
	// (matches reference binary behavior).
	if cfg.elapsed {
		cfg.incremental = false
	}
	cfg.format = selectFormat(
		hasCustomFormat, customFormat, cfg.incremental || cfg.elapsed)
	return cfg, nil
}

// selectFormat determines the format string based on mode and user input.
// R3.3, R4.2: custom format overrides defaults for both -i and -s.
func selectFormat(hasCustom bool, custom string, delta bool) string {
	if hasCustom {
		return custom
	}
	if delta {
		return defaultIncrementalFmt
	}
	return defaultStrftimeFmt
}

// timestampStdin reads stdin and prepends timestamps.
// R1.1: read line by line, prepend timestamp + space.
// R1.3: flush stdout after each line.
// R1.4: preserve original newline, do not add extra.
// R1.5: pass through partial lines.
// R1.6: exit 0 on EOF.
// R3.1: -i shows elapsed time since previous line.
// R3.2: -i uses TZ=GMT for elapsed formatting.
// R4.1: -s shows elapsed time since start.
// R4.2: -s uses TZ=GMT for elapsed formatting.
func timestampStdin(cfg config) int {
	reader := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	var lastTime time.Time
	var startTime time.Time
	var gmt *time.Location
	if cfg.incremental || cfg.elapsed {
		gmt, _ = time.LoadLocation("GMT")
		now := time.Now()
		lastTime = now
		startTime = now
	}
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			now := time.Now()
			ts := formatTimestamp(cfg, now, &lastTime, startTime, gmt)
			fmt.Fprintf(w, "%s %s", ts, line)
			w.Flush()
		}
		if err != nil {
			break
		}
	}
	return 0
}

// formatTimestamp produces the timestamp string for one line.
// R2.4: a single time sample is used for both second and subsecond.
// R4.1: -s uses time since start (monotonically increasing).
func formatTimestamp(
	cfg config, now time.Time, lastTime *time.Time,
	startTime time.Time, gmt *time.Location,
) string {
	if cfg.incremental {
		delta := now.Sub(*lastTime)
		*lastTime = now
		deltaTime := time.Unix(0, delta.Nanoseconds()).In(gmt)
		return formatStrftime(cfg.format, deltaTime)
	}
	if cfg.elapsed {
		delta := now.Sub(startTime)
		deltaTime := time.Unix(0, delta.Nanoseconds()).In(gmt)
		return formatStrftime(cfg.format, deltaTime)
	}
	return formatStrftime(cfg.format, now)
}

// strftimeSimple maps strftime specifiers to Go time.Format tokens
// for specifiers that have direct Go format equivalents.
var strftimeSimple = map[byte]string{
	'a': "Mon", 'A': "Monday",
	'b': "Jan", 'B': "January", 'h': "Jan",
	'd': "02", 'e': "_2",
	'H': "15", 'I': "03",
	'j': "002",
	'm': "01", 'M': "04",
	'p': "PM",
	'S': "05",
	'y': "06", 'Y': "2006",
	'z': "-0700", 'Z': "MST",
	'n': "\n", 't': "\t",
	'T': "15:04:05",
	'R': "15:04",
	'D': "01/02/06",
	'F': "2006-01-02",
	'r': "03:04:05 PM",
	'X': "15:04:05",
}

// formatStrftime formats a time value using a strftime format string.
// R2.2: supports standard strftime(3) conversion specifications.
// R2.3: supports %.S, %.s, %.T subsecond extensions.
func formatStrftime(format string, t time.Time) string {
	var b strings.Builder
	b.Grow(len(format) * 2)
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		if format[i] == '.' && i+1 < len(format) {
			i++
			writeSubsecondSpec(&b, format[i], t)
		} else {
			writeSpecifier(&b, format[i], t)
		}
	}
	return b.String()
}

// writeSubsecondSpec handles ts-specific subsecond extensions.
// R2.3: %.S = seconds.microseconds, %.s = epoch.microseconds,
// %.T = HH:MM:SS.microseconds. Six decimal places.
func writeSubsecondSpec(b *strings.Builder, spec byte, t time.Time) {
	usec := t.Nanosecond() / 1000
	switch spec {
	case 'S':
		fmt.Fprintf(b, "%02d.%06d", t.Second(), usec)
	case 's':
		fmt.Fprintf(b, "%d.%06d", t.Unix(), usec)
	case 'T':
		fmt.Fprintf(b, "%s.%06d", t.Format("15:04:05"), usec)
	default:
		b.WriteString("%.")
		b.WriteByte(spec)
	}
}

// writeSpecifier writes a single strftime specifier to the builder.
func writeSpecifier(b *strings.Builder, spec byte, t time.Time) {
	if spec == '%' {
		b.WriteByte('%')
		return
	}
	if goToken, ok := strftimeSimple[spec]; ok {
		b.WriteString(t.Format(goToken))
		return
	}
	writeComplexSpec(b, spec, t)
}

// writeComplexSpec handles specifiers requiring computation
// beyond simple Go format token substitution.
func writeComplexSpec(b *strings.Builder, spec byte, t time.Time) {
	switch spec {
	case 's':
		fmt.Fprintf(b, "%d", t.Unix())
	case 'C':
		fmt.Fprintf(b, "%02d", t.Year()/100)
	case 'k':
		fmt.Fprintf(b, "%2d", t.Hour())
	case 'l':
		fmt.Fprintf(b, "%2d", hour12(t))
	case 'P':
		writeLowerMeridiem(b, t)
	case 'u':
		writeISOWeekday(b, t)
	case 'w':
		fmt.Fprintf(b, "%d", int(t.Weekday()))
	case 'U':
		fmt.Fprintf(b, "%02d", weekNumber(t, time.Sunday))
	case 'W':
		fmt.Fprintf(b, "%02d", weekNumber(t, time.Monday))
	case 'V':
		_, w := t.ISOWeek()
		fmt.Fprintf(b, "%02d", w)
	case 'G':
		y, _ := t.ISOWeek()
		fmt.Fprintf(b, "%04d", y)
	case 'g':
		y, _ := t.ISOWeek()
		fmt.Fprintf(b, "%02d", y%100)
	case 'c':
		// R2.2: C locale equivalent "%a %b %e %T %Y"
		b.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
	case 'x':
		b.WriteString(t.Format("01/02/06"))
	default:
		b.WriteByte('%')
		b.WriteByte(spec)
	}
}

// hour12 returns the 12-hour clock value (1-12).
func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		return 12
	}
	return h
}

// writeLowerMeridiem writes "am" or "pm" based on the hour.
func writeLowerMeridiem(b *strings.Builder, t time.Time) {
	if t.Hour() < 12 {
		b.WriteString("am")
	} else {
		b.WriteString("pm")
	}
}

// writeISOWeekday writes the ISO weekday number (1=Mon, 7=Sun).
func writeISOWeekday(b *strings.Builder, t time.Time) {
	d := int(t.Weekday())
	if d == 0 {
		d = 7
	}
	fmt.Fprintf(b, "%d", d)
}

// weekNumber computes the week number with the given day as week start.
// %U uses Sunday, %W uses Monday.
func weekNumber(t time.Time, firstDay time.Weekday) int {
	yday := t.YearDay() - 1
	wday := int(t.Weekday()) - int(firstDay)
	if wday < 0 {
		wday += 7
	}
	return (yday + 7 - wday) / 7
}
