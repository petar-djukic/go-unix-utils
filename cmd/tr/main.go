// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tr implements GNU tr: translate or delete characters.
//
// Implements prd054-tr R1.1 (character translation), R1.2 (stdin/stdout I/O),
// R1.3 (SET specifications: ranges, escapes, repetition),
// R1.4 (POSIX character classes).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "tr"

var (
	errVersion = errors.New("version requested")
	errHelp    = errors.New("help requested")
)

// setToken is a parsed element from a SET specification.
type setToken struct {
	chars  []byte
	fill   byte
	isFill bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and performs character translation.
// R1.1: translate SET1 to SET2. R1.2: read stdin, write stdout.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	set1Spec, set2Spec, err := parseArgs(args)
	if err != nil {
		return handleParseError(err, stdout, stderr)
	}
	set1, err := expandSet(set1Spec, 0)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	set2, err := expandSet(set2Spec, len(set1))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	// R1.1: extend SET2 to match SET1 length.
	set2 = extendSet(set2, len(set1))
	table := buildTable(set1, set2)
	if err := translateStream(stdin, stdout, table); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// handleParseError dispatches --version, --help, and real errors.
func handleParseError(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, errVersion) {
		fmt.Fprintln(stdout, "tr (go-unix-utils)")
		return 0
	}
	if errors.Is(err, errHelp) {
		printUsage(stdout)
		return 0
	}
	fmt.Fprintf(stderr, "%s: %s\n", programName, err)
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName)
	return 1
}

// printUsage writes usage information.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SET1 [SET2]\n", programName)
	fmt.Fprintln(w, "Translate characters from standard input, writing to standard output.")
}

// parseArgs extracts SET1 and SET2 from the command-line arguments.
func parseArgs(args []string) (string, string, error) {
	var positional []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone {
			positional = append(positional, arg)
			continue
		}
		switch arg {
		case "--":
			flagsDone = true
		case "--help":
			return "", "", errHelp
		case "--version":
			return "", "", errVersion
		default:
			positional = append(positional, arg)
		}
	}
	return validatePositional(positional)
}

// validatePositional checks that two SET arguments are provided.
func validatePositional(positional []string) (string, string, error) {
	if len(positional) < 1 {
		return "", "", fmt.Errorf("missing operand")
	}
	if len(positional) < 2 {
		return "", "", fmt.Errorf("missing operand after '%s'", positional[0])
	}
	return positional[0], positional[1], nil
}

// expandSet parses a SET specification and returns expanded bytes.
// targetLen controls [c*] expansion; pass 0 for SET1.
func expandSet(spec string, targetLen int) ([]byte, error) {
	tokens, err := parseSetTokens(spec)
	if err != nil {
		return nil, err
	}
	return resolveTokens(tokens, targetLen), nil
}

// resolveTokens expands tokens into a byte slice, resolving fill markers.
func resolveTokens(tokens []setToken, targetLen int) []byte {
	fixed := countFixedBytes(tokens)
	var result []byte
	for _, t := range tokens {
		if t.isFill {
			count := max(targetLen-fixed, 1)
			result = append(result, repeatByte(t.fill, count)...)
		} else {
			result = append(result, t.chars...)
		}
	}
	return result
}

// countFixedBytes counts non-fill bytes across all tokens.
func countFixedBytes(tokens []setToken) int {
	n := 0
	for _, t := range tokens {
		if !t.isFill {
			n += len(t.chars)
		}
	}
	return n
}

// parseSetTokens parses a SET specification into tokens.
// R1.3: supports ranges, escapes, repetition. R1.4: POSIX character classes.
func parseSetTokens(spec string) ([]setToken, error) {
	var tokens []setToken
	i := 0
	for i < len(spec) {
		if spec[i] == '[' {
			tok, n, ok, err := parseBracketExpr(spec, i)
			if err != nil {
				return nil, err
			}
			if ok {
				tokens = append(tokens, tok)
				i += n
				continue
			}
		}
		ch, n := parseSingleChar(spec, i)
		i += n
		tok, advance, err := tryParseRange(spec, i, ch)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		i += advance
	}
	return tokens, nil
}

