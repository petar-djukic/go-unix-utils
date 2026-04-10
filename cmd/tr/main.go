// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tr: translate or delete characters.
// Implements srd054-tr R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1-R4.2
// (scaffold and flag parsing with SET expansion).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "tr"

// trMode distinguishes the operating mode of tr.
type trMode int

const (
	modeTranslate trMode = iota
	modeDelete
	modeSqueeze
	modeDeleteSqueeze
)

// config holds parsed command-line options.
type config struct {
	delete     bool
	squeeze    bool
	complement bool
	truncate   bool
	sets       []string
	mode       trMode
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the tr logic and returns the exit code.
// R4.1: returns 0 on success. R4.2: returns 1 on usage errors.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := validateOperands(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	cfg.mode = resolveMode(cfg.delete, cfg.squeeze)
	// Stub: actual translation/delete/squeeze logic not yet implemented.
	fmt.Fprintf(os.Stderr, "%s: translation not implemented\n", progName)
	return 2
}

// resolveMode determines the operating mode from flag combination.
func resolveMode(del, squeeze bool) trMode {
	switch {
	case del && squeeze:
		return modeDeleteSqueeze
	case del:
		return modeDelete
	case squeeze:
		return modeSqueeze
	default:
		return modeTranslate
	}
}

// parseArgs extracts flags and SET operands from command-line arguments.
// R2.1: -d/--delete. R2.2: -s/--squeeze-repeats.
// R2.4: -c/-C/--complement. R1.3: -t/--truncate-set1.
func parseArgs(args []string) (config, error) {
	var cfg config
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.sets = append(cfg.sets, args[i+1:]...)
			return cfg, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.sets = append(cfg.sets, args[i:]...)
			return cfg, nil
		}
		if parsed := parseLongFlag(&cfg, arg); parsed {
			i++
			continue
		}
		if err := parseShortFlags(&cfg, arg); err != nil {
			return cfg, err
		}
		i++
	}
	return cfg, nil
}

// parseLongFlag handles GNU-style long flags. Returns true if consumed.
func parseLongFlag(cfg *config, arg string) bool {
	switch arg {
	case "--delete":
		cfg.delete = true
	case "--squeeze-repeats":
		cfg.squeeze = true
	case "--complement":
		cfg.complement = true
	case "--truncate-set1":
		cfg.truncate = true
	default:
		return false
	}
	return true
}

