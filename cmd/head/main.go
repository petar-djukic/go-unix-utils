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

const helpText = `Usage: head [OPTION]... [FILE]...
Print the first 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
  -c, --bytes=[-]NUM       print the first NUM bytes of each file;
                             with the leading '-', print all but the last
                             NUM bytes of each file
  -n, --lines=[-]NUM       output the first NUM lines instead of the first 10;
                             with the leading '-', output all but the last
                             NUM lines of each file
  -q, --quiet, --silent    never print headers giving file names
  -v, --verbose            always print headers giving file names
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `head (go-unix-utils) dev
`

type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

type options struct {
	mode  countMode
	count int64
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "head: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'head --help' for more information.\n")
		os.Exit(1)
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, file := range files {
		if err := processFile(opts, file); err != nil {
			fmt.Fprintf(os.Stderr, "head: %s\n", err)
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
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
	case flag == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case flag == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case flag == "--lines":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--lines' requires an argument")
		}
		count, err := parseLineCount(remaining[1])
		if err != nil {
			return 0, err
		}
		opts.count = count
		opts.mode = modeLines
		return 2, nil
	case strings.HasPrefix(flag, "--lines="):
		count, err := parseLineCount(flag[len("--lines="):])
		if err != nil {
			return 0, err
		}
		opts.count = count
		opts.mode = modeLines
		return 1, nil
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
			count, err := parseLineCount(val)
			if err != nil {
				return 0, err
			}
			opts.count = count
			opts.mode = modeLines
			return consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func parseLineCount(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, nil
}

func processFile(opts options, name string) error {
	r, closer, err := openInput(name)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}
	return headLines(r, os.Stdout, opts.count)
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

func headLines(r io.Reader, w io.Writer, count int64) error {
	if count == 0 {
		return nil
	}
	if count < 0 {
		return headLinesFromEnd(r, w, -count)
	}
	return headLinesFromStart(r, w, count)
}

func headLinesFromStart(r io.Reader, w io.Writer, n int64) error {
	br := bufio.NewReader(r)
	for range n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			return nil
		}
	}
	return nil
}

func headLinesFromEnd(r io.Reader, w io.Writer, n int64) error {
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
