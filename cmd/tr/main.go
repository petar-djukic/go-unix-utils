// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tr implements the tr (translate or delete characters) command.
// Implements: prd054-tr R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3
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
	set1        string
	set2        string
	deleteMode  bool
	squeezeMode bool
}

// parseArgs parses command-line arguments into a config.
// R1.1: Parse two operand strings (SET1, SET2) from positional arguments.
// R3.1: Parse -d/--delete flag for delete mode.
// R3.2: Parse -s/--squeeze-repeats flag for squeeze mode.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		// D4: Handle --help, --version, --delete, --squeeze-repeats.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--help":
				fmt.Fprintf(os.Stdout, "Usage: tr [OPTION]... SET1 [SET2]\nTranslate characters in SET1 to SET2.\n")
				os.Exit(0)
			case "--version":
				fmt.Println("tr (go-unix-utils)")
				os.Exit(0)
			case "--delete":
				cfg.deleteMode = true
			case "--squeeze-repeats":
				cfg.squeezeMode = true
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		// Short flags: -d, -s, or combined like -ds, -sd.
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			valid := true
			for _, ch := range arg[1:] {
				switch ch {
				case 'd':
					cfg.deleteMode = true
				case 's':
					cfg.squeezeMode = true
				default:
					valid = false
				}
			}
			if !valid {
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		positional = append(positional, arg)
	}

	// Validate operand count based on mode.
	switch {
	case cfg.deleteMode && !cfg.squeezeMode:
		// R3.1: -d alone requires exactly SET1.
		if len(positional) < 1 {
			return nil, fmt.Errorf("missing operand")
		}
		if len(positional) > 1 {
			return nil, fmt.Errorf("extra operand %q\nOnly one string may be given when deleting without squeezing repeats.", positional[1])
		}
		cfg.set1 = positional[0]
	case cfg.deleteMode && cfg.squeezeMode:
		// R3.3: -ds requires SET1 and SET2.
		if len(positional) < 2 {
			return nil, fmt.Errorf("missing operand")
		}
		if len(positional) > 2 {
			return nil, fmt.Errorf("extra operand %q", positional[2])
		}
		cfg.set1 = positional[0]
		cfg.set2 = positional[1]
	case cfg.squeezeMode:
		// R3.2: -s with one operand squeezes SET1; with two, translates then squeezes SET2.
		if len(positional) < 1 {
			return nil, fmt.Errorf("missing operand")
		}
		if len(positional) > 2 {
			return nil, fmt.Errorf("extra operand %q", positional[2])
		}
		cfg.set1 = positional[0]
		if len(positional) == 2 {
			cfg.set2 = positional[1]
		}
	default:
		// Translation mode: requires SET1 and SET2.
		if len(positional) < 2 {
			return nil, fmt.Errorf("missing operand")
		}
		if len(positional) > 2 {
			return nil, fmt.Errorf("extra operand %q", positional[2])
		}
		cfg.set1 = positional[0]
		cfg.set2 = positional[1]
	}

	return cfg, nil
}

// byteSet builds a 256-element boolean lookup table from a byte slice.
func byteSet(chars []byte) [256]bool {
	var set [256]bool
	for _, b := range chars {
		set[b] = true
	}
	return set
}

