// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd054-tr R1.1–R1.4: basic character translation, stdin/stdout I/O,
// SET specifications (ranges, escapes, repetition, POSIX character classes).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tr"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and performs character translation.
// R1.1: translates SET1 chars to SET2 chars.
// R1.2: reads stdin, writes stdout.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	sets, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	set1, err := expandSet(sets[0], 0)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	set2, err := expandSet(sets[1], len(set1))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if len(set2) == 0 {
		fmt.Fprintf(stderr,
			"%s: when not truncating set1, string2 must be non-empty\n", progName)
		return 1
	}
	set2 = padSet(set2, len(set1))
	return translateIO(stdin, stdout, buildTable(set1, set2))
}

// parseArgs extracts SET1 and SET2 from command-line arguments.
// Returns (sets, -1) on success, (nil, exitCode) on terminal condition.
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
	var sets []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone {
			sets = append(sets, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--help" {
			printHelp(stdout)
			return nil, 0
		}
		if arg == "--version" {
			printVersion(stdout)
			return nil, 0
		}
		sets = append(sets, arg)
	}
	return validateSets(sets, stderr)
}

// validateSets checks that exactly two SET operands are provided.
func validateSets(sets []string, stderr io.Writer) ([]string, int) {
	if len(sets) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return nil, 1
	}
	if len(sets) == 1 {
		fmt.Fprintf(stderr, "%s: missing operand after '%s'\n",
			progName, sets[0])
		printTryHelp(stderr)
		return nil, 1
	}
	if len(sets) > 2 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n",
			progName, sets[2])
		printTryHelp(stderr)
		return nil, 1
	}
	return sets, -1
}

// padSet extends set to targetLen by repeating its last character.
// R1.1: if SET2 is shorter, last character of SET2 repeats.
func padSet(set []byte, targetLen int) []byte {
	if len(set) >= targetLen || len(set) == 0 {
		return set
	}
	last := set[len(set)-1]
	for len(set) < targetLen {
		set = append(set, last)
	}
	return set
}

// buildTable creates a 256-byte identity translation table and overrides
// entries where SET1 maps to SET2.
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

// translateIO reads from r, applies the translation table, and writes to w.
// R1.2: reads stdin, writes stdout.
func translateIO(r io.Reader, w io.Writer, table [256]byte) int {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	buf := make([]byte, 32*1024)
	for {
		n, err := br.Read(buf)
		for i := 0; i < n; i++ {
			buf[i] = table[buf[i]]
		}
		if _, werr := bw.Write(buf[:n]); werr != nil {
			return 1
		}
		if err != nil {
			break
		}
	}
	if err := bw.Flush(); err != nil {
		return 1
	}
	return 0
}

// setParser holds state for parsing a SET specification string.
type setParser struct {
	spec    string
	pos     int
	fillLen int // target length for [c*] fill repetition
}

// expandSet parses a SET specification into its byte expansion.
// fillLen is the target length for [c*] fill repetition.
func expandSet(spec string, fillLen int) ([]byte, error) {
	p := &setParser{spec: spec, fillLen: fillLen}
	return p.parse()
}

// parse iterates through the spec and expands all SET constructs.
// R1.3: ranges, escapes, repetition. R1.4: POSIX classes.
func (p *setParser) parse() ([]byte, error) {
	var result []byte
	for p.pos < len(p.spec) {
		if p.spec[p.pos] == '[' {
			if expanded, ok, err := p.tryBracket(len(result)); err != nil {
				return nil, err
			} else if ok {
				result = append(result, expanded...)
				continue
			}
		}
		ch, err := p.nextChar()
		if err != nil {
			return nil, err
		}
		if rng, ok, rerr := p.tryRange(ch); rerr != nil {
			return nil, rerr
		} else if ok {
			result = append(result, rng...)
		} else {
			result = append(result, ch)
		}
	}
	return result, nil
}

