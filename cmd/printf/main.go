// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd073-printf: Format and Print Data.
// Covers R1.1-R1.4 (format string, escape sequences, argument cycling, defaults),
// R2.1-R2.2 (width, precision, flags).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "printf"

func main() {
	// D1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		os.Exit(1)
	}
	os.Exit(runPrintf(os.Args[1], os.Args[2:]))
}

// runPrintf processes the format string, cycling through args.
// R1.3: re-applies FORMAT until all arguments are consumed.
func runPrintf(format string, args []string) int {
	exitCode := 0
	argIdx := 0
	for {
		startIdx := argIdx
		newIdx, ec, stopped := processFormat(format, args, argIdx)
		argIdx = newIdx
		if ec != 0 {
			exitCode = ec
		}
		if stopped {
			return 0
		}
		if argIdx >= len(args) || argIdx <= startIdx {
			break
		}
	}
	return exitCode
}

// processFormat walks the format string once, writing output to stdout.
// Returns new argument index, exit code, and whether %b \c stopped output.
func processFormat(format string, args []string, argIdx int) (int, int, bool) {
	var buf strings.Builder
	exitCode := 0
	i := 0
	for i < len(format) {
		switch format[i] {
		case '\\':
			i = writeEscape(format, i, &buf)
		case '%':
			var ec int
			var stopped bool
			i, argIdx, ec, stopped = writeDirective(format, i, args, argIdx, &buf)
			if ec != 0 {
				exitCode = ec
			}
			if stopped {
				// best-effort flush before stopping
				_, _ = os.Stdout.WriteString(buf.String())
				return argIdx, exitCode, true
			}
		default:
			buf.WriteByte(format[i])
			i++
		}
	}
	// best-effort write to stdout
	_, _ = os.Stdout.WriteString(buf.String())
	return argIdx, exitCode, false
}

// writeEscape interprets a backslash escape at format[pos].
// R3.1: \\, \a, \b, \f, \n, \r, \t, \v, \NNN, \xHH, \uHHHH, \UHHHHHHHH.
func writeEscape(format string, pos int, buf *strings.Builder) int {
	if pos+1 >= len(format) {
		buf.WriteByte('\\')
		return pos + 1
	}
	switch format[pos+1] {
	case '\\':
		buf.WriteByte('\\')
	case 'a':
		buf.WriteByte(0x07)
	case 'b':
		buf.WriteByte(0x08)
	case 'f':
		buf.WriteByte(0x0C)
	case 'n':
		buf.WriteByte('\n')
	case 'r':
		buf.WriteByte('\r')
	case 't':
		buf.WriteByte('\t')
	case 'v':
		buf.WriteByte(0x0B)
	case '0', '1', '2', '3', '4', '5', '6', '7':
		return writeOctalEsc(format, pos+1, buf)
	case 'x':
		return writeHexEsc(format, pos+2, buf)
	case 'u':
		return writeUnicodeEsc(format, pos+2, 4, buf)
	case 'U':
		return writeUnicodeEsc(format, pos+2, 8, buf)
	default:
		buf.WriteByte('\\')
		buf.WriteByte(format[pos+1])
	}
	return pos + 2
}

// writeOctalEsc reads up to 3 octal digits from format[pos].
func writeOctalEsc(format string, pos int, buf *strings.Builder) int {
	val, count := 0, 0
	for count < 3 && pos+count < len(format) &&
		format[pos+count] >= '0' && format[pos+count] <= '7' {
		val = val*8 + int(format[pos+count]-'0')
		count++
	}
	buf.WriteByte(byte(val & 0xFF))
	return pos + count
}

// writeHexEsc reads up to 2 hex digits from format[pos] for \xHH.
func writeHexEsc(format string, pos int, buf *strings.Builder) int {
	val, count := 0, 0
	for count < 2 && pos+count < len(format) {
		d := hexDigitVal(format[pos+count])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		buf.WriteString("\\x")
		return pos
	}
	buf.WriteByte(byte(val))
	return pos + count
}

// writeUnicodeEsc reads up to maxDigits hex digits for \u or \U.
func writeUnicodeEsc(format string, pos, maxDigits int, buf *strings.Builder) int {
	val, count := 0, 0
	for count < maxDigits && pos+count < len(format) {
		d := hexDigitVal(format[pos+count])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		writeUnicodeFallback(buf, maxDigits)
		return pos
	}
	buf.WriteRune(rune(val))
	return pos + count
}

