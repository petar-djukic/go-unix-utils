// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd018-head: Print the First Lines or Bytes of Files.
// Covers R1.1-R1.5 (line-count mode, default 10 lines, stdin, line termination),
// R2.1 (-c/--bytes byte-count mode), and --help/--version.
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultLines = 10

// countMode specifies whether head counts lines or bytes.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// config holds parsed flag state.
type config struct {
	mode     countMode
	count    int64
	negative bool // true when count means "all but last N"
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
// R1.4: when no files given, reads stdin. R4.1/R4.2: exit code handling.
func run(cfg config, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	multiFile := len(files) > 1
	exitCode := 0
	needSep := false

	for _, name := range files {
		if err := processFile(cfg, name, multiFile, needSep); err != nil {
			fmt.Fprintf(os.Stderr, "head: %v\n", err)
			exitCode = 1
			continue
		}
		needSep = true
	}

	return exitCode
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

	// R3.1: print header when multiple files.
	if showHeader {
		printHeader(displayName, needSep)
	}

	return outputContent(r, cfg)
}

// openInput opens a named file or returns stdin for "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// printHeader prints the GNU-style multi-file header.
// R1.4/R3.1: '==> FILENAME <==' with blank line between files.
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
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, append([]byte(nil), line...))
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	end := int64(len(lines)) - n
	if end < 0 {
		end = 0
	}

	for i := int64(0); i < end; i++ {
		if _, err := os.Stdout.Write(lines[i]); err != nil {
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
		arg := args[i]
		switch {
		case arg == "--":
			files = append(files, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case strings.HasPrefix(arg, "--lines="):
			exit = applyCount(&cfg, modeLines, arg[len("--lines="):])
		case strings.HasPrefix(arg, "--bytes="):
			exit = applyCount(&cfg, modeBytes, arg[len("--bytes="):])
		case arg == "-n" || arg == "--lines":
			i, exit = consumeNext(args, i, &cfg, modeLines)
		case arg == "-c" || arg == "--bytes":
			i, exit = consumeNext(args, i, &cfg, modeBytes)
		case strings.HasPrefix(arg, "-n"):
			exit = applyCount(&cfg, modeLines, arg[2:])
		case strings.HasPrefix(arg, "-c"):
			exit = applyCount(&cfg, modeBytes, arg[2:])
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr, "head: unrecognized option '%s'\n", arg)
			return config{}, nil, 1
		default:
			files = append(files, arg)
		}
		if exit >= 0 {
			return config{}, nil, exit
		}
	}

	return
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

// consumeNext reads the next argument as a count value. Returns the
// updated index and exit code (-1 for success, 1 for error).
func consumeNext(args []string, i int, cfg *config, m countMode) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"head: option requires an argument -- '%s'\n", args[i])
		return i, 1
	}
	return i + 1, applyCount(cfg, m, args[i+1])
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
