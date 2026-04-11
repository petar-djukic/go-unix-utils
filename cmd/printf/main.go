// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/printf: format and print data.
// Implements srd073-printf R1, R2, R3, R4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// exitOK and exitErr are the process exit codes per R4.1 and R4.2.
const (
	exitOK  = 0
	exitErr = 1
)

func main() {
	// R4.1: install SIGPIPE handler for pipe-safe output.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "printf: usage: printf FORMAT [ARGUMENT]...\n")
		os.Exit(exitErr)
	}

	format := args[0]
	arguments := args[1:]

	exitCode := runPrintf(format, arguments)
	os.Exit(exitCode)
}

// runPrintf processes the format string with the given arguments.
// R3.2: reuses the format string when arguments remain after one pass.
// R4.1: returns 0 on success. R4.2: returns 1 on error.
func runPrintf(format string, args []string) int {
	hadError := false
	argIdx := 0
	for {
		consumed, err := processFormat(format, args, argIdx)
		if err {
			hadError = true
		}
		if consumed == argIdx && argIdx >= len(args) {
			break
		}
		argIdx = consumed
		if argIdx >= len(args) {
			break
		}
	}
	if hadError {
		return exitErr
	}
	return exitOK
}

// processFormat interprets the format string once, consuming arguments
// starting at argIdx. Returns the new argIdx and whether an error occurred.
// R1.1: literal text and conversion specifiers.
// R3.1: C-style escape sequences in FORMAT.
func processFormat(format string, args []string, argIdx int) (int, bool) {
	hadError := false
	i := 0
	for i < len(format) {
		if format[i] == '\\' {
			r, advance := parseFormatEscape(format, i)
			writeRune(r)
			i += advance
			continue
		}
		if format[i] == '%' {
			newI, newArgIdx, err := handleDirective(
				format, i, args, argIdx,
			)
			if err {
				hadError = true
			}
			i = newI
			argIdx = newArgIdx
			continue
		}
		os.Stdout.Write([]byte{format[i]})
		i++
	}
	return argIdx, hadError
}

// parseFormatEscape interprets a backslash escape in the format string.
// R3.1: \\, \a, \b, \f, \n, \r, \t, \v, \NNN, \xHH, \uHHHH, \UHHHHHHHH.
// Returns the rune to emit and the number of bytes consumed.
func parseFormatEscape(s string, pos int) (rune, int) {
	if pos+1 >= len(s) {
		return '\\', 1
	}
	switch s[pos+1] {
	case '\\':
		return '\\', 2
	case 'a':
		return '\a', 2
	case 'b':
		return '\b', 2
	case 'f':
		return '\f', 2
	case 'n':
		return '\n', 2
	case 'r':
		return '\r', 2
	case 't':
		return '\t', 2
	case 'v':
		return '\v', 2
	case '0', '1', '2', '3', '4', '5', '6', '7':
		return parseOctalEscape(s, pos+1)
	case 'x':
		return parseHexEscape(s, pos+2, 2)
	case 'u':
		return parseHexEscape(s, pos+2, 4)
	case 'U':
		return parseHexEscape(s, pos+2, 8)
	default:
		return '\\', 1
	}
}

// parseOctalEscape reads up to 3 octal digits from s[pos:].
// Returns the rune and total bytes consumed (including leading backslash).
func parseOctalEscape(s string, pos int) (rune, int) {
	val := 0
	count := 0
	for i := pos; i < len(s) && count < 3; i++ {
		d := s[i]
		if d < '0' || d > '7' {
			break
		}
		val = val*8 + int(d-'0')
		count++
	}
	// +1 for the leading backslash
	return rune(val & 0xFF), count + 1
}

