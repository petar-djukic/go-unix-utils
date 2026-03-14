// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uniq implements the uniq (report or filter adjacent duplicate lines) command.
// Implements: prd028-uniq R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4
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

// config holds all parsed command-line options.
type config struct {
	count       bool // -c: prefix lines with count
	repeated    bool // -d: only print duplicate lines (one per run)
	allRepeated bool // -D: print all duplicate lines (every copy)
	unique      bool // -u: only print unique lines
	ignoreCase  bool // -i: case-insensitive comparison (R3.1)
	skipFields  int  // -f N: skip first N fields before comparing (R3.2)
	skipChars   int  // -s N: skip first N characters before comparing (R3.3)
	checkChars  int  // -w N: compare only first N characters (R3.4); -1 means all
	checkCharsSet bool // true when -w was explicitly provided
	inFile      string
	outFile     string
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	var positional []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			positional = append(positional, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "-" {
			positional = append(positional, arg)
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--count":
				cfg.count = true
			case arg == "--repeated":
				cfg.repeated = true
			case arg == "--all-repeated":
				cfg.allRepeated = true
			case arg == "--unique":
				cfg.unique = true
			case arg == "--ignore-case":
				cfg.ignoreCase = true
			case arg == "--skip-fields" || strings.HasPrefix(arg, "--skip-fields="):
				val, err := parseLongOptValue(arg, "--skip-fields", args, &i)
				if err != nil {
					return nil, err
				}
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid number of fields to skip: '%s'", val)
				}
				cfg.skipFields = n
			case arg == "--skip-chars" || strings.HasPrefix(arg, "--skip-chars="):
				val, err := parseLongOptValue(arg, "--skip-chars", args, &i)
				if err != nil {
					return nil, err
				}
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid number of bytes to skip: '%s'", val)
				}
				cfg.skipChars = n
			case arg == "--check-chars" || strings.HasPrefix(arg, "--check-chars="):
				val, err := parseLongOptValue(arg, "--check-chars", args, &i)
				if err != nil {
					return nil, err
				}
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid number of bytes to compare: '%s'", val)
				}
				cfg.checkChars = n
				cfg.checkCharsSet = true
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		// Short flags.
		if strings.HasPrefix(arg, "-") {
			rest := arg[1:]
			for j := 0; j < len(rest); j++ {
				ch := rest[j]
				switch ch {
				case 'c':
					cfg.count = true
				case 'd':
					cfg.repeated = true
				case 'D':
					cfg.allRepeated = true
				case 'u':
					cfg.unique = true
				case 'i':
					cfg.ignoreCase = true
				case 'f':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					n, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("invalid number of fields to skip: '%s'", val)
					}
					cfg.skipFields = n
					j = len(rest) // consumed rest
				case 's':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					n, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("invalid number of bytes to skip: '%s'", val)
					}
					cfg.skipChars = n
					j = len(rest)
				case 'w':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					n, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("invalid number of bytes to compare: '%s'", val)
					}
					cfg.checkChars = n
					cfg.checkCharsSet = true
					j = len(rest)
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			continue
		}

		positional = append(positional, arg)
	}

	// R1.3: Optional input file as first positional, optional output file as second.
	if len(positional) > 2 {
		return nil, fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) >= 1 {
		cfg.inFile = positional[0]
	}
	if len(positional) >= 2 {
		cfg.outFile = positional[1]
	}

	return cfg, nil
}

// parseLongOptValue extracts the value for a long option, either from
// --opt=value form or from the next argument.
func parseLongOptValue(arg, name string, args []string, i *int) (string, error) {
	if strings.HasPrefix(arg, name+"=") {
		return arg[len(name)+1:], nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option '%s' requires an argument", name)
	}
	return args[*i], nil
}

// parseShortOptValue extracts the value for a short option that takes an
// argument. If characters remain after the flag letter in the same token,
// they form the value; otherwise the next argument is consumed.
func parseShortOptValue(rest string, j int, args []string, i *int) (string, error) {
	if j+1 < len(rest) {
		return rest[j+1:], nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option requires an argument -- '%c'", rest[j])
	}
	return args[*i], nil
}

// isBlank returns true if r is a space or tab (POSIX blank class).
func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}

