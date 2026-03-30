// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/date implements GNU date: display and format date and time.
// Implements prd060-date R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName      = "date"
	defaultFormat = "%a %b %e %H:%M:%S %Z %Y"
)

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	t, err := resolveTime(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	fmt.Println(strftime(t, cfg.format))
}

type config struct {
	format  string
	dateStr string
	utc     bool
	refFile string
}

// parseArgs parses command-line arguments for date.
// R1.1: no arguments uses default format. R1.2: +FORMAT sets custom format.
func parseArgs(args []string) (config, error) {
	cfg := config{format: defaultFormat}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-u" || arg == "--utc" || arg == "--universal":
			cfg.utc = true
		case arg == "-d" || arg == "--date":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("option requires an argument -- 'd'")
			}
			i++
			cfg.dateStr = args[i]
		case strings.HasPrefix(arg, "--date="):
			cfg.dateStr = arg[7:]
		case arg == "-r" || arg == "--reference":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("option requires an argument -- 'r'")
			}
			i++
			cfg.refFile = args[i]
		case strings.HasPrefix(arg, "--reference="):
			cfg.refFile = arg[12:]
		case strings.HasPrefix(arg, "+"):
			cfg.format = arg[1:]
		default:
			return cfg, fmt.Errorf("extra operand '%s'", arg)
		}
	}
	return cfg, nil
}

// resolveTime determines the time to display based on config.
func resolveTime(cfg config) (time.Time, error) {
	t := time.Now()
	if cfg.dateStr != "" {
		parsed, err := parseDate(cfg.dateStr)
		if err != nil {
			return t, fmt.Errorf("invalid date '%s'", cfg.dateStr)
		}
		t = parsed
	}
	if cfg.refFile != "" {
		info, err := os.Stat(cfg.refFile)
		if err != nil {
			return t, err
		}
		t = info.ModTime()
	}
	if cfg.utc {
		t = t.UTC()
	}
	return t, nil
}

var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseDate parses a date string from -d/--date.
// R2.1: displays the date described by STRING instead of now.
// R2.2: supports @EPOCH prefix. R2.3: supports ISO 8601 formats.
// R2.4: returns error for unrecognized formats.
func parseDate(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format")
}

// parseEpoch parses a Unix timestamp (integer or fractional seconds).
func parseEpoch(s string) (time.Time, error) {
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return parseFloatEpoch(s, idx)
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(secs, 0), nil
}

// parseFloatEpoch parses a fractional epoch like "123.456".
func parseFloatEpoch(s string, dotIdx int) (time.Time, error) {
	secs, err := strconv.ParseInt(s[:dotIdx], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	frac := s[dotIdx+1:]
	for len(frac) < 9 {
		frac += "0"
	}
	nsecs, err := strconv.ParseInt(frac[:9], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(secs, nsecs), nil
}

// strftime formats a time using a strftime-style format string.
// R1.2: expands %X conversion specifications.
// R1.4: handles -, _, 0 padding modifiers.
func strftime(t time.Time, format string) string {
	var buf strings.Builder
	buf.Grow(len(format) * 2)
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip %
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}
		var mod byte
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			mod = format[i]
			i++
			if i >= len(format) {
				break
			}
		}
		buf.WriteString(expandSpec(t, format[i], mod))
		i++
	}
	return buf.String()
}

// expandSpec expands a single format specifier, handling compound directives.
func expandSpec(t time.Time, spec byte, mod byte) string {
	switch spec {
	case '%':
		return "%"
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'T':
		return strftime(t, "%H:%M:%S")
	case 'D':
		return strftime(t, "%m/%d/%y")
	case 'F':
		return strftime(t, "%Y-%m-%d")
	case 'R':
		return strftime(t, "%H:%M")
	case 'r':
		return strftime(t, "%I:%M:%S %p")
	case 'c':
		return strftime(t, "%a %b %e %T %Y")
	case 'x':
		return strftime(t, "%m/%d/%y")
	case 'X':
		return strftime(t, "%H:%M:%S")
	default:
		return expandValue(t, spec, mod)
	}
}

