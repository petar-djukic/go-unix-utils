// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: sort [OPTION]... [FILE]...
Write sorted concatenation of all FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
Ordering options:

  -n, --numeric-sort          compare according to string numerical value
  -r, --reverse               reverse the result of comparisons

Other options:

  -o, --output=FILE           write result to FILE instead of standard output
  -s, --stable                stabilize sort by disabling last-resort comparison
  -u, --unique                with -c, check for strict ordering;
                                without -c, output only the first of an equal run
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `sort (go-unix-utils) dev
`

type options struct {
	reverse     bool
	unique      bool
	stable      bool
	numericSort bool
	outputFile  string
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

	lines = sortAndFilter(lines, opts)

	var w io.Writer = os.Stdout
	if opts.outputFile != "" {
		f, err := os.Create(opts.outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sort: %s\n", err)
			os.Exit(2)
		}
		defer f.Close()
		w = f
	}
	writeLines(lines, w)
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
			n, err := parseLongFlag(arg, &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			extra, err := parseShortFlags(arg[1:], &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += 1 + extra
			continue
		}
		files = append(files, arg)
		i++
	}

	return opts, files, nil
}

func parseLongFlag(flag string, opts *options, remaining []string) (int, error) {
	if strings.HasPrefix(flag, "--output=") {
		opts.outputFile = flag[len("--output="):]
		return 1, nil
	}

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
	case "--unique":
		opts.unique = true
		return 1, nil
	case "--stable":
		opts.stable = true
		return 1, nil
	case "--numeric-sort":
		opts.numericSort = true
		return 1, nil
	case "--output":
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option '--output' requires an argument")
		}
		opts.outputFile = remaining[0]
		return 2, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, opts *options, remaining []string) (int, error) {
	for idx := 0; idx < len(flags); idx++ {
		switch flags[idx] {
		case 'r':
			opts.reverse = true
		case 'u':
			opts.unique = true
		case 's':
			opts.stable = true
		case 'n':
			opts.numericSort = true
		case 'o':
			rest := flags[idx+1:]
			if rest != "" {
				opts.outputFile = rest
				return 0, nil
			}
			if len(remaining) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 'o'")
			}
			opts.outputFile = remaining[0]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[idx])
		}
	}
	return 0, nil
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

func sortAndFilter(lines []string, opts options) []string {
	keyCmp := buildKeyCmp(opts)

	fullCmp := keyCmp
	if opts.numericSort && !opts.stable {
		fullCmp = func(a, b string) int {
			if r := keyCmp(a, b); r != 0 {
				return r
			}
			return strings.Compare(a, b)
		}
	}

	sortCmp := fullCmp
	if opts.reverse {
		sortCmp = func(a, b string) int { return fullCmp(b, a) }
	}

	if opts.stable {
		slices.SortStableFunc(lines, sortCmp)
	} else {
		slices.SortFunc(lines, sortCmp)
	}

	if opts.unique {
		lines = dedup(lines, keyCmp)
	}

	return lines
}

func buildKeyCmp(opts options) func(string, string) int {
	if opts.numericSort {
		return func(a, b string) int {
			na, nb := parseNumeric(a), parseNumeric(b)
			if na < nb {
				return -1
			}
			if na > nb {
				return 1
			}
			return 0
		}
	}
	return strings.Compare
}

func parseNumeric(s string) float64 {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i == start || (i == start+1 && (s[start] == '-' || s[start] == '+')) {
		return 0
	}
	val, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0
	}
	return val
}

func dedup(lines []string, cmp func(string, string) int) []string {
	if len(lines) == 0 {
		return lines
	}
	result := []string{lines[0]}
	for _, line := range lines[1:] {
		if cmp(line, result[len(result)-1]) != 0 {
			result = append(result, line)
		}
	}
	return result
}

func writeLines(lines []string, w io.Writer) {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		fmt.Fprintln(bw, line)
	}
	bw.Flush()
}
