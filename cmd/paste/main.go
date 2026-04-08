// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/paste: merge lines of files side by side.
// Implements srd027-paste R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set via -ldflags; defaults to "dev".
var version = "dev"

// config holds the parsed command-line options.
// R1.3: types for parallel merge, serial mode, delimiter cycling, and file/stdin management.
type config struct {
	delimiters []rune
	serial     bool
	files      []string
}

// fileReader wraps a file and its buffered reader for parallel merge.
// R1.3: manages file/stdin readers for parallel and serial modes.
type fileReader struct {
	name   string
	file   *os.File
	reader *bufio.Reader
	done   bool
}

// parseDelimiters parses the delimiter string, handling escape sequences.
// R2.2: recognizes \n, \t, \\, and \0.
func parseDelimiters(s string) []rune {
	var delims []rune
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			switch runes[i+1] {
			case 'n':
				delims = append(delims, '\n')
				i++
			case 't':
				delims = append(delims, '\t')
				i++
			case '\\':
				delims = append(delims, '\\')
				i++
			case '0':
				delims = append(delims, 0)
				i++
			default:
				delims = append(delims, runes[i])
			}
		} else {
			delims = append(delims, runes[i])
		}
	}
	if len(delims) == 0 {
		delims = []rune{'\t'}
	}
	return delims
}

// parseArgs parses command-line arguments into a config.
// R1.1: flag parsing for -d and -s.
// R1.4: --version and --help handling.
func parseArgs(args []string) (config, bool, error) {
	cfg := config{
		delimiters: []rune{'\t'},
	}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if flagsDone || (arg != "-" && !strings.HasPrefix(arg, "-")) {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}

		skip, done, err := parseFlag(&cfg, arg, args, i)
		if err != nil {
			return config{}, false, err
		}
		if done {
			return cfg, true, nil
		}
		i += skip
	}

	return cfg, false, nil
}

// parseFlag handles a single flag argument, returning extra args consumed
// and whether the program should exit after printing version/help.
func parseFlag(cfg *config, arg string, args []string, i int) (int, bool, error) {
	if arg == "--version" {
		fmt.Fprintf(os.Stdout, "paste (%s)\n", version)
		return 0, true, nil
	}
	if arg == "--help" {
		printUsage()
		return 0, true, nil
	}
	if arg == "-s" || arg == "--serial" {
		cfg.serial = true
		return 0, false, nil
	}
	return parseDelimFlag(cfg, arg, args, i)
}

// parseDelimFlag handles -d DELIM and --delimiters=DELIM.
// R2.1: DELIM may be a single character or a list for cycling.
func parseDelimFlag(cfg *config, arg string, args []string, i int) (int, bool, error) {
	if strings.HasPrefix(arg, "--delimiters=") {
		cfg.delimiters = parseDelimiters(arg[len("--delimiters="):])
		return 0, false, nil
	}
	if arg == "--delimiters" {
		if i+1 >= len(args) {
			return 0, false, fmt.Errorf("option '--delimiters' requires an argument")
		}
		cfg.delimiters = parseDelimiters(args[i+1])
		return 1, false, nil
	}
	if strings.HasPrefix(arg, "-d") {
		rest := arg[2:]
		if rest != "" {
			cfg.delimiters = parseDelimiters(rest)
			return 0, false, nil
		}
		if i+1 >= len(args) {
			return 0, false, fmt.Errorf("option requires an argument -- 'd'")
		}
		cfg.delimiters = parseDelimiters(args[i+1])
		return 1, false, nil
	}
	return 0, false, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
}

// printUsage prints usage information to stdout.
// R1.4: --help prints usage and exits 0.
func printUsage() {
	fmt.Fprint(os.Stdout, `Usage: paste [OPTION]... [FILE]...
Write lines consisting of the sequentially corresponding lines from
each FILE, separated by TABs, to standard output.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
  -d, --delimiters=LIST   reuse characters from LIST instead of TABs
  -s, --serial            paste one file at a time instead of in parallel
      --help        display this help and exit
      --version     output version information and exit
`)
}

// openFileReader opens a file or stdin for reading.
// R1.3: stdin is used when "-" is given as a filename.
func openFileReader(name string) (*fileReader, error) {
	fr := &fileReader{name: name}
	if name == "-" {
		fr.file = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", name, err)
		}
		fr.file = f
	}
	fr.reader = bufio.NewReader(fr.file)
	return fr, nil
}