// parseHexEscape reads up to maxDigits hex digits from s[pos:].
// Returns the rune and total bytes consumed (including \x/\u/\U prefix).
func parseHexEscape(s string, pos, maxDigits int) (rune, int) {
	val := 0
	count := 0
	for i := pos; i < len(s) && count < maxDigits; i++ {
		d := hexDigitVal(s[i])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		return '\\', 1
	}
	// prefix length: \x=2, \u=2, \U=2
	return rune(val), count + 2
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

// handleDirective processes a %-directive at format[pos].
// R1.2: integer specifiers. R1.3: float specifiers. R1.4: string/char.
// R2.1: width and precision. R2.2: flags. R2.3: * for width/precision.
// R2.4: %% literal. R3.3: missing arguments default to 0 or "".
// Returns updated format index, argument index, and error flag.
func handleDirective(
	format string, pos int, args []string, argIdx int,
) (int, int, bool) {
	i := pos + 1 // skip '%'
	if i >= len(format) {
		os.Stdout.Write([]byte{'%'})
		return i, argIdx, false
	}
	// R2.4: literal percent
	if format[i] == '%' {
		os.Stdout.Write([]byte{'%'})
		return i + 1, argIdx, false
	}
	spec, endIdx := parseConvSpec(format, i)
	return applyConvSpec(spec, format[pos:endIdx], args, argIdx, endIdx)
}

// convSpec holds the parsed components of a conversion specifier.
type convSpec struct {
	flags     string
	width     string
	widthStar bool
	prec      string
	precStar  bool
	hasPrec   bool
	verb      byte
}

// parseConvSpec parses flags, width, precision, and verb from format[pos:].
// Returns the parsed spec and the index past the verb character.
func parseConvSpec(format string, pos int) (convSpec, int) {
	var spec convSpec
	i := pos
	// R2.2: parse flags
	for i < len(format) && isFlag(format[i]) {
		spec.flags += string(format[i])
		i++
	}
	// R2.1/R2.3: parse width
	if i < len(format) && format[i] == '*' {
		spec.widthStar = true
		i++
	} else {
		i = scanDigits(format, i, &spec.width)
	}
	// R2.1/R2.3: parse precision
	if i < len(format) && format[i] == '.' {
		spec.hasPrec = true
		i++
		if i < len(format) && format[i] == '*' {
			spec.precStar = true
			i++
		} else {
			i = scanDigits(format, i, &spec.prec)
		}
	}
	// verb
	if i < len(format) {
		spec.verb = format[i]
		i++
	}
	return spec, i
}

// isFlag returns true if c is a printf flag character.
func isFlag(c byte) bool {
	return c == '-' || c == '+' || c == ' ' || c == '0' || c == '#'
}

// scanDigits reads decimal digits from format[pos:] into target.
func scanDigits(format string, pos int, target *string) int {
	for pos < len(format) && format[pos] >= '0' && format[pos] <= '9' {
		*target += string(format[pos])
		pos++
	}
	return pos
}

// applyConvSpec formats one argument per the conversion specifier.
// Returns updated format index, argument index, and error flag.
func applyConvSpec(
	spec convSpec, _ string, args []string, argIdx int, endIdx int,
) (int, int, bool) {
	hadError := false
	// Resolve * width
	width := 0
	if spec.widthStar {
		width, argIdx = consumeIntArg(args, argIdx)
	}
	// Resolve * precision
	prec := 0
	if spec.precStar {
		prec, argIdx = consumeIntArg(args, argIdx)
	}
	switch spec.verb {
	case 'd', 'i':
		argIdx, hadError = fmtInteger(spec, width, prec, args, argIdx, "d")
	case 'o':
		argIdx, hadError = fmtInteger(spec, width, prec, args, argIdx, "o")
	case 'u':
		argIdx, hadError = fmtUnsigned(spec, width, prec, args, argIdx)
	case 'x':
		argIdx, hadError = fmtInteger(spec, width, prec, args, argIdx, "x")
	case 'X':
		argIdx, hadError = fmtInteger(spec, width, prec, args, argIdx, "X")
	case 'f', 'F':
		argIdx, hadError = fmtFloat(spec, width, prec, args, argIdx, spec.verb)
	case 'e', 'E':
		argIdx, hadError = fmtFloat(spec, width, prec, args, argIdx, spec.verb)
	case 'g', 'G':
		argIdx, hadError = fmtFloat(spec, width, prec, args, argIdx, spec.verb)
	case 's':
		argIdx = fmtString(spec, width, prec, args, argIdx)
	case 'c':
		argIdx = fmtChar(args, argIdx)
	case 'b':
		argIdx = fmtBackslash(args, argIdx)
	default:
		fmt.Fprintf(os.Stderr, "printf: %%%c: invalid directive\n", spec.verb)
		hadError = true
	}
	return endIdx, argIdx, hadError
}

// consumeIntArg returns the next argument as an int and advances argIdx.
// R3.3: returns 0 if no arguments remain.
func consumeIntArg(args []string, argIdx int) (int, int) {
	if argIdx >= len(args) {
		return 0, argIdx
	}
	val, _ := parseIntegerArg(args[argIdx])
	return int(val), argIdx + 1
}

// consumeStringArg returns the next argument as a string.
// R3.3: returns "" if no arguments remain.
func consumeStringArg(args []string, argIdx int) (string, int) {
	if argIdx >= len(args) {
		return "", argIdx
	}
	return args[argIdx], argIdx + 1
}

// parseIntegerArg parses a string as an integer, handling quote prefix.
// R3.4: leading ' or " means use the character value.
func parseIntegerArg(s string) (int64, error) {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return int64(r), nil
	}
	// Try base-0 parsing (handles 0x, 0o, 0b prefixes)
	val, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("'%s': expected a numeric value", s)
	}
	return val, nil
}

// parseFloatArg parses a string as a float64, handling quote prefix.
// R3.4: leading ' or " means use the character value.
func parseFloatArg(s string) (float64, error) {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return float64(r), nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("'%s': expected a numeric value", s)
	}
	return val, nil
}

