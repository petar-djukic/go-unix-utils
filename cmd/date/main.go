// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd060-date R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultFormat = "%a %b %e %H:%M:%S %Z %Y"

type config struct {
	format    string
	hasFormat bool
	dateStr   string
	utc       bool
	refFile   string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "date: %s\n", err)
		os.Exit(1)
	}
	t, err := resolveTime(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "date: %s\n", err)
		os.Exit(1)
	}
	fmt.Println(formatOutput(t, cfg.format, cfg.hasFormat))
}

func parseArgs(args []string) (config, error) {
	var cfg config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "+"):
			cfg.format = arg[1:]
			cfg.hasFormat = true
		case arg == "-d" || arg == "--date":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option requires an argument -- 'd'")
			}
			cfg.dateStr = args[i]
		case strings.HasPrefix(arg, "--date="):
			cfg.dateStr = strings.TrimPrefix(arg, "--date=")
		case arg == "-u" || arg == "--utc" || arg == "--universal":
			cfg.utc = true
		case arg == "-r" || arg == "--reference":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option requires an argument -- 'r'")
			}
			cfg.refFile = args[i]
		case strings.HasPrefix(arg, "--reference="):
			cfg.refFile = strings.TrimPrefix(arg, "--reference=")
		}
	}
	return cfg, nil
}

func resolveTime(cfg config) (time.Time, error) {
	t := time.Now()
	if cfg.refFile != "" {
		info, err := os.Stat(cfg.refFile)
		if err != nil {
			return t, fmt.Errorf("%s: No such file or directory", cfg.refFile)
		}
		t = info.ModTime()
	}
	if cfg.dateStr != "" {
		parsed, err := parseDateString(cfg.dateStr)
		if err != nil {
			return t, err
		}
		t = parsed
	}
	if cfg.utc {
		t = t.UTC()
	}
	return t, nil
}

var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parseDateString(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		sec, err := strconv.ParseInt(s[1:], 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date '%s'", s)
		}
		return time.Unix(sec, 0), nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date '%s'", s)
}

func formatOutput(t time.Time, format string, hasFormat bool) string {
	if !hasFormat {
		return formatTime(t, defaultFormat)
	}
	return formatTime(t, format)
}

func formatTime(t time.Time, format string) string {
	var b strings.Builder
	b.Grow(len(format) * 2)
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			b.WriteByte(format[i])
			i++
			continue
		}
		i++
		if i >= len(format) {
			b.WriteByte('%')
			break
		}
		var mod byte
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			mod = format[i]
			i++
			if i >= len(format) {
				b.WriteByte('%')
				b.WriteByte(mod)
				break
			}
		}
		b.WriteString(formatSpec(t, format[i], mod))
		i++
	}
	return b.String()
}

func formatSpec(t time.Time, spec, mod byte) string {
	raw, width, pad := specValue(t, spec)
	return padValue(raw, width, pad, mod)
}

func specValue(t time.Time, spec byte) (string, int, byte) {
	if raw, w, p, ok := numSpec(t, spec); ok {
		return raw, w, p
	}
	if raw, ok := textSpec(t, spec); ok {
		return raw, 0, 0
	}
	return "%" + string(spec), 0, 0
}

func numSpec(t time.Time, spec byte) (string, int, byte, bool) {
	switch spec {
	case 'Y':
		return strconv.Itoa(t.Year()), 4, '0', true
	case 'C':
		return strconv.Itoa(t.Year() / 100), 2, '0', true
	case 'y':
		return strconv.Itoa(t.Year() % 100), 2, '0', true
	case 'm':
		return strconv.Itoa(int(t.Month())), 2, '0', true
	case 'd':
		return strconv.Itoa(t.Day()), 2, '0', true
	case 'e':
		return strconv.Itoa(t.Day()), 2, ' ', true
	case 'H':
		return strconv.Itoa(t.Hour()), 2, '0', true
	case 'k':
		return strconv.Itoa(t.Hour()), 2, ' ', true
	case 'I':
		return strconv.Itoa(hour12(t)), 2, '0', true
	case 'l':
		return strconv.Itoa(hour12(t)), 2, ' ', true
	case 'M':
		return strconv.Itoa(t.Minute()), 2, '0', true
	case 'S':
		return strconv.Itoa(t.Second()), 2, '0', true
	case 'j':
		return strconv.Itoa(t.YearDay()), 3, '0', true
	case 'N':
		return strconv.Itoa(t.Nanosecond()), 9, '0', true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), 0, 0, true
	case 'u':
		d := int(t.Weekday())
		if d == 0 {
			d = 7
		}
		return strconv.Itoa(d), 0, 0, true
	case 'w':
		return strconv.Itoa(int(t.Weekday())), 0, 0, true
	default:
		return "", 0, 0, false
	}
}

func textSpec(t time.Time, spec byte) (string, bool) {
	switch spec {
	case 'a':
		return t.Format("Mon"), true
	case 'A':
		return t.Format("Monday"), true
	case 'b', 'h':
		return t.Format("Jan"), true
	case 'B':
		return t.Format("January"), true
	case 'Z':
		return t.Format("MST"), true
	case 'z':
		return t.Format("-0700"), true
	case 'p':
		return t.Format("PM"), true
	case 'P':
		return strings.ToLower(t.Format("PM")), true
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

func padValue(raw string, width int, pad byte, mod byte) string {
	if width == 0 || mod == '-' {
		return raw
	}
	switch mod {
	case '_':
		pad = ' '
	case '0':
		pad = '0'
	}
	if len(raw) >= width {
		return raw
	}
	return strings.Repeat(string(pad), width-len(raw)) + raw
}

func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}
