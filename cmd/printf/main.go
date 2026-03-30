// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/printf implements GNU printf: format and print data.
// Implements prd073-printf R1.1-R1.4.
package main

import (
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

// run processes printf arguments and returns the exit code.
// R1.1: first argument is FORMAT; remaining arguments supply directive values.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "%s: usage: printf FORMAT [ARGUMENT]...\n", progName)
		return 1
	}
	format := args[0]
	fmtArgs := args[1:]
	exitCode := 0
	for {
		remaining, hadErr := processFormat(stdout, stderr, format, fmtArgs)
		if hadErr {
			exitCode = 1
		}
		if len(remaining) == 0 || len(remaining) == len(fmtArgs) {
			break
		}
		fmtArgs = remaining
	}
	return exitCode
}

// processFormat walks the format string once, writing output and consuming args.
// Returns unconsumed arguments and whether any error occurred.
func processFormat(w, errW io.Writer, format string, args []string) ([]string, bool) {
	argIdx := 0
	hadError := false
	for i := 0; i < len(format); {
		switch {
		case format[i] == '\\':
			i += processEscape(w, format, i)
		case format[i] == '%':
			n, used, err := handleDirective(w, format[i:], sliceFrom(args, argIdx))
			argIdx += used
			i += n
			if err != nil {
				fmt.Fprintf(errW, "%s: %v\n", progName, err)
				hadError = true
			}
		default:
			writeByte(w, format[i])
			i++
		}
	}
	if argIdx >= len(args) {
		return nil, hadError
	}
	return args[argIdx:], hadError
}

// sliceFrom returns args[idx:] or nil if idx >= len(args).
func sliceFrom(args []string, idx int) []string {
	if idx >= len(args) {
		return nil
	}
	return args[idx:]
}

// handleDirective processes one %-directive, writes output to w.
// Returns format chars consumed, args consumed, and any error.
func handleDirective(w io.Writer, format string, args []string) (int, int, error) {
	if len(format) < 2 {
		writeByte(w, '%')
		return 1, 0, nil
	}
	if format[1] == '%' {
		writeByte(w, '%')
		return 2, 0, nil
	}
	spec, verb, consumed := parseDirective(format[1:])
	if verb == 0 {
		return 1 + consumed, 0, fmt.Errorf("'%s': missing format character", format[:1+consumed])
	}
	arg := ""
	if len(args) > 0 {
		arg = args[0]
	}
	result, err := formatVerb(spec, verb, arg)
	_, _ = fmt.Fprint(w, result) // stdout write; SIGPIPE handler manages broken pipe
	return 1 + consumed, 1, err
}

// parseDirective parses flags, width, precision, and verb after '%'.
// Returns the spec string (flags+width+precision), verb byte, and chars consumed.
func parseDirective(s string) (string, byte, int) {
	i := 0
	for i < len(s) && isFlag(s[i]) {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i >= len(s) {
		return s, 0, i
	}
	return s[:i], s[i], i + 1
}

func isFlag(c byte) bool {
	return c == '-' || c == '+' || c == ' ' || c == '0' || c == '#'
}

// formatVerb formats an argument according to the conversion verb.
// R1.2: integer verbs. R1.3: float verbs. R1.4: string, char, b verbs.
func formatVerb(spec string, verb byte, arg string) (string, error) {
	switch {
	case isIntVerb(verb):
		return formatInt(spec, verb, arg)
	case isFloatVerb(verb):
		return formatFloat(spec, verb, arg)
	case verb == 's':
		return fmt.Sprintf("%"+spec+"s", arg), nil
	case verb == 'c':
		return formatChar(spec, arg), nil
	case verb == 'b':
		return interpretBEscapes(arg), nil
	default:
		return "", fmt.Errorf("%%%c: invalid directive", verb)
	}
}

func isIntVerb(v byte) bool {
	return v == 'd' || v == 'i' || v == 'o' || v == 'u' || v == 'x' || v == 'X'
}

func isFloatVerb(v byte) bool {
	return v == 'f' || v == 'F' || v == 'e' || v == 'E' || v == 'g' || v == 'G'
}

// formatInt formats an integer argument.
// R1.2: %d/%i (signed), %o (octal), %u (unsigned), %x/%X (hex).
func formatInt(spec string, verb byte, arg string) (string, error) {
	val, err := parseIntArg(arg)
	switch verb {
	case 'd', 'i':
		return fmt.Sprintf("%"+spec+"d", val), err
	case 'u':
		return fmt.Sprintf("%"+spec+"d", uint64(val)), err
	default: // o, x, X
		return fmt.Sprintf("%"+spec+string(verb), uint64(val)), err
	}
}

// formatFloat formats a floating-point argument.
// R1.3: %f/%F, %e/%E, %g/%G.
func formatFloat(spec string, verb byte, arg string) (string, error) {
	val, err := parseFloatArg(arg)
	goSpec := spec
	if (verb == 'g' || verb == 'G') && !strings.Contains(spec, ".") {
		goSpec = spec + ".6"
	}
	goVerb := verb
	if verb == 'F' {
		goVerb = 'f'
	}
	result := fmt.Sprintf("%"+goSpec+string(goVerb), val)
	if verb == 'F' {
		result = strings.ToUpper(result)
	}
	return result, err
}

// formatChar formats a character argument.
// R1.4: %c prints the first character of the argument.
func formatChar(spec string, arg string) string {
	var r rune
	if len(arg) > 0 {
		r, _ = utf8.DecodeRuneInString(arg)
	}
	return fmt.Sprintf("%"+spec+"c", r)
}

// parseIntArg parses a string as an integer, supporting 0x and 0 prefixes.
func parseIntArg(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	val, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("'%s': expected a numeric value", s)
	}
	return val, nil
}

