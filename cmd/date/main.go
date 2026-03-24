// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd060-date: Display and Format Date and Time.
// Covers R1.1-R1.4 (entry point, default output, format strings, directives),
// R3.1 (-u/--utc/--universal UTC mode), and -R/--rfc-email RFC 5322 format.
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
	progName       = "date"
	defaultFormat  = "%a %b %e %H:%M:%S %Z %Y"
	rfcEmailFormat = "%a, %d %b %Y %H:%M:%S %z"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// padType controls padding behavior for numeric format directives.
// R1.4: padding modifiers %-d (no padding), %_d (space), %0d (zero).
type padType int

const (
	padDefault padType = iota
	padNone
	padSpace
	padZero
)

// config holds parsed command-line options.
type config struct {
	utc         bool
	rfcEmail    bool
	format      string
	showHelp    bool
	showVersion bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints the formatted date. Returns exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 1
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	return printDate(cfg)
}

// printDate formats and prints the date/time per the configuration.
// R1.1: default output with current time. R3.1: UTC mode.
func printDate(cfg config) int {
	now := time.Now()
	if cfg.utc {
		now = now.UTC()
	}
	format := selectFormat(cfg)
	result := formatTime(now, format)
	if _, err := fmt.Fprintln(os.Stdout, result); err != nil {
		return 1
	}
	return 0
}

// selectFormat returns the format string based on config flags.
// R1.2: +FORMAT argument. -R/--rfc-email shortcut.
func selectFormat(cfg config) string {
	if cfg.format != "" {
		return cfg.format
	}
	if cfg.rfcEmail {
		return rfcEmailFormat
	}
	return defaultFormat
}

// parseArgs processes all command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	for i := range len(args) {
		if args[i] == "--" {
			return parseOperands(&cfg, args[i+1:])
		}
		if err := parseArg(&cfg, args[i]); err != nil {
			return cfg, err
		}
		if cfg.showHelp || cfg.showVersion {
			return cfg, nil
		}
	}
	return cfg, nil
}

// parseOperands handles arguments after --.
func parseOperands(cfg *config, args []string) (config, error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") {
			cfg.format = arg[1:]
		} else {
			return *cfg, fmt.Errorf("extra operand '%s'", arg)
		}
	}
	return *cfg, nil
}

// parseArg processes a single command-line argument.
func parseArg(cfg *config, arg string) error {
	switch {
	case arg == "--help":
		cfg.showHelp = true
	case arg == "--version":
		cfg.showVersion = true
	case arg == "--utc" || arg == "--universal":
		cfg.utc = true
	case arg == "--rfc-email":
		cfg.rfcEmail = true
	case strings.HasPrefix(arg, "--"):
		return fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(cfg, arg[1:])
	case strings.HasPrefix(arg, "+"):
		cfg.format = arg[1:]
	default:
		return fmt.Errorf("extra operand '%s'", arg)
	}
	return nil
}

