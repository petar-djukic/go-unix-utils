// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd018-head: Print the First Lines or Bytes of Files.
// Covers R1.1-R1.5 (line-count mode, default 10 lines, stdin, line termination),
// R2.1-R2.2 (-c/--bytes byte-count mode, negative counts),
// R3.1-R3.5 (multi-file headers, quiet/verbose, error handling),
// R4.1-R4.2 (exit codes), and --help/--version.
// TODO: prd018 non_goal: -z/--zero-terminated is excluded per E6.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultLines = 10

// countMode specifies whether head counts lines or bytes.
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

// config holds parsed flag state.
type config struct {
	mode     countMode
	count    int64
	negative bool       // true when count means "all but last N"
	header   headerMode // controls header display
}

func main() {
	// R1.5/shared protocol: SIGPIPE handler for piped output.
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, files))
}

// run processes all input files and returns the exit code.
// R1.4: when no files given, reads stdin.
// R3.5/R4.1/R4.2: continues after errors, exit 1 if any fail.
func run(cfg config, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	showHeader := resolveHeaderMode(cfg.header, len(files))
	exitCode := 0
	needSep := false

	for _, name := range files {
		if err := processFile(cfg, name, showHeader, needSep); err != nil {
			fmt.Fprintf(os.Stderr, "head: %v\n", err)
			exitCode = 1
			continue
		}
		needSep = true
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
func processFile(cfg config, name string, showHeader, needSep bool) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}

	displayName := name
	if name == "-" {
		displayName = "standard input"
	}

	if showHeader {
		printHeader(displayName, needSep)
	}

	return outputContent(r, cfg)
}

// openInput opens a named file or returns stdin for "-".
// R3.5: formats error messages to match GNU head output.
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

// formatOpenError produces a GNU-compatible error message for file open failures.
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

// outputContent dispatches to the appropriate output function based on config.
func outputContent(r io.Reader, cfg config) error {
	if cfg.negative {
		if cfg.mode == modeBytes {
			return outputAllButLastBytes(r, cfg.count)
		}
		return outputAllButLastLines(r, cfg.count)
	}
	if cfg.mode == modeBytes {
		// R2.1: byte-count mode with io.LimitReader.
		_, err := io.Copy(os.Stdout, io.LimitReader(r, cfg.count))
		return err
	}
	return outputFirstLines(r, cfg.count)
}

// outputFirstLines writes the first n lines from r to stdout.
// R1.1/R1.2: line-count output preserving original line endings.
// R1.5: a line is terminated by '\n'; unterminated final line counts.
func outputFirstLines(r io.Reader, n int64) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var count int64

	for count < n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := os.Stdout.Write(line); werr != nil {
				return werr
			}
			count++
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}

	return nil
}

// outputAllButLastLines reads all lines, then outputs all but the last n.
// R1.3: negative line count requires buffering entire input.
func outputAllButLastLines(r io.Reader, n int64) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}

	end := int64(len(lines)) - n
	if end < 0 {
		end = 0
	}

	return writeLines(lines[:end])
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

// outputAllButLastBytes reads all content, then outputs all but the last n bytes.
// R2.1/R1.3: negative byte count requires buffering entire input.
func outputAllButLastBytes(r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	end := int64(len(data)) - n
	if end < 0 {
		end = 0
	}

	_, werr := os.Stdout.Write(data[:end])
	return werr
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
func parseOneArg(arg string, args []string, i *int, cfg *config, files *[]string) int {
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
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		fmt.Fprintf(os.Stderr, "head: unrecognized option '%s'\n", arg)
		return 1
	default:
		*files = append(*files, arg)
	}
	return -1
}

// applyCount parses a count value and applies it to cfg. Returns -1 on
// success or 1 on error.
func applyCount(cfg *config, m countMode, val string) int {
	if err := setCount(cfg, m, val); err != nil {
		fmt.Fprintf(os.Stderr, "head: %v\n", err)
		return 1
	}
	return -1
}

// consumeNextVal reads the next argument as a count value. Returns
// exit code (-1 for success, 1 for error).
func consumeNextVal(args []string, i *int, cfg *config, m countMode) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"head: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applyCount(cfg, m, args[*i])
}

// setCount parses a count string and updates cfg.
// R1.2: positive N prints first N. Negative N (prefixed with '-') prints
// all but last N.
func setCount(cfg *config, m countMode, val string) error {
	cfg.mode = m
	cfg.negative = false

	if strings.HasPrefix(val, "-") {
		cfg.negative = true
		val = val[1:]
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil || n < 0 {
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
// R2.1: --help prints usage and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: head [OPTION]... [FILE]...
Print the first 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=[-]NUM       print the first NUM bytes of each file;
                             with the leading '-', print all but the last
                             NUM bytes of each file
  -n, --lines=[-]NUM       print the first NUM lines instead of the first 10;
                             with the leading '-', print all but the last
                             NUM lines of each file
  -q, --quiet, --silent    never print headers giving file names
  -v, --verbose            always print headers giving file names

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
// R2.1: --version prints version and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "head (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
