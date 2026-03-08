// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the tac utility for concatenating and printing files
// in reverse.
//
// Implements prd021-tac: core reversal behavior (R1), separator options (R2),
// exit codes and SIGPIPE (R3).
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line options.
type flags struct {
	separator string // -s: record separator (default "\n")
	before    bool   // -b: separator before record instead of after
	regex     bool   // -r: interpret separator as regex
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, files := parseArgs(os.Args[1:])

	exitCode := 0

	if len(files) == 0 {
		if err := processReader(os.Stdout, os.Stdin, f); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			exitCode = 1
		}
	} else {
		for _, name := range files {
			var r io.Reader
			if name == "-" {
				r = os.Stdin
			} else {
				file, err := os.Open(name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "tac: failed to open '%s' for reading: No such file or directory\n", name)
					exitCode = 1
					continue
				}
				r = file
				defer file.Close()
			}
			if err := processReader(os.Stdout, r, f); err != nil {
				fmt.Fprintf(os.Stderr, "tac: %v\n", err)
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and file names.
func parseArgs(args []string) (flags, []string) {
	var f flags
	f.separator = "\n"
	var files []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long flags.
		if strings.HasPrefix(arg, "--") {
			switch {
			case strings.HasPrefix(arg, "--separator="):
				f.separator = arg[len("--separator="):]
			case arg == "--separator":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "tac: option '--separator' requires an argument\n")
					os.Exit(1)
				}
				f.separator = args[i]
			case arg == "--before":
				f.before = true
			case arg == "--regex":
				f.regex = true
			default:
				fmt.Fprintf(os.Stderr, "tac: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short flags.
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'b':
					f.before = true
					j++
				case 'r':
					f.regex = true
					j++
				case 's':
					rest := arg[j+1:]
					if rest != "" {
						f.separator = rest
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "tac: option requires an argument -- 's'\n")
							os.Exit(1)
						}
						f.separator = args[i]
					}
					j = len(arg) // consumed
				default:
					fmt.Fprintf(os.Stderr, "tac: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		files = append(files, arg)
	}

	return f, files
}

// processReader reads all data from r, reverses the records, and writes to w.
func processReader(w io.Writer, r io.Reader, f flags) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if f.regex {
		return processRegex(w, data, f)
	}
	return processString(w, data, f)
}

// processString handles reversal with a fixed string separator.
//
// GNU tac model: each record includes the separator that terminates it.
// In default mode (separator after), records are "text+sep" chunks.
// The last chunk may lack a separator if the input doesn't end with one.
// In -b mode (separator before), records are "sep+text" chunks.
// The first chunk may lack a separator if the input doesn't start with one.
// Records are reversed and concatenated.
func processString(w io.Writer, data []byte, f flags) error {
	input := string(data)
	sep := f.separator

	if f.before {
		return processBeforeString(w, input, sep)
	}

	// Find all separator positions.
	var records []string
	pos := 0
	for {
		idx := strings.Index(input[pos:], sep)
		if idx < 0 {
			break
		}
		// Record includes the text up to and including the separator.
		end := pos + idx + len(sep)
		records = append(records, input[pos:end])
		pos = end
	}
	// Trailing text after the last separator (record without separator).
	if pos < len(input) {
		records = append(records, input[pos:])
	}

	// Reverse records.
	reverse(records)

	// Write concatenated reversed records.
	for _, rec := range records {
		if _, err := fmt.Fprint(w, rec); err != nil {
			return err
		}
	}

	return nil
}

// processBeforeString handles -b with a fixed string separator.
// In -b mode, each record includes the separator that precedes it.
func processBeforeString(w io.Writer, input, sep string) error {
	// Find all separator positions.
	var sepPositions []int
	pos := 0
	for {
		idx := strings.Index(input[pos:], sep)
		if idx < 0 {
			break
		}
		sepPositions = append(sepPositions, pos+idx)
		pos += idx + len(sep)
	}

	if len(sepPositions) == 0 {
		// No separator found; output as-is.
		_, err := fmt.Fprint(w, input)
		return err
	}

	// Build records: each record = separator + text until next separator.
	// The first chunk (before first separator) has no separator.
	var records []string

	// Leading text before the first separator.
	if sepPositions[0] > 0 {
		records = append(records, input[:sepPositions[0]])
	}

	for i, sp := range sepPositions {
		var end int
		if i+1 < len(sepPositions) {
			end = sepPositions[i+1]
		} else {
			end = len(input)
		}
		records = append(records, input[sp:end])
	}

	// Reverse records.
	reverse(records)

	// Write concatenated reversed records.
	for _, rec := range records {
		if _, err := fmt.Fprint(w, rec); err != nil {
			return err
		}
	}

	return nil
}

// processRegex handles reversal with a regex separator.
func processRegex(w io.Writer, data []byte, f flags) error {
	re, err := regexp.Compile(f.separator)
	if err != nil {
		return fmt.Errorf("invalid regex '%s': %w", f.separator, err)
	}

	input := string(data)
	locs := re.FindAllStringIndex(input, -1)

	if len(locs) == 0 {
		// No separator found; output as-is.
		_, werr := fmt.Fprint(w, input)
		return werr
	}

	if f.before {
		return processBeforeRegex(w, input, locs)
	}

	// Build records: each record = text + matched separator.
	// Last chunk (after last separator) has no separator.
	var records []string
	pos := 0
	for _, loc := range locs {
		records = append(records, input[pos:loc[1]])
		pos = loc[1]
	}
	if pos < len(input) {
		records = append(records, input[pos:])
	}

	// Reverse records.
	reverse(records)

	// Write concatenated reversed records.
	for _, rec := range records {
		if _, err := fmt.Fprint(w, rec); err != nil {
			return err
		}
	}

	return nil
}

// processBeforeRegex handles -b -r: separator precedes each record, regex mode.
func processBeforeRegex(w io.Writer, input string, locs [][]int) error {
	// Build records: each record = matched separator + text until next separator.
	// Leading text before the first separator has no separator.
	var records []string

	if locs[0][0] > 0 {
		records = append(records, input[:locs[0][0]])
	}

	for i, loc := range locs {
		var end int
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			end = len(input)
		}
		records = append(records, input[loc[0]:end])
	}

	// Reverse records.
	reverse(records)

	// Write concatenated reversed records.
	for _, rec := range records {
		if _, err := fmt.Fprint(w, rec); err != nil {
			return err
		}
	}

	return nil
}

// reverse reverses a string slice in place.
func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
