// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd060-date R1.1–R1.4: default output, format strings,
// strftime conversion specifications, and GNU padding extensions.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "date"

// defaultFormat is the GNU date default in the C locale.
// R1.1: used when no +FORMAT argument is given.
const defaultFormat = "%a %b %e %H:%M:%S %Z %Y"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

// run parses arguments, formats the date, and prints the result.
// R1.1: no arguments uses the default format.
// R1.2: +FORMAT uses the specified format string.
func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	format, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	fmt.Fprintln(stdout, strftime(now, format))
	return 0
}

// parseArgs extracts the format string from arguments.
// Returns (format, exitCode); exitCode -1 means continue.
func parseArgs(args []string, stdout, stderr io.Writer) (string, int) {
	var format string
	formatSet := false
	for _, arg := range args {
		code := handleArg(arg, &format, &formatSet, stdout, stderr)
		if code >= 0 {
			return "", code
		}
	}
	if !formatSet {
		return defaultFormat, -1
	}
	return format, -1
}

// handleArg processes a single argument. Returns exit code >= 0
// for terminal conditions, -1 to continue.
func handleArg(arg string, format *string, formatSet *bool, stdout, stderr io.Writer) int {
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 0
	case arg == "--version":
		printVersion(stdout)
		return 0
	case len(arg) > 0 && arg[0] == '+':
		if *formatSet {
			return reportExtra(arg, stderr)
		}
		*format = arg[1:]
		*formatSet = true
		return -1
	case len(arg) > 0 && arg[0] == '-':
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	default:
		return reportExtra(arg, stderr)
	}
}

// reportExtra prints an "extra operand" error and returns exit code 1.
func reportExtra(arg string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, arg)
	printTryHelp(stderr)
	return 1
}

// strftime formats a time using a strftime-style format string.
// R1.2: processes % directives. R1.4: handles padding modifiers.
func strftime(t time.Time, format string) string {
	var buf strings.Builder
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
		// R1.4: check for padding modifier (-, _, 0).
		var pad byte
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			pad = format[i]
			i++
			if i >= len(format) {
				buf.WriteByte('%')
				buf.WriteByte(pad)
				break
			}
		}
		buf.WriteString(formatDirective(t, format[i], pad))
		i++
	}
	return buf.String()
}

// formatDirective dispatches a single format specifier to the
// appropriate formatting function. R1.3, R1.4.
func formatDirective(t time.Time, spec byte, pad byte) string {
	if s, ok := formatNumericDate(t, spec, pad); ok {
		return s
	}
	if s, ok := formatNumericTime(t, spec, pad); ok {
		return s
	}
	if s, ok := formatNumericWeek(t, spec, pad); ok {
		return s
	}
	if s, ok := formatText(t, spec); ok {
		return s
	}
	if s, ok := formatComposite(t, spec); ok {
		return s
	}
	if s, ok := formatLiteral(spec); ok {
		return s
	}
	return "%" + string(spec)
}

// formatNumericDate handles date-related numeric specifiers.
// R1.3: %Y, %m, %d. R1.4: padding modifiers applied via numericStr.
func formatNumericDate(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'Y':
		return numericStr(t.Year(), 4, '0', pad), true
	case 'm':
		return numericStr(int(t.Month()), 2, '0', pad), true
	case 'd':
		return numericStr(t.Day(), 2, '0', pad), true
	case 'C':
		return numericStr(t.Year()/100, 2, '0', pad), true
	case 'y':
		return numericStr(t.Year()%100, 2, '0', pad), true
	case 'e':
		return numericStr(t.Day(), 2, ' ', pad), true
	case 'j':
		return numericStr(t.YearDay(), 3, '0', pad), true
	case 'G':
		y, _ := t.ISOWeek()
		return numericStr(y, 4, '0', pad), true
	case 'g':
		y, _ := t.ISOWeek()
		return numericStr(y%100, 2, '0', pad), true
	}
	return "", false
}

// formatNumericTime handles time-related numeric specifiers.
// R1.3: %H, %M, %S, %s, %N.
func formatNumericTime(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'H':
		return numericStr(t.Hour(), 2, '0', pad), true
	case 'M':
		return numericStr(t.Minute(), 2, '0', pad), true
	case 'S':
		return numericStr(t.Second(), 2, '0', pad), true
	case 'I':
		return numericStr(hour12(t.Hour()), 2, '0', pad), true
	case 'k':
		return numericStr(t.Hour(), 2, ' ', pad), true
	case 'l':
		return numericStr(hour12(t.Hour()), 2, ' ', pad), true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), true
	case 'N':
		return numericStr(t.Nanosecond(), 9, '0', pad), true
	}
	return "", false
}

