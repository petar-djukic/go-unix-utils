// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd004-ts R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R2.1, R2.2.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultFormat = "%b %d %H:%M:%S"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

func run() int {
	format := parseFormat(os.Args[1:])
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			ts := formatTime(time.Now(), format)
			fmt.Fprintf(os.Stdout, "%s %s", ts, line)
		}
		if err != nil {
			break
		}
	}
	return 0
}

func parseFormat(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return defaultFormat
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
	if raw, ok := compositeSpec(t, spec); ok {
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
	case 'U':
		yday := t.YearDay()
		wday := int(t.Weekday())
		return strconv.Itoa((yday + 6 - wday) / 7), 2, '0', true
	case 'W':
		yday := t.YearDay()
		wday := int(t.Weekday())
		if wday == 0 {
			wday = 7
		}
		return strconv.Itoa((yday + 6 - (wday - 1)) / 7), 2, '0', true
	case 'V':
		_, week := t.ISOWeek()
		return strconv.Itoa(week), 2, '0', true
	case 'G':
		year, _ := t.ISOWeek()
		return strconv.Itoa(year), 4, '0', true
	case 'g':
		year, _ := t.ISOWeek()
		return strconv.Itoa(year % 100), 2, '0', true
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

func compositeSpec(t time.Time, spec byte) (string, bool) {
	switch spec {
	case 'T':
		return formatTime(t, "%H:%M:%S"), true
	case 'D':
		return formatTime(t, "%m/%d/%y"), true
	case 'F':
		return formatTime(t, "%Y-%m-%d"), true
	case 'R':
		return formatTime(t, "%H:%M"), true
	case 'r':
		return formatTime(t, "%I:%M:%S %p"), true
	case 'c':
		return formatTime(t, "%a %b %e %T %Y"), true
	case 'x':
		return formatTime(t, "%m/%d/%y"), true
	case 'X':
		return formatTime(t, "%T"), true
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
