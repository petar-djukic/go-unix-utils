// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail: Print the last lines or bytes of files.
// R1 (line-count mode), R2 (byte-count mode), R3 (multi-file headers),
// R4 (exit codes and differential testing).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultLines = 10

// countMode distinguishes between line-count and byte-count modes.
// R1: line mode is the default; R2: byte mode is selected by -c.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// headerMode controls header display for multi-file output.
type headerMode int

const (
	headerAuto    headerMode = iota // R3.1/R3.2: headers only for multiple files
	headerQuiet                     // R3.3: suppress all headers
	headerVerbose                   // R3.4: always show headers
)

// config holds the parsed command-line flags for tail.
// R1.1–R1.4: line-count mode fields.
// R2.1–R2.3: byte-count mode fields.
// R3.1–R3.4: header control fields.
type config struct {
	// mode selects line-count vs byte-count (R1 vs R2).
	mode countMode
	// count is the number of lines or bytes to output (R1.2, R2.1).
	count int64
	// fromStart is true when the +N prefix is used (R1.3, R2.2).
	fromStart bool
	// header controls header display (R3.1–R3.4).
	header headerMode
	// zeroTerminated uses NUL instead of newline as line delimiter.
	zeroTerminated bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, files))
}

// run processes all input files and returns the exit code.
// R1.4: when no files given, reads stdin.
// R4.1/R4.2: exit 0 on success, exit 1 if any file fails.
func run(cfg config, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	showHeader := resolveHeaderMode(cfg.header, len(files))
	exitCode := 0
	needSep := false

	for _, name := range files {
		if err := processFile(cfg, name, showHeader, needSep); err != nil {
			// R4.4: print error and continue processing remaining files.
			fmt.Fprintf(os.Stderr, "tail: %v\n", err)
			exitCode = 1
		} else {
			needSep = true
		}
	}

	return exitCode
}

// resolveHeaderMode determines whether headers should be printed.
// R3.1/R3.2: auto shows headers for multiple files.
// R3.3: quiet suppresses all headers. R3.4: verbose always shows.
func resolveHeaderMode(hm headerMode, fileCount int) bool {
	switch hm {
	case headerQuiet:
		return false
	case headerVerbose:
		return true
	default:
		return fileCount > 1
	}
}

// processFile reads and outputs content from a single file or stdin.
// R4.2/R4.4: returns error on open failure.
func processFile(cfg config, name string, showHeader, needSep bool) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}

	if showHeader {
		printHeader(fileDisplayName(name), needSep)
	}

	return outputContent(r, cfg)
}

// fileDisplayName returns the display name for a file argument.
// R3.1: stdin ("-") displays as "standard input".
func fileDisplayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// openInput opens a named file or returns stdin for "-".
// R1.4: reads from stdin when file is "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError produces a GNU-compatible error message.
func formatOpenError(name string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("cannot open '%s' for reading: %v", name, pathErr.Err)
	}
	return fmt.Errorf("cannot open '%s' for reading: %v", name, err)
}

// printHeader prints the GNU-style multi-file header.
// R3.1: '==> FILENAME <==' with blank line between files.
func printHeader(name string, needSep bool) {
	if needSep {
		fmt.Println()
	}
	fmt.Printf("==> %s <==\n", name)
}

// outputContent dispatches to the appropriate output function.
func outputContent(r io.Reader, cfg config) error {
	if cfg.fromStart {
		if cfg.mode == modeBytes {
			return outputFromByte(r, cfg.count)
		}
		return outputFromLine(r, cfg.count)
	}
	if cfg.mode == modeBytes {
		return outputLastBytes(r, cfg.count)
	}
	return outputLastLines(r, cfg.count)
}

// outputLastLines reads all lines, then outputs the last n.
// R1.1/R1.2: line-count output preserving original line endings.
func outputLastLines(r io.Reader, n int64) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	start := int64(len(lines)) - n
	if start < 0 {
		start = 0
	}
	return writeLines(lines[start:])
}

// outputFromLine outputs starting from line n to the end.
// R1.3: +N means start from line N (1-based).
func outputFromLine(r io.Reader, n int64) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var lineNum int64

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNum++
			if lineNum >= n {
				if _, werr := os.Stdout.Write(line); werr != nil {
					return werr
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// outputLastBytes reads all data, then outputs the last n bytes.
// R2.1: byte-count mode.
func outputLastBytes(r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := int64(len(data)) - n
	if start < 0 {
		start = 0
	}
	_, werr := os.Stdout.Write(data[start:])
	return werr
}

// outputFromByte outputs starting from byte n to the end.
// R2.2: +N means start from byte N (1-based).
func outputFromByte(r io.Reader, n int64) error {
	if n > 1 {
		discarded, err := io.CopyN(io.Discard, r, n-1)
		if err != nil && err != io.EOF {
			return err
		}
		if discarded < n-1 {
			return nil
		}
	}
	_, err := io.Copy(os.Stdout, r)
	return err
}

// readAllLines reads all lines from r and returns them.
func readAllLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, append([]byte(nil), line...))
		}
		if err != nil {
			if err == io.EOF {
				return lines, nil
			}
			return nil, err
		}
	}
}

