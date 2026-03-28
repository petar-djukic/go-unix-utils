// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail: Print the last lines or bytes of files.
// R1 (line-count mode), R2 (byte-count mode, R2.3 zero-terminated),
// R3 (multi-file headers), R4 (exit codes and differential testing).
// Follow mode: R3.1 (-f), R3.2 (--follow=name), R3.3 (--pid), R3.4 (--max-unchanged-stats).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultLines = 10

// countMode distinguishes between line-count and byte-count modes.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// followMode selects follow behavior.
type followMode int

const (
	followNone       followMode = iota
	followDescriptor                   // -f / --follow / --follow=descriptor
	followName                         // --follow=name / -F
)

// headerMode controls header display for multi-file output.
type headerMode int

const (
	headerAuto    headerMode = iota
	headerQuiet
	headerVerbose
)

// config holds the parsed command-line flags for tail.
type config struct {
	mode              countMode
	count             int64
	fromStart         bool
	header            headerMode
	zeroTerminated    bool
	follow            followMode
	sleepInterval     float64
	pid               int
	maxUnchangedStats int
	retry             bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, files))
}

// delimiter returns the line delimiter byte based on -z flag.
// R2.3: NUL when --zero-terminated, newline otherwise.
func (c config) delimiter() byte {
	if c.zeroTerminated {
		return 0
	}
	return '\n'
}

// run processes all input files and returns the exit code.
func run(cfg config, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	showHeader := resolveHeaderMode(cfg.header, len(files))

	if cfg.follow != followNone {
		return runFollow(cfg, files, showHeader)
	}
	return runStatic(cfg, files, showHeader)
}

// runStatic processes files without follow mode.
func runStatic(cfg config, files []string, showHeader bool) int {
	exitCode := 0
	needSep := false

	for _, name := range files {
		if err := processFile(cfg, name, showHeader, needSep); err != nil {
			fmt.Fprintf(os.Stderr, "tail: %v\n", err)
			exitCode = 1
		} else {
			needSep = true
		}
	}

	return exitCode
}

// resolveHeaderMode determines whether headers should be printed.
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

	if showHeader {
		printHeader(fileDisplayName(name), needSep)
	}

	return outputContent(r, cfg)
}

// fileDisplayName returns the display name for a file argument.
func fileDisplayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// openInput opens a named file or returns stdin for "-".
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
		return outputFromLine(r, cfg.count, cfg.delimiter())
	}
	if cfg.mode == modeBytes {
		return outputLastBytes(r, cfg.count)
	}
	return outputLastLines(r, cfg.count, cfg.delimiter())
}

// outputLastLines reads all lines using the given delimiter, outputs the last n.
func outputLastLines(r io.Reader, n int64, delim byte) error {
	lines, err := readAllLines(r, delim)
	if err != nil {
		return err
	}
	start := int64(len(lines)) - n
	if start < 0 {
		start = 0
	}
	return writeLines(lines[start:])
}

