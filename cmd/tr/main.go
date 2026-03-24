// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd054-tr R1.1–R1.4, R2.1–R2.2: core character translation.
//
// Covers basic SET1→SET2 translation, SET expansion (ranges, octal and
// backslash escapes, POSIX classes, repetition, equivalence classes),
// SET2 padding, and binary data handling via stdin/stdout.
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line state.
type config struct {
	delete     bool
	squeeze    bool
	complement bool
	truncate   bool
	set1       string
	set2       string
}

func main() {
	// D1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()
	cfg, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg))
}

// --- Dispatch ---

// run expands SET specifications and dispatches to translate mode.
// R1.1: translates each SET1 byte to the corresponding SET2 byte.
func run(cfg config) int {
	if cfg.delete {
		fmt.Fprintln(os.Stderr, "tr: delete mode not yet supported")
		return 1
	}
	if cfg.squeeze {
		fmt.Fprintln(os.Stderr, "tr: squeeze mode not yet supported")
		return 1
	}
	set1, err := expandSet(cfg.set1, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tr: %v\n", err)
		return 1
	}
	set2, err := expandSet(cfg.set2, len(set1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tr: %v\n", err)
		return 1
	}
	return runTranslate(set1, set2, cfg)
}

// runTranslate performs byte-by-byte translation from SET1 to SET2.
// R1.1: maps SET1[i] to SET2[i].
// R1.3: pads SET2 by repeating its last character.
func runTranslate(set1, set2 []byte, cfg config) int {
	if len(set2) == 0 {
		fmt.Fprintln(os.Stderr,
			"tr: when not truncating set1, string2 must be non-empty")
		return 1
	}
	if cfg.complement {
		set1 = complementSet(set1)
	}
	set2 = padSet2(set1, set2)
	table := buildTransTable(set1, set2)
	if err := processStdin(table); err != nil {
		fmt.Fprintf(os.Stderr, "tr: write error: %v\n", err)
		return 1
	}
	return 0
}

// --- Translation table and I/O ---

// buildTransTable creates a 256-byte translation table.
// R1.1: identity map with SET1[i] → SET2[i] overlaid.
func buildTransTable(set1, set2 []byte) [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i, c := range set1 {
		if i < len(set2) {
			table[c] = set2[i]
		}
	}
	return table
}

// padSet2 extends SET2 by repeating its last character to match SET1.
// R1.3: the last character of SET2 fills the gap.
func padSet2(set1, set2 []byte) []byte {
	if len(set2) >= len(set1) || len(set2) == 0 {
		return set2
	}
	last := set2[len(set2)-1]
	padded := make([]byte, len(set1))
	copy(padded, set2)
	for i := len(set2); i < len(set1); i++ {
		padded[i] = last
	}
	return padded
}

// complementSet returns all byte values NOT in the given set, in order.
func complementSet(set []byte) []byte {
	var inSet [256]bool
	for _, c := range set {
		inSet[c] = true
	}
	var result []byte
	for i := 0; i < 256; i++ {
		if !inSet[byte(i)] {
			result = append(result, byte(i))
		}
	}
	return result
}

// processStdin reads stdin, translates via table, writes to stdout.
// R1.4: handles all 256 byte values (binary data).
// D2: uses buffered I/O for performance.
func processStdin(table [256]byte) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		translateBuf(buf[:n], table)
		if _, wErr := writer.Write(buf[:n]); wErr != nil {
			return wErr
		}
		if err != nil {
			if err == io.EOF {
				return writer.Flush()
			}
			return err
		}
	}
}

// translateBuf applies the translation table to each byte in-place.
func translateBuf(buf []byte, table [256]byte) {
	for i, b := range buf {
		buf[i] = table[b]
	}
}

// --- SET expansion ---

// expandSet expands a SET specification string into a byte slice.
// R1.2: handles ranges (a-z), POSIX classes, repetition, equivalence.
// R2.1: handles octal escapes (\NNN).
// R2.2: handles backslash escapes (\n, \t, \\, etc.).
func expandSet(spec string, otherLen int) ([]byte, error) {
	var result []byte
	i := 0
	for i < len(spec) {
		n, chars, err := tryBracketExpr(spec, i, otherLen, len(result))
		if err != nil {
			return nil, err
		}
		if n > 0 {
			result = append(result, chars...)
			i += n
			continue
		}
		ch, consumed := nextChar(spec, i)
		i += consumed
		if isRangeDash(spec, i) {
			end, ec := nextChar(spec, i+1)
			result = append(result, rangeBytes(ch, end)...)
			i += 1 + ec
			continue
		}
		result = append(result, ch)
	}
	return result, nil
}

// isRangeDash checks if position i in spec starts a range delimiter
// (a dash followed by a non-bracket character).
func isRangeDash(spec string, i int) bool {
	return i+1 < len(spec) && spec[i] == '-' && spec[i+1] != '['
}

// nextChar reads the next byte from spec at position i, handling escapes.
func nextChar(spec string, i int) (byte, int) {
	if i >= len(spec) {
		return 0, 0
	}
	if spec[i] == '\\' && i+1 < len(spec) {
		return parseEscape(spec, i)
	}
	return spec[i], 1
}