// tryBracket attempts to parse a bracket construct at the current position.
// Returns (bytes, true, nil) on success, (nil, false, nil) if not a bracket.
func (p *setParser) tryBracket(currentLen int) ([]byte, bool, error) {
	saved := p.pos
	p.pos++ // skip '['
	if p.pos >= len(p.spec) {
		p.pos = saved
		return nil, false, nil
	}
	if p.spec[p.pos] == ':' {
		return p.parseClass(saved)
	}
	return p.parseRepeat(saved, p.fillLen-currentLen)
}

// parseClass parses [:classname:] and returns the expanded bytes.
// R1.4: POSIX character classes.
func (p *setParser) parseClass(saved int) ([]byte, bool, error) {
	p.pos++ // skip ':'
	rest := p.spec[p.pos:]
	end := strings.Index(rest, ":]")
	if end < 0 {
		p.pos = saved
		return nil, false, nil
	}
	name := rest[:end]
	p.pos += end + 2 // skip past ":]"
	chars, err := classBytes(name)
	if err != nil {
		return nil, false, err
	}
	return chars, true, nil
}

// parseRepeat parses [c*N] or [c*] repetition constructs.
// R1.3: repetition in SET specifications.
func (p *setParser) parseRepeat(saved, remaining int) ([]byte, bool, error) {
	ch, err := p.nextChar()
	if err != nil || p.pos >= len(p.spec) || p.spec[p.pos] != '*' {
		p.pos = saved
		return nil, false, nil
	}
	p.pos++ // skip '*'
	closeIdx := strings.IndexByte(p.spec[p.pos:], ']')
	if closeIdx < 0 {
		p.pos = saved
		return nil, false, nil
	}
	countStr := p.spec[p.pos : p.pos+closeIdx]
	p.pos += closeIdx + 1 // skip past ']'
	count, cerr := repeatCount(countStr, remaining)
	if cerr != nil {
		return nil, false, cerr
	}
	result := make([]byte, count)
	for i := range result {
		result[i] = ch
	}
	return result, true, nil
}

// repeatCount interprets the count string for [c*N] or [c*].
func repeatCount(s string, remaining int) (int, error) {
	if s == "" {
		if remaining < 0 {
			return 0, nil
		}
		return remaining, nil
	}
	if len(s) > 1 && s[0] == '0' {
		n, err := strconv.ParseInt(s, 8, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid repeat count '%s'", s)
		}
		return int(n), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid repeat count '%s'", s)
	}
	return n, nil
}

// tryRange checks if the current position has a range operator and expands it.
// R1.3: range notation (e.g., a-z) in SET specifications.
func (p *setParser) tryRange(start byte) ([]byte, bool, error) {
	if p.pos >= len(p.spec) || p.spec[p.pos] != '-' {
		return nil, false, nil
	}
	if p.pos+1 >= len(p.spec) {
		return nil, false, nil
	}
	saved := p.pos
	p.pos++ // skip '-'
	end, err := p.nextChar()
	if err != nil {
		p.pos = saved
		return nil, false, nil
	}
	if start > end {
		return nil, false, fmt.Errorf(
			"range-endpoints of '%c-%c' are in reverse collating sequence order",
			rune(start), rune(end))
	}
	return byteRange(start, end), true, nil
}

// nextChar reads the next character from the spec, handling escapes.
func (p *setParser) nextChar() (byte, error) {
	if p.pos >= len(p.spec) {
		return 0, fmt.Errorf("unexpected end of string")
	}
	if p.spec[p.pos] != '\\' {
		ch := p.spec[p.pos]
		p.pos++
		return ch, nil
	}
	return p.parseEscape()
}

// parseEscape handles backslash escape sequences.
// R1.3: \n, \t, \\, \a, \b, \f, \r, \v, \NNN (octal).
func (p *setParser) parseEscape() (byte, error) {
	p.pos++ // skip backslash
	if p.pos >= len(p.spec) {
		return '\\', nil
	}
	ch := p.spec[p.pos]
	p.pos++
	return p.resolveEscape(ch)
}

