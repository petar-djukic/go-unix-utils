// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd073-printf R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.2.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type printState struct {
	args    []string
	argIdx  int
	hadErr  bool
	stopped bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "printf: usage: printf FORMAT [ARGUMENT]...\n")
		os.Exit(1)
	}
	os.Exit(runPrintf(args[0], args[1:]))
}

func runPrintf(format string, operands []string) int {
	ps := &printState{args: operands}
	for {
		start := ps.argIdx
		processFormat(format, ps)
		if ps.stopped || ps.argIdx <= start || ps.argIdx >= len(ps.args) {
			break
		}
	}
	if ps.hadErr {
		return 1
	}
	return 0
}

func processFormat(format string, ps *printState) {
	var out strings.Builder
	i := 0
	for i < len(format) && !ps.stopped {
		switch format[i] {
		case '\\':
			i = processEscape(format, i+1, &out)
		case '%':
			i = processDirective(format, i+1, ps, &out)
		default:
			out.WriteByte(format[i])
			i++
		}
	}
	if _, err := os.Stdout.WriteString(out.String()); err != nil {
		os.Exit(1)
	}
}

func processEscape(s string, i int, out *strings.Builder) int {
	if i >= len(s) {
		out.WriteByte('\\')
		return i
	}
	switch s[i] {
	case '\\':
		out.WriteByte('\\')
	case 'a':
		out.WriteByte(0x07)
	case 'b':
		out.WriteByte(0x08)
	case 'f':
		out.WriteByte(0x0C)
	case 'n':
		out.WriteByte('\n')
	case 'r':
		out.WriteByte('\r')
	case 't':
		out.WriteByte('\t')
	case 'v':
		out.WriteByte(0x0B)
	case '0', '1', '2', '3', '4', '5', '6', '7':
		return parseOctal(s, i, 3, out)
	case 'x':
		return parseHex(s, i+1, 2, out)
	case 'u':
		return parseUnicode(s, i+1, 4, out)
	case 'U':
		return parseUnicode(s, i+1, 8, out)
	default:
		out.WriteByte('\\')
		out.WriteByte(s[i])
	}
	return i + 1
}

func parseOctal(s string, i, maxDigits int, out *strings.Builder) int {
	val := 0
	for k := 0; k < maxDigits && i < len(s) && s[i] >= '0' && s[i] <= '7'; k++ {
		val = val*8 + int(s[i]-'0')
		i++
	}
	out.WriteByte(byte(val))
	return i
}

func parseHex(s string, i, maxDigits int, out *strings.Builder) int {
	val := 0
	count := 0
	for count < maxDigits && i < len(s) {
		d, ok := hexVal(s[i])
		if !ok {
			break
		}
		val = val*16 + d
		count++
		i++
	}
	if count == 0 {
		out.WriteString("\\x")
		return i
	}
	out.WriteByte(byte(val))
	return i
}

func parseUnicode(s string, i, maxDigits int, out *strings.Builder) int {
	val := 0
	count := 0
	for count < maxDigits && i < len(s) {
		d, ok := hexVal(s[i])
		if !ok {
			break
		}
		val = val*16 + d
		count++
		i++
	}
	if count == 0 {
		out.WriteByte('\\')
		if maxDigits == 4 {
			out.WriteByte('u')
		} else {
			out.WriteByte('U')
		}
		return i
	}
	out.WriteRune(rune(val))
	return i
}

func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

func processDirective(
	format string, i int, ps *printState, out *strings.Builder,
) int {
	start := i - 1
	if i >= len(format) {
		out.WriteByte('%')
		return i
	}
	if format[i] == '%' {
		out.WriteByte('%')
		return i + 1
	}
	var dir strings.Builder
	dir.WriteByte('%')
	for i < len(format) && strings.IndexByte("-+# 0", format[i]) >= 0 {
		dir.WriteByte(format[i])
		i++
	}
	i = appendWidthOrPrec(format, i, ps, &dir)
	if i < len(format) && format[i] == '.' {
		dir.WriteByte('.')
		i++
		i = appendWidthOrPrec(format, i, ps, &dir)
	}
	if i >= len(format) {
		fmt.Fprintf(os.Stderr,
			"printf: '%%%s': missing format character\n", dir.String()[1:])
		ps.hadErr = true
		return i
	}
	switch format[i] {
	case 'd', 'i', 'o', 'u', 'x', 'X', 'f', 'F', 'e', 'E', 'g', 'G', 's', 'c', 'b':
		formatSpec(format[i], dir.String(), ps, out)
	default:
		end := i + 1
		if end < len(format) {
			end++
		}
		fmt.Fprintf(os.Stderr,
			"printf: %s: invalid conversion specification\n", format[start:end])
		ps.hadErr = true
		ps.stopped = true
	}
	return i + 1
}

