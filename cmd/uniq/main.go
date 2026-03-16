// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq R1.1-R1.4: cmd/uniq reads input line by line and
// suppresses adjacent duplicate lines, writing unique lines to stdout or an
// output file. Reads from stdin when no file argument is given, or from a
// named file. A second positional argument specifies the output file.
// Installs SIGPIPE handler per shared protocol.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU uniq format.
const progName = "uniq"

func main() {
	// R1.4: install SIGPIPE handler for graceful exit when piped.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s [OPTION]... [INPUT [OUTPUT]]\n"+
					"Filter adjacent matching lines from INPUT (or standard input),\n"+
					"writing to OUTPUT (or standard output).\n\n"+
					"With no options, matching lines are merged to the first occurrence.\n\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		// Unrecognized long options.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
			os.Exit(1)
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Short flags — none wired for R1.1-R1.4, report invalid.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
			os.Exit(1)
		}
	}

	// R1.2: first positional arg is input file, second is output file.
	inputName := "-"
	outputName := ""
	if len(files) >= 1 {
		inputName = files[0]
	}
	if len(files) >= 2 {
		outputName = files[1]
	}
	if len(files) > 2 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, files[2])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// Open input.
	var input io.Reader
	if inputName == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(inputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, inputName, unwrapPathError(err))
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}

	// Open output.
	var output io.Writer
	if outputName == "" {
		output = os.Stdout
	} else {
		f, err := os.Create(outputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, outputName, unwrapPathError(err))
			os.Exit(1)
		}
		defer f.Close()
		output = f
	}

	w := bufio.NewWriter(output)
	exitCode := uniqLines(input, w)

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// uniqLines reads from r and writes deduplicated adjacent lines to w.
// R1.1: suppresses all but the first occurrence of any run of identical
// adjacent lines. R1.2: lines appearing only once are written through.
// R1.4: comparison is case-sensitive and includes the full line content.
func uniqLines(r io.Reader, w *bufio.Writer) int {
	br := bufio.NewReader(r)
	var prevLine string
	hasPrev := false

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			// Strip trailing newline for comparison, but preserve it for output.
			content := line
			hasNewline := line[len(line)-1] == '\n'
			if hasNewline {
				content = line[:len(line)-1]
			}

			if !hasPrev || content != prevLine {
				if _, werr := w.WriteString(content); werr != nil {
					return 1
				}
				// GNU uniq always outputs a trailing newline, even if
				// the last input line lacked one.
				if werr := w.WriteByte('\n'); werr != nil {
					return 1
				}
				prevLine = content
				hasPrev = true
			}
		}
		if err != nil {
			if err == io.EOF {
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// unwrapPathError extracts the inner error from an *os.PathError and
// capitalizes the first letter to match GNU error message format
// (e.g., "No such file or directory" instead of "no such file or directory").
func unwrapPathError(err error) string {
	inner := err
	if pe, ok := err.(*os.PathError); ok {
		inner = pe.Err
	}
	msg := inner.Error()
	if len(msg) == 0 {
		return msg
	}
	runes := []rune(msg)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
