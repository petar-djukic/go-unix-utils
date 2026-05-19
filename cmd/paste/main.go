// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd027-paste.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	delimiters string
	serial     bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(w, opts, files)
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func run(w *bufio.Writer, opts options, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	if opts.serial {
		return pasteSerial(w, opts, files)
	}
	return pasteParallel(w, opts, files)
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{delimiters: "\t"}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		switch {
		case a == "-s":
			opts.serial = true
		case a == "-d" || strings.HasPrefix(a, "-d"):
			val, idx, err := flagValue(a, "-d", args, i)
			if err != nil {
				return options{}, nil, err
			}
			i = idx
			opts.delimiters = parseDelimiters(val)
		case strings.HasPrefix(a, "-") && a != "-":
			return options{}, nil, fmt.Errorf("invalid option -- '%s'", a[1:])
		default:
			files = append(files, a)
		}
	}
	return opts, files, nil
}

func flagValue(a, flag string, args []string, i int) (string, int, error) {
	if a == flag {
		if i+1 >= len(args) {
			return "", i, fmt.Errorf("option requires an argument -- '%s'", flag[1:])
		}
		return args[i+1], i + 1, nil
	}
	return a[len(flag):], i, nil
}

func parseDelimiters(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '0':
				b.WriteByte(0)
			default:
				b.WriteByte(s[i+1])
			}
			i++
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func pasteParallel(w *bufio.Writer, opts options, files []string) int {
	scanners, closers, err := openInputs(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %s\n", err)
		return 1
	}
	defer closeAll(closers)
	done := make([]bool, len(scanners))
	for {
		fields := make([]string, len(scanners))
		anyActive := false
		for i, s := range scanners {
			if !done[i] && s.Scan() {
				fields[i] = s.Text()
				anyActive = true
			} else {
				done[i] = true
			}
		}
		if !anyActive {
			break
		}
		if err := writeFields(w, opts.delimiters, fields); err != nil {
			return 1
		}
	}
	return 0
}

func writeFields(w *bufio.Writer, delims string, fields []string) error {
	var err error
	for i, f := range fields {
		if i > 0 && len(delims) > 0 {
			d := delims[(i-1)%len(delims)]
			if d != 0 {
				err = w.WriteByte(d)
			}
		}
		if err == nil {
			_, err = w.WriteString(f)
		}
	}
	if err == nil {
		err = w.WriteByte('\n')
	}
	return err
}

func openInputs(files []string) ([]*bufio.Scanner, []io.Closer, error) {
	scanners := make([]*bufio.Scanner, 0, len(files))
	var closers []io.Closer
	var stdinScanner *bufio.Scanner
	for _, name := range files {
		if name == "-" {
			if stdinScanner == nil {
				stdinScanner = bufio.NewScanner(os.Stdin)
			}
			scanners = append(scanners, stdinScanner)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			closeAll(closers)
			return nil, nil, err
		}
		closers = append(closers, f)
		scanners = append(scanners, bufio.NewScanner(f))
	}
	return scanners, closers, nil
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close()
	}
}

func pasteSerial(w *bufio.Writer, opts options, files []string) int {
	for _, name := range files {
		var s *bufio.Scanner
		if name == "-" {
			s = bufio.NewScanner(os.Stdin)
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "paste: %s\n", err)
				return 1
			}
			defer f.Close()
			s = bufio.NewScanner(f)
		}
		var lines []string
		for s.Scan() {
			lines = append(lines, s.Text())
		}
		if err := writeFields(w, opts.delimiters, lines); err != nil {
			return 1
		}
	}
	return 0
}