// close closes the underlying file if it is not stdin.
func (fr *fileReader) close() {
	if fr.file != os.Stdin {
		fr.file.Close() // best-effort close
	}
}

// readLine reads one line from the file reader, stripping the trailing newline.
// Returns the line content and whether a line was read.
func (fr *fileReader) readLine() (string, bool) {
	if fr.done {
		return "", false
	}
	line, err := fr.reader.ReadString('\n')
	if len(line) > 0 {
		line = strings.TrimRight(line, "\n")
		return line, true
	}
	if err != nil {
		fr.done = true
		return "", false
	}
	return line, true
}

// pasteParallel merges lines from all files in parallel mode.
// R1.1: read one line from each file per output line, joined by delimiter.
// R1.2: shorter files contribute empty fields until all are exhausted.
// TODO: full implementation in a subsequent task.
func pasteParallel(cfg config, w *bufio.Writer) int {
	readers := make([]*fileReader, 0, len(cfg.files))
	for _, name := range cfg.files {
		fr, err := openFileReader(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "paste: %s\n", err)
			return 1
		}
		readers = append(readers, fr)
	}
	defer func() {
		for _, fr := range readers {
			fr.close()
		}
	}()

	for {
		allDone := true
		for idx, fr := range readers {
			if idx > 0 {
				delimIdx := (idx - 1) % len(cfg.delimiters)
				d := cfg.delimiters[delimIdx]
				if d != 0 {
					if _, err := w.WriteRune(d); err != nil {
						fmt.Fprintf(os.Stderr, "paste: write error\n")
						return 1
					}
				}
			}
			line, ok := fr.readLine()
			if ok {
				allDone = false
				if _, err := w.WriteString(line); err != nil {
					fmt.Fprintf(os.Stderr, "paste: write error\n")
					return 1
				}
			} else if !fr.done {
				allDone = false
			}
		}
		if allDone {
			break
		}
		if err := w.WriteByte('\n'); err != nil {
			fmt.Fprintf(os.Stderr, "paste: write error\n")
			return 1
		}
	}

	return 0
}

// pasteSerial processes files one at a time in serial mode.
// R3.1: all lines of one file joined with delimiter on a single output line.
// R3.2: delimiter list cycles across fields within the output line.
// TODO: full implementation in a subsequent task.
func pasteSerial(cfg config, w *bufio.Writer) int {
	for _, name := range cfg.files {
		fr, err := openFileReader(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "paste: %s\n", err)
			return 1
		}
		delimIdx := 0
		first := true
		for {
			line, ok := fr.readLine()
			if !ok {
				break
			}
			if !first {
				d := cfg.delimiters[delimIdx%len(cfg.delimiters)]
				if d != 0 {
					if _, err := w.WriteRune(d); err != nil {
						fmt.Fprintf(os.Stderr, "paste: write error\n")
						fr.close()
						return 1
					}
				}
				delimIdx++
			}
			if _, err := w.WriteString(line); err != nil {
				fmt.Fprintf(os.Stderr, "paste: write error\n")
				fr.close()
				return 1
			}
			first = false
		}
		if err := w.WriteByte('\n'); err != nil {
			fmt.Fprintf(os.Stderr, "paste: write error\n")
			fr.close()
			return 1
		}
		fr.close()
	}
	return 0
}

// run executes the paste logic, returning the exit code.
// R1.1: main function flow dispatches to parallel or serial mode.
// R4.1: exit 0 on success.
// R4.2: exit 1 on file open failure.
// R4.3: exit 1 on write error.
func run(cfg config) int {
	w := bufio.NewWriter(os.Stdout)
	var exitCode int
	if cfg.serial {
		exitCode = pasteSerial(cfg, w)
	} else {
		exitCode = pasteParallel(cfg, w)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "paste: write error\n")
		return 1
	}
	return exitCode
}

// R4.4: SIGPIPE handler installed at start.
// R1.1: main skeleton with flag parsing and dispatch.
// R1.2: argument validation requires at least one operand or defaults to stdin.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, done, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'paste --help' for more information.\n")
		os.Exit(1)
	}
	if done {
		os.Exit(0)
	}

	// R1.4: when no files are given, default to stdin passthrough.
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}

	os.Exit(run(cfg))
}

