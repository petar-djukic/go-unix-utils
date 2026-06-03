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
	"regexp"
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

type nlState struct {
	lineNum    int
	section    string
	blankCount int
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
	state := &nlState{
		lineNum: opts.startNum,
		section: "body",
	}
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, f := range files {
		if err := processFile(f, opts, state, w); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "nl: %s\n", err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "nl: %s\n", err)
		exitCode = 1
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

func processFile(path string, opts options, state *nlState, w *bufio.Writer) error {
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
	return processInput(r, opts, state, w)
}

func sectionStyle(section string, opts options) string {
	switch section {
	case "header":
		return opts.headerStyle
	case "footer":
		return opts.footerStyle
	default:
		return opts.bodyStyle
	}
}

func shouldNumber(style string, line string, blankCount int, joinBlankCount int) bool {
	empty := line == ""
	switch {
	case style == "a":
		if !empty {
			return true
		}
		return blankCount >= joinBlankCount
	case style == "t":
		return !empty
	case style == "n":
		return false
	case strings.HasPrefix(style, "p"):
		if empty {
			return false
		}
		re, err := regexp.Compile(style[1:])
		if err != nil {
			return false
		}
		return re.MatchString(line)
	}
	return false
}

func formatNumber(num int, width int, format string) string {
	switch format {
	case "ln":
		return fmt.Sprintf("%-*d", width, num)
	case "rz":
		return fmt.Sprintf("%0*d", width, num)
	default:
		return fmt.Sprintf("%*d", width, num)
	}
}

func processInput(r io.Reader, opts options, state *nlState, w *bufio.Writer) error {
	scanner := bufio.NewScanner(r)

	headerDelim := opts.delimiter + opts.delimiter + opts.delimiter
	bodyDelim := opts.delimiter + opts.delimiter
	footerDelim := opts.delimiter

	for scanner.Scan() {
		line := scanner.Text()

		if line == headerDelim {
			state.section = "header"
			if !opts.noReset {
				state.lineNum = opts.startNum
			}
			state.blankCount = 0
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
			continue
		}
		if line == bodyDelim {
			state.section = "body"
			if !opts.noReset {
				state.lineNum = opts.startNum
			}
			state.blankCount = 0
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
			continue
		}
		if line == footerDelim {
			state.section = "footer"
			if !opts.noReset {
				state.lineNum = opts.startNum
			}
			state.blankCount = 0
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
			continue
		}

		style := sectionStyle(state.section, opts)

		if line == "" {
			state.blankCount++
		} else {
			state.blankCount = 0
		}

		if shouldNumber(style, line, state.blankCount, opts.joinBlankCount) {
			numStr := formatNumber(state.lineNum, opts.width, opts.numberFormat)
			if _, err := fmt.Fprintf(w, "%s%s%s\n", numStr, opts.separator, line); err != nil {
				return err
			}
			state.lineNum += opts.increment
			if line == "" {
				state.blankCount = 0
			}
		} else {
			pad := strings.Repeat(" ", opts.width+len(opts.separator))
			if _, err := fmt.Fprintf(w, "%s%s\n", pad, line); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}
