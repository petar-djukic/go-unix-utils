// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tr implements the tr (translate or delete characters) command.
// Implements: prd054-tr R1.1, R1.2, R1.3, R1.4
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

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "tr: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "tr: %v\n", err)
		os.Exit(1)
	}
}

// config holds all parsed command-line options.
type config struct {
	set1 string
	set2 string
}

// parseArgs parses command-line arguments into a config.
// R1.1: Parse two operand strings (SET1, SET2) from positional arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		// D4: Handle --help and --version.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--help":
				fmt.Fprintf(os.Stdout, "Usage: tr [OPTION]... SET1 [SET2]\nTranslate characters in SET1 to SET2.\n")
				os.Exit(0)
			case "--version":
				fmt.Println("tr (go-unix-utils)")
				os.Exit(0)
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return nil, fmt.Errorf("unrecognized option '%s'", arg)
		}

		positional = append(positional, arg)
	}

	if len(positional) < 2 {
		return nil, fmt.Errorf("missing operand")
	}
	if len(positional) > 2 {
		return nil, fmt.Errorf("extra operand %q", positional[2])
	}

	cfg.set1 = positional[0]
	cfg.set2 = positional[1]
	return cfg, nil
}

// run executes the tr transliteration with the given configuration.
// R1.2: Reads from stdin and writes translated output to stdout.
func run(cfg *config, r io.Reader, w io.Writer) error {
	// R1.3, R1.4: Expand SET specifications (ranges, escapes, classes, repetition).
	set1Bytes, err := expandSet(cfg.set1)
	if err != nil {
		return err
	}
	set2Bytes, err := expandSet(cfg.set2)
	if err != nil {
		return err
	}

	if len(set1Bytes) == 0 {
		return fmt.Errorf("when not truncating set1, string2 must be non-empty")
	}

	// R1.1: When SET2 is shorter than SET1, extend SET2 by repeating its last character.
	if len(set2Bytes) > 0 && len(set2Bytes) < len(set1Bytes) {
		last := set2Bytes[len(set2Bytes)-1]
		for len(set2Bytes) < len(set1Bytes) {
			set2Bytes = append(set2Bytes, last)
		}
	}

	// Build a 256-byte translation table.
	// R1.1: Translate each character in SET1 to the corresponding character in SET2.
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i, b := range set1Bytes {
		if i < len(set2Bytes) {
			table[b] = set2Bytes[i]
		}
	}

	// D3: Read stdin byte-by-byte and apply translation table.
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}
		if writeErr := bw.WriteByte(table[b]); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

// expandSet expands a SET specification string into a byte slice.
// R1.3: Supports individual characters, ranges (a-z), octal escapes (\NNN),
// backslash escapes (\n, \t, \\, \a, \b, \f, \r, \v), and repetition ([c*N], [c*]).
// R1.4: Supports POSIX character classes [:alnum:], [:alpha:], etc.
func expandSet(spec string) ([]byte, error) {
	s := []byte(spec)
	var result []byte
	i := 0
	for i < len(s) {
		// R1.4: Check for bracket expressions: [:class:], [=c=], [c*N].
		if s[i] == '[' {
			consumed, expanded, err := tryBracketExpr(s, i)
			if err != nil {
				return nil, err
			}
			if consumed > 0 {
				result = append(result, expanded...)
				i += consumed
				continue
			}
		}

		// R1.3: Backslash escapes.
		if s[i] == '\\' && i+1 < len(s) {
			b, advance := parseEscape(s, i)
			result = append(result, b)
			i += advance
			continue
		}

		// R1.3: Character range (e.g. a-z). Dash between two characters.
		if s[i] == '-' && len(result) > 0 && i+1 < len(s) {
			start := result[len(result)-1]
			var end byte
			advance := 2 // '-' + end char
			if s[i+1] == '\\' && i+2 < len(s) {
				end, advance = parseEscape(s, i+1)
				advance++ // add 1 for the '-'
			} else {
				end = s[i+1]
			}
			if start > end {
				return nil, fmt.Errorf("range-endpoints of '%c-%c' are in reverse collating sequence order", start, end)
			}
			// start is already in result, add start+1..end.
			for c := int(start) + 1; c <= int(end); c++ {
				result = append(result, byte(c))
			}
			i += advance
			continue
		}

		// Individual character.
		result = append(result, s[i])
		i++
	}
	return result, nil
}

// tryBracketExpr attempts to parse a bracket expression starting at s[pos].
// Returns (bytes consumed, expanded bytes, error). Returns (0, nil, nil) if
// the bracket at pos does not form a recognized expression.
func tryBracketExpr(s []byte, pos int) (int, []byte, error) {
	if pos+1 >= len(s) {
		return 0, nil, nil
	}

	// [:class:] -- POSIX character class.
	if s[pos+1] == ':' {
		end := findDelimEnd(s, pos, ':')
		if end > 0 {
			className := string(s[pos+2 : end])
			chars, err := expandCharClass(className)
			if err != nil {
				return 0, nil, err
			}
			return end + 2 - pos, chars, nil
		}
	}

	// [=c=] -- equivalence class. Under LC_ALL=C, equivalent to the character itself.
	if s[pos+1] == '=' {
		end := findDelimEnd(s, pos, '=')
		if end > 0 && end == pos+3 {
			ch := s[pos+2]
			return end + 2 - pos, []byte{ch}, nil
		}
	}

	// [c*N] or [c*] -- repetition. Character at pos+1, '*' at pos+2.
	if pos+2 < len(s) && s[pos+2] == '*' {
		return parseRepetition(s, pos, s[pos+1], pos+2)
	}

	return 0, nil, nil
}

