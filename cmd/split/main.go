// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd067-split R1.1–R1.4: basic file splitting by line count
// with configurable prefix and stdin support.
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

const (
	progName         = "split"
	defaultLines     = 1000
	defaultPrefix    = "x"
	defaultSuffixLen = 2
)

// config holds parsed command-line options for split.
type config struct {
	lines     int
	prefix    string
	suffixLen int
	inputFile string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes the split operation.
// R1.1: default 1000-line split. R1.2: custom prefix.
// R1.3: -l/--lines option. R1.4: stdin or file input.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			return printHelp(stdout)
		case "--version":
			return printVersion(stdout)
		}
	}
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := executeSplit(cfg, stdin); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// printHelp writes usage information to stdout.
func printHelp(w io.Writer) int {
	fmt.Fprintln(w, "Usage: split [OPTION]... [FILE [PREFIX]]")
	fmt.Fprintln(w, "Output pieces of FILE to PREFIXaa, PREFIXab, ...;")
	fmt.Fprintln(w, "default size is 1000 lines, and default PREFIX is 'x'.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -l, --lines=NUMBER   put NUMBER lines/records per output file")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
	return 0
}

// printVersion writes version information to stdout.
func printVersion(w io.Writer) int {
	fmt.Fprintln(w, "split (go-unix-utils)")
	return 0
}

// parseArgs extracts configuration from command-line arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{
		lines:     defaultLines,
		prefix:    defaultPrefix,
		suffixLen: defaultSuffixLen,
		inputFile: "-",
	}
	positional, err := parseOptions(args, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, applyPositional(cfg, positional)
}

// parseOptions processes flag arguments and returns remaining positional args.
func parseOptions(args []string, cfg *config) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return positional, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		extra, err := parseSingleOption(args, i, cfg)
		if err != nil {
			return nil, err
		}
		i += extra
	}
	return positional, nil
}

// parseSingleOption handles one flag and returns extra args consumed.
func parseSingleOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--lines=") {
		return 0, parseLineCount(arg[len("--lines="):], cfg)
	}
	if arg == "--lines" {
		if i+1 >= len(args) {
			return 0, fmt.Errorf("option '--lines' requires an argument")
		}
		return 1, parseLineCount(args[i+1], cfg)
	}
	if strings.HasPrefix(arg, "-l") {
		return parseDashL(args, i, cfg)
	}
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseDashL handles the -l flag with attached or separate value.
func parseDashL(args []string, i int, cfg *config) (int, error) {
	val := args[i][2:]
	if val == "" {
		if i+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- 'l'")
		}
		return 1, parseLineCount(args[i+1], cfg)
	}
	return 0, parseLineCount(val, cfg)
}

// parseLineCount parses and validates a line count string.
func parseLineCount(s string, cfg *config) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid number of lines: '%s'", s)
	}
	cfg.lines = n
	return nil
}

// applyPositional sets inputFile and prefix from positional arguments.
func applyPositional(cfg *config, positional []string) error {
	if len(positional) > 2 {
		return fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) > 0 {
		cfg.inputFile = positional[0]
	}
	if len(positional) > 1 {
		cfg.prefix = positional[1]
	}
	return nil
}

// executeSplit opens the input and splits it by lines.
func executeSplit(cfg *config, stdin io.Reader) error {
	reader, closer, err := openInput(cfg.inputFile, stdin)
	if err != nil {
		return err
	}
	defer closer()
	return splitByLines(reader, cfg)
}

// openInput returns a reader for the specified file or stdin.
// R1.4: when path is "-", reads from stdin.
func openInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// splitByLines reads from r and writes chunks of cfg.lines lines each.
// R1.1: generates output files with alphabetic suffixes (xaa, xab, ...).
func splitByLines(r io.Reader, cfg *config) error {
	br := bufio.NewReader(r)
	for fileIdx := 0; ; fileIdx++ {
		suffix, err := generateSuffix(fileIdx, cfg.suffixLen)
		if err != nil {
			return err
		}
		wrote, err := writeChunk(br, cfg.prefix+suffix, cfg.lines)
		if err != nil {
			return err
		}
		if !wrote {
			break
		}
	}
	return nil
}

// writeChunk checks for remaining data and delegates to writeChunkData.
// Returns false when input is exhausted.
func writeChunk(br *bufio.Reader, filename string, maxLines int) (bool, error) {
	if _, err := br.Peek(1); err == io.EOF {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return writeChunkData(br, filename, maxLines)
}

// writeChunkData creates a file and writes up to maxLines lines to it.
func writeChunkData(br *bufio.Reader, filename string, maxLines int) (bool, error) {
	f, err := os.Create(filename)
	if err != nil {
		return false, err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for lineCount := 0; lineCount < maxLines; {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := bw.Write(line); werr != nil {
				return true, werr
			}
			if line[len(line)-1] == '\n' {
				lineCount++
			}
		}
		if readErr == io.EOF {
			return true, bw.Flush()
		}
		if readErr != nil {
			return true, readErr
		}
	}
	return true, bw.Flush()
}

// generateSuffix returns an alphabetic suffix for the given index.
// R1.1: suffixes follow the pattern aa, ab, ..., az, ba, ..., zz.
func generateSuffix(index, length int) (string, error) {
	suffix := make([]byte, length)
	n := index
	for i := length - 1; i >= 0; i-- {
		suffix[i] = 'a' + byte(n%26)
		n /= 26
	}
	if n > 0 {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return string(suffix), nil
}