// tryParseRange checks if the current position forms a range (c-d).
// Returns the token (single char or range) and extra bytes consumed.
func tryParseRange(spec string, pos int, startCh byte) (setToken, int, error) {
	if pos >= len(spec) || spec[pos] != '-' || pos+1 >= len(spec) {
		return setToken{chars: []byte{startCh}}, 0, nil
	}
	if spec[pos+1] == '[' {
		return setToken{chars: []byte{startCh}}, 0, nil
	}
	endCh, n := parseSingleChar(spec, pos+1)
	if endCh < startCh {
		return setToken{}, 0, fmt.Errorf(
			"range-endpoints of '%c-%c' are in reverse collating sequence order",
			startCh, endCh)
	}
	return setToken{chars: byteRange(startCh, endCh)}, 1 + n, nil
}

// parseBracketExpr tries to parse [:class:], [=c=], or [c*n] at spec[pos].
func parseBracketExpr(spec string, pos int) (setToken, int, bool, error) {
	rest := spec[pos:]
	if len(rest) < 4 {
		return setToken{}, 0, false, nil
	}
	if strings.HasPrefix(rest, "[:") {
		return parseCharClass(spec, pos)
	}
	if strings.HasPrefix(rest, "[=") {
		return parseEquivClass(spec, pos)
	}
	return tryRepetition(spec, pos)
}

// parseCharClass parses [:name:] at spec[pos].
// R1.4: POSIX character classes.
func parseCharClass(spec string, pos int) (setToken, int, bool, error) {
	rest := spec[pos+2:]
	endIdx := strings.Index(rest, ":]")
	if endIdx < 0 {
		return setToken{}, 0, false, nil
	}
	name := rest[:endIdx]
	consumed := 2 + endIdx + 2
	chars, err := charClassBytes(name)
	if err != nil {
		return setToken{}, 0, false, err
	}
	return setToken{chars: chars}, consumed, true, nil
}

// parseEquivClass parses [=c=] at spec[pos].
// Under LC_ALL=C, equivalence class is just the character itself.
func parseEquivClass(spec string, pos int) (setToken, int, bool, error) {
	rest := spec[pos+2:]
	endIdx := strings.Index(rest, "=]")
	if endIdx < 0 || endIdx != 1 {
		return setToken{}, 0, false, nil
	}
	ch := rest[0]
	return setToken{chars: []byte{ch}}, 5, true, nil
}

// tryRepetition tries to parse [c*n] or [c*] at spec[pos].
// R1.3: repetition in SET specifications.
func tryRepetition(spec string, pos int) (setToken, int, bool, error) {
	ch, n := parseSingleChar(spec, pos+1)
	starIdx := pos + 1 + n
	if starIdx >= len(spec) || spec[starIdx] != '*' {
		return setToken{}, 0, false, nil
	}
	closeIdx := strings.IndexByte(spec[starIdx:], ']')
	if closeIdx < 0 {
		return setToken{}, 0, false, nil
	}
	numStr := spec[starIdx+1 : starIdx+closeIdx]
	consumed := starIdx + closeIdx - pos + 1
	return buildRepToken(ch, numStr, consumed)
}

// buildRepToken creates a repetition token from the char and count string.
func buildRepToken(ch byte, numStr string, consumed int) (setToken, int, bool, error) {
	if numStr == "" {
		return setToken{fill: ch, isFill: true}, consumed, true, nil
	}
	count, err := parseRepeatCount(numStr)
	if err != nil {
		return setToken{}, 0, false, nil
	}
	if count == 0 {
		return setToken{fill: ch, isFill: true}, consumed, true, nil
	}
	return setToken{chars: repeatByte(ch, count)}, consumed, true, nil
}