// run executes the tr operation with the given configuration.
// R1.2: Reads from stdin and writes translated output to stdout.
func run(cfg *config, r io.Reader, w io.Writer) error {
	set1Bytes, err := expandSet(cfg.set1)
	if err != nil {
		return err
	}

	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)

	switch {
	case cfg.deleteMode && !cfg.squeezeMode:
		// R3.1: Delete mode -- remove all characters in SET1.
		if err := runDelete(br, bw, set1Bytes); err != nil {
			return err
		}
	case cfg.deleteMode && cfg.squeezeMode:
		// R3.3: Combined delete-squeeze -- delete SET1, then squeeze SET2.
		set2Bytes, err := expandSet(cfg.set2)
		if err != nil {
			return err
		}
		if err := runDeleteSqueeze(br, bw, set1Bytes, set2Bytes); err != nil {
			return err
		}
	case cfg.squeezeMode && cfg.set2 != "":
		// R3.2: Squeeze with translation -- translate SET1→SET2, then squeeze SET2.
		set2Bytes, err := expandSet(cfg.set2)
		if err != nil {
			return err
		}
		table := buildTranslationTable(set1Bytes, set2Bytes)
		if err := runTranslateSqueeze(br, bw, table, set2Bytes); err != nil {
			return err
		}
	case cfg.squeezeMode:
		// R3.2: Squeeze only -- squeeze repeated characters in SET1.
		if err := runSqueeze(br, bw, set1Bytes); err != nil {
			return err
		}
	default:
		// R1.1: Translation mode.
		set2Bytes, err := expandSet(cfg.set2)
		if err != nil {
			return err
		}
		if len(set1Bytes) == 0 {
			return fmt.Errorf("when not truncating set1, string2 must be non-empty")
		}
		table := buildTranslationTable(set1Bytes, set2Bytes)
		if err := runTranslate(br, bw, table); err != nil {
			return err
		}
	}

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

// buildTranslationTable creates a 256-byte identity table and maps SET1 bytes
// to SET2 bytes. When SET2 is shorter, its last character is repeated.
func buildTranslationTable(set1Bytes, set2Bytes []byte) [256]byte {
	// R1.1: When SET2 is shorter than SET1, extend SET2 by repeating its last character.
	if len(set2Bytes) > 0 && len(set2Bytes) < len(set1Bytes) {
		last := set2Bytes[len(set2Bytes)-1]
		for len(set2Bytes) < len(set1Bytes) {
			set2Bytes = append(set2Bytes, last)
		}
	}

	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i, b := range set1Bytes {
		if i < len(set2Bytes) {
			table[b] = set2Bytes[i]
		}
	}
	return table
}

// runTranslate applies the translation table to each byte from input.
func runTranslate(br *bufio.Reader, bw *bufio.Writer, table [256]byte) error {
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		if writeErr := bw.WriteByte(table[b]); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
}

// runDelete removes all characters in deleteSet from the input.
// R3.1: Delete mode.
func runDelete(br *bufio.Reader, bw *bufio.Writer, deleteChars []byte) error {
	set := byteSet(deleteChars)
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		if set[b] {
			continue
		}
		if writeErr := bw.WriteByte(b); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
}

// runSqueeze replaces runs of repeated characters in squeezeChars with a single occurrence.
// R3.2: Squeeze-only mode (no translation).
func runSqueeze(br *bufio.Reader, bw *bufio.Writer, squeezeChars []byte) error {
	set := byteSet(squeezeChars)
	lastByte := -1
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		if set[b] && int(b) == lastByte {
			continue
		}
		lastByte = int(b)
		if writeErr := bw.WriteByte(b); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
}

// runTranslateSqueeze translates via table, then squeezes repeated characters in SET2.
// R3.2: Squeeze with translation.
func runTranslateSqueeze(br *bufio.Reader, bw *bufio.Writer, table [256]byte, squeezeChars []byte) error {
	set := byteSet(squeezeChars)
	lastByte := -1
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		translated := table[b]
		if set[translated] && int(translated) == lastByte {
			continue
		}
		lastByte = int(translated)
		if writeErr := bw.WriteByte(translated); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
}

// runDeleteSqueeze deletes characters in SET1, then squeezes repeated characters in SET2.
// R3.3: Combined -ds mode.
func runDeleteSqueeze(br *bufio.Reader, bw *bufio.Writer, deleteChars, squeezeChars []byte) error {
	deleteSet := byteSet(deleteChars)
	squeezeSet := byteSet(squeezeChars)
	lastByte := -1
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		if deleteSet[b] {
			continue
		}
		if squeezeSet[b] && int(b) == lastByte {
			continue
		}
		lastByte = int(b)
		if writeErr := bw.WriteByte(b); writeErr != nil {
			return fmt.Errorf("write error: %w", writeErr)
		}
	}
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
	// R2.4: [c*0] is equivalent to [c*] -- repeat to fill SET1 length.
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

	// R2.4: When count is 0, treat as [c*] -- repeat to fill.
	if count == 0 {
		return consumed, []byte{ch}, nil
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
