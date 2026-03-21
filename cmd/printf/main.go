// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd073-printf R1.1–R1.4, R2.1–R2.4, R3.1–R3.4: format string
// processing, conversion specifiers (integer, float, string, char, %b),
// width/precision/flags, escape sequences, argument cycling, and missing
// argument defaults.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "printf"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the printf logic and returns the exit code.
// R1.1: interprets FORMAT as a printf-style format string.
func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}
	format := args[0]
	if format == "--help" {
		printHelp(stdout)
		return 0
	}
	if format == "--version" {
		printVersion(stdout)
		return 0
	}
	return processFormat(format, args[1:], stdout, stderr)
}

// processFormat interprets the format string, consuming arguments.
// R3.2: repeats the format when arguments remain after one pass.
func processFormat(format string, args []string, stdout, stderr *os.File) int {
	w := bufio.NewWriter(stdout)
	exitCode := 0
	argIdx := 0
	hasDir := formatHasDirective(format)
	for {
		startIdx := argIdx
		code, stop := processOnce(format, args, &argIdx, w, stderr)
		if code != 0 {
			exitCode = 1
		}
		if stop || !hasDir || argIdx >= len(args) || argIdx == startIdx {
			break
		}
	}
	if err := w.Flush(); err != nil && exitCode == 0 {
		exitCode = 1
	}
	return exitCode
}

// formatHasDirective returns true if format contains a % directive.
func formatHasDirective(format string) bool {
	for i := 0; i < len(format)-1; i++ {
		if format[i] == '%' && format[i+1] != '%' {
			return true
		}
	}
	return false
}

// processOnce walks the format string once, outputting text and directives.
// Returns exit code and whether output should stop (\c in %b).
func processOnce(format string, args []string, argIdx *int, w *bufio.Writer, stderr io.Writer) (int, bool) {
	exitCode := 0
	for i := 0; i < len(format); {
		switch format[i] {
		case '\\':
			s, n := interpretEscape(format[i:])
			w.WriteString(s)
			i += n
		case '%':
			code, n, stop := handlePercent(format[i:], args, argIdx, w, stderr)
			if code != 0 {
				exitCode = code
			}
			i += n
			if stop {
				return exitCode, true
			}
		default:
			w.WriteByte(format[i])
			i++
		}
	}
	return exitCode, false
}

// handlePercent processes a format directive starting at %.
// Returns exit code, bytes consumed, and stop flag.
func handlePercent(format string, args []string, argIdx *int, w *bufio.Writer, stderr io.Writer) (int, int, bool) {
	if len(format) > 1 && format[1] == '%' {
		w.WriteByte('%')
		return 0, 2, false
	}
	d, n := parseDirective(format)
	if n == 0 {
		fmt.Fprintf(stderr, "%s: '%%%c': invalid directive\n", progName, safeChar(format, 1))
		consumed := minInt(2, len(format))
		w.WriteString(format[:consumed])
		return 1, consumed, false
	}
	code, stop := writeDirective(d, args, argIdx, w, stderr)
	return code, n, stop
}

// safeChar returns the byte at pos or '?' if out of range.
func safeChar(s string, pos int) byte {
	if pos < len(s) {
		return s[pos]
	}
	return '?'
}

// minInt returns the smaller of a and b.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// directive represents a parsed conversion specifier.
type directive struct {
	flags     string // R2.2: '-', '+', ' ', '0', '#'
	width     string // numeric or "*"
	precision string // numeric or "*"
	hasDot    bool
	verb      byte // R1.2–R1.4: d, i, o, u, x, X, f, e, g, F, E, G, s, c, b
}

