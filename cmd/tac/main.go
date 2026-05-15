// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd021-tac R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tac: %s\n", err)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, f := range files {
		if err := processFile(f, opts); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "tac: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

type options struct {
	separator string
	before    bool
	regex     bool
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{separator: "\n"}
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" {
			files = append(files, arg)
			i++
			continue
		}
		if strings.HasPrefix(arg, "--separator=") {
			opts.separator = arg[len("--separator="):]
			i++
			continue
		}
		if arg == "--separator" {
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("option '--separator' requires an argument")
			}
			opts.separator = args[i+1]
			i += 2
			continue
		}
		if arg == "--before" {
			opts.before = true
			i++
			continue
		}
		if arg == "--regex" {
			opts.regex = true
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + consumed
			continue
		}
		files = append(files, arg)
		i++
	}
	return opts, files, nil
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'b':
			opts.before = true
		case 'r':
			opts.regex = true
		case 's':
			if rest := flags[j+1:]; rest != "" {
				opts.separator = rest
			} else if len(remaining) > consumed {
				opts.separator = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 's'")
			}
			return consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func processFile(path string, opts options) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	output := reverseInput(string(data), opts)
	_, werr := os.Stdout.WriteString(output)
	return werr
}

func reverseInput(input string, opts options) string {
	if input == "" {
		return ""
	}
	matches := findMatches(input, opts)
	if len(matches) == 0 {
		return input
	}
	if opts.before {
		return reverseBefore(input, matches)
	}
	return reverseAfter(input, matches)
}

func findMatches(input string, opts options) [][]int {
	if opts.regex {
		re, err := regexp.Compile("(?U)" + opts.separator)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: %s: %s\n", opts.separator, err)
			return nil
		}
		return re.FindAllStringIndex(input, -1)
	}
	var matches [][]int
	sep := opts.separator
	start := 0
	for {
		idx := strings.Index(input[start:], sep)
		if idx < 0 {
			break
		}
		absIdx := start + idx
		matches = append(matches, []int{absIdx, absIdx + len(sep)})
		start = absIdx + len(sep)
	}
	return matches
}

func reverseAfter(input string, matches [][]int) string {
	var chunks []string
	start := 0
	for _, m := range matches {
		chunks = append(chunks, input[start:m[1]])
		start = m[1]
	}
	if tail := input[start:]; tail != "" {
		chunks = append(chunks, tail)
	}
	reverseSlice(chunks)
	return strings.Join(chunks, "")
}

func reverseBefore(input string, matches [][]int) string {
	var chunks []string
	if matches[0][0] > 0 {
		chunks = append(chunks, input[:matches[0][0]])
	}
	for i, m := range matches {
		end := len(input)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		chunks = append(chunks, input[m[0]:end])
	}
	reverseSlice(chunks)
	return strings.Join(chunks, "")
}

func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
