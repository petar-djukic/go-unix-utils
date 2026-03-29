// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/paste implements GNU paste: merge lines of files.
//
// Implements prd027-paste R1.1 (parallel merge), R1.2 (unequal line counts),
// R1.3 (stdin via dash), R1.4 (no-file passthrough and SIGPIPE).
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

const programName = "paste"

var (
	errVersion = errors.New("version requested")
	errHelp    = errors.New("help requested")
)

// pasteOptions holds parsed flag state.
type pasteOptions struct {
	serial    bool
	delimiter string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags and dispatches to parallel or serial paste.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		return handleParseError(err, stdout, stderr)
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := executePaste(files, stdin, bw, stderr, opts)
	if flushErr := bw.Flush(); flushErr != nil {
		fmt.Fprintf(stderr, "%s: write error\n", programName)
		return 1
	}
	return exitCode
}

// executePaste dispatches to serial or parallel mode.
func executePaste(files []string, stdin io.Reader, w *bufio.Writer, stderr io.Writer, opts pasteOptions) int {
	if opts.serial {
		return serialPaste(files, stdin, w, stderr, opts)
	}
	return parallelPaste(files, stdin, w, stderr, opts)
}

// handleParseError dispatches --version, --help, and real errors.
func handleParseError(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, errVersion) {
		fmt.Fprintln(stdout, "paste (go-unix-utils)")
		return 0
	}
	if errors.Is(err, errHelp) {
		printHelp(stdout)
		return 0
	}
	fmt.Fprintf(stderr, "%s: %s\n", programName, err)
	return 1
}

// printHelp writes usage information.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", programName)
	fmt.Fprintln(w, "Write lines consisting of the sequentially corresponding lines from")
	fmt.Fprintln(w, "each FILE, separated by TABs, to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -d, --delimiters=LIST   reuse characters from LIST instead of TABs")
	fmt.Fprintln(w, "  -s, --serial            paste one file at a time instead of in parallel")
	fmt.Fprintln(w, "      --help              display this help and exit")
	fmt.Fprintln(w, "      --version           output version information and exit")
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (pasteOptions, []string, error) {
	opts := pasteOptions{delimiter: "\t"}
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		var err error
		i, err = parseFlag(&opts, args, i)
		if err != nil {
			return opts, nil, err
		}
	}
	return opts, files, nil
}

// parseFlag handles a single flag at args[i].
func parseFlag(opts *pasteOptions, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--version":
		return i, errVersion
	case arg == "--help":
		return i, errHelp
	case arg == "-s" || arg == "--serial":
		opts.serial = true
	case arg == "-d":
		i++
		if i >= len(args) {
			return i, fmt.Errorf("option requires an argument -- 'd'")
		}
		opts.delimiter = args[i]
	case strings.HasPrefix(arg, "-d"):
		opts.delimiter = arg[2:]
	case strings.HasPrefix(arg, "--delimiters="):
		opts.delimiter = arg[len("--delimiters="):]
	default:
		return i, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
	return i, nil
}

// openInputs opens all files and returns scanners and closers.
// R1.3: "-" means stdin; multiple "-" share the same scanner.
func openInputs(files []string, stdin io.Reader) ([]*bufio.Scanner, []io.Closer, error) {
	scanners := make([]*bufio.Scanner, len(files))
	closers := make([]io.Closer, len(files))
	var stdinScanner *bufio.Scanner
	for i, name := range files {
		if name == "-" {
			if stdinScanner == nil {
				stdinScanner = bufio.NewScanner(stdin)
			}
			scanners[i] = stdinScanner
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			closeAll(closers)
			return nil, nil, fmt.Errorf("%s: No such file or directory", name)
		}
		scanners[i] = bufio.NewScanner(f)
		closers[i] = f
	}
	return scanners, closers, nil
}

// closeAll closes all non-nil closers.
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		if c != nil {
			c.Close() // best-effort close
		}
	}
}

// parallelPaste opens all files and merges them line by line.
// R1.1: join lines with delimiter. R1.2: empty fields for exhausted files.
func parallelPaste(files []string, stdin io.Reader, w *bufio.Writer, stderr io.Writer, opts pasteOptions) int {
	scanners, closers, err := openInputs(files, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	defer closeAll(closers)
	return mergeParallel(scanners, w, opts)
}

// mergeParallel reads one line from each scanner per iteration.
func mergeParallel(scanners []*bufio.Scanner, w *bufio.Writer, opts pasteOptions) int {
	for {
		line, active := buildParallelLine(scanners, opts)
		if !active {
			break
		}
		if _, err := w.WriteString(line); err != nil {
			return 1
		}
		if _, err := w.WriteString("\n"); err != nil {
			return 1
		}
	}
	return 0
}

// buildParallelLine reads one line from each scanner and joins them.
func buildParallelLine(scanners []*bufio.Scanner, opts pasteOptions) (string, bool) {
	parts := make([]string, len(scanners))
	anyActive := false
	for i, s := range scanners {
		if s.Scan() {
			parts[i] = s.Text()
			anyActive = true
		}
	}
	if !anyActive {
		return "", false
	}
	return strings.Join(parts, opts.delimiter), true
}

// serialPaste processes files one at a time, joining all lines per file.
// R1.2 (task R2): each file's lines are joined into a single output line.
func serialPaste(files []string, stdin io.Reader, w *bufio.Writer, stderr io.Writer, opts pasteOptions) int {
	var stdinScanner *bufio.Scanner
	for _, name := range files {
		if err := serialPasteOne(name, &stdinScanner, stdin, w, opts); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName, err)
			return 1
		}
	}
	return 0
}

// serialPasteOne reads all lines from one file and writes them as a single line.
func serialPasteOne(name string, stdinScanner **bufio.Scanner, stdin io.Reader, w *bufio.Writer, opts pasteOptions) error {
	scanner, closer, err := openOneInput(name, stdinScanner, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	return writeSerialLines(scanner, w, opts)
}

// openOneInput opens a single file or returns the shared stdin scanner.
func openOneInput(name string, stdinScanner **bufio.Scanner, stdin io.Reader) (*bufio.Scanner, io.Closer, error) {
	if name == "-" {
		if *stdinScanner == nil {
			*stdinScanner = bufio.NewScanner(stdin)
		}
		return *stdinScanner, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: No such file or directory", name)
	}
	return bufio.NewScanner(f), f, nil
}

// writeSerialLines reads all lines from a scanner and writes them joined.
func writeSerialLines(scanner *bufio.Scanner, w *bufio.Writer, opts pasteOptions) error {
	first := true
	for scanner.Scan() {
		if !first {
			if _, err := w.WriteString(opts.delimiter); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(scanner.Text()); err != nil {
			return err
		}
		first = false
	}
	_, err := w.WriteString("\n")
	return err
}