// parseDirective parses %[flags][width][.precision]verb.
// Returns the directive and total bytes consumed (0 on failure).
func parseDirective(format string) (directive, int) {
	var d directive
	i := 1 // skip %
	for i < len(format) && isFlagChar(format[i]) {
		d.flags += string(format[i])
		i++
	}
	if i < len(format) && format[i] == '*' {
		d.width = "*"
		i++
	} else {
		d.width, i = scanDigits(format, i)
	}
	if i < len(format) && format[i] == '.' {
		d.hasDot = true
		i++
		if i < len(format) && format[i] == '*' {
			d.precision = "*"
			i++
		} else {
			d.precision, i = scanDigits(format, i)
		}
	}
	if i < len(format) && isVerbChar(format[i]) {
		d.verb = format[i]
		return d, i + 1
	}
	return directive{}, 0
}

// isFlagChar returns true for printf flag characters.
func isFlagChar(c byte) bool {
	return c == '-' || c == '+' || c == ' ' || c == '0' || c == '#'
}

// scanDigits scans decimal digits starting at pos.
func scanDigits(s string, pos int) (string, int) {
	start := pos
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		pos++
	}
	return s[start:pos], pos
}

// isVerbChar returns true for supported conversion specifier characters.
func isVerbChar(c byte) bool {
	return strings.IndexByte("diouxXfeEgGFscb", c) >= 0
}

// writeDirective formats a directive with arguments and writes output.
// Returns exit code and stop flag.
func writeDirective(d directive, args []string, argIdx *int, w *bufio.Writer, stderr io.Writer) (int, bool) {
	width := resolveStarArg(d.width, args, argIdx)
	prec := resolveStarArg(d.precision, args, argIdx)
	arg := consumeArg(args, argIdx)
	if d.verb == 'b' {
		return handleBVerb(arg, w)
	}
	result, code := formatVerb(d, width, prec, arg, stderr)
	w.WriteString(result)
	return code, false
}

// resolveStarArg resolves "*" by consuming the next argument.
func resolveStarArg(spec string, args []string, argIdx *int) string {
	if spec == "*" {
		return consumeArg(args, argIdx)
	}
	return spec
}

// consumeArg returns the next argument, or "" if exhausted (R3.3).
func consumeArg(args []string, argIdx *int) string {
	if *argIdx >= len(args) {
		return ""
	}
	arg := args[*argIdx]
	*argIdx++
	return arg
}

// handleBVerb handles the %b conversion specifier.
// Returns exit code and stop flag (\c encountered).
func handleBVerb(arg string, w *bufio.Writer) (int, bool) {
	text, stop := interpretBArgument(arg)
	w.WriteString(text)
	return 0, stop
}

// formatVerb dispatches to the appropriate format function.
func formatVerb(d directive, width, prec, arg string, stderr io.Writer) (string, int) {
	switch d.verb {
	case 'd', 'i':
		return fmtIntVerb(d, width, prec, arg, 'd', stderr)
	case 'o':
		return fmtUintVerb(d, width, prec, arg, 'o', stderr)
	case 'u':
		return fmtUintVerb(d, width, prec, arg, 'd', stderr)
	case 'x':
		return fmtUintVerb(d, width, prec, arg, 'x', stderr)
	case 'X':
		return fmtUintVerb(d, width, prec, arg, 'X', stderr)
	case 'f', 'F', 'e', 'E', 'g', 'G':
		return fmtFloatVerb(d, width, prec, arg, stderr)
	case 's':
		return fmtStrVerb(d, width, prec, arg), 0
	case 'c':
		return fmtCharVerb(d, width, arg), 0
	default:
		return "", 0
	}
}

// fmtIntVerb formats an integer conversion specifier.
func fmtIntVerb(d directive, width, prec, arg string, verb byte, stderr io.Writer) (string, int) {
	val, err := parseIntArg(arg)
	if err != nil {
		reportNumericError(arg, stderr)
		return fmt.Sprintf(buildFmtStr(d, width, prec, verb), int64(0)), 1
	}
	return fmt.Sprintf(buildFmtStr(d, width, prec, verb), val), 0
}

// fmtUintVerb formats an unsigned integer conversion (%u, %o, %x, %X).
// Uses uint64 to match GNU printf unsigned behavior for negative inputs.
func fmtUintVerb(d directive, width, prec, arg string, verb byte, stderr io.Writer) (string, int) {
	val, err := parseIntArg(arg)
	if err != nil {
		reportNumericError(arg, stderr)
		return fmt.Sprintf(buildFmtStr(d, width, prec, verb), uint64(0)), 1
	}
	return fmt.Sprintf(buildFmtStr(d, width, prec, verb), uint64(val)), 0
}

