// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd060-date R1.1–R1.4: default output, format strings,
// strftime conversion specifications, and GNU padding extensions.
// Implements prd060-date R2.1–R2.4: --date/-d flag input parsing.
// Implements prd060-date R3.1–R3.4: UTC mode, reference file time,
// error handling for missing files, stdout-only output.
// Implements prd060-date R4.1–R4.2: exit 0 on success, exit 1 on
// invalid date or missing reference file.
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

// dateLayouts lists time formats to try when parsing absolute date strings.
// R2.3: ISO 8601 and common GNU-recognized formats. Tried in order;
// more specific layouts come first to avoid ambiguous partial matches.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 MST",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05",
	"02 Jan 2006",
	"Jan 02, 2006 15:04:05",
	"Jan 02, 2006",
	"January 02, 2006 15:04:05",
	"January 02, 2006",
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

// dateOpts holds parsed command-line options for the date utility.
type dateOpts struct {
	format  string
	dateStr string
	dateSet bool
	refFile string
	refSet  bool
	utc     bool
}

// run parses arguments, formats the date, and prints the result.
// R1.1: no arguments uses the default format.
// R1.2: +FORMAT uses the specified format string.
// R2.1: -d/--date uses the specified date string.
// R3.1: -u/--utc/--universal uses UTC.
// R3.2: -r/--reference uses file modification time.
func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	t, err := resolveTime(now, opts, stderr)
	if err != nil {
		return 1
	}
	// R3.1: convert to UTC if requested.
	if opts.utc {
		t = t.UTC()
	}
	fmt.Fprintln(stdout, strftime(t, opts.format))
	return 0
}

// resolveTime returns the time to display based on options.
// R2.1: -d STRING overrides the current time.
// R3.2: -r FILE uses the file's modification time.
func resolveTime(now time.Time, opts dateOpts, stderr io.Writer) (time.Time, error) {
	if opts.refSet {
		return resolveRefFile(opts.refFile, stderr)
	}
	if opts.dateSet {
		return resolveDateStr(now, opts.dateStr, stderr)
	}
	return now, nil
}

// resolveRefFile returns the modification time of the referenced file.
// R3.2, R3.3: prints error and returns error if file does not exist.
func resolveRefFile(path string, stderr io.Writer) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %s\n", progName, path, stripPathError(err))
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// stripPathError extracts the underlying message from a *os.PathError,
// removing the duplicated path and operation prefix.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// resolveDateStr parses a date string or returns midnight for empty strings.
// R2.1: -d STRING overrides the current time.
func resolveDateStr(now time.Time, dateStr string, stderr io.Writer) (time.Time, error) {
	if dateStr == "" {
		// GNU date treats -d "" as midnight of the current day.
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	}
	t, err := parseDate(dateStr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid date '%s'\n", progName, dateStr)
		return time.Time{}, err
	}
	return t, nil
}

// parseArgs extracts options from args.
// Returns (opts, exitCode); exitCode -1 means continue.
func parseArgs(args []string, stdout, stderr io.Writer) (dateOpts, int) {
	var opts dateOpts
	formatSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if consumed, ds, code := handleDateFlag(args, i, stderr); code == 0 {
			opts.dateStr = ds
			opts.dateSet = true
			i += consumed
			continue
		} else if code > 0 {
			return dateOpts{}, code
		}
		if consumed, rf, code := handleRefFlag(args, i, stderr); code == 0 {
			opts.refFile = rf
			opts.refSet = true
			i += consumed
			continue
		} else if code > 0 {
			return dateOpts{}, code
		}
		if handleUTCFlag(arg) {
			opts.utc = true
			continue
		}
		code := handleArg(arg, &opts.format, &formatSet, stdout, stderr)
		if code >= 0 {
			return dateOpts{}, code
		}
	}
	if !formatSet {
		opts.format = defaultFormat
	}
	return opts, -1
}

// handleUTCFlag returns true if arg is -u, --utc, or --universal.
// R3.1: UTC display mode.
func handleUTCFlag(arg string) bool {
	return arg == "-u" || arg == "--utc" || arg == "--universal"
}

// handleRefFlag checks if args[i] is a -r/--reference flag and extracts
// the file path. Returns (extra consumed, value, status).
// Status: 0 = matched, >0 = matched with error exit code, -1 = not matched.
// R3.2: supports -r FILE, -rFILE, --reference=FILE, --reference FILE.
func handleRefFlag(args []string, i int, stderr io.Writer) (int, string, int) {
	arg := args[i]
	if strings.HasPrefix(arg, "--reference=") {
		return 0, arg[len("--reference="):], 0
	}
	if arg == "-r" || arg == "--reference" {
		if i+1 >= len(args) {
			fmt.Fprintf(stderr, "%s: option requires an argument -- 'r'\n", progName)
			printTryHelp(stderr)
			return 0, "", 1
		}
		return 1, args[i+1], 0
	}
	if len(arg) > 2 && arg[:2] == "-r" {
		return 0, arg[2:], 0
	}
	return 0, "", -1
}

// handleDateFlag checks if args[i] is a -d/--date flag and extracts
// the date string. Returns (extra consumed, value, status).
// Status: 0 = matched, >0 = matched with error exit code, -1 = not matched.
// R2.1: supports -d STRING, -dSTRING, --date=STRING, --date STRING.
func handleDateFlag(args []string, i int, stderr io.Writer) (int, string, int) {
	arg := args[i]
	if strings.HasPrefix(arg, "--date=") {
		return 0, arg[7:], 0
	}
	if arg == "-d" || arg == "--date" {
		if i+1 >= len(args) {
			fmt.Fprintf(stderr, "%s: option requires an argument -- 'd'\n", progName)
			printTryHelp(stderr)
			return 0, "", 1
		}
		return 1, args[i+1], 0
	}
	if len(arg) > 2 && arg[:2] == "-d" {
		return 0, arg[2:], 0
	}
	return 0, "", -1
}

// parseDate parses a date string into a time.Time.
// R2.2: handles @EPOCH prefix. R2.3: tries ISO 8601 layouts.
func parseDate(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	return parseAbsoluteDate(s)
}

// parseEpoch parses a Unix epoch timestamp string.
// R2.2: @EPOCH format (e.g., "@1234567890").
func parseEpoch(s string) (time.Time, error) {
	epoch, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid epoch: %w", err)
	}
	return time.Unix(epoch, 0), nil
}

// parseAbsoluteDate tries known date formats in order.
// R2.3: ISO 8601 and common GNU-recognized formats.
func parseAbsoluteDate(s string) (time.Time, error) {
	for _, layout := range dateLayouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date")
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
	fmt.Fprintln(w, "  -d, --date=STRING    display time described by STRING")
	fmt.Fprintln(w, "  -r, --reference=FILE display the last modification time of FILE")
	fmt.Fprintln(w, "  -u, --utc, --universal  print or set UTC")
	fmt.Fprintln(w, "      --help           display this help and exit")
	fmt.Fprintln(w, "      --version        output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