// parseRepeatCount parses a repeat count (octal if starts with 0).
func parseRepeatCount(s string) (int, error) {
	base := 10
	if len(s) > 1 && s[0] == '0' {
		base = 8
	}
	val, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

// parseSingleChar parses one character at spec[pos], handling escapes.
func parseSingleChar(spec string, pos int) (byte, int) {
	if pos >= len(spec) {
		return 0, 0
	}
	if spec[pos] == '\\' && pos+1 < len(spec) {
		return parseEscape(spec, pos)
	}
	return spec[pos], 1
}

// parseEscape parses a backslash escape sequence at spec[pos].
// R1.3: \n, \t, \\, \a, \b, \f, \r, \v, and \NNN octal escapes.
func parseEscape(spec string, pos int) (byte, int) {
	next := spec[pos+1]
	switch next {
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
	case '\\':
		return '\\', 2
	}
	if next >= '0' && next <= '7' {
		return parseOctal(spec, pos+1)
	}
	return next, 2
}

// parseOctal parses an octal escape starting at spec[start].
// Returns the byte value and total consumed count (including backslash).
func parseOctal(spec string, start int) (byte, int) {
	end := start
	for end < len(spec) && end-start < 3 && spec[end] >= '0' && spec[end] <= '7' {
		end++
	}
	val, _ := strconv.ParseUint(spec[start:end], 8, 32)
	return byte(val), 1 + (end - start)
}

// charClassBytes returns bytes in the named POSIX character class (C locale).
// R1.4: alnum, alpha, blank, cntrl, digit, graph, lower, print, punct,
// space, upper, xdigit.
func charClassBytes(name string) ([]byte, error) {
	switch name {
	case "upper":
		return byteRange('A', 'Z'), nil
	case "lower":
		return byteRange('a', 'z'), nil
	case "digit":
		return byteRange('0', '9'), nil
	case "xdigit":
		return concatSlices(byteRange('0', '9'), byteRange('A', 'F'), byteRange('a', 'f')), nil
	case "alpha":
		return concatSlices(byteRange('A', 'Z'), byteRange('a', 'z')), nil
	case "alnum":
		return concatSlices(byteRange('0', '9'), byteRange('A', 'Z'), byteRange('a', 'z')), nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "space":
		return []byte{'\t', '\n', '\v', '\f', '\r', ' '}, nil
	case "cntrl":
		return append(byteRange(0, 31), 127), nil
	case "print":
		return byteRange(32, 126), nil
	case "graph":
		return byteRange(33, 126), nil
	case "punct":
		return punctBytes(), nil
	default:
		return nil, fmt.Errorf("invalid character class '%s'", name)
	}
}

// punctBytes returns all printable non-alphanumeric bytes (C locale).
func punctBytes() []byte {
	var r []byte
	for i := 33; i <= 126; i++ {
		b := byte(i)
		if !isAlphaNum(b) {
			r = append(r, b)
		}
	}
	return r
}

// isAlphaNum returns true if b is alphanumeric in the C locale.
func isAlphaNum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// extendSet extends set to targetLen by repeating the last byte.
// R1.1: if SET2 is shorter than SET1, the last character of SET2 is repeated.
func extendSet(set []byte, targetLen int) []byte {
	if len(set) == 0 || len(set) >= targetLen {
		return set
	}
	last := set[len(set)-1]
	for len(set) < targetLen {
		set = append(set, last)
	}
	return set
}

// buildTable creates a 256-byte translation table mapping SET1 to SET2.
// R1.1: each character in SET1 maps to the corresponding character in SET2.
func buildTable(set1, set2 []byte) [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i, b := range set1 {
		if i < len(set2) {
			table[b] = set2[i]
		}
	}
	return table
}

// translateStream reads from r, applies the translation table, writes to w.
// R1.2: reads stdin and writes translated output to stdout.
func translateStream(r io.Reader, w io.Writer, table [256]byte) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	buf := make([]byte, 32*1024)
	for {
		n, err := br.Read(buf)
		translateBuf(buf[:n], table)
		if _, werr := bw.Write(buf[:n]); werr != nil {
			return werr
		}
		if err == io.EOF {
			return bw.Flush()
		}
		if err != nil {
			return err
		}
	}
}

// translateBuf applies the translation table to each byte in buf in-place.
func translateBuf(buf []byte, table [256]byte) {
	for i, b := range buf {
		buf[i] = table[b]
	}
}

// byteRange returns a slice of bytes from start to end inclusive.
func byteRange(start, end byte) []byte {
	if start > end {
		return nil
	}
	r := make([]byte, 0, int(end-start)+1)
	for i := int(start); i <= int(end); i++ {
		r = append(r, byte(i))
	}
	return r
}

// concatSlices concatenates multiple byte slices.
func concatSlices(slices ...[]byte) []byte {
	var result []byte
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// repeatByte returns a slice of count copies of b.
func repeatByte(b byte, count int) []byte {
	r := make([]byte, count)
	for i := range r {
		r[i] = b
	}
	return r
}
