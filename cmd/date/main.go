// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/date: display and format date and time.
// Implements srd060-date R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "date"

// defaultFormat is the GNU date default output format.
// R1.1: used when no +FORMAT argument is given.
const defaultFormat = "%a %b %e %H:%M:%S %Z %Y"

// config holds parsed command-line options.
type config struct {
	format  string
	dateStr string
	utc     bool
	err     bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the date logic and returns the exit code.
// R1.1: default output with no arguments. R4.1: exit 0 on success.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.err {
		return 1
	}
	t, err := resolveTime(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	fmt.Println(formatTime(t, cfg.format))
	return 0
}

// parseArgs extracts flags and the +FORMAT argument.
// R1.2: recognizes +FORMAT as the output format string.
func parseArgs(args []string) config {
	cfg := config{format: defaultFormat}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "+") {
			cfg.format = arg[1:]
			continue
		}
		i = parseFlag(&cfg, args, i)
	}
	return cfg
}

// parseFlag processes a single flag argument, returning the updated index.
func parseFlag(cfg *config, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "-d" || arg == "--date":
		if i+1 < len(args) {
			cfg.dateStr = args[i+1]
			return i + 1
		}
		fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
		cfg.err = true
	case strings.HasPrefix(arg, "--date="):
		cfg.dateStr = arg[len("--date="):]
	case arg == "-u" || arg == "--utc" || arg == "--universal":
		cfg.utc = true
	default:
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
		cfg.err = true
	}
	return i
}

// resolveTime determines the time to display based on config.
func resolveTime(cfg config) (time.Time, error) {
	loc := time.Local
	if cfg.utc {
		loc = time.UTC
	}
	if cfg.dateStr == "" {
		return time.Now().In(loc), nil
	}
	t, err := parseDateString(cfg.dateStr, loc)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(loc), nil
}

// parseDateString parses a date string from the -d flag.
func parseDateString(s string, loc *time.Location) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	return parseISO(s, loc)
}

// parseEpoch parses @SECONDS epoch timestamps.
func parseEpoch(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date '@%s'", s)
	}
	return time.Unix(sec, 0), nil
}

// parseISO tries common ISO 8601 date string formats.
func parseISO(s string, loc *time.Location) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date '%s'", s)
}

// formatTime applies a strftime-style format string to a time value.
// R1.2: +FORMAT processing. R1.3: strftime conversions.
// R1.4: GNU padding modifiers (%-X, %_X, %0X).
func formatTime(t time.Time, format string) string {
	var b strings.Builder
	for i := 0; i < len(format); {
		if format[i] != '%' {
			b.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			b.WriteByte('%')
			break
		}
		var pad byte
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			pad = format[i]
			i++
			if i >= len(format) {
				break
			}
		}
		b.WriteString(fmtSpec(t, format[i], pad))
		i++
	}
	return b.String()
}

// fmtSpec dispatches a single format specifier.
// R1.3: standard strftime specs. R1.4: %P and padding modifiers.
func fmtSpec(t time.Time, spec byte, pad byte) string {
	if s, ok := fmtDateSpec(t, spec, pad); ok {
		return s
	}
	if s, ok := fmtWeekSpec(t, spec, pad); ok {
		return s
	}
	if s, ok := fmtTimeSpec(t, spec, pad); ok {
		return s
	}
	if s, ok := fmtTextSpec(t, spec); ok {
		return s
	}
	return fmtCompositeSpec(t, spec, pad)
}

// fmtDateSpec handles date-related specifiers.
// R1.3: %Y, %m, %d, %j.
func fmtDateSpec(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'Y':
		return padNum(t.Year(), 4, pad, '0'), true
	case 'y':
		return padNum(t.Year()%100, 2, pad, '0'), true
	case 'C':
		return padNum(t.Year()/100, 2, pad, '0'), true
	case 'm':
		return padNum(int(t.Month()), 2, pad, '0'), true
	case 'd':
		return padNum(t.Day(), 2, pad, '0'), true
	case 'e':
		return padNum(t.Day(), 2, pad, '_'), true
	case 'j':
		return padNum(t.YearDay(), 3, pad, '0'), true
	}
	return "", false
}