// outputFromLine outputs starting from line n using the given delimiter.
func outputFromLine(r io.Reader, n int64, delim byte) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var lineNum int64

	for {
		line, err := br.ReadBytes(delim)
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

// readAllLines reads all lines from r split by the given delimiter.
func readAllLines(r io.Reader, delim byte) ([][]byte, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte

	for {
		line, err := br.ReadBytes(delim)
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

// parseOneArg handles a single argument for core flags.
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
	case arg == "-z" || arg == "--zero-terminated":
		cfg.zeroTerminated = true
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
	default:
		return parseExtendedArg(arg, args, i, cfg, files)
	}
	return -1
}

// parseExtendedArg handles follow-mode flags and file arguments.
func parseExtendedArg(
	arg string, args []string, i *int, cfg *config, files *[]string,
) int {
	switch {
	case isFollowArg(arg):
		applyFollowArg(arg, cfg)
	case arg == "--retry":
		cfg.retry = true
	case strings.HasPrefix(arg, "--pid="):
		return parsePIDValue(cfg, arg[len("--pid="):])
	case arg == "--pid":
		return consumeAndParse(args, i, "pid", func(v string) int {
			return parsePIDValue(cfg, v)
		})
	case strings.HasPrefix(arg, "--sleep-interval="):
		return parseSleepValue(cfg, arg[len("--sleep-interval="):])
	case arg == "-s" || arg == "--sleep-interval":
		return consumeAndParse(args, i, "sleep-interval", func(v string) int {
			return parseSleepValue(cfg, v)
		})
	case strings.HasPrefix(arg, "-s") && len(arg) > 2:
		return parseSleepValue(cfg, arg[2:])
	case strings.HasPrefix(arg, "--max-unchanged-stats="):
		return parseMaxUnchanged(cfg, arg[len("--max-unchanged-stats="):])
	case arg == "--max-unchanged-stats":
		return consumeAndParse(args, i, "max-unchanged-stats", func(v string) int {
			return parseMaxUnchanged(cfg, v)
		})
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		fmt.Fprintf(os.Stderr, "tail: unrecognized option '%s'\n", arg)
		return 1
	default:
		*files = append(*files, arg)
	}
	return -1
}

// consumeAndParse reads the next argument and passes it to the parser.
func consumeAndParse(
	args []string, i *int, label string, parse func(string) int,
) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"tail: option requires an argument -- '%s'\n", label)
		return 1
	}
	*i++
	return parse(args[*i])
}

// isFollowArg returns true for follow-mode flag variants.
func isFollowArg(arg string) bool {
	return arg == "-f" || arg == "--follow" ||
		arg == "--follow=descriptor" || arg == "--follow=name" ||
		arg == "-F"
}

// applyFollowArg sets follow mode from the given flag.
func applyFollowArg(arg string, cfg *config) {
	switch arg {
	case "-f", "--follow", "--follow=descriptor":
		cfg.follow = followDescriptor
	case "--follow=name":
		cfg.follow = followName
	case "-F":
		cfg.follow = followName
		cfg.retry = true
	}
}

// parsePIDValue parses a PID string and stores it in config.
// R3.3: --pid=PID terminates follow when process dies.
func parsePIDValue(cfg *config, val string) int {
	pid, err := strconv.Atoi(val)
	if err != nil || pid < 0 {
		fmt.Fprintf(os.Stderr, "tail: invalid PID: '%s'\n", val)
		return 1
	}
	cfg.pid = pid
	return -1
}

// parseSleepValue parses a sleep interval and stores it in config.
// R3.1: --sleep-interval=N configures polling frequency.
func parseSleepValue(cfg *config, val string) int {
	s, err := strconv.ParseFloat(val, 64)
	if err != nil || s < 0 {
		fmt.Fprintf(os.Stderr,
			"tail: invalid number of seconds: '%s'\n", val)
		return 1
	}
	cfg.sleepInterval = s
	return -1
}

// parseMaxUnchanged parses the max-unchanged-stats value.
// R3.4: reopen after N iterations with no change.
func parseMaxUnchanged(cfg *config, val string) int {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr,
			"tail: invalid maximum unchanged stats count: '%s'\n", val)
		return 1
	}
	cfg.maxUnchangedStats = n
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
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: tail [OPTION]... [FILE]...
Print the last 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=[+]NUM       output the last NUM bytes; or use -c +NUM to
                              output starting with byte NUM of each file
  -f, --follow[=HOW]       output appended data as the file grows;
                              an absent option argument means 'descriptor'
  -F                        same as --follow=name --retry
  -n, --lines=[+]NUM       output the last NUM lines, instead of the last 10;
                              or use -n +NUM to output starting with line NUM
      --max-unchanged-stats=N  with --follow=name, reopen a FILE which has not
                              changed size in N iterations (default 5)
      --pid=PID             with -f, terminate after process ID PID dies
  -q, --quiet, --silent    never output headers giving file names
      --retry               keep trying to open a file if it is inaccessible
  -s, --sleep-interval=N   with -f, sleep for approximately N seconds
                              (default 1.0) between iterations
  -v, --verbose             always output headers giving file names
  -z, --zero-terminated    line delimiter is NUL, not newline

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
