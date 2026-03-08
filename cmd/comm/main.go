// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the comm utility for comparing two sorted files line by line.
//
// Implements prd029-comm: three-column comparison output (R1), column suppression (R2),
// order checking and output delimiter (R3), exit codes and SIGPIPE (R4).
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
	suppress1       bool
	suppress2       bool
	suppress3       bool
	checkOrder      bool // true = fatal on disorder
	nocheckOrder    bool // true = suppress disorder warnings
	outputDelimiter string
	file1           string
	file2           string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		os.Exit(2)
	}

	os.Exit(run(cfg))
}

func run(cfg config) int {
	r1, closer1, err := openInput(cfg.file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}
	if closer1 != nil {
		defer closer1()
	}

	r2, closer2, err := openInput(cfg.file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}
	if closer2 != nil {
		defer closer2()
	}

	w := bufio.NewWriter(os.Stdout)

	scanner1 := bufio.NewScanner(r1)
	scanner2 := bufio.NewScanner(r2)

	delim := cfg.outputDelimiter

	have1 := scanner1.Scan()
	have2 := scanner2.Scan()
	var prev1, prev2 string
	first1, first2 := true, true

	for have1 && have2 {
		line1 := scanner1.Text()
		line2 := scanner2.Text()

		// R3: Order checking for file1.
		if !first1 && !cfg.nocheckOrder && line1 < prev1 {
			fmt.Fprintf(os.Stderr, "comm: file 1 is not in sorted order\n")
			if cfg.checkOrder {
				return 1
			}
		}
		// R3: Order checking for file2.
		if !first2 && !cfg.nocheckOrder && line2 < prev2 {
			fmt.Fprintf(os.Stderr, "comm: file 2 is not in sorted order\n")
			if cfg.checkOrder {
				return 1
			}
		}
		prev1 = line1
		prev2 = line2
		first1 = false
		first2 = false

		if line1 < line2 {
			if !cfg.suppress1 {
				if writeLine(w, line1, 0, delim) != nil {
					return 1
				}
			}
			have1 = scanner1.Scan()
		} else if line2 < line1 {
			if !cfg.suppress2 {
				col := 1
				if cfg.suppress1 {
					col--
				}
				if writeLine(w, line2, col, delim) != nil {
					return 1
				}
			}
			have2 = scanner2.Scan()
		} else {
			if !cfg.suppress3 {
				col := 2
				if cfg.suppress1 {
					col--
				}
				if cfg.suppress2 {
					col--
				}
				if writeLine(w, line1, col, delim) != nil {
					return 1
				}
			}
			have1 = scanner1.Scan()
			have2 = scanner2.Scan()
		}
	}

	// Drain remaining lines from file1.
	for have1 {
		line1 := scanner1.Text()
		if !first1 && !cfg.nocheckOrder && line1 < prev1 {
			fmt.Fprintf(os.Stderr, "comm: file 1 is not in sorted order\n")
			if cfg.checkOrder {
				return 1
			}
		}
		prev1 = line1
		first1 = false
		if !cfg.suppress1 {
			if writeLine(w, line1, 0, delim) != nil {
				return 1
			}
		}
		have1 = scanner1.Scan()
	}

	// Drain remaining lines from file2.
	for have2 {
		line2 := scanner2.Text()
		if !first2 && !cfg.nocheckOrder && line2 < prev2 {
			fmt.Fprintf(os.Stderr, "comm: file 2 is not in sorted order\n")
			if cfg.checkOrder {
				return 1
			}
		}
		prev2 = line2
		first2 = false
		if !cfg.suppress2 {
			col := 1
			if cfg.suppress1 {
				col--
			}
			if writeLine(w, line2, col, delim) != nil {
				return 1
			}
		}
		have2 = scanner2.Scan()
	}

	if err := scanner1.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}
	if err := scanner2.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "comm: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		return 1
	}

	return 0
}

// writeLine writes a line at the given column indentation level.
func writeLine(w *bufio.Writer, line string, col int, delim string) error {
	for range col {
		if _, err := w.WriteString(delim); err != nil {
			return err
		}
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}

// openInput opens a file for reading. "-" means stdin.
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// parseArgs parses command-line arguments.
func parseArgs(args []string) (config, error) {
	var cfg config
	cfg.outputDelimiter = "\t"
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			positional = append(positional, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--check-order":
				cfg.checkOrder = true
			case arg == "--nocheck-order":
				cfg.nocheckOrder = true
			case strings.HasPrefix(arg, "--output-delimiter="):
				cfg.outputDelimiter = arg[len("--output-delimiter="):]
			default:
				return cfg, fmt.Errorf("unrecognized option '%s'", arg)
			}
			i++
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			// Short options: -1, -2, -3 can be combined (e.g., -12).
			for _, ch := range arg[1:] {
				switch ch {
				case '1':
					cfg.suppress1 = true
				case '2':
					cfg.suppress2 = true
				case '3':
					cfg.suppress3 = true
				default:
					return cfg, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			i++
			continue
		}

		positional = append(positional, arg)
		i++
	}

	if len(positional) != 2 {
		if len(positional) < 2 {
			return cfg, fmt.Errorf("missing operand")
		}
		return cfg, fmt.Errorf("extra operand '%s'", positional[2])
	}

	cfg.file1 = positional[0]
	cfg.file2 = positional[1]

	return cfg, nil
}
