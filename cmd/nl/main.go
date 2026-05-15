// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd022-nl R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

type options struct {
	bodyStyle      string
	headerStyle    string
	footerStyle    string
	numberFormat   string
	width          int
	separator      string
	startNum       int
	increment      int
	delimiter      string
	noReset        bool
	joinBlankCount int
}

func defaultOptions() options {
	return options{
		bodyStyle:      "t",
		headerStyle:    "n",
		footerStyle:    "n",
		numberFormat:   "rn",
		width:          6,
		separator:      "\t",
		startNum:       1,
		increment:      1,
		delimiter:      `\:`,
		noReset:        false,
		joinBlankCount: 1,
	}
}

func run(args []string) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: %s\n", err)
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
			fmt.Fprintf(os.Stderr, "nl: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func parseArgs(args []string) (options, []string, error) {
	opts := defaultOptions()
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
		if len(arg) > 1 && arg[0] == '-' {
			consumed, err := parseFlag(arg, args[i+1:], &opts)
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

func parseFlag(arg string, remaining []string, opts *options) (int, error) {
	flag := arg[1:]
	switch {
	case flag == "p":
		opts.noReset = true
		return 0, nil
	case strings.HasPrefix(flag, "b"):
		return setStyle(flag[1:], remaining, &opts.bodyStyle, 'b')
	case strings.HasPrefix(flag, "h"):
		return setStyle(flag[1:], remaining, &opts.headerStyle, 'h')
	case strings.HasPrefix(flag, "f"):
		return setStyle(flag[1:], remaining, &opts.footerStyle, 'f')
	case strings.HasPrefix(flag, "n"):
		return setStringOpt(flag[1:], remaining, &opts.numberFormat, 'n')
	case strings.HasPrefix(flag, "s"):
		return setStringOpt(flag[1:], remaining, &opts.separator, 's')
	case strings.HasPrefix(flag, "d"):
		return setStringOpt(flag[1:], remaining, &opts.delimiter, 'd')
	case strings.HasPrefix(flag, "w"):
		return setIntOpt(flag[1:], remaining, &opts.width, 'w')
	case strings.HasPrefix(flag, "v"):
		return setIntOpt(flag[1:], remaining, &opts.startNum, 'v')
	case strings.HasPrefix(flag, "i"):
		return setIntOpt(flag[1:], remaining, &opts.increment, 'i')
	case strings.HasPrefix(flag, "l"):
		return setIntOpt(flag[1:], remaining, &opts.joinBlankCount, 'l')
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", flag)
	}
}

func setStyle(val string, remaining []string, dst *string, flag byte) (int, error) {
	if val != "" {
		*dst = val
		return 0, nil
	}
	if len(remaining) > 0 {
		*dst = remaining[0]
		return 1, nil
	}
	return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func setStringOpt(val string, remaining []string, dst *string, flag byte) (int, error) {
	if val != "" {
		*dst = val
		return 0, nil
	}
	if len(remaining) > 0 {
		*dst = remaining[0]
		return 1, nil
	}
	return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func setIntOpt(val string, remaining []string, dst *int, flag byte) (int, error) {
	if val != "" {
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("invalid number for -%c: '%s'", flag, val)
		}
		*dst = n
		return 0, nil
	}
	if len(remaining) > 0 {
		n, err := strconv.Atoi(remaining[0])
		if err != nil {
			return 0, fmt.Errorf("invalid number for -%c: '%s'", flag, remaining[0])
		}
		*dst = n
		return 1, nil
	}
	return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func processFile(path string, opts options) error {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	return processInput(r, opts)
}

func processInput(r io.Reader, opts options) error {
	w := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(r)
	lineNum := opts.startNum
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := fmt.Fprintf(w, "%*d%s%s\n", opts.width, lineNum, opts.separator, line); err != nil {
			return err
		}
		lineNum += opts.increment
	}
	return w.Flush()
}