// formatNumericWeek handles week and day-of-week numeric specifiers.
// R1.3: %u (1-7), %w (0-6).
func formatNumericWeek(t time.Time, spec byte, pad byte) (string, bool) {
	switch spec {
	case 'u':
		return numericStr(dayOfWeekMon(t), 1, '0', pad), true
	case 'w':
		return numericStr(int(t.Weekday()), 1, '0', pad), true
	case 'U':
		return numericStr(weekNumSunday(t), 2, '0', pad), true
	case 'W':
		return numericStr(weekNumMonday(t), 2, '0', pad), true
	case 'V':
		_, w := t.ISOWeek()
		return numericStr(w, 2, '0', pad), true
	}
	return "", false
}

// formatText handles non-numeric text specifiers.
// R1.3: %A, %B, %Z. R1.4: %P.
func formatText(t time.Time, spec byte) (string, bool) {
	switch spec {
	case 'A':
		return t.Weekday().String(), true
	case 'a':
		return t.Weekday().String()[:3], true
	case 'B':
		return t.Month().String(), true
	case 'b', 'h':
		return t.Month().String()[:3], true
	case 'Z':
		name, _ := t.Zone()
		return name, true
	case 'z':
		return tzOffset(t), true
	case 'p':
		return ampmUpper(t.Hour()), true
	case 'P':
		return ampmLower(t.Hour()), true
	}
	return "", false
}

// formatComposite handles composite format specifiers that expand
// to other format strings.
func formatComposite(t time.Time, spec byte) (string, bool) {
	switch spec {
	case 'D':
		return strftime(t, "%m/%d/%y"), true
	case 'F':
		return strftime(t, "%Y-%m-%d"), true
	case 'T':
		return strftime(t, "%H:%M:%S"), true
	case 'R':
		return strftime(t, "%H:%M"), true
	case 'r':
		return strftime(t, "%I:%M:%S %p"), true
	case 'c':
		return strftime(t, "%a %b %e %H:%M:%S %Y"), true
	case 'x':
		return strftime(t, "%m/%d/%y"), true
	case 'X':
		return strftime(t, "%H:%M:%S"), true
	}
	return "", false
}

// formatLiteral handles literal escape sequences.
func formatLiteral(spec byte) (string, bool) {
	switch spec {
	case '%':
		return "%", true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	}
	return "", false
}

// numericStr formats an integer with the given width and padding.
// R1.4: padMod overrides the default padding character.
func numericStr(val, width int, defPad, padMod byte) string {
	s := strconv.Itoa(val)
	if padMod == '-' {
		return s
	}
	if len(s) >= width {
		return s
	}
	p := defPad
	if padMod == '_' {
		p = ' '
	} else if padMod == '0' {
		p = '0'
	}
	return strings.Repeat(string(p), width-len(s)) + s
}

// hour12 converts a 24-hour value to 12-hour (1-12).
func hour12(h int) int {
	h = h % 12
	if h == 0 {
		return 12
	}
	return h
}

// dayOfWeekMon returns the day of week with Monday=1, Sunday=7.
// R1.3: %u specifier.
func dayOfWeekMon(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

// weekNumSunday returns the week number with Sunday as the first day (0-53).
func weekNumSunday(t time.Time) int {
	return (t.YearDay() + 6 - int(t.Weekday())) / 7
}

// weekNumMonday returns the week number with Monday as the first day (0-53).
func weekNumMonday(t time.Time) int {
	wdayMon := (int(t.Weekday()) + 6) % 7
	return (t.YearDay() + 6 - wdayMon) / 7
}

// tzOffset returns the timezone offset in +HHMM format.
func tzOffset(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("%s%02d%02d", sign, offset/3600, (offset%3600)/60)
}

// ampmUpper returns "AM" or "PM" based on the hour.
func ampmUpper(hour int) string {
	if hour >= 12 {
		return "PM"
	}
	return "AM"
}

// ampmLower returns "am" or "pm" based on the hour. R1.4: %P.
func ampmLower(hour int) string {
	if hour >= 12 {
		return "pm"
	}
	return "am"
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [+FORMAT]\n", progName)
	fmt.Fprintln(w, "Display the current time in the given FORMAT.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --help        display this help and exit")
	fmt.Fprintln(w, "      --version     output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
