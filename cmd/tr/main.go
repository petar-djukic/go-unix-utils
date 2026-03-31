// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tr implements GNU tr: translate or delete characters.
//
// Implements prd054-tr R1.1 (character translation), R1.2 (stdin/stdout I/O),
// R1.3 (SET specifications: ranges, escapes, repetition),
// R1.4 (POSIX character classes), R2.1 (delete mode), R2.2 (squeeze mode),
// R2.3 (delete+squeeze), R2.4 (complement mode), R3.1 (class case conversion),
// R3.2 (empty SET2 validation), R3.3 (equivalence classes),
// R4.1 (exit 0 on success), R4.2 (exit 1 on usage errors).
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

// noTryError is a semantic validation error that should not trigger
// the "Try --help" message, matching GNU tr behavior.
type noTryError struct {
	msg string
}

func (e *noTryError) Error() string { return e.msg }

// config holds parsed command-line flags.
// R2.1: delete, R2.2: squeeze, R2.4: complement.
type config struct {
	delete     bool
	squeeze    bool
	complement bool
}

// streamConfig holds the configuration for stream processing.
type streamConfig struct {
	doDelete   bool
	doSqueeze  bool
	table      [256]byte
	deleteSet  [256]bool
	squeezeSet [256]bool
}

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

// run parses arguments and performs character translation/deletion/squeezing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, set1Spec, set2Spec, err := parseConfig(args)
	if err != nil {
		return handleParseError(err, stdout, stderr)
	}
	set1, set2, err := expandSets(cfg, set1Spec, set2Spec)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	sc := buildStreamConfig(cfg, set1, set2)
	if err := processStream(stdin, stdout, &sc); err != nil {
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
	var nte *noTryError
	if !errors.As(err, &nte) {
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName)
	}
	return 1
}

// printUsage writes usage information.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SET1 [SET2]\n", programName)
	fmt.Fprintln(w, "Translate, squeeze, and/or delete characters from standard input,")
	fmt.Fprintln(w, "writing to standard output.")
}

// parseConfig extracts flags and SET specifications from arguments.
func parseConfig(args []string) (config, string, string, error) {
	cfg, positional, err := parseFlags(args)
	if err != nil {
		return cfg, "", "", err
	}
	set1, set2, err := validatePositional(cfg, positional)
	return cfg, set1, set2, err
}

// parseFlags separates flags from positional arguments.
func parseFlags(args []string) (config, []string, error) {
	var cfg config
	var positional []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if err := applyFlag(arg, &cfg); err != nil {
			return cfg, nil, err
		}
	}
	return cfg, positional, nil
}

// applyFlag applies a single flag argument to the config.
func applyFlag(arg string, cfg *config) error {
	switch arg {
	case "--help":
		return errHelp
	case "--version":
		return errVersion
	case "--delete":
		cfg.delete = true
	case "--squeeze-repeats":
		cfg.squeeze = true
	case "--complement":
		cfg.complement = true
	default:
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		return parseShortFlags(arg[1:], cfg)
	}
	return nil
}