// --- Escape parsing ---

// escapeMap maps backslash-escaped characters to their byte values.
// R2.2: \\, \a, \b, \f, \n, \r, \t, \v.
var escapeMap = map[byte]byte{
	'\\': '\\',
	'a':  '\a',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'v':  '\v',
}

// parseEscape handles backslash sequences starting at spec[i].
// R2.2: named escapes. R2.1: octal escapes.
func parseEscape(spec string, i int) (byte, int) {
	ch := spec[i+1]
	if esc, ok := escapeMap[ch]; ok {
		return esc, 2
	}
	if ch >= '0' && ch <= '7' {
		return parseOctal(spec, i)
	}
	return ch, 2
}

// parseOctal parses a \NNN octal escape starting at spec[i].
// R2.1: reads 1–3 octal digits after the backslash.
func parseOctal(spec string, i int) (byte, int) {
	start := i + 1
	end := start
	for end < len(spec) && end < start+3 && spec[end] >= '0' && spec[end] <= '7' {
		end++
	}
	val := 0
	for j := start; j < end; j++ {
		val = val*8 + int(spec[j]-'0')
	}
	if val > 255 {
		val = 255
	}
	return byte(val), end - i
}

// --- Bracket expressions ---

// tryBracketExpr attempts to parse a bracket expression at spec[i].
// Returns (consumed, chars, error). consumed=0 means no bracket expr.
func tryBracketExpr(spec string, i, otherLen, curLen int) (int, []byte, error) {
	if spec[i] != '[' || i+1 >= len(spec) {
		return 0, nil, nil
	}
	switch spec[i+1] {
	case ':':
		return parsePosixClass(spec, i)
	case '=':
		return parseEquivClass(spec, i)
	default:
		return parseRepeat(spec, i, otherLen, curLen)
	}
}

// parsePosixClass parses [:classname:] at spec[i].
// R1.2: POSIX character classes under LC_ALL=C.
func parsePosixClass(spec string, i int) (int, []byte, error) {
	rest := spec[i:]
	end := strings.Index(rest, ":]")
	if end < 3 {
		return 0, nil, nil
	}
	className := rest[2:end]
	chars, err := posixClassChars(className)
	if err != nil {
		return 0, nil, err
	}
	return end + 2, chars, nil
}

// parseEquivClass parses [=c=] at spec[i].
// R1.2: equivalence classes (identity mapping under LC_ALL=C).
func parseEquivClass(spec string, i int) (int, []byte, error) {
	rest := spec[i:]
	if len(rest) < 5 || rest[3] != '=' || rest[4] != ']' {
		return 0, nil, nil
	}
	return 5, []byte{rest[2]}, nil
}

// parseRepeat parses [c*n] or [c*] at spec[i].
// R1.2: repeated character expansion.
func parseRepeat(spec string, i, otherLen, curLen int) (int, []byte, error) {
	rest := spec[i:]
	if len(rest) < 4 {
		return 0, nil, nil
	}
	pos := 1
	ch, consumed := nextChar(rest, pos)
	pos += consumed
	if pos >= len(rest) || rest[pos] != '*' {
		return 0, nil, nil
	}
	pos++
	endBracket := strings.IndexByte(rest[pos:], ']')
	if endBracket < 0 {
		return 0, nil, nil
	}
	countStr := rest[pos : pos+endBracket]
	total := pos + endBracket + 1
	count, err := resolveRepeatCount(countStr, otherLen, curLen)
	if err != nil {
		return 0, nil, err
	}
	return total, repeatByte(ch, count), nil
}