// buildFmt reconstructs a Go fmt verb string from the parsed spec.
func buildFmt(spec convSpec, width, prec int, verb string) string {
	var b strings.Builder
	b.WriteByte('%')
	b.WriteString(spec.flags)
	if spec.widthStar {
		b.WriteString(strconv.Itoa(width))
	} else {
		b.WriteString(spec.width)
	}
	if spec.hasPrec {
		b.WriteByte('.')
		if spec.precStar {
			b.WriteString(strconv.Itoa(prec))
		} else {
			b.WriteString(spec.prec)
		}
	}
	b.WriteString(verb)
	return b.String()
}

// fmtInteger formats one argument as a signed integer.
// R1.2: %d, %i, %o, %x, %X.
func fmtInteger(
	spec convSpec, width, prec int, args []string, argIdx int, verb string,
) (int, bool) {
	s, newIdx := consumeStringArg(args, argIdx)
	val, err := parseIntegerArg(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: %s\n", err)
		fmt.Fprintf(os.Stdout, buildFmt(spec, width, prec, verb), int64(0))
		return newIdx, true
	}
	fmt.Fprintf(os.Stdout, buildFmt(spec, width, prec, verb), val)
	return newIdx, false
}

// fmtUnsigned formats one argument as an unsigned decimal integer.
// R1.2: %u (unsigned decimal).
func fmtUnsigned(
	spec convSpec, width, prec int, args []string, argIdx int,
) (int, bool) {
	s, newIdx := consumeStringArg(args, argIdx)
	val, err := parseIntegerArg(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: %s\n", err)
		fmt.Fprintf(os.Stdout, buildFmt(spec, width, prec, "d"), uint64(0))
		return newIdx, true
	}
	fmt.Fprintf(os.Stdout, buildFmt(spec, width, prec, "d"), uint64(val))
	return newIdx, false
}

// fmtFloat formats one argument as a floating-point number.
// R1.3: %f, %e, %g and uppercase variants.
func fmtFloat(
	spec convSpec, width, prec int, args []string, argIdx int, verb byte,
) (int, bool) {
	s, newIdx := consumeStringArg(args, argIdx)
	val, err := parseFloatArg(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: %s\n", err)
		fmtStr := buildFmt(spec, width, prec, string(verb))
		fmt.Fprintf(os.Stdout, fmtStr, float64(0))
		return newIdx, true
	}
	fmtStr := buildFmt(spec, width, prec, string(verb))
	fmt.Fprintf(os.Stdout, fmtStr, val)
	return newIdx, false
}

// fmtString formats one argument as a string.
// R1.4: %s.
func fmtString(
	spec convSpec, width, prec int, args []string, argIdx int,
) int {
	s, newIdx := consumeStringArg(args, argIdx)
	fmtStr := buildFmt(spec, width, prec, "s")
	fmt.Fprintf(os.Stdout, fmtStr, s)
	return newIdx
}

// fmtChar outputs the first character of the argument.
// R1.4: %c.
func fmtChar(args []string, argIdx int) int {
	s, newIdx := consumeStringArg(args, argIdx)
	if len(s) > 0 {
		r, _ := utf8.DecodeRuneInString(s)
		writeRune(r)
	}
	return newIdx
}

// fmtBackslash interprets backslash escapes in the argument string.
// R1.4: %b interprets escapes like echo -e.
func fmtBackslash(args []string, argIdx int) int {
	s, newIdx := consumeStringArg(args, argIdx)
	i := 0
	for i < len(s) {
		if s[i] == '\\' {
			r, advance := parseBEscape(s, i)
			if r == -1 {
				// \c: stop all output
				return newIdx
			}
			writeRune(r)
			i += advance
			continue
		}
		os.Stdout.Write([]byte{s[i]})
		i++
	}
	return newIdx
}

// parseBEscape parses echo-style escapes for %b.
// Returns -1 to signal \c (stop all output).
func parseBEscape(s string, pos int) (rune, int) {
	if pos+1 >= len(s) {
		return '\\', 1
	}
	switch s[pos+1] {
	case '\\':
		return '\\', 2
	case 'a':
		return '\a', 2
	case 'b':
		return '\b', 2
	case 'c':
		return -1, 2
	case 'f':
		return '\f', 2
	case 'n':
		return '\n', 2
	case 'r':
		return '\r', 2
	case 't':
		return '\t', 2
	case 'v':
		return '\v', 2
	case '0':
		return parseBOctal(s, pos+2)
	default:
		return '\\', 1
	}
}

// parseBOctal reads up to 3 octal digits for %b \0NNN escapes.
func parseBOctal(s string, pos int) (rune, int) {
	val := 0
	count := 0
	for i := pos; i < len(s) && count < 3; i++ {
		d := s[i]
		if d < '0' || d > '7' {
			break
		}
		val = val*8 + int(d-'0')
		count++
	}
	// +2 for the \0 prefix
	return rune(val & 0xFF), count + 2
}

// writeRune writes a single rune to stdout.
func writeRune(r rune) {
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	os.Stdout.Write(buf[:n])
}