// writeLines writes all lines to stdout.
func writeLines(lines [][]byte) error {
	for _, line := range lines {
		if _, err := os.Stdout.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (cfg config, files []string, exit int) {
	cfg.count = defaultLines
	exit = -1

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			files = append(files, args[i+1:]...)
			return
		}
		exit = parseOneArg(args[i], args, &i, &cfg, &files)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}

	return
}

// parseOneArg handles a single argument and returns exit code (-1 to continue).
func parseOneArg(
	arg string, args []string, i *int, cfg *config, files *[]string,
) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "-q" || arg == "--quiet" || arg == "--silent":
		cfg.header = headerQuiet
	case arg == "-v" || arg == "--verbose":
		cfg.header = headerVerbose
	case strings.HasPrefix(arg, "--lines="):
		return applyCount(cfg, modeLines, arg[len("--lines="):])
	case strings.HasPrefix(arg, "--bytes="):
		return applyCount(cfg, modeBytes, arg[len("--bytes="):])
	case arg == "-n" || arg == "--lines":
		return consumeNextVal(args, i, cfg, modeLines)
	case arg == "-c" || arg == "--bytes":
		return consumeNextVal(args, i, cfg, modeBytes)
	case strings.HasPrefix(arg, "-n"):
		return applyCount(cfg, modeLines, arg[2:])
	case strings.HasPrefix(arg, "-c"):
		return applyCount(cfg, modeBytes, arg[2:])
	case isNumericArg(arg):
		return applyCount(cfg, modeLines, arg[1:])
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		fmt.Fprintf(os.Stderr, "tail: unrecognized option '%s'\n", arg)
		return 1
	default:
		*files = append(*files, arg)
	}
	return -1
}

// isNumericArg returns true for legacy -NUM tail syntax (e.g. -5).
func isNumericArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	return arg[1] >= '0' && arg[1] <= '9'
}

// applyCount parses a count value and applies it to cfg.
func applyCount(cfg *config, m countMode, val string) int {
	if err := setCount(cfg, m, val); err != nil {
		fmt.Fprintf(os.Stderr, "tail: %v\n", err)
		return 1
	}
	return -1
}

// consumeNextVal reads the next argument as a count value.
func consumeNextVal(
	args []string, i *int, cfg *config, m countMode,
) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"tail: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applyCount(cfg, m, args[*i])
}

// setCount parses a count string and updates cfg.
// R1.2/R1.3: +N means from start; plain N or -N means from end.
// R2.3: multiplier suffixes via sizeparse.
func setCount(cfg *config, m countMode, val string) error {
	cfg.mode = m
	cfg.fromStart = false

	if strings.HasPrefix(val, "+") {
		cfg.fromStart = true
		val = val[1:]
	} else if strings.HasPrefix(val, "-") {
		val = val[1:]
	}

	n, err := sizeparse.Parse(val)
	if err != nil {
		return fmt.Errorf("invalid number of %s: '%s'", modeLabel(m), val)
	}
	if n < 0 {
		return fmt.Errorf("invalid number of %s: '%s'", modeLabel(m), val)
	}

	cfg.count = n
	return nil
}

// modeLabel returns the human-readable label for the count mode.
func modeLabel(m countMode) string {
	if m == modeBytes {
		return "bytes"
	}
	return "lines"
}

// printHelp writes usage information to stdout and returns exit code.
// R1.4: --help prints usage and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: tail [OPTION]... [FILE]...
Print the last 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=[+]NUM      output the last NUM bytes; or use -c +NUM to
                             output starting with byte NUM of each file
  -n, --lines=[+]NUM      output the last NUM lines, instead of the last 10;
                             or use -n +NUM to output starting with line NUM
  -q, --quiet, --silent   never output headers giving file names
  -v, --verbose            always output headers giving file names

      --help     display this help and exit
      --version  output version information and exit

NUM may have a multiplier suffix:
b 512, kB 1000, K 1024, MB 1000*1000, M 1024*1024,
GB 1000*1000*1000, G 1024*1024*1024, and so on for T, P, E.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "tail (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
