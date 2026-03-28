// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd021-tac: Concatenate and Print Files in Reverse.
// Covers R1.1-R1.4 (core reversal), R2.1-R2.4 (separator options),
// R3.1-R3.4 (exit codes and SIGPIPE).
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tac"

func main() {
	// R3.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// tacOptions holds the parsed flag state for a tac invocation.
type tacOptions struct {
	separator string // -s: record separator (R2.1), default "\n"
	before    bool   // -b: separator before record (R2.2)
	regex     bool   // -r: interpret separator as regex (R2.3)
}

// parseFlags parses GNU tac-compatible flags from the argument list.
// R2.1: -s SEP sets the separator. R2.2: -b sets before mode.
// R2.3: -r sets regex mode.
func parseFlags(args []string) (tacOptions, []string) {
	opts := tacOptions{separator: "\n"}
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
		if arg == "--before" {
			opts.before = true
			continue
		}
		if arg == "--regex" {
			opts.regex = true
			continue
		}
		if handled, advance := parseSepLong(arg, args, i, &opts); handled {
			i += advance
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			advance := applyShortFlags(arg[1:], args, i, &opts)
			i += advance
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// parseSepLong handles --separator=SEP and --separator SEP forms.
// Returns (handled, extra args consumed).
func parseSepLong(arg string, args []string, i int, opts *tacOptions) (bool, int) {
	if arg == "--separator" {
		if i+1 < len(args) {
			opts.separator = args[i+1]
			return true, 1
		}
		fmt.Fprintf(os.Stderr, "%s: option '--separator' requires an argument\n", progName)
		os.Exit(1)
	}
	if strings.HasPrefix(arg, "--separator=") {
		opts.separator = arg[len("--separator="):]
		return true, 0
	}
	return false, 0
}

// applyShortFlags applies short flags like -b, -s SEP, -r.
// Returns number of extra args consumed.
func applyShortFlags(flags string, args []string, idx int, opts *tacOptions) int {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'b':
			opts.before = true
		case 'r':
			// R2.3: interpret separator as regular expression.
			opts.regex = true
		case 's':
			rest := flags[j+1:]
			if len(rest) > 0 {
				opts.separator = rest
				return consumed
			}
			if idx+1+consumed < len(args) {
				consumed++
				opts.separator = args[idx+consumed]
			} else {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\n", progName)
				os.Exit(1)
			}
			return consumed
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
			os.Exit(1)
		}
	}
	return consumed
}

// run processes all files with the given options and returns the exit code.
// R1.3: reads stdin when no files given or when "-" is a filename.
// R1.4: each file processed independently in argument order.
// R3.1: returns 0 on success. R3.2: returns 1 on file error.
// R3.3: returns 1 on write error.
func run(opts tacOptions, files []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := processFile(name, opts, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile reads the named file (or stdin for "-"), reverses records,
// and writes them to stdout.
func processFile(name string, opts tacOptions, stdin io.Reader, stdout io.Writer) error {
	data, err := readInput(name, stdin)
	if err != nil {
		return err
	}
	return writeReversed(data, opts, stdout)
}

// readInput reads the entire contents of a file or stdin.
func readInput(name string, stdin io.Reader) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(name)
}

// writeReversed splits data into records on the separator, reverses them,
// and writes to w.
// R1.1: records written in reverse order.
// R1.2: trailing separator is terminator, not before empty record.
// R2.1: custom separator via -s.
// R2.2: -b places separator before each record.
// R2.3, R2.4: -r interprets separator as regex.
func writeReversed(data []byte, opts tacOptions, w io.Writer) error {
	input := string(data)
	if opts.regex {
		return writeReversedRegex(input, opts, w)
	}
	if opts.before {
		return writeReversedBefore(input, opts.separator, w)
	}
	return writeReversedAfter(input, opts.separator, w)
}

// writeReversedRegex handles -r mode: separator is a regex pattern.
// R2.3: each match of the pattern becomes a record boundary.
// R2.4: when -s and -r combined, records are text between matches.
func writeReversedRegex(input string, opts tacOptions, w io.Writer) error {
	re, err := regexp.Compile(opts.separator)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", opts.separator, err)
	}
	if opts.before {
		return writeRegexBefore(input, re, w)
	}
	return writeRegexAfter(input, re, w)
}

// writeRegexAfter splits on regex matches in after mode (separator follows record).
func writeRegexAfter(input string, re *regexp.Regexp, w io.Writer) error {
	locs := re.FindAllStringIndex(input, -1)
	if len(locs) == 0 {
		_, err := io.WriteString(w, input)
		return err
	}
	records := buildAfterRecords(input, locs)
	return writeRecordsReversed(records, w)
}

// buildAfterRecords constructs records where each separator is attached
// to the end of the preceding record.
func buildAfterRecords(input string, locs [][]int) []string {
	records := make([]string, 0, len(locs)+1)
	prev := 0
	for _, loc := range locs {
		records = append(records, input[prev:loc[1]])
		prev = loc[1]
	}
	// R1.2: trailing text after last match (no trailing separator).
	if prev < len(input) {
		records = append(records, input[prev:])
	}
	return records
}

// writeRegexBefore splits on regex matches in before mode (separator precedes record).
func writeRegexBefore(input string, re *regexp.Regexp, w io.Writer) error {
	locs := re.FindAllStringIndex(input, -1)
	if len(locs) == 0 {
		_, err := io.WriteString(w, input)
		return err
	}
	records := buildBeforeRecords(input, locs)
	return writeRecordsReversed(records, w)
}

// buildBeforeRecords constructs records where each separator is attached
// to the beginning of the following record.
func buildBeforeRecords(input string, locs [][]int) []string {
	records := make([]string, 0, len(locs)+1)
	// Text before first match has no separator prefix.
	if locs[0][0] > 0 {
		records = append(records, input[:locs[0][0]])
	}
	for i, loc := range locs {
		end := len(input)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		records = append(records, input[loc[0]:end])
	}
	return records
}

// writeRecordsReversed writes records in reverse order.
func writeRecordsReversed(records []string, w io.Writer) error {
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := io.WriteString(w, records[i]); err != nil {
			return err
		}
	}
	return nil
}

