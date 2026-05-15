// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd027-paste.
package main

import (
	"bufio"
	"fmt"
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

func parseDelimiters(_ string) string {
	panic("not implemented")
}

func pasteParallel(_ *bufio.Writer, _ options, _ []string) int {
	panic("not implemented")
}

func pasteSerial(_ *bufio.Writer, _ options, _ []string) int {
	panic("not implemented")
}

