// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd069-join R1.1–R1.4, R2.1, R2.2, R2.4.
// R1.1: Read two sorted files, join on common field (default: field 1).
// R1.2: Default whitespace field splitting, single-space output separator.
// R1.3: Unpaired lines are suppressed by default.
// R1.4: "-" reads from stdin.
// R2.1: -1 FIELD and -2 FIELD set join fields for each file.
// R2.2: -j FIELD sets the join field for both files.
// R2.4: -t CHAR sets both input and output field separator.
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

// joinConfig holds parsed command-line options.
type joinConfig struct {
	field1 int    // join field for file 1 (0-indexed)
	field2 int    // join field for file 2 (0-indexed)
	sep    string // field separator; empty means whitespace
	hasSep bool   // true if -t was specified
}

// outputSep returns the output field separator.
func (c joinConfig) outputSep() string {
	if c.hasSep {
		return c.sep
	}
	return " "
}

// lineReader reads lines one at a time from an io.Reader,
// keeping the current line available for peek-style access.
type lineReader struct {
	scanner *bufio.Scanner
	valid   bool
	line    string
}

func newLineReader(r io.Reader) *lineReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lr := &lineReader{scanner: s}
	lr.advance()
	return lr
}

func (lr *lineReader) advance() {
	lr.valid = lr.scanner.Scan()
	if lr.valid {
		lr.line = lr.scanner.Text()
	}
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, file1, file2, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	return executeJoin(cfg, file1, file2)
}

// executeJoin opens both inputs, performs the join, and flushes output.
func executeJoin(cfg joinConfig, file1, file2 string) int {
	r1, c1, err := openInput(file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	if c1 != nil {
		defer c1.Close()
	}
	r2, c2, err := openInput(file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	if c2 != nil {
		defer c2.Close()
	}
	w := bufio.NewWriter(os.Stdout)
	joinStreams(w, r1, r2, cfg)
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "join: write error: %v\n", err)
		return 1
	}
	return 0
}

// parseArgs extracts flags and two file operands from command-line arguments.
func parseArgs(args []string) (joinConfig, string, string, error) {
	cfg := joinConfig{}
	var operands []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		consumed, err := parseFlag(args[i], args, &i, &cfg)
		if err != nil {
			return cfg, "", "", err
		}
		if !consumed {
			operands = append(operands, args[i])
		}
	}
	if len(operands) != 2 {
		return cfg, "", "", fmt.Errorf("missing operand")
	}
	return cfg, operands[0], operands[1], nil
}

// parseFlag attempts to parse a single flag argument. Returns true if consumed.
func parseFlag(arg string, args []string, i *int, cfg *joinConfig) (bool, error) {
	if arg == "-" || !strings.HasPrefix(arg, "-") || len(arg) < 2 {
		return false, nil
	}
	flag := arg[:2]
	rest := arg[2:]
	switch flag {
	case "-1":
		n, err := parseFieldValue(rest, args, i, "1")
		if err != nil {
			return true, err
		}
		cfg.field1 = n
		return true, nil
	case "-2":
		n, err := parseFieldValue(rest, args, i, "2")
		if err != nil {
			return true, err
		}
		cfg.field2 = n
		return true, nil
	case "-j":
		return parseJFlag(rest, args, i, cfg)
	case "-t":
		return true, parseSepValue(rest, args, i, cfg)
	default:
		return false, fmt.Errorf("invalid option -- '%c'", arg[1])
	}
}

// parseJFlag handles -j FIELD, setting both join fields.
func parseJFlag(rest string, args []string, i *int, cfg *joinConfig) (bool, error) {
	n, err := parseFieldValue(rest, args, i, "j")
	if err != nil {
		return true, err
	}
	cfg.field1 = n
	cfg.field2 = n
	return true, nil
}

// parseFieldValue extracts a 1-indexed field number from inline or next arg.
func parseFieldValue(rest string, args []string, i *int, flagChar string) (int, error) {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- '%s'", flagChar)
		}
		*i++
		val = args[*i]
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid field number: '%s'", val)
	}
	return n - 1, nil // convert to 0-indexed
}

// parseSepValue extracts the separator character from inline or next arg.
func parseSepValue(rest string, args []string, i *int, cfg *joinConfig) error {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- 't'")
		}
		*i++
		val = args[*i]
	}
	cfg.sep = val
	cfg.hasSep = true
	return nil
}

// openInput opens a file for reading, or returns stdin for "-".
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	return f, f, nil
}

// splitLine splits a line into fields based on separator configuration.
// With -t: exact single-character split. Without: whitespace runs.
func splitLine(line string, cfg joinConfig) []string {
	if cfg.hasSep {
		return strings.Split(line, cfg.sep)
	}
	return strings.Fields(line)
}

// getKey extracts the join field value from a fields slice.
func getKey(fields []string, fieldIdx int) string {
	if fieldIdx < len(fields) {
		return fields[fieldIdx]
	}
	return ""
}

// joinStreams performs the merge-join of two sorted inputs.
func joinStreams(w *bufio.Writer, r1, r2 io.Reader, cfg joinConfig) {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)
	for lr1.valid && lr2.valid {
		f1 := splitLine(lr1.line, cfg)
		f2 := splitLine(lr2.line, cfg)
		key1 := getKey(f1, cfg.field1)
		key2 := getKey(f2, cfg.field2)
		cmp := strings.Compare(key1, key2)
		if cmp < 0 {
			lr1.advance()
			continue
		}
		if cmp > 0 {
			lr2.advance()
			continue
		}
		joinGroup(w, lr1, lr2, cfg, key1)
	}
}

// joinGroup joins all file1 and file2 lines sharing the current key.
// Buffers file2 lines with the matching key, then cross-joins with
// all file1 lines that also match.
func joinGroup(w *bufio.Writer, lr1, lr2 *lineReader, cfg joinConfig, key string) {
	var group2 [][]string
	for lr2.valid {
		f2 := splitLine(lr2.line, cfg)
		if getKey(f2, cfg.field2) != key {
			break
		}
		group2 = append(group2, f2)
		lr2.advance()
	}
	for lr1.valid {
		f1 := splitLine(lr1.line, cfg)
		if getKey(f1, cfg.field1) != key {
			break
		}
		for _, f2 := range group2 {
			writeJoinLine(w, key, f1, f2, cfg)
		}
		lr1.advance()
	}
}

// writeJoinLine writes one joined output line: join field, then remaining
// fields from file1, then remaining fields from file2.
func writeJoinLine(w *bufio.Writer, key string, f1, f2 []string, cfg joinConfig) {
	sep := cfg.outputSep()
	w.WriteString(key)
	writeRemainingFields(w, f1, cfg.field1, sep)
	writeRemainingFields(w, f2, cfg.field2, sep)
	w.WriteByte('\n')
}

// writeRemainingFields writes all fields except the join field,
// each preceded by the separator.
func writeRemainingFields(w *bufio.Writer, fields []string, joinIdx int, sep string) {
	for i, f := range fields {
		if i == joinIdx {
			continue
		}
		w.WriteString(sep)
		w.WriteString(f)
	}
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
