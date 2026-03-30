// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/join implements GNU join: join lines of two files on a common field.
//
// Implements prd069-join R1.1 (default join on first field),
// R1.2 (whitespace field separator, space output separator),
// R1.3 (suppress unpairable lines by default),
// R1.4 (stdin via '-').
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "join"

// joinConfig holds parsed flags for a join invocation.
type joinConfig struct {
	file1 string
	file2 string
}

// lineReader wraps a bufio.Scanner with field splitting for join operations.
type lineReader struct {
	scanner *bufio.Scanner
	fields  []string
	hasLine bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments, opens files, and performs the join.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	return executeJoin(cfg, stdin, stdout, stderr)
}

// executeJoin opens files and runs the join operation.
func executeJoin(cfg joinConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	r1, c1, err := openFile(cfg.file1, stdin)
	if err != nil {
		printFileError(stderr, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close
	}
	r2, c2, err := openFile(cfg.file2, stdin)
	if err != nil {
		printFileError(stderr, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close
	}
	if err := joinFiles(r1, r2, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// parseArgs extracts flags and the two file operands.
func parseArgs(args []string) (joinConfig, error) {
	var cfg joinConfig
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		return cfg, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if len(files) < 2 {
		return cfg, fmt.Errorf("missing operand")
	}
	if len(files) > 2 {
		return cfg, fmt.Errorf("extra operand '%s'", files[2])
	}
	cfg.file1 = files[0]
	cfg.file2 = files[1]
	return cfg, nil
}

// openFile opens a file for reading. "-" means stdin. R1.4.
func openFile(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// printFileError writes a GNU-compatible file error message to stderr.
func printFileError(stderr io.Writer, err error) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(stderr, "%s: %s: %v\n", programName, pe.Path, pe.Err)
		return
	}
	fmt.Fprintf(stderr, "%s: %v\n", programName, err)
}

// newLineReader creates a lineReader and reads the first line.
func newLineReader(r io.Reader) *lineReader {
	lr := &lineReader{scanner: bufio.NewScanner(r)}
	lr.advance()
	return lr
}

// advance reads the next line and splits it into fields.
func (lr *lineReader) advance() {
	lr.hasLine = lr.scanner.Scan()
	if lr.hasLine {
		lr.fields = strings.Fields(lr.scanner.Text())
	}
}

// key returns the join field value (first field by default).
func (lr *lineReader) key() string {
	if len(lr.fields) == 0 {
		return ""
	}
	return lr.fields[0]
}

// joinFiles reads two sorted inputs and writes joined output. R1.1, R1.2, R1.3.
func joinFiles(r1, r2 io.Reader, w io.Writer) error {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)
	bw := bufio.NewWriter(w)
	for lr1.hasLine && lr2.hasLine {
		if err := joinStep(lr1, lr2, bw); err != nil {
			bw.Flush() // best-effort flush
			return err
		}
	}
	if err := lr1.scanner.Err(); err != nil {
		return err
	}
	if err := lr2.scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// joinStep compares keys and dispatches to match or skip.
func joinStep(lr1, lr2 *lineReader, bw *bufio.Writer) error {
	k1 := lr1.key()
	k2 := lr2.key()
	if k1 < k2 {
		// R1.3: unpairable from file1, suppress by default.
		lr1.advance()
		return nil
	}
	if k1 > k2 {
		// R1.3: unpairable from file2, suppress by default.
		lr2.advance()
		return nil
	}
	return processMatch(lr1, lr2, bw)
}

// processMatch handles matching keys by collecting file2 group and pairing.
func processMatch(lr1, lr2 *lineReader, bw *bufio.Writer) error {
	key := lr1.key()
	group2 := collectGroup(lr2, key)
	for lr1.hasLine && lr1.key() == key {
		for _, f2 := range group2 {
			if err := writePair(bw, key, lr1.fields, f2); err != nil {
				return err
			}
		}
		lr1.advance()
	}
	return nil
}

// collectGroup gathers all consecutive lines with the given key from lr.
func collectGroup(lr *lineReader, key string) [][]string {
	var group [][]string
	for lr.hasLine && lr.key() == key {
		fields := make([]string, len(lr.fields))
		copy(fields, lr.fields)
		group = append(group, fields)
		lr.advance()
	}
	return group
}

// writePair writes one joined output line: join_field, file1 rest, file2 rest.
// R1.2: output separator is a single space by default.
func writePair(bw *bufio.Writer, key string, f1, f2 []string) error {
	parts := []string{key}
	if len(f1) > 1 {
		parts = append(parts, f1[1:]...)
	}
	if len(f2) > 1 {
		parts = append(parts, f2[1:]...)
	}
	if _, err := bw.WriteString(strings.Join(parts, " ")); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}