// parseFloatArg parses a string as a float64.
func parseFloatArg(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("'%s': expected a numeric value", s)
	}
	return val, nil
}

// processEscape handles a backslash escape in the format string at pos.
// Writes the decoded value to w. Returns the number of format chars consumed.
func processEscape(w io.Writer, format string, pos int) int {
	if pos+1 >= len(format) {
		writeByte(w, '\\')
		return 1
	}
	c := format[pos+1]
	if r, ok := simpleEscape(c); ok {
		writeByte(w, r)
		return 2
	}
	return processExtendedEscape(w, format, pos, c)
}

// simpleEscape maps single-character escape codes to their byte values.
func simpleEscape(c byte) (byte, bool) {
	switch c {
	case '\\':
		return '\\', true
	case 'a':
		return '\a', true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return '\v', true
	default:
		return 0, false
	}
}

// processExtendedEscape handles octal, hex, and unicode escapes in the format.
func processExtendedEscape(w io.Writer, format string, pos int, c byte) int {
	switch {
	case c >= '0' && c <= '7':
		val, n := parseOctalDigits(format, pos+1, 3)
		writeByte(w, byte(val))
		return 1 + n
	case c == 'x':
		val, n := parseHexDigits(format, pos+2, 2)
		if n == 0 {
			writeByte(w, '\\')
			return 1
		}
		writeByte(w, byte(val))
		return 2 + n
	case c == 'u':
		val, n := parseHexDigits(format, pos+2, 4)
		if n == 0 {
			writeByte(w, '\\')
			return 1
		}
		writeRune(w, rune(val))
		return 2 + n
	case c == 'U':
		val, n := parseHexDigits(format, pos+2, 8)
		if n == 0 {
			writeByte(w, '\\')
			return 1
		}
		writeRune(w, rune(val))
		return 2 + n
	default:
		writeByte(w, '\\')
		return 1
	}
}

// interpretBEscapes processes backslash escapes in a %b argument string.
// R1.4: %b interprets escapes like echo -e.
func interpretBEscapes(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' || i+1 >= len(s) {
			buf.WriteByte(s[i])
			i++
			continue
		}
		n, stop := writeBEscape(&buf, s, i+1)
		if stop {
			break
		}
		i += 1 + n
	}
	return buf.String()
}

// writeBEscape decodes one escape in a %b argument and writes it to buf.
// Returns chars consumed after the backslash and whether \c was encountered.
func writeBEscape(buf *strings.Builder, s string, pos int) (int, bool) {
	c := s[pos]
	if c == 'c' {
		return 0, true
	}
	if r, ok := bSimpleEscape(c); ok {
		buf.WriteByte(r)
		return 1, false
	}
	return writeBExtendedEscape(buf, s, pos, c)
}

// bSimpleEscape maps %b single-character escape codes to byte values.
func bSimpleEscape(c byte) (byte, bool) {
	switch c {
	case '\\':
		return '\\', true
	case 'a':
		return '\a', true
	case 'b':
		return '\b', true
	case 'e':
		return 0x1B, true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return '\v', true
	default:
		return 0, false
	}
}

// writeBExtendedEscape handles \0NNN and \xHH in %b arguments.
func writeBExtendedEscape(buf *strings.Builder, s string, pos int, c byte) (int, bool) {
	switch {
	case c == '0':
		val, n := parseOctalDigits(s, pos+1, 3)
		buf.WriteByte(byte(val))
		return 1 + n, false
	case c == 'x':
		val, n := parseHexDigits(s, pos+1, 2)
		if n == 0 {
			buf.WriteByte('\\')
			return 0, false
		}
		buf.WriteByte(byte(val))
		return 1 + n, false
	default:
		buf.WriteByte('\\')
		return 0, false
	}
}

// parseOctalDigits parses up to max octal digits from s starting at start.
func parseOctalDigits(s string, start, max int) (int, int) {
	val, count := 0, 0
	for i := start; i < len(s) && count < max; i++ {
		if s[i] < '0' || s[i] > '7' {
			break
		}
		val = val*8 + int(s[i]-'0')
		count++
	}
	return val, count
}

// parseHexDigits parses up to max hex digits from s starting at start.
func parseHexDigits(s string, start, max int) (int, int) {
	val, count := 0, 0
	for i := start; i < len(s) && count < max; i++ {
		d := hexDigitVal(s[i])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	return val, count
}

// hexDigitVal returns the numeric value of a hex digit, or -1 if not hex.
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

func writeByte(w io.Writer, b byte) {
	_, _ = w.Write([]byte{b}) // stdout write; SIGPIPE handler manages broken pipe
}

func writeRune(w io.Writer, r rune) {
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	_, _ = w.Write(buf[:n]) // stdout write; SIGPIPE handler manages broken pipe
}