// splitAfterRecords splits input into chunks where each chunk is content+sep,
// except the last chunk may lack a trailing separator.
// GNU tac model: separators are attached to the record they follow.
func splitAfterRecords(input, sep string) []string {
	parts := strings.Split(input, sep)
	if len(parts) == 0 {
		return nil
	}
	// Rejoin: each part except the last gets sep appended
	records := make([]string, 0, len(parts))
	for i := 0; i < len(parts)-1; i++ {
		records = append(records, parts[i]+sep)
	}
	// Last part has no separator (or is empty if input ended with sep)
	last := parts[len(parts)-1]
	if last != "" {
		records = append(records, last)
	}
	return records
}

// writeReversedAfter handles default mode: separator follows each record.
// R1.2: trailing separator terminates last record (no empty trailing record).
func writeReversedAfter(input, sep string, w io.Writer) error {
	records := splitAfterRecords(input, sep)
	return writeRecordsReversed(records, w)
}

// splitBeforeRecords splits input into chunks where each chunk is sep+content,
// except the first chunk may lack a leading separator.
func splitBeforeRecords(input, sep string) []string {
	parts := strings.Split(input, sep)
	if len(parts) == 0 {
		return nil
	}
	records := make([]string, 0, len(parts))
	// First part has no separator (or is empty if input started with sep)
	first := parts[0]
	if first != "" {
		records = append(records, first)
	}
	for i := 1; i < len(parts); i++ {
		records = append(records, sep+parts[i])
	}
	return records
}

// writeReversedBefore handles -b mode: separator precedes each record.
func writeReversedBefore(input, sep string, w io.Writer) error {
	records := splitBeforeRecords(input, sep)
	return writeRecordsReversed(records, w)
}
