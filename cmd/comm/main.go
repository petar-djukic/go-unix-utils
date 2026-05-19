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
	suppress [3]bool
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
	var opts options
	var files []string
	for _, arg := range args {
		if arg == "--" {
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
		fmt.Fprintf(os.Stderr, "comm: %s\n", err)
		return 1
	}
	if c1 != nil {
		defer c1.Close()
	}
	r2, c2, err := openInput(path2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %s\n", err)
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
	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		var err error
		if line1 < line2 {
			err = writeLine(w, opts, 1, line1)
			have1 = s1.Scan()
		} else if line2 < line1 {
			err = writeLine(w, opts, 2, line2)
			have2 = s2.Scan()
		} else {
			err = writeLine(w, opts, 3, line1)
			have1 = s1.Scan()
			have2 = s2.Scan()
		}
		if err != nil {
			return epipeOr1(err)
		}
	}
	return drain(w, opts, s1, s2, have1, have2)
}

func drain(w *bufio.Writer, opts *options, s1, s2 *bufio.Scanner, have1, have2 bool) int {
	for have1 {
		if err := writeLine(w, opts, 1, s1.Text()); err != nil {
			return epipeOr1(err)
		}
		have1 = s1.Scan()
	}
	for have2 {
		if err := writeLine(w, opts, 2, s2.Text()); err != nil {
			return epipeOr1(err)
		}
		have2 = s2.Scan()
	}
	return 0
}

func writeLine(w *bufio.Writer, opts *options, col int, line string) error {
	if opts.suppress[col-1] {
		return nil
	}
	tabs := 0
	for i := 0; i < col-1; i++ {
		if !opts.suppress[i] {
			tabs++
		}
	}
	for i := 0; i < tabs; i++ {
		if _, err := w.WriteString("\t"); err != nil {
			return err
		}
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func epipeOr1(err error) int {
	if errors.Is(err, syscall.EPIPE) {
		return 0
	}
	return 1
}
