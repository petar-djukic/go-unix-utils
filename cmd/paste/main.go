// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the paste utility for merging lines of files.
//
// Implements prd027-paste: parallel merge (R1), delimiter configuration (R2),
// serial mode (R3), exit codes and SIGPIPE (R4).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type config struct {
	delimiters []byte
	serial     bool
	files      []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

func run(cfg config) int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	inputs := cfg.files
	if len(inputs) == 0 {
		inputs = []string{"-"}
	}

	var exitCode int
	if cfg.serial {
		exitCode = runSerial(w, inputs, cfg.delimiters)
	} else {
		exitCode = runParallel(w, inputs, cfg.delimiters)
	}

	if err := w.Flush(); err != nil {
		return 1
	}
	return exitCode
}

// runParallel merges corresponding lines from all files. R1.1, R1.2.
func runParallel(w *bufio.Writer, files []string, delimiters []byte) int {
	readers := make([]*bufio.Scanner, len(files))
	closers := make([]io.Closer, len(files))

	for i, name := range files {
		if name == "-" {
			readers[i] = bufio.NewScanner(os.Stdin)
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "paste: %s: No such file or directory\n", name)
				return 1
			}
			closers[i] = f
			readers[i] = bufio.NewScanner(f)
		}
	}
	defer func() {
		for _, c := range closers {
			if c != nil {
				c.Close()
			}
		}
	}()

	nDelims := len(delimiters)

	for {
		anyData := false
		var line strings.Builder

		for i, r := range readers {
			if i > 0 {
				d := delimiters[(i-1)%nDelims]
				if d != 0 {
					line.WriteByte(d)
				}
			}
			if r != nil && r.Scan() {
				anyData = true
				line.WriteString(r.Text())
			} else {
				// Check if this scanner just finished (still had data last time we need to track).
				// We mark exhausted scanners as nil after they stop producing data.
				if r != nil {
					readers[i] = nil
				}
			}
		}

		if !anyData {
			break
		}

		line.WriteByte('\n')
		if _, err := w.WriteString(line.String()); err != nil {
			return 1
		}
	}

	return 0
}

// runSerial processes one file at a time, joining all its lines. R3.1, R3.2.
func runSerial(w *bufio.Writer, files []string, delimiters []byte) int {
	nDelims := len(delimiters)

	for _, name := range files {
		var scanner *bufio.Scanner
		if name == "-" {
			scanner = bufio.NewScanner(os.Stdin)
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "paste: %s: No such file or directory\n", name)
				return 1
			}
			defer f.Close()
			scanner = bufio.NewScanner(f)
		}

		fieldIdx := 0
		for scanner.Scan() {
			if fieldIdx > 0 {
				d := delimiters[(fieldIdx-1)%nDelims]
				if d != 0 {
					if _, err := w.Write([]byte{d}); err != nil {
						return 1
					}
				}
			}
			if _, err := w.WriteString(scanner.Text()); err != nil {
				return 1
			}
			fieldIdx++
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return 1
		}
	}

	return 0
}

// parseDelimiters expands escape sequences in a delimiter string. R2.2.
func parseDelimiters(s string) []byte {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result = append(result, '\n')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case '\\':
				result = append(result, '\\')
				i += 2
			case '0':
				// \0 means empty string delimiter (no separator).
				// We use 0 byte as a sentinel; in output we skip writing it.
				result = append(result, 0)
				i += 2
			default:
				result = append(result, s[i])
				i++
			}
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return result
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	cfg := config{
		delimiters: []byte{'\t'},
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			cfg.files = append(cfg.files, args[i:]...)
			break
		}

		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 's':
					cfg.serial = true
					j++
				case 'd':
					rest := arg[j+1:]
					if rest != "" {
						cfg.delimiters = parseDelimiters(rest)
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option requires an argument -- 'd'")
						}
						cfg.delimiters = parseDelimiters(args[i])
					}
					j = len(arg)
				default:
					return cfg, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			i++
			continue
		}

		cfg.files = append(cfg.files, arg)
		i++
	}

	if len(cfg.delimiters) == 0 {
		cfg.delimiters = []byte{'\t'}
	}

	return cfg, nil
}