// parseShortFlags processes combined short flags (e.g., -uR).
func parseShortFlags(cfg *config, flags string) error {
	for _, ch := range flags {
		switch ch {
		case 'u':
			cfg.utc = true
		case 'R':
			cfg.rfcEmail = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// formatTime translates a strftime-style format string using the given time.
// R1.2: custom format. R1.3: standard directives. R1.4: GNU extensions.
func formatTime(t time.Time, format string) string {
	var buf strings.Builder
	for i := 0; i < len(format); {
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
		pad, adv := parsePadModifier(format, i)
		i += adv
		if i >= len(format) {
			break
		}
		buf.WriteString(expandDirective(t, format[i], pad))
		i++
	}
	return buf.String()
}

// parsePadModifier checks for a padding modifier at position i in format.
// R1.4: %-X (no padding), %_X (space), %0X (zero).
func parsePadModifier(format string, i int) (padType, int) {
	switch format[i] {
	case '-':
		return padNone, 1
	case '_':
		return padSpace, 1
	case '0':
		return padZero, 1
	}
	return padDefault, 0
}

// expandDirective returns the formatted value for a single format directive.
func expandDirective(t time.Time, ch byte, pad padType) string {
	if v, ok := expandLiteral(ch); ok {
		return v
	}
	if v, ok := expandTextDirective(t, ch); ok {
		return v
	}
	if v, ok := expandDateDirective(t, ch, pad); ok {
		return v
	}
	if v, ok := expandTimeDirective(t, ch, pad); ok {
		return v
	}
	if v, ok := expandCompound(t, ch); ok {
		return v
	}
	return "%" + string(ch)
}

// expandLiteral handles literal escape directives (%%, %n, %t).
func expandLiteral(ch byte) (string, bool) {
	switch ch {
	case '%':
		return "%", true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	}
	return "", false
}

// expandTextDirective handles directives that produce text strings.
// R1.3: %A (weekday name), %B (month name), %Z (timezone), %z (offset).
func expandTextDirective(t time.Time, ch byte) (string, bool) {
	switch ch {
	case 'a':
		return t.Format("Mon"), true
	case 'A':
		return t.Format("Monday"), true
	case 'b', 'h':
		return t.Format("Jan"), true
	case 'B':
		return t.Format("January"), true
	case 'p':
		return t.Format("PM"), true
	case 'P':
		return strings.ToLower(t.Format("PM")), true
	case 'Z':
		return t.Format("MST"), true
	case 'z':
		return t.Format("-0700"), true
	}
	return "", false
}

// expandDateDirective handles date-related numeric directives.
// R1.3: %Y, %m, %d, %j, %u, %w. R1.4: padding modifiers apply.
func expandDateDirective(t time.Time, ch byte, pad padType) (string, bool) {
	switch ch {
	case 'Y':
		return padNum(t.Year(), 4, padZero, pad), true
	case 'y':
		return padNum(t.Year()%100, 2, padZero, pad), true
	case 'C':
		return padNum(t.Year()/100, 2, padZero, pad), true
	case 'm':
		return padNum(int(t.Month()), 2, padZero, pad), true
	case 'd':
		return padNum(t.Day(), 2, padZero, pad), true
	case 'e':
		return padNum(t.Day(), 2, padSpace, pad), true
	case 'j':
		return padNum(t.YearDay(), 3, padZero, pad), true
	case 'u':
		return padNum(isoWeekday(t), 0, padNone, pad), true
	case 'w':
		return padNum(int(t.Weekday()), 0, padNone, pad), true
	case 'U':
		return padNum(sundayWeekNum(t), 2, padZero, pad), true
	case 'W':
		return padNum(mondayWeekNum(t), 2, padZero, pad), true
	case 'V':
		_, w := t.ISOWeek()
		return padNum(w, 2, padZero, pad), true
	case 'G':
		y, _ := t.ISOWeek()
		return padNum(y, 4, padZero, pad), true
	case 'g':
		y, _ := t.ISOWeek()
		return padNum(y%100, 2, padZero, pad), true
	}
	return "", false
}

// expandTimeDirective handles time-related numeric directives.
// R1.3: %H, %M, %S, %s, %N. R1.4: %P (lowercase am/pm).
func expandTimeDirective(t time.Time, ch byte, pad padType) (string, bool) {
	switch ch {
	case 'H':
		return padNum(t.Hour(), 2, padZero, pad), true
	case 'k':
		return padNum(t.Hour(), 2, padSpace, pad), true
	case 'I':
		return padNum(hour12(t), 2, padZero, pad), true
	case 'l':
		return padNum(hour12(t), 2, padSpace, pad), true
	case 'M':
		return padNum(t.Minute(), 2, padZero, pad), true
	case 'S':
		return padNum(t.Second(), 2, padZero, pad), true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), true
	case 'N':
		return fmt.Sprintf("%09d", t.Nanosecond()), true
	}
	return "", false
}

// expandCompound handles directives that expand to other format strings.
func expandCompound(t time.Time, ch byte) (string, bool) {
	switch ch {
	case 'D':
		return formatTime(t, "%m/%d/%y"), true
	case 'F':
		return formatTime(t, "%Y-%m-%d"), true
	case 'T':
		return formatTime(t, "%H:%M:%S"), true
	case 'R':
		return formatTime(t, "%H:%M"), true
	case 'r':
		return formatTime(t, "%I:%M:%S %p"), true
	case 'c':
		return formatTime(t, "%a %b %e %H:%M:%S %Y"), true
	case 'x':
		return formatTime(t, "%m/%d/%y"), true
	case 'X':
		return formatTime(t, "%H:%M:%S"), true
	}
	return "", false
}

// hour12 converts 24-hour time to 12-hour time (1-12).
func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}

// isoWeekday returns the ISO weekday number (1=Monday, 7=Sunday).
func isoWeekday(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

// sundayWeekNum returns the week number with Sunday as the first day (0-53).
func sundayWeekNum(t time.Time) int {
	yday := t.YearDay()
	wday := int(t.Weekday())
	return (yday + 6 - wday) / 7
}

// mondayWeekNum returns the week number with Monday as the first day (0-53).
func mondayWeekNum(t time.Time) int {
	yday := t.YearDay()
	wday := int(t.Weekday())
	if wday == 0 {
		wday = 7
	}
	return (yday + 7 - wday) / 7
}

// padNum formats a number with the specified width and padding.
func padNum(val, width int, defaultPad, override padType) string {
	p := defaultPad
	if override != padDefault {
		p = override
	}
	s := strconv.Itoa(val)
	if width == 0 || p == padNone || len(s) >= width {
		return s
	}
	gap := width - len(s)
	switch p {
	case padSpace:
		return strings.Repeat(" ", gap) + s
	default:
		return strings.Repeat("0", gap) + s
	}
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

const helpText = `Usage: date [OPTION]... [+FORMAT]
Display the current time in the given FORMAT.

  -R, --rfc-email          output date and time in RFC 5322 format
  -u, --utc, --universal   print or set Coordinated Universal Time (UTC)
      --help               display this help and exit
      --version            output version information and exit

FORMAT controls the output. Interpreted sequences are:
  %%   a literal %              %n   a newline
  %a   abbreviated weekday      %A   full weekday name
  %b   abbreviated month        %B   full month name
  %d   day of month (01..31)    %e   day of month, space padded
  %m   month (01..12)           %Y   year
  %H   hour (00..23)            %I   hour (01..12)
  %M   minute (00..59)          %S   second (00..60)
  %p   AM or PM                 %P   am or pm
  %s   seconds since epoch      %N   nanoseconds
  %Z   timezone abbreviation    %z   +hhmm numeric timezone
  %F   full date (%Y-%m-%d)     %T   time (%H:%M:%S)
  %D   date (%m/%d/%y)          %R   24-hour time (%H:%M)
  %j   day of year (001..366)   %u   day of week (1..7, Mon=1)
  %w   day of week (0..6, Sun=0)
`

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := os.Stdout.WriteString(helpText)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