func appendWidthOrPrec(
	format string, i int, ps *printState, dir *strings.Builder,
) int {
	if i < len(format) && format[i] == '*' {
		w := consumeIntArg(ps)
		dir.WriteString(strconv.Itoa(w))
		return i + 1
	}
	for i < len(format) && format[i] >= '0' && format[i] <= '9' {
		dir.WriteByte(format[i])
		i++
	}
	return i
}

func consumeIntArg(ps *printState) int {
	s := consumeArg(ps)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func consumeArg(ps *printState) string {
	if ps.argIdx < len(ps.args) {
		s := ps.args[ps.argIdx]
		ps.argIdx++
		return s
	}
	ps.argIdx++
	return ""
}

func formatSpec(spec byte, dir string, ps *printState, out *strings.Builder) {
	arg := consumeArg(ps)
	switch spec {
	case 'd', 'i':
		fmtSigned(dir+"d", arg, ps, out)
	case 'o':
		fmtUnsigned(dir+"o", arg, ps, out)
	case 'u':
		fmtUnsigned(dir+"d", arg, ps, out)
	case 'x':
		fmtUnsigned(dir+"x", arg, ps, out)
	case 'X':
		fmtUnsigned(dir+"X", arg, ps, out)
	case 'f', 'F', 'e', 'E', 'g', 'G':
		fmtFloat(dir+string(spec), arg, ps, out)
	case 's':
		fmt.Fprintf(out, dir+"s", arg)
	case 'c':
		fmtChar(dir, arg, out)
	case 'b':
		fmtBStr(arg, ps, out)
	}
}

func fmtSigned(goFmt, arg string, ps *printState, out *strings.Builder) {
	n, err := parseIntArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: '%s': expected a numeric value\n", arg)
		ps.hadErr = true
	}
	fmt.Fprintf(out, goFmt, n)
}

func fmtUnsigned(goFmt, arg string, ps *printState, out *strings.Builder) {
	n, err := parseIntArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: '%s': expected a numeric value\n", arg)
		ps.hadErr = true
	}
	if n == 0 {
		goFmt = strings.ReplaceAll(goFmt, "#", "")
	}
	fmt.Fprintf(out, goFmt, uint64(n))
}

func fmtFloat(goFmt, arg string, ps *printState, out *strings.Builder) {
	f, err := parseFloatArg(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "printf: '%s': expected a numeric value\n", arg)
		ps.hadErr = true
	}
	fmt.Fprintf(out, goFmt, f)
}

func fmtChar(dir, arg string, out *strings.Builder) {
	var r rune
	if arg != "" {
		r, _ = utf8.DecodeRuneInString(arg)
	}
	fmt.Fprintf(out, dir+"c", r)
}

func fmtBStr(arg string, ps *printState, out *strings.Builder) {
	var buf strings.Builder
	i := 0
	for i < len(arg) {
		if arg[i] != '\\' || i+1 >= len(arg) {
			buf.WriteByte(arg[i])
			i++
			continue
		}
		var stopped bool
		i, stopped = processBEscape(arg, i+1, &buf)
		if stopped {
			ps.stopped = true
			break
		}
	}
	out.WriteString(buf.String())
}

func processBEscape(s string, i int, out *strings.Builder) (int, bool) {
	switch s[i] {
	case '\\':
		out.WriteByte('\\')
	case 'a':
		out.WriteByte(0x07)
	case 'b':
		out.WriteByte(0x08)
	case 'c':
		return i + 1, true
	case 'f':
		out.WriteByte(0x0C)
	case 'n':
		out.WriteByte('\n')
	case 'r':
		out.WriteByte('\r')
	case 't':
		out.WriteByte('\t')
	case 'v':
		out.WriteByte(0x0B)
	case '0':
		return parseBOctal(s, i+1, out), false
	case 'x':
		return parseBHex(s, i+1, out), false
	default:
		out.WriteByte('\\')
		out.WriteByte(s[i])
	}
	return i + 1, false
}

func parseBOctal(s string, i int, out *strings.Builder) int {
	val := byte(0)
	for k := 0; k < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; k++ {
		val = val*8 + (s[i] - '0')
		i++
	}
	out.WriteByte(val)
	return i
}

func parseBHex(s string, i int, out *strings.Builder) int {
	val := byte(0)
	count := 0
	for count < 2 && i < len(s) {
		d, ok := hexVal(s[i])
		if !ok {
			break
		}
		val = val*16 + byte(d)
		count++
		i++
	}
	if count == 0 {
		out.WriteString("\\x")
		return i
	}
	out.WriteByte(val)
	return i
}

func parseIntArg(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if (s[0] == '\'' || s[0] == '"') && len(s) >= 2 {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return int64(r), nil
	}
	return strconv.ParseInt(s, 0, 64)
}

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
