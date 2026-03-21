// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd054-tr R1.1–R1.4: basic character translation, stdin/stdout I/O,
// SET specifications (ranges, escapes, repetition, POSIX character classes).
// Implements prd054-tr R2.1–R2.4: delete, squeeze, combined, and complement modes.
// Implements prd054-tr R3.1–R3.3: squeeze repeats, delete-complement, squeeze-complement.
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

// trConfig holds parsed command-line flags and operands.
type trConfig struct {
	delete     bool
	squeeze    bool
	complement bool
	sets       []string
}

// ioMode controls which transformations processIO applies.
type ioMode struct {
	table      *[256]byte // translate table; nil = no translation
	deleteSet  *[256]bool // delete membership; nil = no deletion
	squeezeSet *[256]bool // squeeze membership; nil = no squeezing
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and dispatches to the appropriate I/O mode.
// R1.1–R1.4: character translation. R2.1–R2.4: delete, squeeze, complement.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, code := parseFlags(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	set1, err := expandSet(cfg.sets[0], 0)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if cfg.complement {
		set1 = complementSet(set1)
	}
	return dispatchMode(cfg, set1, stdin, stdout, stderr)
}

// dispatchMode selects the I/O mode based on flags. R2.1–R2.3.
func dispatchMode(cfg *trConfig, set1 []byte, r io.Reader, w, stderr io.Writer) int {
	switch {
	case cfg.delete && cfg.squeeze:
		return execDeleteSqueeze(set1, cfg.sets[1], r, w, stderr)
	case cfg.delete:
		ds := buildMemberSet(set1)
		return processIO(r, w, ioMode{deleteSet: &ds})
	case cfg.squeeze && len(cfg.sets) == 1:
		ss := buildMemberSet(set1)
		return processIO(r, w, ioMode{squeezeSet: &ss})
	case cfg.squeeze:
		return execTranslateSqueeze(set1, cfg.sets[1], r, w, stderr)
	default:
		return execTranslate(set1, cfg.sets[1], r, w, stderr)
	}
}

// execTranslate expands SET2, builds a table, and runs plain translation.
func execTranslate(set1 []byte, spec string, r io.Reader, w, stderr io.Writer) int {
	set2, err := expandAndPad(set1, spec, stderr)
	if err != nil {
		return 1
	}
	table := buildTable(set1, set2)
	return processIO(r, w, ioMode{table: &table})
}

// execTranslateSqueeze translates and squeezes repeated SET2 characters. R2.2.
func execTranslateSqueeze(set1 []byte, spec string, r io.Reader, w, stderr io.Writer) int {
	set2, err := expandAndPad(set1, spec, stderr)
	if err != nil {
		return 1
	}
	table := buildTable(set1, set2)
	ss := buildMemberSet(set2)
	return processIO(r, w, ioMode{table: &table, squeezeSet: &ss})
}

// execDeleteSqueeze deletes SET1 characters and squeezes SET2 characters. R2.3.
func execDeleteSqueeze(set1 []byte, spec string, r io.Reader, w, stderr io.Writer) int {
	set2, err := expandSet(spec, 0)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	ds := buildMemberSet(set1)
	ss := buildMemberSet(set2)
	return processIO(r, w, ioMode{deleteSet: &ds, squeezeSet: &ss})
}

// expandAndPad expands SET2, validates non-empty, and pads to SET1 length.
func expandAndPad(set1 []byte, spec string, stderr io.Writer) ([]byte, error) {
	set2, err := expandSet(spec, len(set1))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return nil, err
	}
	if len(set2) == 0 {
		msg := "when not truncating set1, string2 must be non-empty"
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return nil, fmt.Errorf("%s", msg)
	}
	return padSet(set2, len(set1)), nil
}

// parseFlags extracts flags and operands from the command line.
func parseFlags(args []string, stdout, stderr io.Writer) (*trConfig, int) {
	cfg := &trConfig{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		switch arg {
		case "--help":
			printHelp(stdout)
			return nil, 0
		case "--version":
			printVersion(stdout)
			return nil, 0
		}
		if code := applyFlags(cfg, arg, stderr); code >= 0 {
			return nil, code
		}
		i++
	}
	cfg.sets = append(cfg.sets, args[i:]...)
	if code := validateOperands(cfg, stderr); code >= 0 {
		return nil, code
	}
	return cfg, -1
}

// applyFlags dispatches a flag argument to long or short flag parsing.
func applyFlags(cfg *trConfig, arg string, stderr io.Writer) int {
	if arg[1] == '-' {
		return applyLongFlag(cfg, arg, stderr)
	}
	return applyShortFlags(cfg, arg[1:], stderr)
}