// fmtFloatVerb formats a floating-point conversion specifier.
func fmtFloatVerb(d directive, width, prec, arg string, stderr io.Writer) (string, int) {
	val, err := parseFloatArg(arg)
	code := 0
	if err != nil {
		reportNumericError(arg, stderr)
		val = 0
		code = 1
	}
	goVerb := d.verb
	if goVerb == 'F' {
		goVerb = 'f'
	}
	result := fmt.Sprintf(buildFmtStr(d, width, prec, goVerb), val)
	if d.verb == 'F' {
		result = strings.ToUpper(result)
	}
	return result, code
}

// fmtStrVerb formats a %s conversion specifier.
func fmtStrVerb(d directive, width, prec, arg string) string {
	return fmt.Sprintf(buildFmtStr(d, width, prec, 's'), arg)
}

// fmtCharVerb formats a %c conversion specifier with width support.
// R2.1: width specifies minimum field width for the character.
func fmtCharVerb(d directive, width, arg string) string {
	var r rune
	if arg == "" {
		r = 0
	} else {
		r, _ = utf8.DecodeRuneInString(arg)
	}
	return fmt.Sprintf(buildFmtStr(d, width, "", 'c'), r)
}

// reportNumericError writes a numeric conversion error to stderr.
func reportNumericError(arg string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: '%s': expected a numeric value\n", progName, arg)
}

// buildFmtStr constructs a Go fmt format string from directive components.
func buildFmtStr(d directive, width, prec string, verb byte) string {
	var b strings.Builder
	b.WriteByte('%')
	b.WriteString(d.flags)
	b.WriteString(width)
	if d.hasDot {
		b.WriteByte('.')
		b.WriteString(prec)
	}
	b.WriteByte(verb)
	return b.String()
}

// parseIntArg parses a string argument as int64.
// R3.3: empty → 0. R3.4: quote prefix → char value.
func parseIntArg(arg string) (int64, error) {
	if arg == "" {
		return 0, nil
	}
	if isQuotePrefix(arg) {
		return charValue(arg), nil
	}
	val, err := strconv.ParseInt(arg, 0, 64)
	if err == nil {
		return val, nil
	}
	fval, ferr := strconv.ParseFloat(arg, 64)
	if ferr != nil {
		return 0, err
	}
	return int64(fval), nil
}

// parseFloatArg parses a string argument as float64.
// R3.3: empty → 0. R3.4: quote prefix → char value.
func parseFloatArg(arg string) (float64, error) {
	if arg == "" {
		return 0, nil
	}
	if isQuotePrefix(arg) {
		return float64(charValue(arg)), nil
	}
	if hasHexPrefix(arg) {
		val, err := strconv.ParseInt(arg, 0, 64)
		if err == nil {
			return float64(val), nil
		}
	}
	return strconv.ParseFloat(arg, 64)
}

// isQuotePrefix returns true if arg starts with ' or " (R3.4).
func isQuotePrefix(arg string) bool {
	return len(arg) >= 2 && (arg[0] == '\'' || arg[0] == '"')
}

// hasHexPrefix returns true if arg starts with 0x or 0X.
func hasHexPrefix(arg string) bool {
	return len(arg) > 2 && arg[0] == '0' && (arg[1] == 'x' || arg[1] == 'X')
}

// charValue returns the Unicode code point after the quote prefix (R3.4).
func charValue(arg string) int64 {
	r, _ := utf8.DecodeRuneInString(arg[1:])
	return int64(r)
}