// compareKey extracts the portion of line used for comparison, applying
// -f (skip fields), -s (skip chars), and -w (check chars) adjustments.
//
// R3.2: -f N skips the first N whitespace-separated fields.
// R3.3: -s N skips the first N characters after field skipping.
// R3.4: -w N limits comparison to the first N characters after adjustments.
func compareKey(line string, cfg *config) string {
	s := line
	// R3.2: Skip fields — GNU uniq skips leading blanks then non-blanks per field.
	if cfg.skipFields > 0 {
		idx := 0
		fieldsSkipped := 0
		for fieldsSkipped < cfg.skipFields && idx < len(s) {
			// Skip leading blanks (space/tab).
			for idx < len(s) && isBlank(rune(s[idx])) {
				idx++
			}
			// Skip the non-blank field.
			for idx < len(s) && !isBlank(rune(s[idx])) {
				idx++
			}
			fieldsSkipped++
		}
		s = s[idx:]
	}

	// R3.3: Skip characters (bytes in LC_ALL=C).
	if cfg.skipChars > 0 {
		if cfg.skipChars >= len(s) {
			s = ""
		} else {
			s = s[cfg.skipChars:]
		}
	}

	// R3.4: Limit comparison to first N characters (bytes in LC_ALL=C).
	// When -w is explicitly set, even -w 0 means "compare 0 characters".
	if cfg.checkCharsSet {
		if cfg.checkChars < len(s) {
			s = s[:cfg.checkChars]
		}
	}

	// R3.1: Case-insensitive folding.
	if cfg.ignoreCase {
		s = strings.ToUpper(s)
	}

	return s
}

// linesEqual returns true if two lines are equal under the configured
// comparison options (-i, -f, -s, -w).
func linesEqual(a, b string, cfg *config) bool {
	return compareKey(a, cfg) == compareKey(b, cfg)
}

// run executes the uniq logic with the given configuration.
func run(cfg *config) error {
	// Open input.
	var reader io.Reader
	if cfg.inFile == "" || cfg.inFile == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(cfg.inFile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
		reader = f
	}

	// Open output.
	var w *bufio.Writer
	if cfg.outFile == "" {
		w = bufio.NewWriter(os.Stdout)
	} else {
		f, err := os.Create(cfg.outFile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
		w = bufio.NewWriter(f)
	}

	if err := processUniq(reader, w, cfg); err != nil {
		return err
	}

	// R4.3: Flush buffered output; report write error.
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// processUniq reads lines from reader, groups adjacent duplicates, and writes
// output to w according to the flags in cfg.
//
// R1.1: Suppress all but the first occurrence of any run of identical adjacent lines.
// R1.2: Non-adjacent duplicates are unaffected.
// R1.4: Comparison is case-sensitive; each line includes its newline terminator.
// R2.2: -D emits every copy of each duplicate run inline.
func processUniq(reader io.Reader, w *bufio.Writer, cfg *config) error {
	if cfg.allRepeated {
		return processAllRepeated(reader, w, cfg)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var prevLine string
	var count int
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			count = 1
			first = false
			continue
		}

		// R1.4, R3.1-R3.4: Compare using configured options.
		if linesEqual(line, prevLine, cfg) {
			count++
			continue
		}

		// End of a run; emit the previous group.
		if err := emitGroup(w, prevLine, count, cfg); err != nil {
			return err
		}

		prevLine = line
		count = 1
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	// Emit the last group.
	if !first {
		if err := emitGroup(w, prevLine, count, cfg); err != nil {
			return err
		}
	}

	return nil
}

// processAllRepeated handles -D mode, which emits every copy of each duplicate
// run. Unlike the default mode, lines are emitted inline as the run grows rather
// than once at the end.
//
// R2.2: -D prints all lines of each run that appears more than once.
func processAllRepeated(reader io.Reader, w *bufio.Writer, cfg *config) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var prevLine string
	var count int
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			count = 1
			first = false
			continue
		}

		if linesEqual(line, prevLine, cfg) {
			// R2.2: Duplicate found; emit the deferred first copy if this is
			// the second occurrence, then emit the current copy.
			if count == 1 {
				if err := writeLine(w, prevLine); err != nil {
					return err
				}
			}
			if err := writeLine(w, line); err != nil {
				return err
			}
			count++
			continue
		}

		// New run starts; reset.
		prevLine = line
		count = 1
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	return nil
}

// writeLine writes a single line followed by a newline to w.
func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// emitGroup writes a single group (line repeated count times) to w, respecting
// the -c, -d, and -u flags.
//
// R1.3: -c prefixes output with the count, right-justified in a 7-wide field.
// R1.4: -d prints only groups with count > 1; -u prints only groups with count == 1.
func emitGroup(w *bufio.Writer, line string, count int, cfg *config) error {
	// R2.1: -d suppresses lines that appear only once.
	if cfg.repeated && count == 1 {
		return nil
	}
	// R2.3: -u suppresses lines that appear more than once.
	if cfg.unique && count > 1 {
		return nil
	}

	// R2.4: -c prefixes each line with the occurrence count.
	// R2.4: -c prefixes each line with the occurrence count.
	if cfg.count {
		if _, err := fmt.Fprintf(w, "%7d %s\n", count, line); err != nil {
			return err
		}
		return nil
	}

	return writeLine(w, line)
}