// expandValue expands a simple directive with padding applied.
func expandValue(t time.Time, spec byte, mod byte) string {
	val, defPad, width := rawValue(t, spec)
	if width == 0 || mod == '-' {
		return val
	}
	pad := defPad
	switch mod {
	case '_':
		pad = ' '
	case '0':
		pad = '0'
	}
	for len(val) < width {
		val = string(pad) + val
	}
	return val
}

// rawValue dispatches to category-specific value functions.
func rawValue(t time.Time, spec byte) (string, byte, int) {
	if v, p, w, ok := rawDateValue(t, spec); ok {
		return v, p, w
	}
	if v, p, w, ok := rawTimeValue(t, spec); ok {
		return v, p, w
	}
	return rawWeekValue(t, spec)
}

// rawDateValue returns values for date-related directives.
// R1.3: %Y, %m, %d, %A, %B, %j, %u, %w.
func rawDateValue(t time.Time, spec byte) (string, byte, int, bool) {
	switch spec {
	case 'Y':
		return strconv.Itoa(t.Year()), '0', 4, true
	case 'y':
		return strconv.Itoa(t.Year() % 100), '0', 2, true
	case 'C':
		return strconv.Itoa(t.Year() / 100), '0', 2, true
	case 'm':
		return strconv.Itoa(int(t.Month())), '0', 2, true
	case 'd':
		return strconv.Itoa(t.Day()), '0', 2, true
	case 'e':
		return strconv.Itoa(t.Day()), ' ', 2, true
	case 'j':
		return strconv.Itoa(t.YearDay()), '0', 3, true
	case 'A':
		return t.Weekday().String(), 0, 0, true
	case 'a':
		return t.Weekday().String()[:3], 0, 0, true
	case 'B':
		return t.Month().String(), 0, 0, true
	case 'b', 'h':
		return t.Month().String()[:3], 0, 0, true
	case 'u':
		d := int(t.Weekday())
		if d == 0 {
			d = 7
		}
		return strconv.Itoa(d), 0, 0, true
	case 'w':
		return strconv.Itoa(int(t.Weekday())), 0, 0, true
	}
	return "", 0, 0, false
}

// rawTimeValue returns values for time-related directives.
// R1.3: %H, %M, %S, %s, %N, %Z. R1.4: %P.
func rawTimeValue(t time.Time, spec byte) (string, byte, int, bool) {
	switch spec {
	case 'H':
		return strconv.Itoa(t.Hour()), '0', 2, true
	case 'k':
		return strconv.Itoa(t.Hour()), ' ', 2, true
	case 'I':
		return strconv.Itoa(hour12(t)), '0', 2, true
	case 'l':
		return strconv.Itoa(hour12(t)), ' ', 2, true
	case 'M':
		return strconv.Itoa(t.Minute()), '0', 2, true
	case 'S':
		return strconv.Itoa(t.Second()), '0', 2, true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), 0, 0, true
	case 'N':
		return strconv.Itoa(t.Nanosecond()), '0', 9, true
	case 'p':
		if t.Hour() < 12 {
			return "AM", 0, 0, true
		}
		return "PM", 0, 0, true
	case 'P':
		if t.Hour() < 12 {
			return "am", 0, 0, true
		}
		return "pm", 0, 0, true
	case 'Z':
		name, _ := t.Zone()
		return name, 0, 0, true
	case 'z':
		return t.Format("-0700"), 0, 0, true
	}
	return "", 0, 0, false
}

// rawWeekValue returns values for week-number and ISO-week directives.
func rawWeekValue(t time.Time, spec byte) (string, byte, int) {
	switch spec {
	case 'U':
		wday := int(t.Weekday())
		return strconv.Itoa((t.YearDay() + 6 - wday) / 7), '0', 2
	case 'W':
		mday := (int(t.Weekday()) + 6) % 7
		return strconv.Itoa((t.YearDay() + 6 - mday) / 7), '0', 2
	case 'V':
		_, w := t.ISOWeek()
		return strconv.Itoa(w), '0', 2
	case 'G':
		y, _ := t.ISOWeek()
		return strconv.Itoa(y), '0', 4
	case 'g':
		y, _ := t.ISOWeek()
		return strconv.Itoa(y % 100), '0', 2
	default:
		return "%" + string(spec), 0, 0
	}
}

// hour12 converts a 24-hour time to 12-hour format (1-12).
func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}