// interpretEscape processes a C-style escape in the format string.
// R3.1: \\, \a, \b, \f, \n, \r, \t, \v, \NNN, \xHH, \uHHHH, \UHHHHHHHH.
func interpretEscape(s string) (string, int) {
	if len(s) < 2 {
		return "\\", 1
	}
	if r, ok := namedEscape(s[1]); ok {
		return r, 2
	}
	switch s[1] {
	case 'x':
		return hexEscape(s[2:], 2)
	case 'u':
		return unicodeEscape(s[2:], 4, 'u')
	case 'U':
		return unicodeEscape(s[2:], 8, 'U')
	default:
		if isOctalDigit(s[1]) {
			val, n := parseOctalDigits(s[1:], 3)
			return string([]byte{byte(val)}), 1 + n
		}
		return string(s[:2]), 2
	}
}

// hexEscape parses \xHH and returns result with total consumed bytes.
// Outputs a raw byte, not UTF-8 encoded rune, to match GNU printf.
func hexEscape(digits string, prefixLen int) (string, int) {
	val, n := parseHexDigits(digits, 2)
	if n == 0 {
		return "\\x", prefixLen
	}
	return string([]byte{byte(val)}), prefixLen + n
}

// unicodeEscape parses \uHHHH or \UHHHHHHHH.
func unicodeEscape(digits string, maxDigits int, prefix byte) (string, int) {
	val, n := parseHexDigits(digits, maxDigits)
	if n == 0 {
		return string([]byte{'\\', prefix}), 2
	}
	return string(rune(val)), 2 + n
}

// interpretBEscape processes escape sequences in a %b argument.
// Uses \0NNN for octal (different from format string's \NNN).
func interpretBEscape(s string) (string, int) {
	if len(s) < 2 {
		return "\\", 1
	}
	if r, ok := namedEscape(s[1]); ok {
		return r, 2
	}
	switch s[1] {
	case 'x':
		return hexEscape(s[2:], 2)
	case '0':
		val, n := parseOctalDigits(s[2:], 3)
		if n == 0 {
			return "\x00", 2
		}
		return string([]byte{byte(val)}), 2 + n
	default:
		return string(s[:2]), 2
	}
}

// namedEscape returns the replacement for a named escape character.
func namedEscape(c byte) (string, bool) {
	switch c {
	case '\\':
		return "\\", true
	case 'a':
		return "\a", true
	case 'b':
		return "\b", true
	case 'f':
		return "\f", true
	case 'n':
		return "\n", true
	case 'r':
		return "\r", true
	case 't':
		return "\t", true
	case 'v':
		return "\v", true
	}
	return "", false
}

// interpretBArgument interprets escape sequences in a %b argument.
// Returns the interpreted text and whether \c was encountered.
func interpretBArgument(arg string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(arg); {
		if arg[i] != '\\' {
			b.WriteByte(arg[i])
			i++
			continue
		}
		if i+1 < len(arg) && arg[i+1] == 'c' {
			return b.String(), true
		}
		s, n := interpretBEscape(arg[i:])
		b.WriteString(s)
		i += n
	}
	return b.String(), false
}

// parseOctalDigits parses up to maxDigits octal digits.
func parseOctalDigits(s string, maxDigits int) (int, int) {
	val := 0
	n := 0
	for n < maxDigits && n < len(s) && isOctalDigit(s[n]) {
		val = val*8 + int(s[n]-'0')
		n++
	}
	return val, n
}

// parseHexDigits parses up to maxDigits hex digits.
func parseHexDigits(s string, maxDigits int) (int, int) {
	val := 0
	n := 0
	for n < maxDigits && n < len(s) && isHexDigit(s[n]) {
		val = val*16 + hexDigitVal(s[n])
		n++
	}
	return val, n
}

// isOctalDigit returns true for '0'-'7'.
func isOctalDigit(c byte) bool {
	return c >= '0' && c <= '7'
}

// isHexDigit returns true for hex digit characters.
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// hexDigitVal returns the numeric value of a hex digit.
func hexDigitVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s FORMAT [ARGUMENT]...\n", progName)
	fmt.Fprintf(w, "  or:  %s OPTION\n", progName)
	fmt.Fprintln(w, "Print ARGUMENT(s) according to FORMAT.")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
