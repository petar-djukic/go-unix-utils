// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uniq implements GNU uniq: report or filter adjacent duplicate lines.
//
// Implements prd028-uniq R1.1-R1.4 (adjacent-line deduplication, I/O, SIGPIPE),
// R2.1 (-d duplicate-only), R2.2 (-D all-duplicates), R2.3 (-u unique-only),
// R2.4 (-c count prefix).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "uniq"

// options holds the parsed command-line flags for output selection.
type options struct {
	count      bool // R2.4: -c prefix lines by count
	dupOnly    bool // R2.1: -d only print duplicate lines (one per run)
	allDup     bool // R2.2: -D print all duplicate lines
	uniqueOnly bool // R2.3: -u only print unique lines
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags and positional arguments, then processes input.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, positional, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	inputFile, outputFile := extractPositional(positional)
	r, closer, err := openInput(inputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	w, wCloser, err := openOutput(outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if wCloser != nil {
		defer wCloser.Close() // best-effort close
	}
	if err := process(r, w, opts); err != nil {
		fmt.Fprintf(stderr, "%s: write error\n", programName)
		return 1
	}
	return 0
}

// parseFlags parses flag arguments and returns options and remaining positional args.
func parseFlags(args []string) (options, []string, error) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts options
	fs.BoolVar(&opts.count, "c", false, "prefix lines by count")
	fs.BoolVar(&opts.dupOnly, "d", false, "only print duplicate lines")
	fs.BoolVar(&opts.allDup, "D", false, "print all duplicate lines")
	fs.BoolVar(&opts.uniqueOnly, "u", false, "only print unique lines")
	if err := fs.Parse(args); err != nil {
		return options{}, nil, err
	}
	return opts, fs.Args(), nil
}

// extractPositional extracts input-file and output-file from positional args.
// R1.2: first positional is input, second is output. Both are optional.
func extractPositional(args []string) (string, string) {
	inputFile := "-"
	outputFile := ""
	if len(args) >= 1 {
		inputFile = args[0]
	}
	if len(args) >= 2 {
		outputFile = args[1]
	}
	return inputFile, outputFile
}

// openInput returns a reader and optional closer for the given filename.
// R1.3: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: No such file or directory", name)
	}
	return f, f, nil
}

// openOutput returns a writer and optional closer for the given filename.
// Empty string means stdout.
func openOutput(name string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if name == "" {
		return stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	return f, f, nil
}

// process reads lines and writes output according to the selected options.
// It tracks runs of identical adjacent lines and flushes each run when
// a different line is encountered or input ends.
func process(r io.Reader, w io.Writer, opts options) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	bw := bufio.NewWriter(w)
	prev := ""
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if count == 0 {
			prev = line
			count = 1
			continue
		}
		if line == prev {
			count++
			continue
		}
		if err := flushRun(bw, prev, count, opts); err != nil {
			return err
		}
		prev = line
		count = 1
	}
	if count > 0 {
		if err := flushRun(bw, prev, count, opts); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// flushRun writes a completed run of identical lines according to opts.
// R2.1: -d prints one copy if count > 1.
// R2.2: -D prints all copies if count > 1.
// R2.3: -u prints one copy if count == 1.
// R2.4: -c prefixes with the run count.
func flushRun(w *bufio.Writer, line string, count int, opts options) error {
	if opts.allDup {
		return flushAllDup(w, line, count)
	}
	if opts.dupOnly && count <= 1 {
		return nil
	}
	if opts.uniqueOnly && count != 1 {
		return nil
	}
	return writeCountedLine(w, line, count, opts.count)
}

// flushAllDup writes every copy of a duplicate run (R2.2: -D).
// Lines that appear only once are suppressed.
func flushAllDup(w *bufio.Writer, line string, count int) error {
	if count <= 1 {
		return nil
	}
	for range count {
		if err := writeLine(w, line); err != nil {
			return err
		}
	}
	return nil
}

// writeCountedLine writes a line with an optional count prefix (R2.4: -c).
func writeCountedLine(w *bufio.Writer, line string, count int, showCount bool) error {
	if showCount {
		_, err := fmt.Fprintf(w, "%7d %s\n", count, line)
		return err
	}
	return writeLine(w, line)
}

// writeLine writes a single line followed by a newline.
func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
