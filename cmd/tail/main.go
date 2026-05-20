// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

type options struct {
	count     int64
	fromStart bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "tail: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'tail --help' for more information.\n")
		os.Exit(1)
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, file := range files {
		r, closer, err := openInput(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail: %s\n", err)
			exitCode = 1
			continue
		}
		if err := tailLines(r, os.Stdout, opts); err != nil {
			fmt.Fprintf(os.Stderr, "tail: %s\n", err)
			exitCode = 1
		}
		if closer != nil {
			closer.Close()
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
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

func tailLines(r io.Reader, w io.Writer, opts options) error {
	if opts.fromStart {
		return tailLinesFromStart(r, w, opts.count)
	}
	return tailLinesFromEnd(r, w, opts.count)
}

func tailLinesFromEnd(r io.Reader, w io.Writer, n int64) error {
	if n <= 0 {
		return nil
	}
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
	start := max(int64(len(lines))-n, 0)
	for _, line := range lines[start:] {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

func tailLinesFromStart(r io.Reader, w io.Writer, n int64) error {
	br := bufio.NewReader(r)
	lineNum := int64(1)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 && lineNum >= n {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			break
		}
		lineNum++
	}
	return nil
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{count: 10}
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + n
			continue
		}
		files = append(files, arg)
		i++
	}

	return opts, files, nil
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--lines":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--lines' requires an argument")
		}
		return 2, applyLineCount(remaining[1], opts)
	case strings.HasPrefix(flag, "--lines="):
		return 1, applyLineCount(flag[len("--lines="):], opts)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'n':
			var val string
			if rest := flags[j+1:]; rest != "" {
				val = rest
			} else if len(remaining) > consumed {
				val = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 'n'")
			}
			return consumed, applyLineCount(val, opts)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func applyLineCount(s string, opts *options) error {
	count, fromStart, err := parseLineCount(s)
	if err != nil {
		return err
	}
	opts.count = count
	opts.fromStart = fromStart
	return nil
}

func parseLineCount(s string) (int64, bool, error) {
	if strings.HasPrefix(s, "+") {
		n, err := strconv.ParseInt(s[1:], 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("invalid number of lines: '%s'", s)
		}
		return n, true, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, false, nil
}
