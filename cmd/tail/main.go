// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tail: print the last lines or bytes of files.
// Implements srd055-tail R1.1-R1.4, R2.1-R2.3, R3.1-R3.4, R4.1-R4.4.
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

// progName is used in diagnostic messages.
const progName = "tail"

// defaultLines is the number of lines printed when no -n flag is given.
// R1.1: default is 10 lines.
const defaultLines int64 = 10

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: tail [OPTION]... [FILE]...
Print the last 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=[+]NUM       output the last NUM bytes; or use -c +NUM to
                             output starting with byte NUM of each file
  -n, --lines=[+]NUM       output the last NUM lines, instead of the last 10;
                             or use -n +NUM to output starting with line NUM
  -q, --quiet, --silent    never output headers giving file names
  -v, --verbose            always output headers giving file names
      --help     display this help and exit
      --version  output version information and exit

NUM may have a multiplier suffix:
b 512, kB 1000, K 1024, MB 1000*1000, M 1024*1024,
GB 1000*1000*1000, G 1024*1024*1024, and so on for T, P, E, Z, Y.
`

// countMode distinguishes line-count from byte-count mode.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// config holds parsed command-line options.
type config struct {
	mode    countMode
	count   int64
	fromPos bool // R1.3: +N means start from position N
	quiet   bool
	verbose bool
	files   []string
	err     bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	// R3.2: handle --version and --help before argument parsing.
	if handleInfoFlags(os.Args[1:]) {
		return
	}

	os.Exit(run(os.Args[1:]))
}

// handleInfoFlags checks for --version and --help, prints and exits 0.
// Returns true if a flag was handled (caller should return).
func handleInfoFlags(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--version":
			fmt.Println(versionText)
			return true
		case "--help":
			fmt.Print(helpText)
			return true
		case "--":
			return false
		}
	}
	return false
}

// run executes the tail logic and returns the exit code.
// R4.1: returns 0 when all files processed successfully.
// R4.2: returns 1 when any file cannot be opened or read.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.err {
		return 1
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	showHeader := shouldShowHeader(&cfg)
	exitCode := 0
	printed := 0
	for _, name := range cfg.files {
		if err := processOneFile(name, &cfg, showHeader, &printed); err != nil {
			reportError(name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// shouldShowHeader determines whether file headers should be printed.
// R3.2: no header for single file by default.
// R3.3: -q suppresses all headers.
// R3.4: -v forces headers even for single file.
func shouldShowHeader(cfg *config) bool {
	if cfg.quiet {
		return false
	}
	if cfg.verbose {
		return true
	}
	return len(cfg.files) > 1
}

// displayName returns the user-visible name for a file argument.
// R3.1: stdin ("-") is displayed as "standard input" to match GNU tail.
func displayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// printHeader writes the GNU tail-format header for a file.
// R3.1: format is "==> FILENAME <==" with blank line between files.
func printHeader(name string, first bool) {
	if !first {
		fmt.Fprintln(os.Stdout)
	}
	fmt.Fprintf(os.Stdout, "==> %s <==\n", displayName(name))
}

// processOneFile opens a file, prints a header if needed, then outputs content.
// R4.4: prints error and continues; header is printed only on successful open.
func processOneFile(name string, cfg *config, showHeader bool, printed *int) error {
	r, closer, err := openInput(name)
	if err != nil {
		return err
	}
	defer closer()
	if showHeader {
		printHeader(name, *printed == 0)
	}
	*printed++
	if cfg.mode == modeBytes {
		return processByteMode(r, cfg.count, cfg.fromPos)
	}
	return processLineMode(r, cfg.count, cfg.fromPos)
}

// processLineMode handles line-count output.
// R1.1, R1.2: print last N lines.
// R1.3: +N prints starting from line N.
func processLineMode(r io.Reader, count int64, fromPos bool) error {
	if fromPos {
		return printFromLineN(r, count)
	}
	return printLastNLines(r, count)
}

// processByteMode handles byte-count output.
// R2.1: -c NUM prints last NUM bytes.
// R2.2: -c +NUM prints starting from byte NUM.
func processByteMode(r io.Reader, count int64, fromPos bool) error {
	if fromPos {
		return printFromByteN(r, count)
	}
	return printLastNBytes(r, count)
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.4: stdin when file argument is "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// printLastNLines writes the last n lines from r to stdout.
// R1.1, R1.2: output last N lines using a ring buffer.
func printLastNLines(r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	return drainLineRing(bufio.NewReader(r), int(n))
}

// drainLineRing reads all lines into a ring buffer, then outputs the last n.
func drainLineRing(br *bufio.Reader, n int) error {
	ring := make([][]byte, n)
	idx := 0
	total := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			ring[idx] = append(ring[idx][:0], line...)
			idx = (idx + 1) % n
			total++
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	return writeRing(ring, idx, total, n)
}

// writeRing outputs buffered lines from the ring buffer in order.
func writeRing(ring [][]byte, idx, total, n int) error {
	count := min(n, total)
	start := idx - count
	if start < 0 {
		start += n
	}
	for i := range count {
		pos := (start + i) % n
		if _, err := os.Stdout.Write(ring[pos]); err != nil {
			return err
		}
	}
	return nil
}

// printFromLineN writes lines starting from line number n to end.
// R1.3: +N means start from line N (1-based).
func printFromLineN(r io.Reader, n int64) error {
	br := bufio.NewReader(r)
	for skip := int64(1); skip < n; skip++ {
		_, err := br.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	_, err := io.Copy(os.Stdout, br)
	return err
}

// printLastNBytes writes the last n bytes from r to stdout.
// R2.1: -c NUM prints last NUM bytes.
func printLastNBytes(r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := max(0, int64(len(data))-n)
	_, werr := os.Stdout.Write(data[start:])
	return werr
}

// printFromByteN writes bytes starting from byte number n to end.
// R2.2: +N means start from byte N (1-based).
func printFromByteN(r io.Reader, n int64) error {
	if n > 1 {
		if _, err := io.CopyN(io.Discard, r, n-1); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	_, err := io.Copy(os.Stdout, r)
	return err
}

// parseArgs extracts flags and file arguments.
// R3.4: unrecognized options print error to stderr and set err=true.
func parseArgs(args []string) config {
	cfg := config{count: defaultLines}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg
		}
		if consumed := parseHeaderFlag(&cfg, arg); consumed > 0 {
			continue
		}
		if consumed := parseCountFlag(&cfg, args, i); consumed > 0 {
			i += consumed - 1
			continue
		}
		if isUnrecognizedFlag(arg) {
			reportUnrecognized(arg)
			cfg.err = true
			return cfg
		}
		cfg.files = append(cfg.files, arg)
	}
	return cfg
}

// isUnrecognizedFlag returns true if arg looks like an unrecognized option.
// R3.4: flags start with "-" but are not "-" (stdin) or a numeric legacy arg.
func isUnrecognizedFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		return false
	}
	// Legacy GNU tail allows plain -NUM as shorthand for -n NUM.
	if len(arg) > 1 && arg[1] >= '0' && arg[1] <= '9' {
		return false
	}
	return true
}

// reportUnrecognized prints a GNU-compatible error for unrecognized options.
// R3.4: format matches GNU tail unrecognized option output.
func reportUnrecognized(arg string) {
	if strings.HasPrefix(arg, "--") {
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
	} else {
		// Short option: extract the invalid character.
		invalid := arg[1:]
		fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n", progName, invalid)
	}
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// parseHeaderFlag handles -q/--quiet/--silent and -v/--verbose flags.
// R3.3: -q suppresses headers.
// R3.4: -v forces headers.
func parseHeaderFlag(cfg *config, arg string) int {
	switch {
	case arg == "-q" || arg == "--quiet" || arg == "--silent":
		cfg.quiet = true
		cfg.verbose = false
		return 1
	case arg == "-v" || arg == "--verbose":
		cfg.verbose = true
		cfg.quiet = false
		return 1
	}
	return 0
}

// parseCountFlag handles -n/--lines and -c/--bytes flags.
func parseCountFlag(cfg *config, args []string, i int) int {
	mode, numStr, consumed := matchCountFlag(args[i], args, i)
	if consumed == 0 {
		return 0
	}
	cfg.mode = mode
	cfg.count, cfg.fromPos, cfg.err = parseCount(numStr, mode)
	return consumed
}

// matchCountFlag identifies -n/-c flags and extracts the numeric string.
func matchCountFlag(arg string, args []string, i int) (countMode, string, int) {
	switch arg {
	case "-n", "--lines":
		if i+1 < len(args) {
			return modeLines, args[i+1], 2
		}
		return modeLines, "", 1
	case "-c", "--bytes":
		if i+1 < len(args) {
			return modeBytes, args[i+1], 2
		}
		return modeBytes, "", 1
	default:
		return matchCountFlagPrefix(arg)
	}
}

// matchCountFlagPrefix handles prefixed forms like -n5, --lines=5, -c10, --bytes=10.
func matchCountFlagPrefix(arg string) (countMode, string, int) {
	switch {
	case strings.HasPrefix(arg, "--lines="):
		return modeLines, arg[len("--lines="):], 1
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
		return modeLines, arg[2:], 1
	case strings.HasPrefix(arg, "--bytes="):
		return modeBytes, arg[len("--bytes="):], 1
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'c':
		return modeBytes, arg[2:], 1
	}
	return 0, "", 0
}

// parseCount parses a count string, detecting the + prefix for from-position mode.
// R3.1: suffix multipliers supported via sizeparse.Parse for both modes.
func parseCount(s string, mode countMode) (int64, bool, bool) {
	fromPos := false
	raw := s
	if strings.HasPrefix(s, "+") {
		fromPos = true
		raw = s[1:]
	}
	n, err := sizeparse.Parse(raw)
	if err != nil {
		label := "lines"
		if mode == modeBytes {
			label = "bytes"
		}
		fmt.Fprintf(os.Stderr, "%s: invalid number of %s: '%s'\n",
			progName, label, s)
		return 0, false, true
	}
	return n, fromPos, false
}

// reportError prints a GNU-compatible diagnostic to stderr.
// R4.4: prints error and continues with remaining files.
func reportError(name string, err error) {
	dname := displayName(name)
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		fmt.Fprintf(os.Stderr, "%s: cannot open '%s' for reading: %s\n",
			progName, dname, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, dname, err)
}
