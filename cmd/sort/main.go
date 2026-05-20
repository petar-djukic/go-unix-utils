// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: sort [OPTION]... [FILE]...
Write sorted concatenation of all FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
Ordering options:

  -r, --reverse               reverse the result of comparisons
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `sort (go-unix-utils) dev
`

type options struct {
	reverse bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'sort --help' for more information.\n")
		os.Exit(2)
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	lines, err := readAllLines(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %s\n", err)
		os.Exit(2)
	}

	sortLines(lines, opts)
	writeLines(lines, os.Stdout)
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			if err := parseShortFlags(arg[1:], &opts); err != nil {
				return opts, nil, err
			}
			i++
			continue
		}
		files = append(files, arg)
		i++
	}

	return opts, files, nil
}

func parseLongFlag(flag string, opts *options) (int, error) {
	switch flag {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case "--reverse":
		opts.reverse = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, opts *options) error {
	for _, ch := range flags {
		switch ch {
		case 'r':
			opts.reverse = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

func readAllLines(files []string) ([]string, error) {
	var lines []string
	for _, file := range files {
		r, closer, err := openInput(file)
		if err != nil {
			return nil, err
		}
		fileLines, err := readLines(r)
		if closer != nil {
			closer.Close()
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
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

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func sortLines(lines []string, opts options) {
	slices.Sort(lines)
	if opts.reverse {
		slices.Reverse(lines)
	}
}

func writeLines(lines []string, w io.Writer) {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		fmt.Fprintln(bw, line)
	}
	bw.Flush()
}