// parseShortFlags parses combined short flags like -ds, -cds.
// R2.1: -d, R2.2: -s, R2.4: -c/-C.
func parseShortFlags(flags string, cfg *config) error {
	for _, ch := range flags {
		switch ch {
		case 'd':
			cfg.delete = true
		case 's':
			cfg.squeeze = true
		case 'c', 'C':
			cfg.complement = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// validatePositional checks positional argument count for the given mode.
func validatePositional(cfg config, pos []string) (string, string, error) {
	if len(pos) == 0 {
		return "", "", fmt.Errorf("missing operand")
	}
	if cfg.delete && !cfg.squeeze {
		return validateDeleteOnly(pos)
	}
	if cfg.squeeze && !cfg.delete && len(pos) == 1 {
		return pos[0], "", nil
	}
	if len(pos) < 2 {
		return "", "", fmt.Errorf("missing operand after '%s'\n"+
			"Two strings must be given when translating.", pos[0])
	}
	if len(pos) > 2 {
		return "", "", fmt.Errorf("extra operand '%s'", pos[2])
	}
	// R3.2: when translating (not deleting), SET2 must not be empty.
	if !cfg.delete && pos[1] == "" {
		return "", "", &noTryError{
			msg: "when not truncating set1, string2 must be non-empty"}
	}
	// R3.3: equivalence classes may not appear in SET2 when translating.
	if !cfg.delete && containsEquivClass(pos[1]) {
		return "", "", &noTryError{
			msg: "[=c=] expressions may not appear in string2 when translating"}
	}
	return pos[0], pos[1], nil
}

// validateDeleteOnly validates positional args for -d without -s.
// R2.1: SET2 is not used with -d alone.
func validateDeleteOnly(pos []string) (string, string, error) {
	if len(pos) > 1 {
		return "", "", fmt.Errorf(
			"extra operand '%s'\nOnly one string may be given when "+
				"deleting without squeezing repeats.", pos[1])
	}
	return pos[0], "", nil
}

// expandSets parses and expands SET1 and SET2 specifications.
// R2.4: complements SET1 when -c is set.
func expandSets(cfg config, set1Spec, set2Spec string) ([]byte, []byte, error) {
	set1, err := expandSet(set1Spec, 0)
	if err != nil {
		return nil, nil, err
	}
	if cfg.complement {
		set1 = complementSet(set1)
	}
	set2, err := expandSet(set2Spec, len(set1))
	if err != nil {
		return nil, nil, err
	}
	if !cfg.delete && len(set2) > 0 {
		set2 = extendSet(set2, len(set1))
	}
	return set1, set2, nil
}

// complementSet returns all bytes 0-255 not in the given set.
// R2.4: complement operation for SET1.
func complementSet(set []byte) []byte {
	var present [256]bool
	for _, b := range set {
		present[b] = true
	}
	var result []byte
	for i := 0; i < 256; i++ {
		if !present[byte(i)] {
			result = append(result, byte(i))
		}
	}
	return result
}

// buildStreamConfig creates processing configuration from parsed sets.
// R2.1: delete mode, R2.2: squeeze mode, R2.3: combined delete+squeeze.
func buildStreamConfig(cfg config, set1, set2 []byte) streamConfig {
	var sc streamConfig
	if cfg.delete {
		sc.doDelete = true
		sc.deleteSet = memberSet(set1)
	} else {
		sc.table = buildTable(set1, set2)
	}
	if cfg.squeeze {
		sc.doSqueeze = true
		if len(set2) > 0 {
			sc.squeezeSet = memberSet(set2)
		} else {
			sc.squeezeSet = memberSet(set1)
		}
	}
	return sc
}

// memberSet returns a 256-bool array marking membership.
func memberSet(set []byte) [256]bool {
	var m [256]bool
	for _, b := range set {
		m[b] = true
	}
	return m
}

// processStream reads from r, applies the stream config, writes to w.
// R1.2: reads stdin and writes to stdout.
func processStream(r io.Reader, w io.Writer, sc *streamConfig) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	buf := make([]byte, 32*1024)
	prev := -1
	for {
		n, err := br.Read(buf)
		var werr error
		prev, werr = processChunk(bw, buf[:n], sc, prev)
		if werr != nil {
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

// processChunk processes a buffer chunk through the stream configuration.
// Handles translate, delete, and squeeze in a single pass.
func processChunk(w *bufio.Writer, buf []byte, sc *streamConfig, prev int) (int, error) {
	for _, b := range buf {
		if sc.doDelete && sc.deleteSet[b] {
			continue
		}
		out := b
		if !sc.doDelete {
			out = sc.table[b]
		}
		if sc.doSqueeze && sc.squeezeSet[out] && int(out) == prev {
			continue
		}
		if err := w.WriteByte(out); err != nil {
			return prev, err
		}
		prev = int(out)
	}
	return prev, nil
}

// --- SET expansion ---

// expandSet parses a SET specification and returns expanded bytes.
// targetLen controls [c*] expansion; pass 0 for SET1.
func expandSet(spec string, targetLen int) ([]byte, error) {
	if spec == "" {
		return nil, nil
	}
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

// containsEquivClass returns true if spec contains a [=c=] equivalence class.
// R3.3: used to reject equivalence classes in SET2 when translating.
func containsEquivClass(spec string) bool {
	for i := range spec {
		if i+4 < len(spec) && spec[i] == '[' && spec[i+1] == '=' &&
			spec[i+3] == '=' && spec[i+4] == ']' {
			return true
		}
	}
	return false
}

// --- Byte utilities ---

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