// resolveEscape maps an escape character to its byte value.
func (p *setParser) resolveEscape(ch byte) (byte, error) {
	switch ch {
	case 'a':
		return '\a', nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'v':
		return '\v', nil
	case '\\':
		return '\\', nil
	}
	if ch >= '0' && ch <= '7' {
		return p.parseOctal(ch)
	}
	return ch, nil
}

// parseOctal parses up to 3 octal digits into a byte value.
func (p *setParser) parseOctal(first byte) (byte, error) {
	val := int(first - '0')
	for i := 0; i < 2 && p.pos < len(p.spec); i++ {
		if p.spec[p.pos] < '0' || p.spec[p.pos] > '7' {
			break
		}
		val = val*8 + int(p.spec[p.pos]-'0')
		p.pos++
	}
	if val > 255 {
		return 0, fmt.Errorf("octal value is out of range")
	}
	return byte(val), nil
}

// classBytes returns the bytes in a POSIX character class in ascending order.
// R1.4: all standard POSIX classes under LC_ALL=C.
func classBytes(name string) ([]byte, error) {
	switch name {
	case "upper":
		return byteRange('A', 'Z'), nil
	case "lower":
		return byteRange('a', 'z'), nil
	case "digit":
		return byteRange('0', '9'), nil
	case "xdigit":
		return xdigitBytes(), nil
	case "alpha":
		return alphaBytes(), nil
	case "alnum":
		return alnumBytes(), nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "cntrl":
		return ctrlBytes(), nil
	case "space":
		return spaceBytes(), nil
	case "graph":
		return byteRange(33, 126), nil
	case "print":
		return byteRange(32, 126), nil
	case "punct":
		return punctBytes(), nil
	default:
		return nil, fmt.Errorf("invalid character class '%s'", name)
	}
}

// byteRange returns bytes from start to end inclusive.
func byteRange(start, end byte) []byte {
	result := make([]byte, 0, int(end-start)+1)
	for c := start; c <= end; c++ {
		result = append(result, c)
	}
	return result
}

// xdigitBytes returns [:xdigit:] in byte order: 0-9, A-F, a-f.
func xdigitBytes() []byte {
	r := byteRange('0', '9')
	r = append(r, byteRange('A', 'F')...)
	return append(r, byteRange('a', 'f')...)
}

// alphaBytes returns [:alpha:] in byte order: A-Z, a-z.
func alphaBytes() []byte {
	return append(byteRange('A', 'Z'), byteRange('a', 'z')...)
}

// alnumBytes returns [:alnum:] in byte order: 0-9, A-Z, a-z.
func alnumBytes() []byte {
	r := byteRange('0', '9')
	r = append(r, byteRange('A', 'Z')...)
	return append(r, byteRange('a', 'z')...)
}

// ctrlBytes returns [:cntrl:] in byte order: 0-31, 127.
func ctrlBytes() []byte {
	return append(byteRange(0, 31), 127)
}

// spaceBytes returns [:space:]: \t, \n, \v, \f, \r, space.
func spaceBytes() []byte {
	return append(byteRange(9, 13), ' ')
}

// punctBytes returns [:punct:] — printable non-alphanumeric non-space.
func punctBytes() []byte {
	var r []byte
	for c := byte(33); c <= 126; c++ {
		if !isAlnum(c) {
			r = append(r, c)
		}
	}
	return r
}

// isAlnum returns true if c is alphanumeric in the C locale.
func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z')
}

// printHelp writes usage information.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SET1 [SET2]\n", progName)
	fmt.Fprintln(w, "Translate, squeeze, and/or delete characters from standard input,")
	fmt.Fprintln(w, "writing to standard output.")
}

// printVersion writes version information.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
