// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd029-comm.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	suppress        [3]bool
	checkOrder      bool
	noCheckOrder    bool
	outputDelimiter string
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, file1, file2, ok := parseArgs(os.Args[1:])
	if !ok {
		fmt.Fprintf(os.Stderr, "comm: missing operand\n")
		os.Exit(1)
	}
	exitCode := run(w, &opts, file1, file2)
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (options, string, string, bool) {
	opts := options{outputDelimiter: "\t"}
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			continue
		}
		if arg == "--check-order" {
			opts.checkOrder = true
			continue
		}
		if arg == "--nocheck-order" {
			opts.noCheckOrder = true
			continue
		}
		if arg == "--output-delimiter" && i+1 < len(args) {
			i++
			opts.outputDelimiter = args[i]
			continue
		}
		if strings.HasPrefix(arg, "--output-delimiter=") {
			opts.outputDelimiter = arg[len("--output-delimiter="):]
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			for _, ch := range arg[1:] {
				switch ch {
				case '1':
					opts.suppress[0] = true
				case '2':
					opts.suppress[1] = true
				case '3':
					opts.suppress[2] = true
				}
			}
			continue
		}
		files = append(files, arg)
	}
	if len(files) != 2 {
		return opts, "", "", false
	}
	return opts, files[0], files[1], true
}

func run(w *bufio.Writer, opts *options, path1, path2 string) int {
	r1, c1, err := openInput(path1)
	if err != nil {
		printFileError(path1, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close()
	}
	r2, c2, err := openInput(path2)
	if err != nil {
		printFileError(path2, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close()
	}
	return compare(w, opts, r1, r2)
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func compare(w *bufio.Writer, opts *options, r1, r2 io.Reader) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	have1 := s1.Scan()
	have2 := s2.Scan()
	var prev1, prev2 string
	var hasPrev1, hasPrev2 bool
	var warned1, warned2, violated bool
	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		var err error
		if line1 < line2 {
			err = writeLine(w, opts, 1, line1)
			prev1 = line1
			hasPrev1 = true
			have1 = s1.Scan()
			if have1 {
				if code := checkOrder(opts, s1.Text(), prev1, 1, &warned1, &violated); code >= 0 {
					return code
				}
			}
		} else if line2 < line1 {
			err = writeLine(w, opts, 2, line2)
			prev2 = line2
			hasPrev2 = true
			have2 = s2.Scan()
			if have2 {
				if code := checkOrder(opts, s2.Text(), prev2, 2, &warned2, &violated); code >= 0 {
					return code
				}
			}
		} else {
			err = writeLine(w, opts, 3, line1)
			prev1 = line1
			prev2 = line2
			hasPrev1 = true
			hasPrev2 = true
			have1 = s1.Scan()
			have2 = s2.Scan()
			if opts.checkOrder {
				if have1 {
					if code := checkOrder(opts, s1.Text(), prev1, 1, &warned1, &violated); code >= 0 {
						return code
					}
				}
				if have2 {
					if code := checkOrder(opts, s2.Text(), prev2, 2, &warned2, &violated); code >= 0 {
						return code
					}
				}
			}
		}
		if err != nil {
			return epipeOr1(err)
		}
	}
	code := drainWithOrder(w, opts, s1, s2, have1, have2, prev1, prev2, hasPrev1, hasPrev2, &warned1, &warned2, &violated)
	if code != 0 {
		return code
	}
	if violated {
		fmt.Fprintf(os.Stderr, "comm: input is not in sorted order\n")
		return 1
	}
	return 0
}

func checkOrder(opts *options, cur, prev string, fileNum int, warned, violated *bool) int {
	if opts.noCheckOrder || cur >= prev {
		return -1
	}
	*violated = true
	if !*warned {
		*warned = true
		fmt.Fprintf(os.Stderr, "comm: file %d is not in sorted order\n", fileNum)
	}
	if opts.checkOrder {
		return 1
	}
	return -1
}

func drainWithOrder(w *bufio.Writer, opts *options, s1, s2 *bufio.Scanner, have1, have2 bool, prev1, prev2 string, hasPrev1, hasPrev2 bool, warned1, warned2 *bool, violated *bool) int {
	for have1 {
		line := s1.Text()
		if hasPrev1 {
			if code := checkOrder(opts, line, prev1, 1, warned1, violated); code >= 0 {
				return code
			}
		}
		if err := writeLine(w, opts, 1, line); err != nil {
			return epipeOr1(err)
		}
		prev1 = line
		hasPrev1 = true
		have1 = s1.Scan()
	}
	for have2 {
		line := s2.Text()
		if hasPrev2 {
			if code := checkOrder(opts, line, prev2, 2, warned2, violated); code >= 0 {
				return code
			}
		}
		if err := writeLine(w, opts, 2, line); err != nil {
			return epipeOr1(err)
		}
		prev2 = line
		hasPrev2 = true
		have2 = s2.Scan()
	}
	return 0
}

func writeLine(w *bufio.Writer, opts *options, col int, line string) error {
	if opts.suppress[col-1] {
		return nil
	}
	delims := 0
	for i := 0; i < col-1; i++ {
		if !opts.suppress[i] {
			delims++
		}
	}
	for i := 0; i < delims; i++ {
		if _, err := w.WriteString(opts.outputDelimiter); err != nil {
			return err
		}
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func printFileError(name string, err error) {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		fmt.Fprintf(os.Stderr, "comm: %s: %s\n", pe.Path, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "comm: %s: %s\n", name, err)
}

func epipeOr1(err error) int {
	if errors.Is(err, syscall.EPIPE) {
		return 0
	}
	return 1
}