// resolveRepeatCount parses the count in [c*n] or infers it for [c*].
// A count starting with "0" (and longer than 1 digit) is parsed as octal.
func resolveRepeatCount(s string, otherLen, curLen int) (int, error) {
	if s == "" {
		count := otherLen - curLen
		if count < 0 {
			count = 0
		}
		return count, nil
	}
	if strings.HasPrefix(s, "0") && len(s) > 1 {
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

// repeatByte returns a slice of n copies of ch.
func repeatByte(ch byte, n int) []byte {
	result := make([]byte, n)
	for i := range result {
		result[i] = ch
	}
	return result
}

// --- POSIX character classes ---

// posixClassChars returns the bytes belonging to a named POSIX class
// under LC_ALL=C.
func posixClassChars(name string) ([]byte, error) {
	switch name {
	case "upper":
		return rangeBytes('A', 'Z'), nil
	case "lower":
		return rangeBytes('a', 'z'), nil
	case "digit":
		return rangeBytes('0', '9'), nil
	case "xdigit":
		return xdigitChars(), nil
	case "alpha":
		return alphaChars(), nil
	case "alnum":
		return alnumChars(), nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "space":
		return []byte{'\t', '\n', '\v', '\f', '\r', ' '}, nil
	case "cntrl":
		return ctrlChars(), nil
	case "graph":
		return rangeBytes(0x21, 0x7E), nil
	case "print":
		return rangeBytes(0x20, 0x7E), nil
	case "punct":
		return punctChars(), nil
	default:
		return nil, fmt.Errorf("invalid character class '%s'", name)
	}
}

// rangeBytes returns all byte values from lo to hi inclusive.
func rangeBytes(lo, hi byte) []byte {
	if lo > hi {
		return nil
	}
	result := make([]byte, 0, int(hi-lo)+1)
	for c := lo; ; c++ {
		result = append(result, c)
		if c == hi {
			break
		}
	}
	return result
}

// alphaChars returns [:alpha:] under LC_ALL=C.
func alphaChars() []byte {
	result := rangeBytes('A', 'Z')
	return append(result, rangeBytes('a', 'z')...)
}

// alnumChars returns [:alnum:] under LC_ALL=C.
func alnumChars() []byte {
	result := rangeBytes('0', '9')
	result = append(result, rangeBytes('A', 'Z')...)
	return append(result, rangeBytes('a', 'z')...)
}

// xdigitChars returns [:xdigit:] under LC_ALL=C.
func xdigitChars() []byte {
	result := rangeBytes('0', '9')
	result = append(result, rangeBytes('A', 'F')...)
	return append(result, rangeBytes('a', 'f')...)
}

// ctrlChars returns [:cntrl:] under LC_ALL=C.
func ctrlChars() []byte {
	result := rangeBytes(0, 0x1F)
	return append(result, 0x7F)
}

// punctChars returns [:punct:] under LC_ALL=C.
func punctChars() []byte {
	var result []byte
	for c := byte(0x21); c <= byte(0x7E); c++ {
		if !isAlnum(c) {
			result = append(result, c)
		}
	}
	return result
}

// isAlnum reports whether c is alphanumeric under LC_ALL=C.
func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z')
}

// --- Argument parsing ---

// parseArgs processes command-line arguments and returns config.
// Exit code -1 means continue; >= 0 means exit immediately.
func parseArgs(args []string) (config, int) {
	var cfg config
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		exit := handleArg(arg, args, &i, &cfg, &positional)
		if exit >= 0 {
			return config{}, exit
		}
	}
	return assignSets(cfg, positional)
}

// handleArg dispatches a single argument to the appropriate handler.
// Returns -1 to continue, >= 0 to exit.
func handleArg(
	arg string, args []string, i *int, cfg *config, positional *[]string,
) int {
	switch {
	case arg == "--":
		*positional = append(*positional, args[*i+1:]...)
		*i = len(args)
		return -1
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "--delete":
		cfg.delete = true
		return -1
	case arg == "--squeeze-repeats":
		cfg.squeeze = true
		return -1
	case arg == "--complement":
		cfg.complement = true
		return -1
	case arg == "--truncate-set1":
		cfg.truncate = true
		return -1
	case strings.HasPrefix(arg, "-") && arg != "-":
		return parseShortFlags(arg[1:], cfg)
	default:
		*positional = append(*positional, arg)
		return -1
	}
}

// parseShortFlags processes clustered short flags (e.g., -cs).
func parseShortFlags(flags string, cfg *config) int {
	for _, ch := range flags {
		switch ch {
		case 'c', 'C':
			cfg.complement = true
		case 'd':
			cfg.delete = true
		case 's':
			cfg.squeeze = true
		case 't':
			cfg.truncate = true
		default:
			fmt.Fprintf(os.Stderr, "tr: invalid option -- '%c'\n", ch)
			return 1
		}
	}
	return -1
}

// assignSets assigns positional arguments to SET1 and SET2.
// R3.2: translate mode requires non-empty SET2.
func assignSets(cfg config, positional []string) (config, int) {
	switch len(positional) {
	case 0:
		fmt.Fprintln(os.Stderr, "tr: missing operand")
		return config{}, 1
	case 1:
		if !cfg.delete && !cfg.squeeze {
			fmt.Fprintf(os.Stderr,
				"tr: missing operand after '%s'\n", positional[0])
			return config{}, 1
		}
		cfg.set1 = positional[0]
	case 2:
		cfg.set1 = positional[0]
		cfg.set2 = positional[1]
	default:
		fmt.Fprintf(os.Stderr, "tr: extra operand '%s'\n", positional[2])
		return config{}, 1
	}
	return cfg, -1
}

// --- Help and version ---

// printHelp writes usage information and returns exit code 0.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: tr [OPTION]... SET1 [SET2]
Translate, squeeze, and/or delete characters from standard input,
writing to standard output.

  -c, -C, --complement    use the complement of SET1
  -d, --delete             delete characters in SET1, do not translate
  -s, --squeeze-repeats    replace each sequence of a repeated character
                             that is listed in the last specified SET,
                             with a single occurrence of that character
  -t, --truncate-set1      first truncate SET1 to length of SET2
      --help     display this help and exit
      --version  output version information and exit
`)
	return 0
}

// printVersion writes version information and returns exit code 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "tr (go-unix-utils) %s\n", version)
	return 0
}