// writeUnicodeFallback writes the literal \u or \U when no digits follow.
func writeUnicodeFallback(buf *strings.Builder, maxDigits int) {
	if maxDigits == 4 {
		buf.WriteString("\\u")
	} else {
		buf.WriteString("\\U")
	}
}

// hexDigitVal returns the numeric value of a hex digit, or -1.
func hexDigitVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// writeDirective processes a %-directive at format[pos].
// Returns new pos, new argIdx, exit code, and stopped flag.
func writeDirective(
	format string, pos int, args []string, argIdx int, buf *strings.Builder,
) (int, int, int, bool) {
	pos++ // skip '%'
	if pos >= len(format) {
		buf.WriteByte('%')
		return pos, argIdx, 0, false
	}
	// R2.4: %% produces literal '%'.
	if format[pos] == '%' {
		buf.WriteByte('%')
		return pos + 1, argIdx, 0, false
	}
	spec, endPos := parseSpec(format, pos)
	if endPos == pos {
		fmt.Fprintf(os.Stderr, "%s: %%%c: invalid directive\n", progName, format[pos])
		buf.WriteByte('%')
		buf.WriteByte(format[pos])
		return pos + 1, argIdx, 1, false
	}
	// R1.4/R3.3: missing args default to empty/"0".
	arg := getArg(args, argIdx)
	argIdx++
	ec, stopped := applyDirective(buf, spec, arg)
	return endPos, argIdx, ec, stopped
}

// directiveSpec holds parsed printf directive components.
type directiveSpec struct {
	flags   string
	width   string
	prec    string
	hasPrec bool
	verb    byte
}

// parseSpec extracts flags, width, precision, and verb from format[pos].
// Returns the spec and position after the verb, or pos if invalid.
func parseSpec(format string, pos int) (directiveSpec, int) {
	var s directiveSpec
	start := pos
	for pos < len(format) && strings.IndexByte("-+ #0", format[pos]) >= 0 {
		pos++
	}
	s.flags = format[start:pos]
	wStart := pos
	for pos < len(format) && format[pos] >= '0' && format[pos] <= '9' {
		pos++
	}
	s.width = format[wStart:pos]
	if pos < len(format) && format[pos] == '.' {
		s.hasPrec = true
		pos++
		pStart := pos
		for pos < len(format) && format[pos] >= '0' && format[pos] <= '9' {
			pos++
		}
		s.prec = format[pStart:pos]
	}
	if pos >= len(format) || !isConvVerb(format[pos]) {
		return s, start
	}
	s.verb = format[pos]
	return s, pos + 1
}

// isConvVerb reports whether c is a valid conversion specifier.
func isConvVerb(c byte) bool {
	return strings.IndexByte("diouxXeEfFgGscb", c) >= 0
}

// getArg returns args[idx] or "" if out of range.
func getArg(args []string, idx int) string {
	if idx < len(args) {
		return args[idx]
	}
	return ""
}

// applyDirective formats arg per the spec and writes to buf.
// Returns exit code and whether %b \c stopped output.
func applyDirective(buf *strings.Builder, s directiveSpec, arg string) (int, bool) {
	switch s.verb {
	case 's':
		return applyString(buf, s, arg), false
	case 'c':
		return applyChar(buf, s, arg), false
	case 'b':
		stopped := expandBArg(buf, arg)
		return 0, stopped
	case 'd', 'i':
		return applySignedInt(buf, s, arg), false
	case 'o', 'u', 'x', 'X':
		return applyUnsignedInt(buf, s, arg), false
	default:
		return applyFloat(buf, s, arg), false
	}
}

// buildFmtSpec builds a Go fmt format string from directive components.
func buildFmtSpec(s directiveSpec, goVerb byte) string {
	var b strings.Builder
	b.WriteByte('%')
	b.WriteString(s.flags)
	b.WriteString(s.width)
	if s.hasPrec {
		b.WriteByte('.')
		b.WriteString(s.prec)
	}
	b.WriteByte(goVerb)
	return b.String()
}

// applyString handles the %s directive.
func applyString(buf *strings.Builder, s directiveSpec, arg string) int {
	fmt.Fprintf(buf, buildFmtSpec(s, 's'), arg)
	return 0
}

// applyChar handles the %c directive (first character of argument).
func applyChar(buf *strings.Builder, s directiveSpec, arg string) int {
	var ch rune
	if len(arg) > 0 {
		ch, _ = utf8.DecodeRuneInString(arg)
	}
	fmt.Fprintf(buf, buildFmtSpec(s, 'c'), ch)
	return 0
}

