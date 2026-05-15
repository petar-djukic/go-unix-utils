// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd026-cut.
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
	byteList      string
	charList      string
	fieldList     string
	delimiter     byte
	outputDelim   string
	complement    bool
	onlyDelimited bool
	hasOutputDel  bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		os.Exit(1)
	}
	exitCode := processFiles(w, opts, files)
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func processFiles(w *bufio.Writer, opts options, files []string) int {
	exitCode := 0
	if len(files) == 0 {
		cutLine(os.Stdin, w, opts)
		return exitCode
	}
	for _, name := range files {
		if name == "-" {
			cutLine(os.Stdin, w, opts)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			exitCode = 1
			continue
		}
		cutLine(f, w, opts)
		f.Close()
	}
	return exitCode
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{delimiter: '\t'}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if a == "--complement" {
			opts.complement = true
			continue
		}
		if a == "--output-delimiter" {
			if i+1 >= len(args) {
				return options{}, nil, fmt.Errorf("option '--output-delimiter' requires an argument")
			}
			i++
			opts.outputDelim = args[i]
			opts.hasOutputDel = true
			continue
		}
		if strings.HasPrefix(a, "--output-delimiter=") {
			opts.outputDelim = a[len("--output-delimiter="):]
			opts.hasOutputDel = true
			continue
		}
		advanced, newI, err := parseShortFlag(a, args, i, &opts)
		if err != nil {
			return options{}, nil, err
		}
		i = newI
		if advanced {
			continue
		}
		files = append(files, a)
	}
	return opts, files, validateOpts(opts)
}

func parseShortFlag(a string, args []string, i int, opts *options) (bool, int, error) {
	switch {
	case a == "-b" || strings.HasPrefix(a, "-b"):
		val, idx, err := flagValue(a, "-b", args, i)
		if err != nil {
			return false, i, err
		}
		opts.byteList = val
		return true, idx, nil
	case a == "-c" || strings.HasPrefix(a, "-c"):
		val, idx, err := flagValue(a, "-c", args, i)
		if err != nil {
			return false, i, err
		}
		opts.charList = val
		return true, idx, nil
	case a == "-f" || strings.HasPrefix(a, "-f"):
		val, idx, err := flagValue(a, "-f", args, i)
		if err != nil {
			return false, i, err
		}
		opts.fieldList = val
		return true, idx, nil
	case a == "-d" || strings.HasPrefix(a, "-d"):
		val, idx, err := flagValue(a, "-d", args, i)
		if err != nil {
			return false, i, err
		}
		if len(val) != 1 {
			return false, i, fmt.Errorf("the delimiter must be a single character")
		}
		opts.delimiter = val[0]
		return true, idx, nil
	case a == "-s":
		opts.onlyDelimited = true
		return true, i, nil
	}
	if strings.HasPrefix(a, "-") && a != "-" {
		return false, i, fmt.Errorf("invalid option -- '%s'", a[1:])
	}
	return false, i, nil
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

func validateOpts(opts options) error {
	count := 0
	if opts.byteList != "" {
		count++
	}
	if opts.charList != "" {
		count++
	}
	if opts.fieldList != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}
	if count > 1 {
		return fmt.Errorf("only one type of list may be specified")
	}
	return nil
}

func cutLine(r io.Reader, w *bufio.Writer, opts options) {
	switch {
	case opts.byteList != "":
		cutBytes(r, w, opts)
	case opts.charList != "":
		cutChars(r, w, opts)
	default:
		cutFields(r, w, opts)
	}
}

func cutBytes(_ io.Reader, _ *bufio.Writer, _ options) {
	panic("not implemented")
}

func cutChars(_ io.Reader, _ *bufio.Writer, _ options) {
	panic("not implemented")
}

func cutFields(_ io.Reader, _ *bufio.Writer, _ options) {
	panic("not implemented")
}
