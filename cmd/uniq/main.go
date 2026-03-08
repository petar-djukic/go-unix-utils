// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the uniq utility for filtering adjacent duplicate lines.
//
// Implements prd028-uniq: default deduplication (R1), output selection (R2),
// comparison options (R3), exit codes and SIGPIPE (R4).
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputMode selects which lines to output.
type outputMode int

const (
	modeDefault    outputMode = iota
	modeDupOnly               // -d: only lines with run length > 1
	modeAllDup                // -D: all lines of duplicate runs
	modeUniqueOnly            // -u: only lines with run length == 1
)

// delimMethod controls separator insertion for -D/--all-repeated.
type delimMethod int

const (
	delimNone     delimMethod = iota
	delimPrepend              // empty line before each duplicate group
	delimSeparate             // empty line between duplicate groups
)

// groupMethod controls separator insertion for --group.
type groupMethod int

const (
	groupSeparate groupMethod = iota // empty line between groups (default)
	groupPrepend                     // empty line before each group
	groupAppend                      // empty line after each group
	groupBoth                        // empty line before and after each group
)

// config holds the parsed command-line options.
type config struct {
	count          bool
	mode           outputMode
	allRepMethod   delimMethod
	ignoreCase     bool
	skipFields     int
	skipChars      int
	checkChars     int // 0 means unlimited
	zeroTerminated bool
	groupEnabled   bool
	group          groupMethod
	inputFile      string
	outputFile     string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		os.Exit(1)
	}

	os.Exit(run(cfg))
}