// expandBArg writes arg to buf interpreting echo-style backslash escapes.
// Returns true if \c was encountered (stop all output).
func expandBArg(buf *strings.Builder, arg string) bool {
	i := 0
	for i < len(arg) {
		if arg[i] != '\\' || i+1 >= len(arg) {
			buf.WriteByte(arg[i])
			i++
			continue
		}
		newI, stop := expandBEscape(buf, arg, i)
		i = newI
		if stop {
			return true
		}
	}
	return false
}

// expandBEscape handles one escape in a %b argument at arg[pos].
// Returns new position and whether \c was encountered.
func expandBEscape(buf *strings.Builder, arg string, pos int) (int, bool) {
	switch arg[pos+1] {
	case '\\':
		buf.WriteByte('\\')
	case 'a':
		buf.WriteByte(0x07)
	case 'b':
		buf.WriteByte(0x08)
	case 'c':
		return len(arg), true
	case 'f':
		buf.WriteByte(0x0C)
	case 'n':
		buf.WriteByte('\n')
	case 'r':
		buf.WriteByte('\r')
	case 't':
		buf.WriteByte('\t')
	case 'v':
		buf.WriteByte(0x0B)
	case '0':
		return writeBOctal(buf, arg, pos+2), false
	case 'x':
		return writeBHex(buf, arg, pos+2), false
	default:
		buf.WriteByte('\\')
		buf.WriteByte(arg[pos+1])
	}
	return pos + 2, false
}

// writeBOctal reads up to 3 octal digits for %b \0NNN.
func writeBOctal(buf *strings.Builder, arg string, pos int) int {
	val, count := 0, 0
	for count < 3 && pos+count < len(arg) &&
		arg[pos+count] >= '0' && arg[pos+count] <= '7' {
		val = val*8 + int(arg[pos+count]-'0')
		count++
	}
	buf.WriteByte(byte(val & 0xFF))
	return pos + count
}

// writeBHex reads up to 2 hex digits for %b \xHH.
func writeBHex(buf *strings.Builder, arg string, pos int) int {
	val, count := 0, 0
	for count < 2 && pos+count < len(arg) {
		d := hexDigitVal(arg[pos+count])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		buf.WriteString("\\x")
		return pos
	}
	buf.WriteByte(byte(val))
	return pos + count
}

// applySignedInt handles %d and %i directives.
func applySignedInt(buf *strings.Builder, s directiveSpec, arg string) int {
	val, err := parseIntArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': expected a numeric value\n", progName, arg)
	}
	fmt.Fprintf(buf, buildFmtSpec(s, 'd'), val)
	return boolToEC(err != nil)
}

// applyUnsignedInt handles %o, %u, %x, %X directives.
func applyUnsignedInt(buf *strings.Builder, s directiveSpec, arg string) int {
	val, err := parseIntArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': expected a numeric value\n", progName, arg)
	}
	goVerb := s.verb
	if goVerb == 'u' {
		goVerb = 'd'
	}
	fmt.Fprintf(buf, buildFmtSpec(s, goVerb), uint64(val))
	return boolToEC(err != nil)
}

// applyFloat handles %f, %F, %e, %E, %g, %G directives.
func applyFloat(buf *strings.Builder, s directiveSpec, arg string) int {
	val, err := parseFloatArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': expected a numeric value\n", progName, arg)
	}
	fmt.Fprintf(buf, buildFmtSpec(s, s.verb), val)
	return boolToEC(err != nil)
}

// boolToEC returns 1 if b is true, 0 otherwise.
func boolToEC(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseIntArg parses a string as an integer.
// D4: leading 0 = octal, 0x = hex, quote prefix = character value.
func parseIntArg(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if (s[0] == '\'' || s[0] == '"') && len(s) >= 2 {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return int64(r), nil
	}
	val, err := strconv.ParseInt(s, 0, 64)
	if err == nil {
		return val, nil
	}
	// Try unsigned for large values like 0xFFFFFFFFFFFFFFFF.
	uval, uerr := strconv.ParseUint(s, 0, 64)
	if uerr == nil {
		return int64(uval), nil
	}
	return 0, err
}

// parseFloatArg parses a string as a float.
// D4: quote prefix = character value.
func parseFloatArg(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	if (s[0] == '\'' || s[0] == '"') && len(s) >= 2 {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return float64(r), nil
	}
	return strconv.ParseFloat(s, 64)
}
