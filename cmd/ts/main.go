// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ts: prepend timestamps to stdin lines.
// Implements srd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3,
// R6.1-R6.2, R7.1-R7.3, R8.1-R8.2, R9.1-R9.2, R10.1-R10.3.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
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
	format          string
	hasCustomFormat bool // true when user supplied a format argument
	incremental     bool // -i mode (R3.1)
	elapsed         bool // -s mode (R4.1)
	monotonic       bool // -m mode (R5.1)
	relative        bool // -r mode (R6.1)
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
// R6.1: -r enables relative-time parsing mode.
// R7.2: unrecognized flags produce an error.
// R10.3: -r is mutually exclusive with -i and -s.
func parseArgs(args []string) (config, error) {
	var cfg config
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
		// R6.1: -r enables relative-time parsing mode.
		if arg == "-r" {
			cfg.relative = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return config{}, fmt.Errorf(
				"%s: unrecognized option '%s'", progName, arg)
		}
		cfg.format = arg
		cfg.hasCustomFormat = true
	}
	// R10.3: -r is mutually exclusive with -i and -s. The reference binary
	// silently ignores -i/-s when -r is present, so we match that behavior.
	if cfg.relative {
		cfg.incremental = false
		cfg.elapsed = false
	}
	// R3.4: when both -i and -s are given, -s takes precedence
	// (matches reference binary behavior).
	if cfg.elapsed {
		cfg.incremental = false
	}
	// R7.3: In the Go implementation, timestamp parsing for -r mode is always
	// compiled in (no external dependency like Perl's Date::Parse). The error
	// condition "parsing dependency unavailable" cannot arise.
	if !cfg.hasCustomFormat {
		cfg.format = selectDefaultFormat(cfg.incremental || cfg.elapsed)
	}
	return cfg, nil
}

// selectDefaultFormat determines the default format string based on mode.
// R3.2, R4.2: delta modes default to "%H:%M:%S".
func selectDefaultFormat(delta bool) string {
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
// R1.6, R7.1: exit 0 on EOF.
// R3.1: -i shows elapsed time since previous line.
// R3.2: -i uses TZ=GMT for elapsed formatting.
// R4.1: -s shows elapsed time since start.
// R4.2: -s uses TZ=GMT for elapsed formatting.
// R6.1: -r scans lines for timestamps and replaces with relative age.
// R8.1: wall-clock timestamps respect TZ via time.Now() which uses time.Local.
// R8.2: -i/-s modes use TZ=GMT internally via time.LoadLocation("GMT").
// R10.1: -r with format reformats matched timestamps via strftime.
// R10.2: lines with no recognized timestamp pass through unchanged.
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
			if cfg.relative {
				fmtOverride := ""
				if cfg.hasCustomFormat {
					fmtOverride = cfg.format
				}
				fmt.Fprint(w, processRelativeLine(
					line, now, fmtOverride))
			} else {
				ts := formatTimestamp(
					cfg, now, &lastTime, startTime, gmt)
				fmt.Fprintf(w, "%s %s", ts, line)
			}
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

// --- Relative time mode (-r) R6.1, R6.2, R10.1, R10.2 ---

// tsPattern describes a timestamp regex and its parser for -r mode.
// R6.2: each pattern recognizes a specific timestamp format.
type tsPattern struct {
	re    *regexp.Regexp
	parse func(string, time.Time) (time.Time, error)
}

// relPatterns lists timestamp patterns ordered by specificity.
// R6.2: ISO-8601, RFC 2822, syslog, lastlog.
var relPatterns = []tsPattern{
	{re: relISO8601Re, parse: parseISO8601},
	{re: relRFC2822Re, parse: parseRFC2822},
	{re: relSyslogRe, parse: parseSyslog},
	{re: relLastlogRe, parse: parseLastlog},
}

// R6.2: ISO-8601 format "2024-01-05T14:30:00.000Z".
var relISO8601Re = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?` +
		`(?:Z|[+-]\d{2}:?\d{2})?`)

// R6.2: RFC 2822 format "16 Jun 94 07:29:35 GMT" with optional day.
var relRFC2822Re = regexp.MustCompile(
	`(?:(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun),?\s+)?` +
		`\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)` +
		`\s+\d{2,4}\s+\d{2}:\d{2}:\d{2}\s+` +
		`(?:[A-Z]{2,5}|[+-]\d{4})`)

// R6.2: syslog format "Jan  5 14:30:00".
var relSyslogRe = regexp.MustCompile(
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)` +
		`\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`)