func run(cfg config) int {
	input, inputCloser, err := openInput(cfg.inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		return 1
	}
	if inputCloser != nil {
		defer inputCloser()
	}

	output, outputCloser, err := openOutput(cfg.outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		return 1
	}
	if outputCloser != nil {
		defer outputCloser()
	}

	w := bufio.NewWriter(output)

	scanner := bufio.NewScanner(input)
	if cfg.zeroTerminated {
		scanner.Split(scanNULLines)
	}

	delim := byte('\n')
	if cfg.zeroTerminated {
		delim = 0
	}

	var group []string
	var groupKey string
	outputCount := 0

	flushGroup := func() {
		if len(group) == 0 {
			return
		}
		if writeGroup(w, group, outputCount, cfg, delim) {
			outputCount++
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		key := compareKey(line, cfg)

		if len(group) == 0 {
			group = append(group, line)
			groupKey = key
			continue
		}

		if key == groupKey {
			group = append(group, line)
		} else {
			flushGroup()
			group = group[:0]
			group = append(group, line)
			groupKey = key
		}
	}

	flushGroup()

	// --group=append and --group=both: trailing separator after last group.
	if cfg.groupEnabled && outputCount > 0 &&
		(cfg.group == groupAppend || cfg.group == groupBoth) {
		w.WriteByte(delim)
	}

	if err := w.Flush(); err != nil {
		return 1
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		return 1
	}
	return 0
}

// openInput returns the input reader. If name is empty or "-", returns stdin.
func openInput(name string) (*os.File, func(), error) {
	if name == "" || name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// openOutput returns the output writer. If name is empty, returns stdout.
func openOutput(name string) (*os.File, func(), error) {
	if name == "" {
		return os.Stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// writeGroup outputs a group of identical adjacent lines according to the
// current mode. Returns true if anything was written.
func writeGroup(w *bufio.Writer, lines []string, outputCount int, cfg config, delim byte) bool {
	count := len(lines)
	isDup := count > 1

	// R2 --group mode: output all lines with separators.
	// Trailing separator for append/both is handled by the caller after all
	// groups are flushed, so we only emit a leading separator here.
	if cfg.groupEnabled {
		needBefore := cfg.group == groupPrepend || cfg.group == groupBoth ||
			(cfg.group == groupSeparate && outputCount > 0) ||
			(cfg.group == groupAppend && outputCount > 0)

		if needBefore {
			w.WriteByte(delim)
		}
		for _, line := range lines {
			w.WriteString(line)
			w.WriteByte(delim)
		}
		return true
	}

	switch cfg.mode {
	case modeDefault:
		printLine(w, lines[0], count, cfg.count, delim)
		return true

	case modeDupOnly: // R2.1
		if !isDup {
			return false
		}
		printLine(w, lines[0], count, cfg.count, delim)
		return true

	case modeAllDup: // R2.2
		if !isDup {
			return false
		}
		if cfg.allRepMethod == delimPrepend ||
			(cfg.allRepMethod == delimSeparate && outputCount > 0) {
			w.WriteByte(delim)
		}
		for _, line := range lines {
			w.WriteString(line)
			w.WriteByte(delim)
		}
		return true

	case modeUniqueOnly: // R2.3
		if isDup {
			return false
		}
		printLine(w, lines[0], count, cfg.count, delim)
		return true
	}

	return false
}

// printLine writes a single line, optionally prefixed with count. R2.4.
func printLine(w *bufio.Writer, line string, count int, showCount bool, delim byte) {
	if showCount {
		fmt.Fprintf(w, "%7d %s", count, line)
	} else {
		w.WriteString(line)
	}
	w.WriteByte(delim)
}

// compareKey extracts the portion of a line used for comparison. R3.
func compareKey(line string, cfg config) string {
	s := line

	// R3.2: skip fields.
	if cfg.skipFields > 0 {
		s = skipFields(s, cfg.skipFields)
	}

	// R3.3: skip characters.
	if cfg.skipChars > 0 {
		if cfg.skipChars >= len(s) {
			s = ""
		} else {
			s = s[cfg.skipChars:]
		}
	}

	// R3.4: limit comparison length.
	if cfg.checkChars > 0 && cfg.checkChars < len(s) {
		s = s[:cfg.checkChars]
	}

	// R3.1: case-insensitive.
	if cfg.ignoreCase {
		s = strings.ToUpper(s)
	}

	return s
}

// skipFields skips N whitespace-delimited fields from the start of s. R3.2.
// A field is a run of blanks (space/tab) followed by a run of non-blanks.
func skipFields(s string, n int) string {
	i := 0
	for f := 0; f < n && i < len(s); f++ {
		// Skip blanks.
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		// Skip non-blanks.
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
	}
	return s[i:]
}

// scanNULLines is a bufio.SplitFunc that splits on NUL bytes. R2 -z.
func scanNULLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			positional = append(positional, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			var err error
			err = parseLongOption(arg, args, &i, &cfg)
			if err != nil {
				return cfg, err
			}
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			if err := parseShortOptions(arg, args, &i, &cfg); err != nil {
				return cfg, err
			}
			i++
			continue
		}

		// Positional argument.
		positional = append(positional, arg)
		i++
	}

	if len(positional) > 2 {
		return cfg, fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) >= 1 {
		cfg.inputFile = positional[0]
	}
	if len(positional) >= 2 {
		cfg.outputFile = positional[1]
	}

	return cfg, nil
}

// parseLongOption handles a single --long-option argument. The index i points
// to the current arg and may be advanced if the option consumes the next arg.
func parseLongOption(arg string, _ []string, _ *int, cfg *config) error {
	switch {
	case arg == "--count":
		cfg.count = true
	case arg == "--repeated":
		cfg.mode = modeDupOnly
	case arg == "--all-repeated" || strings.HasPrefix(arg, "--all-repeated="):
		cfg.mode = modeAllDup
		if strings.HasPrefix(arg, "--all-repeated=") {
			method := arg[len("--all-repeated="):]
			switch method {
			case "none":
				cfg.allRepMethod = delimNone
			case "prepend":
				cfg.allRepMethod = delimPrepend
			case "separate":
				cfg.allRepMethod = delimSeparate
			default:
				return fmt.Errorf("invalid argument '%s' for '--all-repeated'", method)
			}
		}
	case arg == "--unique":
		cfg.mode = modeUniqueOnly
	case arg == "--ignore-case":
		cfg.ignoreCase = true
	case strings.HasPrefix(arg, "--skip-fields="):
		n, err := strconv.Atoi(arg[len("--skip-fields="):])
		if err != nil {
			return fmt.Errorf("invalid number of fields to skip: '%s'", arg[len("--skip-fields="):])
		}
		cfg.skipFields = n
	case strings.HasPrefix(arg, "--skip-chars="):
		n, err := strconv.Atoi(arg[len("--skip-chars="):])
		if err != nil {
			return fmt.Errorf("invalid number of bytes to skip: '%s'", arg[len("--skip-chars="):])
		}
		cfg.skipChars = n
	case strings.HasPrefix(arg, "--check-chars="):
		n, err := strconv.Atoi(arg[len("--check-chars="):])
		if err != nil {
			return fmt.Errorf("invalid number of bytes to compare: '%s'", arg[len("--check-chars="):])
		}
		cfg.checkChars = n
	case arg == "--zero-terminated":
		cfg.zeroTerminated = true
	case arg == "--group" || strings.HasPrefix(arg, "--group="):
		cfg.groupEnabled = true
		cfg.group = groupSeparate
		if strings.HasPrefix(arg, "--group=") {
			method := arg[len("--group="):]
			switch method {
			case "separate":
				cfg.group = groupSeparate
			case "prepend":
				cfg.group = groupPrepend
			case "append":
				cfg.group = groupAppend
			case "both":
				cfg.group = groupBoth
			default:
				return fmt.Errorf("invalid argument '%s' for '--group'", method)
			}
		}
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// parseShortOptions handles a cluster of short options (e.g. "-ci").
// The index i points to the current arg and may be advanced if an option
// consumes the next arg.
func parseShortOptions(arg string, args []string, i *int, cfg *config) error {
	j := 1
	for j < len(arg) {
		ch := arg[j]
		switch ch {
		case 'c':
			cfg.count = true
			j++
		case 'd':
			cfg.mode = modeDupOnly
			j++
		case 'D':
			cfg.mode = modeAllDup
			j++
		case 'u':
			cfg.mode = modeUniqueOnly
			j++
		case 'i':
			cfg.ignoreCase = true
			j++
		case 'z':
			cfg.zeroTerminated = true
			j++
		case 'f':
			n, err := consumeNumericArg(arg, j, args, i)
			if err != nil {
				return fmt.Errorf("invalid number of fields to skip: %v", err)
			}
			cfg.skipFields = n
			j = len(arg)
		case 's':
			n, err := consumeNumericArg(arg, j, args, i)
			if err != nil {
				return fmt.Errorf("invalid number of bytes to skip: %v", err)
			}
			cfg.skipChars = n
			j = len(arg)
		case 'w':
			n, err := consumeNumericArg(arg, j, args, i)
			if err != nil {
				return fmt.Errorf("invalid number of bytes to compare: %v", err)
			}
			cfg.checkChars = n
			j = len(arg)
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// consumeNumericArg reads a numeric argument from the remainder of the current
// arg string or the next arg. pos is the index of the option character in arg.
func consumeNumericArg(arg string, pos int, args []string, i *int) (int, error) {
	rest := arg[pos+1:]
	if rest != "" {
		return strconv.Atoi(rest)
	}
	*i++
	if *i >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", arg[pos])
	}
	return strconv.Atoi(args[*i])
}
