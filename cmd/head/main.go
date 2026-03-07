// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the head utility for printing the first lines or bytes
// of files.
//
// Implements prd018-head: line-count mode (R1), byte-count mode (R2),
// multi-file headers (R3), exit codes (R4).
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

// mode selects between line-count and byte-count operation.
type mode int

const (
	modeLines mode = iota
	modeBytes
)

// config holds the parsed command-line options.
type config struct {
	mode     mode
	count    int64 // positive = first N, negative = all but last |N|
	quiet    bool  // -q: suppress headers
	verbose  bool  // -v: force headers
	files    []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg := parseArgs(os.Args[1:])
	exitCode := 0

	inputs := cfg.files
	if len(inputs) == 0 {
		inputs = []string{"-"}
	}

	showHeaders := len(inputs) > 1
	if cfg.quiet {
		showHeaders = false
	}
	if cfg.verbose {
		showHeaders = true
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	printed := 0 // count of files whose output has been written
	for _, name := range inputs {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "head: cannot open '%s' for reading: No such file or directory\n", name)
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}

		if showHeaders {
			if printed > 0 {
				fmt.Fprintf(w, "\n")
			}
			displayName := name
			if name == "-" {
				displayName = "standard input"
			}
			fmt.Fprintf(w, "==> %s <==\n", displayName)
		}
		printed++

		if err := process(w, r, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "head: error reading: %v\n", err)
			exitCode = 1
		}
	}

	w.Flush()
	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) config {
	cfg := config{
		mode:  modeLines,
		count: 10,
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			cfg.files = append(cfg.files, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--lines=") {
			cfg.mode = modeLines
			cfg.count = parseCount(arg[len("--lines="):])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--bytes=") {
			cfg.mode = modeBytes
			cfg.count = parseCount(arg[len("--bytes="):])
			i++
			continue
		}
		if arg == "--quiet" || arg == "--silent" {
			cfg.quiet = true
			i++
			continue
		}
		if arg == "--verbose" {
			cfg.verbose = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			// Check for combined short flags like -q, -v, or -n/-c with value.
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'n':
					cfg.mode = modeLines
					rest := arg[j+1:]
					if rest != "" {
						cfg.count = parseCount(rest)
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "head: option requires an argument -- 'n'\n")
							os.Exit(1)
						}
						cfg.count = parseCount(args[i])
					}
					j = len(arg) // consumed rest
				case 'c':
					cfg.mode = modeBytes
					rest := arg[j+1:]
					if rest != "" {
						cfg.count = parseCount(rest)
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "head: option requires an argument -- 'c'\n")
							os.Exit(1)
						}
						cfg.count = parseCount(args[i])
					}
					j = len(arg) // consumed rest
				case 'q':
					cfg.quiet = true
					j++
				case 'v':
					cfg.verbose = true
					j++
				default:
					// Check if it's a numeric shorthand like -5 (equivalent to -n 5).
					if ch >= '0' && ch <= '9' {
						cfg.mode = modeLines
						cfg.count = parseCount(arg[j:])
						j = len(arg)
					} else {
						fmt.Fprintf(os.Stderr, "head: invalid option -- '%c'\n", ch)
						os.Exit(1)
					}
				}
			}
			i++
			continue
		}

		// File argument.
		cfg.files = append(cfg.files, arg)
		i++
	}

	return cfg
}

// parseCount parses a count string that may have a leading '-' for negative
// counts and an optional multiplier suffix (b, K, KiB, M, MiB, G, GiB).
func parseCount(s string) int64 {
	if s == "" {
		fmt.Fprintf(os.Stderr, "head: invalid number of lines: ''\n")
		os.Exit(1)
	}

	negative := false
	num := s
	if num[0] == '-' {
		negative = true
		num = num[1:]
	}

	// Extract suffix.
	suffixStart := len(num)
	for suffixStart > 0 && !isDigit(num[suffixStart-1]) {
		suffixStart--
	}
	digits := num[:suffixStart]
	suffix := num[suffixStart:]

	if digits == "" {
		fmt.Fprintf(os.Stderr, "head: invalid number: '%s'\n", s)
		os.Exit(1)
	}

	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "head: invalid number: '%s'\n", s)
		os.Exit(1)
	}

	multiplier := int64(1)
	switch suffix {
	case "":
		// no suffix
	case "b":
		multiplier = 512
	case "K", "KiB":
		multiplier = 1024
	case "M", "MiB":
		multiplier = 1024 * 1024
	case "G", "GiB":
		multiplier = 1024 * 1024 * 1024
	default:
		fmt.Fprintf(os.Stderr, "head: invalid suffix in count: '%s'\n", s)
		os.Exit(1)
	}

	n *= multiplier
	if negative {
		n = -n
	}
	return n
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// process reads from r and writes the appropriate portion to w based on cfg.
func process(w *bufio.Writer, r io.Reader, cfg config) error {
	if cfg.count == 0 {
		return nil
	}

	if cfg.mode == modeBytes {
		if cfg.count > 0 {
			return processPositiveBytes(w, r, cfg.count)
		}
		return processNegativeBytes(w, r, -cfg.count)
	}

	if cfg.count > 0 {
		return processPositiveLines(w, r, cfg.count)
	}
	return processNegativeLines(w, r, -cfg.count)
}

// processPositiveLines outputs the first n lines from r.
func processPositiveLines(w *bufio.Writer, r io.Reader, n int64) error {
	br := bufio.NewReader(r)
	var count int64
	for count < n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			count++
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			break
		}
	}
	return nil
}

// processNegativeLines outputs all lines except the last n lines.
func processNegativeLines(w *bufio.Writer, r io.Reader, n int64) error {
	// Buffer all lines, then output all but the last n.
	br := bufio.NewReader(r)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err != nil {
			break
		}
	}

	end := max(int64(len(lines))-n, 0)
	for i := range end {
		if _, err := w.Write(lines[i]); err != nil {
			return err
		}
	}
	return nil
}

// processPositiveBytes outputs the first n bytes from r.
func processPositiveBytes(w *bufio.Writer, r io.Reader, n int64) error {
	_, err := io.CopyN(w, r, n)
	if err == io.EOF {
		return nil
	}
	return err
}

// processNegativeBytes outputs all bytes except the last n bytes.
func processNegativeBytes(w *bufio.Writer, r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	end := max(int64(len(data))-n, 0)
	_, werr := w.Write(data[:end])
	return werr
}