// applyLongFlag handles --delete, --squeeze-repeats, --complement. R2.1–R2.4.
func applyLongFlag(cfg *trConfig, arg string, stderr io.Writer) int {
	switch arg {
	case "--delete":
		cfg.delete = true
	case "--squeeze-repeats":
		cfg.squeeze = true
	case "--complement":
		cfg.complement = true
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// applyShortFlags handles combined short flags like -ds, -cd. R2.1–R2.4.
func applyShortFlags(cfg *trConfig, chars string, stderr io.Writer) int {
	for _, ch := range chars {
		switch ch {
		case 'd':
			cfg.delete = true
		case 's':
			cfg.squeeze = true
		case 'c', 'C':
			cfg.complement = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, ch)
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// validateOperands checks operand count is valid for the active mode.
func validateOperands(cfg *trConfig, stderr io.Writer) int {
	n := len(cfg.sets)
	if n == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	if cfg.delete && !cfg.squeeze && n > 1 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, cfg.sets[1])
		fmt.Fprintf(stderr,
			"Only one string may be given when deleting without squeezing.\n")
		printTryHelp(stderr)
		return 1
	}
	need2 := (cfg.delete && cfg.squeeze) || (!cfg.delete && !cfg.squeeze)
	if need2 && n < 2 {
		fmt.Fprintf(stderr, "%s: missing operand after '%s'\n",
			progName, cfg.sets[0])
		printTryHelp(stderr)
		return 1
	}
	if n > 2 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, cfg.sets[2])
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// processIO reads from r, applies the ioMode transforms, and writes to w.
func processIO(r io.Reader, w io.Writer, mode ioMode) int {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	buf := make([]byte, 32*1024)
	prev := -1
	for {
		n, err := br.Read(buf)
		if werr := processBuf(bw, buf[:n], mode, &prev); werr != nil {
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

// processBuf applies delete, translate, and squeeze to a buffer chunk.
func processBuf(bw *bufio.Writer, buf []byte, mode ioMode, prev *int) error {
	for _, b := range buf {
		if mode.deleteSet != nil && mode.deleteSet[b] {
			continue
		}
		if mode.table != nil {
			b = mode.table[b]
		}
		if mode.squeezeSet != nil && mode.squeezeSet[b] && int(b) == *prev {
			continue
		}
		*prev = int(b)
		if err := bw.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

// buildMemberSet creates a 256-entry lookup table from a byte slice.
func buildMemberSet(set []byte) [256]bool {
	var m [256]bool
	for _, b := range set {
		m[b] = true
	}
	return m
}

// complementSet returns all byte values not in the given set, sorted. R2.4.
func complementSet(set []byte) []byte {
	member := buildMemberSet(set)
	var result []byte
	for i := range 256 {
		if !member[byte(i)] {
			result = append(result, byte(i))
		}
	}
	return result
}

// padSet extends set to targetLen by repeating its last character. R1.1.
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

// setParser holds state for parsing a SET specification string.
type setParser struct {
	spec    string
	pos     int
	fillLen int // target length for [c*] fill repetition
}

// expandSet parses a SET specification into its byte expansion.
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

// tryBracket attempts to parse a bracket construct at current position.
func (p *setParser) tryBracket(currentLen int) ([]byte, bool, error) {
	saved := p.pos
	p.pos++
	if p.pos >= len(p.spec) {
		p.pos = saved
		return nil, false, nil
	}
	if p.spec[p.pos] == ':' {
		return p.parseClass(saved)
	}
	return p.parseRepeat(saved, p.fillLen-currentLen)
}

// parseClass parses [:classname:] and returns expanded bytes. R1.4.
func (p *setParser) parseClass(saved int) ([]byte, bool, error) {
	p.pos++
	rest := p.spec[p.pos:]
	end := strings.Index(rest, ":]")
	if end < 0 {
		p.pos = saved
		return nil, false, nil
	}
	name := rest[:end]
	p.pos += end + 2
	chars, err := classBytes(name)
	if err != nil {
		return nil, false, err
	}
	return chars, true, nil
}

// parseRepeat parses [c*N] or [c*] repetition constructs. R1.3.
func (p *setParser) parseRepeat(saved, remaining int) ([]byte, bool, error) {
	ch, err := p.nextChar()
	if err != nil || p.pos >= len(p.spec) || p.spec[p.pos] != '*' {
		p.pos = saved
		return nil, false, nil
	}
	p.pos++
	closeIdx := strings.IndexByte(p.spec[p.pos:], ']')
	if closeIdx < 0 {
		p.pos = saved
		return nil, false, nil
	}
	countStr := p.spec[p.pos : p.pos+closeIdx]
	p.pos += closeIdx + 1
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

// tryRange checks for a range operator and expands it. R1.3.
func (p *setParser) tryRange(start byte) ([]byte, bool, error) {
	if p.pos >= len(p.spec) || p.spec[p.pos] != '-' || p.pos+1 >= len(p.spec) {
		return nil, false, nil
	}
	saved := p.pos
	p.pos++
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

// parseEscape handles backslash escape sequences. R1.3.
func (p *setParser) parseEscape() (byte, error) {
	p.pos++
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

// classBytes returns the bytes in a POSIX character class. R1.4.
func classBytes(name string) ([]byte, error) {
	switch name {
	case "upper":
		return byteRange('A', 'Z'), nil
	case "lower":
		return byteRange('a', 'z'), nil
	case "digit":
		return byteRange('0', '9'), nil
	case "xdigit":
		r := byteRange('0', '9')
		r = append(r, byteRange('A', 'F')...)
		return append(r, byteRange('a', 'f')...), nil
	case "alpha":
		return append(byteRange('A', 'Z'), byteRange('a', 'z')...), nil
	case "alnum":
		r := byteRange('0', '9')
		r = append(r, byteRange('A', 'Z')...)
		return append(r, byteRange('a', 'z')...), nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "cntrl":
		return append(byteRange(0, 31), 127), nil
	case "space":
		return append(byteRange(9, 13), ' '), nil
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

// punctBytes returns [:punct:] — printable non-alphanumeric non-space.
func punctBytes() []byte {
	var r []byte
	for c := byte(33); c <= 126; c++ {
		if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
			r = append(r, c)
		}
	}
	return r
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
