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
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	args := os.Args[1:]
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "comm: missing operand\n")
		os.Exit(1)
	}
	exitCode := run(w, args[0], args[1])
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func run(w *bufio.Writer, path1, path2 string) int {
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
	return compare(w, r1, r2)
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

func compare(w *bufio.Writer, r1, r2 io.Reader) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	have1 := s1.Scan()
	have2 := s2.Scan()
	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		var err error
		if line1 < line2 {
			err = writeLine(w, 1, line1)
			have1 = s1.Scan()
		} else if line2 < line1 {
			err = writeLine(w, 2, line2)
			have2 = s2.Scan()
		} else {
			err = writeLine(w, 3, line1)
			have1 = s1.Scan()
			have2 = s2.Scan()
		}
		if err != nil {
			return epipeOr1(err)
		}
	}
	return drain(w, s1, s2, have1, have2)
}

func drain(w *bufio.Writer, s1, s2 *bufio.Scanner, have1, have2 bool) int {
	for have1 {
		if err := writeLine(w, 1, s1.Text()); err != nil {
			return epipeOr1(err)
		}
		have1 = s1.Scan()
	}
	for have2 {
		if err := writeLine(w, 2, s2.Text()); err != nil {
			return epipeOr1(err)
		}
		have2 = s2.Scan()
	}
	return 0
}

func writeLine(w *bufio.Writer, col int, line string) error {
	switch col {
	case 2:
		if _, err := w.WriteString("\t"); err != nil {
			return err
		}
	case 3:
		if _, err := w.WriteString("\t\t"); err != nil {
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