// fmtWeekSpec handles week and day-of-week specifiers.
// R1.3: %u (1-7, Monday=1), %w (0-6, Sunday=0).
func fmtWeekSpec(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'u':
		d := int(t.Weekday())
		if d == 0 {
			d = 7
		}
		return fmt.Sprintf("%d", d), true
	case 'w':
		return fmt.Sprintf("%d", int(t.Weekday())), true
	case 'U':
		return padNum(sundayWeek(t), 2, pad, '0'), true
	case 'W':
		return padNum(mondayWeek(t), 2, pad, '0'), true
	case 'V':
		_, w := t.ISOWeek()
		return padNum(w, 2, pad, '0'), true
	case 'G':
		y, _ := t.ISOWeek()
		return fmt.Sprintf("%04d", y), true
	case 'g':
		y, _ := t.ISOWeek()
		return padNum(y%100, 2, pad, '0'), true
	}
	return "", false
}

// sundayWeek returns the week number with Sunday as the first day (strftime %U).
func sundayWeek(t time.Time) int {
	return (t.YearDay() + 6 - int(t.Weekday())) / 7
}

// mondayWeek returns the week number with Monday as the first day (strftime %W).
func mondayWeek(t time.Time) int {
	wday := (int(t.Weekday()) + 6) % 7
	return (t.YearDay() + 6 - wday) / 7
}

// fmtTimeSpec handles time-related specifiers.
// R1.3: %H, %M, %S, %s, %N. R1.4: %P (lowercase am/pm).
func fmtTimeSpec(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'H':
		return padNum(t.Hour(), 2, pad, '0'), true
	case 'I':
		return padNum(hour12(t), 2, pad, '0'), true
	case 'k':
		return padNum(t.Hour(), 2, pad, '_'), true
	case 'l':
		return padNum(hour12(t), 2, pad, '_'), true
	case 'M':
		return padNum(t.Minute(), 2, pad, '0'), true
	case 'S':
		return padNum(t.Second(), 2, pad, '0'), true
	case 's':
		return fmt.Sprintf("%d", t.Unix()), true
	case 'N':
		return padNum(t.Nanosecond(), 9, pad, '0'), true
	case 'p':
		return amPM(t, true), true
	case 'P':
		return amPM(t, false), true
	}
	return "", false
}

// hour12 converts 24-hour to 12-hour format.
func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}

// amPM returns the AM/PM indicator.
// R1.4: %P is GNU extension for lowercase am/pm.
func amPM(t time.Time, upper bool) string {
	if upper {
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	}
	if t.Hour() < 12 {
		return "am"
	}
	return "pm"
}

// fmtTextSpec handles text-based specifiers (names, timezone, escapes).
// R1.3: %A, %B, %Z.
func fmtTextSpec(t time.Time, spec byte) (string, bool) {
	switch spec {
	case 'A':
		return t.Weekday().String(), true
	case 'a':
		return t.Format("Mon"), true
	case 'B':
		return t.Month().String(), true
	case 'b', 'h':
		return t.Format("Jan"), true
	case 'Z':
		return t.Format("MST"), true
	case 'z':
		return t.Format("-0700"), true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	case '%':
		return "%", true
	}
	return "", false
}

// fmtCompositeSpec handles composite and shorthand specifiers.
func fmtCompositeSpec(t time.Time, spec byte, _ byte) string {
	switch spec {
	case 'D':
		return formatTime(t, "%m/%d/%y")
	case 'F':
		return formatTime(t, "%Y-%m-%d")
	case 'T':
		return formatTime(t, "%H:%M:%S")
	case 'R':
		return formatTime(t, "%H:%M")
	case 'r':
		return formatTime(t, "%I:%M:%S %p")
	case 'c':
		return formatTime(t, "%a %b %e %H:%M:%S %Y")
	case 'x':
		return formatTime(t, "%m/%d/%y")
	case 'X':
		return formatTime(t, "%H:%M:%S")
	}
	return "%" + string(spec)
}

// padNum formats a number with the specified width and padding.
// R1.4: %-X removes padding, %_X uses space, %0X uses zero.
func padNum(n, width int, modPad, defaultPad byte) string {
	raw := fmt.Sprintf("%d", n)
	p := defaultPad
	if modPad != 0 {
		p = modPad
	}
	if p == '-' || len(raw) >= width {
		return raw
	}
	ch := byte('0')
	if p == '_' {
		ch = ' '
	}
	return strings.Repeat(string(ch), width-len(raw)) + raw
}