// parseShortFlags handles single-letter flags, possibly combined (e.g. -ds).
func parseShortFlags(cfg *config, arg string) error {
	for _, c := range arg[1:] {
		switch c {
		case 'd':
			cfg.delete = true
		case 's':
			cfg.squeeze = true
		case 'c', 'C':
			cfg.complement = true
		case 't':
			cfg.truncate = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// validateOperands checks that the correct number of SET operands are given.
// R3.2: translate mode requires exactly two SETs.
// R2.1: -d without -s requires exactly one SET.
// R2.3: -d with -s requires exactly two SETs.
// R2.2: -s alone requires one or two SETs.
func validateOperands(cfg *config) error {
	n := len(cfg.sets)
	del, squeeze := cfg.delete, cfg.squeeze
	switch {
	case del && squeeze:
		if n != 2 {
			return fmt.Errorf("two strings must be given when both deleting and squeezing repeats")
		}
	case del:
		if n != 1 {
			return fmt.Errorf("extra operand '%s'", extraOperand(cfg.sets, 1))
		}
	case squeeze:
		if n < 1 || n > 2 {
			return missingOrExtraError(n, 1, 2, cfg.sets)
		}
	default:
		if n != 2 {
			if n < 2 {
				return fmt.Errorf("missing operand")
			}
			return fmt.Errorf("extra operand '%s'", extraOperand(cfg.sets, 2))
		}
	}
	return validateSETs(cfg.sets)
}

// extraOperand returns the first extra operand beyond limit.
func extraOperand(sets []string, limit int) string {
	if limit < len(sets) {
		return sets[limit]
	}
	return ""
}

// missingOrExtraError produces the appropriate error for wrong operand count.
func missingOrExtraError(n, min, max int, sets []string) error {
	if n < min {
		return fmt.Errorf("missing operand")
	}
	return fmt.Errorf("extra operand '%s'", extraOperand(sets, max))
}

// validateSETs expands each SET to verify it is syntactically valid.
func validateSETs(sets []string) error {
	for _, s := range sets {
		if _, err := expandSet(s); err != nil {
			return err
		}
	}
	return nil
}

// expandSet parses a SET specification string and returns the expanded bytes.
// R1.3: supports ranges (a-z), octal escapes (\NNN), backslash escapes,
// and repeat notation ([c*n]).
// R1.4: supports POSIX character classes ([:alpha:], [:digit:], etc.).
func expandSet(s string) ([]byte, error) {
	var result []byte
	i := 0
	for i < len(s) {
		consumed, expanded, err := expandElement(s, i)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
		i += consumed
	}
	return result, nil
}

// expandElement parses one element at position i in the SET string.
// Returns bytes consumed, expanded bytes, and any error.
func expandElement(s string, i int) (int, []byte, error) {
	if s[i] == '[' {
		return expandBracket(s, i)
	}
	if s[i] == '\\' {
		return expandEscape(s, i)
	}
	// Check for range: current char, dash, next char.
	if i+2 < len(s) && s[i+1] == '-' && s[i+2] != ']' {
		return expandRange(s[i], s[i+2])
	}
	return 1, []byte{s[i]}, nil
}

// expandBracket handles bracket expressions: character classes ([:class:]),
// equivalence classes ([=c=]), and repeat notation ([c*n]).
func expandBracket(s string, i int) (int, []byte, error) {
	if end := matchCharClass(s, i); end > 0 {
		return expandCharClass(s, i, end)
	}
	if end := matchEquivClass(s, i); end > 0 {
		return expandEquivClass(s, i, end)
	}
	if end := matchRepeat(s, i); end > 0 {
		return expandRepeat(s, i, end)
	}
	// Literal '['.
	return 1, []byte{'['}, nil
}

// matchCharClass checks if s[i:] starts with "[:name:]" and returns
// the index past the closing ']', or 0 if no match.
func matchCharClass(s string, i int) int {
	if i+3 >= len(s) || s[i+1] != ':' {
		return 0
	}
	end := strings.Index(s[i+2:], ":]")
	if end < 0 {
		return 0
	}
	return i + 2 + end + 2
}

// expandCharClass extracts the class name and returns its expanded bytes.
func expandCharClass(s string, i, end int) (int, []byte, error) {
	name := s[i+2 : end-2]
	bytes, err := classBytes(name)
	if err != nil {
		return 0, nil, err
	}
	return end - i, bytes, nil
}

// matchEquivClass checks if s[i:] starts with "[=c=]" and returns
// the index past the closing ']', or 0 if no match.
func matchEquivClass(s string, i int) int {
	if i+4 >= len(s) || s[i+1] != '=' {
		return 0
	}
	end := strings.Index(s[i+2:], "=]")
	if end < 0 {
		return 0
	}
	return i + 2 + end + 2
}

// expandEquivClass handles [=c=] equivalence class notation.
// R3.3: in LC_ALL=C, equivalence class contains only the character itself.
func expandEquivClass(s string, i, end int) (int, []byte, error) {
	char := s[i+2 : end-2]
	if len(char) != 1 {
		return 0, nil, fmt.Errorf("invalid equivalence class '%s'", s[i:end])
	}
	return end - i, []byte{char[0]}, nil
}

// matchRepeat checks if s[i:] starts with "[c*n]" or "[c*]" and returns
// the index past the closing ']', or 0 if no match.
func matchRepeat(s string, i int) int {
	// Minimum: [c*] = 4 chars.
	if i+3 >= len(s) {
		return 0
	}
	// s[i] is '[', s[i+1] is the char, s[i+2] must be '*'.
	if s[i+2] != '*' {
		return 0
	}
	end := strings.IndexByte(s[i+3:], ']')
	if end < 0 {
		return 0
	}
	return i + 3 + end + 1
}

// expandRepeat handles [c*n] repeat notation.
// R1.3: [c*] repeats c as needed (returns one instance for parsing).
// [c*n] repeats c exactly n times.
func expandRepeat(s string, i, end int) (int, []byte, error) {
	ch := s[i+1]
	countStr := s[i+3 : end-1]
	count, err := parseRepeatCount(countStr)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid repeat count in '%s'", s[i:end])
	}
	result := make([]byte, count)
	for j := range result {
		result[j] = ch
	}
	return end - i, result, nil
}

// parseRepeatCount parses the count in [c*n] notation.
// Empty string means repeat as needed (return 1 for now).
// Leading 0 means octal.
func parseRepeatCount(s string) (int, error) {
	if s == "" {
		return 1, nil
	}
	base := 10
	if strings.HasPrefix(s, "0") && len(s) > 1 {
		base = 8
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// expandEscape handles backslash escape sequences.
// R1.3: \n, \t, \\, \a, \b, \f, \r, \v, and \NNN octal.
func expandEscape(s string, i int) (int, []byte, error) {
	if i+1 >= len(s) {
		return 1, []byte{'\\'}, nil
	}
	next := s[i+1]
	if b, ok := simpleEscape(next); ok {
		return 2, []byte{b}, nil
	}
	if isOctalDigit(next) {
		return expandOctalEscape(s, i+1)
	}
	// Unknown escape: treat backslash + char as literal char.
	return 2, []byte{next}, nil
}

// simpleEscape maps a character after backslash to its byte value.
func simpleEscape(c byte) (byte, bool) {
	switch c {
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
	case '\\':
		return '\\', true
	}
	return 0, false
}

// expandOctalEscape parses up to 3 octal digits starting at position i.
// Returns total consumed (including the leading backslash at i-1).
func expandOctalEscape(s string, i int) (int, []byte, error) {
	end := i
	for end < len(s) && end < i+3 && isOctalDigit(s[end]) {
		end++
	}
	val, err := strconv.ParseUint(s[i:end], 8, 8)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid octal escape '\\%s'", s[i:end])
	}
	// +1 for the backslash.
	return 1 + (end - i), []byte{byte(val)}, nil
}

// isOctalDigit reports whether c is an octal digit.
func isOctalDigit(c byte) bool {
	return c >= '0' && c <= '7'
}

// expandRange produces all bytes from lo to hi inclusive.
// Returns 3 consumed characters (lo, '-', hi).
func expandRange(lo, hi byte) (int, []byte, error) {
	if lo > hi {
		return 0, nil, fmt.Errorf("range-endpoints of '%c-%c' are in reverse collating sequence order", lo, hi)
	}
	size := int(hi-lo) + 1
	result := make([]byte, size)
	for j := range size {
		result[j] = lo + byte(j)
	}
	return 3, result, nil
}

// classBytes returns the expanded byte slice for a POSIX character class.
// R1.4, D4: all classes expanded for LC_ALL=C byte values.
func classBytes(name string) ([]byte, error) {
	switch name {
	case "alnum":
		return mergeClasses(digitBytes(), letterBytes()), nil
	case "alpha":
		return letterBytes(), nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "cntrl":
		return ctrlBytes(), nil
	case "digit":
		return digitBytes(), nil
	case "graph":
		return rangeBytes(0x21, 0x7E), nil
	case "lower":
		return rangeBytes('a', 'z'), nil
	case "print":
		return rangeBytes(0x20, 0x7E), nil
	case "punct":
		return punctBytes(), nil
	case "space":
		return []byte{'\t', '\n', '\v', '\f', '\r', ' '}, nil
	case "upper":
		return rangeBytes('A', 'Z'), nil
	case "xdigit":
		return mergeClasses(digitBytes(), mergeClasses(rangeBytes('A', 'F'), rangeBytes('a', 'f'))), nil
	}
	return nil, fmt.Errorf("invalid character class '%s'", name)
}

// rangeBytes returns a byte slice from lo to hi inclusive.
func rangeBytes(lo, hi byte) []byte {
	result := make([]byte, int(hi-lo)+1)
	for i := range result {
		result[i] = lo + byte(i)
	}
	return result
}

// digitBytes returns '0'-'9'.
func digitBytes() []byte {
	return rangeBytes('0', '9')
}

// letterBytes returns 'A'-'Z' followed by 'a'-'z'.
func letterBytes() []byte {
	return mergeClasses(rangeBytes('A', 'Z'), rangeBytes('a', 'z'))
}

// ctrlBytes returns all control characters (0x00-0x1F and 0x7F).
func ctrlBytes() []byte {
	result := rangeBytes(0x00, 0x1F)
	return append(result, 0x7F)
}

// punctBytes returns all printable non-alphanumeric characters.
func punctBytes() []byte {
	var result []byte
	for b := byte(0x21); b <= 0x7E; b++ {
		if !isAlnum(b) {
			result = append(result, b)
		}
	}
	return result
}

// isAlnum reports whether b is an ASCII alphanumeric character.
func isAlnum(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z')
}

// mergeClasses concatenates two byte slices.
func mergeClasses(a, b []byte) []byte {
	result := make([]byte, len(a)+len(b))
	copy(result, a)
	copy(result[len(a):], b)
	return result
}