// findDelimEnd finds the closing delimiter for bracket expressions like [:name:] or [=c=].
// Returns the position of the closing delimiter character, or 0 if not found.
func findDelimEnd(s []byte, pos int, delim byte) int {
	for j := pos + 2; j < len(s)-1; j++ {
		if s[j] == delim && s[j+1] == ']' {
			return j
		}
	}
	return 0
}

// parseRepetition parses the *N] portion of a repetition expression [c*N].
// ch is the character to repeat, starPos is the position of '*'.
func parseRepetition(s []byte, pos int, ch byte, starPos int) (int, []byte, error) {
	// Find closing ']'.
	j := starPos + 1
	for j < len(s) && s[j] != ']' {
		j++
	}
	if j >= len(s) {
		return 0, nil, nil // no closing bracket, treat as literal
	}

	countStr := string(s[starPos+1 : j])
	consumed := j + 1 - pos

	if countStr == "" {
		// [c*] -- repeat count determined later (used in SET2 to fill).
		// Return a single occurrence; the caller extends SET2 as needed.
		return consumed, []byte{ch}, nil
	}

	// Parse count, which may be octal (leading 0) or decimal.
	var count int64
	var err error
	if strings.HasPrefix(countStr, "0") && len(countStr) > 1 {
		count, err = strconv.ParseInt(countStr, 8, 64)
	} else {
		count, err = strconv.ParseInt(countStr, 10, 64)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("invalid repeat count %q", countStr)
	}

	result := make([]byte, count)
	for k := range result {
		result[k] = ch
	}
	return consumed, result, nil
}

// parseEscape parses a backslash escape sequence starting at s[pos] (which is '\').
// R1.3: Supports \n, \t, \\, \a, \b, \f, \r, \v, and \NNN octal.
// Returns the byte value and the number of bytes consumed.
func parseEscape(s []byte, pos int) (byte, int) {
	if pos+1 >= len(s) {
		return '\\', 1
	}
	ch := s[pos+1]
	switch ch {
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

	// Octal escape: \NNN (1-3 octal digits).
	if ch >= '0' && ch <= '7' {
		val := int(ch - '0')
		consumed := 2
		for k := 1; k < 3; k++ {
			if pos+1+k >= len(s) || s[pos+1+k] < '0' || s[pos+1+k] > '7' {
				break
			}
			val = val*8 + int(s[pos+1+k]-'0')
			consumed++
		}
		return byte(val & 0xFF), consumed
	}

	// Unknown escape: return the character after backslash.
	return ch, 2
}

// expandCharClass expands a POSIX character class name into a byte slice.
// R1.4: Supports [:alnum:], [:alpha:], [:blank:], [:cntrl:], [:digit:], [:graph:],
// [:lower:], [:print:], [:punct:], [:space:], [:upper:], [:xdigit:].
func expandCharClass(name string) ([]byte, error) {
	var result []byte
	switch name {
	case "upper":
		for c := byte('A'); c <= 'Z'; c++ {
			result = append(result, c)
		}
	case "lower":
		for c := byte('a'); c <= 'z'; c++ {
			result = append(result, c)
		}
	case "digit":
		for c := byte('0'); c <= '9'; c++ {
			result = append(result, c)
		}
	case "xdigit":
		for c := byte('0'); c <= '9'; c++ {
			result = append(result, c)
		}
		for c := byte('A'); c <= 'F'; c++ {
			result = append(result, c)
		}
		for c := byte('a'); c <= 'f'; c++ {
			result = append(result, c)
		}
	case "alpha":
		for c := byte('A'); c <= 'Z'; c++ {
			result = append(result, c)
		}
		for c := byte('a'); c <= 'z'; c++ {
			result = append(result, c)
		}
	case "alnum":
		for c := byte('0'); c <= '9'; c++ {
			result = append(result, c)
		}
		for c := byte('A'); c <= 'Z'; c++ {
			result = append(result, c)
		}
		for c := byte('a'); c <= 'z'; c++ {
			result = append(result, c)
		}
	case "blank":
		result = append(result, '\t', ' ')
	case "space":
		result = append(result, '\t', '\n', '\v', '\f', '\r', ' ')
	case "cntrl":
		for c := 0; c <= 31; c++ {
			result = append(result, byte(c))
		}
		result = append(result, 127)
	case "print":
		for c := 32; c <= 126; c++ {
			result = append(result, byte(c))
		}
	case "graph":
		for c := 33; c <= 126; c++ {
			result = append(result, byte(c))
		}
	case "punct":
		for c := 33; c <= 126; c++ {
			b := byte(c)
			if (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				continue
			}
			result = append(result, b)
		}
	default:
		return nil, fmt.Errorf("invalid character class %q", name)
	}
	return result, nil
}