// R6.2: lastlog format "Mon Jan  5 14:30".
var relLastlogRe = regexp.MustCompile(
	`(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+` +
		`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)` +
		`\s+\d{1,2}\s+\d{2}:\d{2}`)

// weekdayPrefixRe strips optional weekday prefix from RFC 2822.
var weekdayPrefixRe = regexp.MustCompile(
	`^(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun),?\s+`)

// processRelativeLine scans a line for timestamps and replaces them.
// R6.1: replace each match with relative age when fmtOverride is empty.
// R10.1: when fmtOverride is non-empty, reformat matched timestamps
// using strftime instead of relative age.
// R10.2: lines with no recognizable timestamp pass through unchanged.
func processRelativeLine(
	line string, now time.Time, fmtOverride string,
) string {
	for _, pat := range relPatterns {
		if pat.re.MatchString(line) {
			p := pat.parse
			return pat.re.ReplaceAllStringFunc(
				line, func(m string) string {
					t, err := p(m, now)
					if err != nil {
						return m
					}
					if fmtOverride != "" {
						return formatStrftime(fmtOverride, t)
					}
					return formatRelativeAge(now.Sub(t))
				})
		}
	}
	// R10.2: no recognizable timestamp, pass through unchanged.
	return line
}

// iso8601Layouts are Go time layouts for ISO-8601 parsing.
var iso8601Layouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

// parseISO8601 parses an ISO-8601 timestamp.
// R6.2: handles "2024-01-05T14:30:00.000Z" and variants.
func parseISO8601(match string, _ time.Time) (time.Time, error) {
	s := normalizeISOTimezone(match)
	for _, layout := range iso8601Layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse ISO-8601: %s", match)
}

// normalizeISOTimezone inserts a colon in numeric timezone if missing.
// Converts "+0000" to "+00:00" for Go time.Parse compatibility.
func normalizeISOTimezone(s string) string {
	n := len(s)
	if n >= 5 && (s[n-5] == '+' || s[n-5] == '-') && s[n-3] != ':' {
		return s[:n-2] + ":" + s[n-2:]
	}
	return s
}

// rfc2822Layouts are Go time layouts for RFC 2822 parsing.
var rfc2822Layouts = []string{
	"2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 06 15:04:05 MST",
	"2 Jan 06 15:04:05 -0700",
}

// parseRFC2822 parses an RFC 2822 timestamp.
// R6.2: handles "16 Jun 94 07:29:35 GMT" with optional day prefix.
func parseRFC2822(match string, _ time.Time) (time.Time, error) {
	s := weekdayPrefixRe.ReplaceAllString(match, "")
	for _, layout := range rfc2822Layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse RFC 2822: %s", match)
}

// parseSyslog parses a syslog-style timestamp.
// R6.2: handles "Jan  5 14:30:00". Year is inferred from now.
func parseSyslog(match string, now time.Time) (time.Time, error) {
	normalized := collapseSpaces(match)
	t, err := time.Parse("Jan 2 15:04:05", normalized)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(
		now.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
}

// parseLastlog parses a lastlog-style timestamp.
// R6.2: handles "Mon Jan  5 14:30". Year is inferred from now.
func parseLastlog(match string, now time.Time) (time.Time, error) {
	normalized := collapseSpaces(match)
	t, err := time.Parse("Mon Jan 2 15:04", normalized)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(
		now.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), 0, 0, time.Local), nil
}

// collapseSpaces replaces runs of whitespace with a single space.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ageUnit pairs a duration in seconds with its label suffix.
type ageUnit struct {
	secs  int64
	label string
}

// ageUnits defines the time unit breakdown for relative age formatting.
var ageUnits = []ageUnit{
	{365 * 24 * 3600, "y"},
	{24 * 3600, "d"},
	{3600, "h"},
	{60, "m"},
	{1, "s"},
}

// formatRelativeAge converts a duration to a human-readable relative
// age string like "15m5s ago".
// R6.1: human-readable relative age output.
func formatRelativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	total := int64(d.Seconds())
	if total == 0 {
		return "0s ago"
	}
	var b strings.Builder
	for _, u := range ageUnits {
		if val := total / u.secs; val > 0 {
			fmt.Fprintf(&b, "%d%s", val, u.label)
			total %= u.secs
		}
	}
	b.WriteString(" ago")
	return b.String()
}
