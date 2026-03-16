// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm R1.1-R1.4, R2.1-R2.4, R3.1-R3.4: cmd/comm reads
// two sorted text files line by line and produces three-column output
// showing lines unique to file 1, unique to file 2, and common to both.
// Supports -1, -2, -3 flags to suppress individual columns, --check-order
// and --nocheck-order to control input order validation,
// --output-delimiter=STRING to replace tab as the column separator, and
// --version/--help flags. Installs SIGPIPE handler per shared protocol.
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

// progName is the name used in error messages to match GNU comm format.
const progName = "comm"

// orderMode controls how comm responds to unsorted input.
// R2.1-R2.3: three modes matching GNU comm behavior.
type orderMode int

const (
	// orderDefault: warn per file, continue processing, print final
	// "input is not in sorted order" message, exit 1.
	orderDefault orderMode = iota
	// orderCheck (--check-order): warn on first unsorted line, stop
	// processing immediately, exit 1.
	orderCheck
	// orderNoCheck (--nocheck-order): no order validation at all.
	orderNoCheck
)

// options holds the parsed command-line flags for column suppression,
// order checking, and output delimiter.
// R1.3: -1, -2, -3 suppress columns 1, 2, 3 respectively.
// R2.1-R2.3: --check-order / --nocheck-order / default.
// R3.4: --output-delimiter=STRING replaces tab as column separator.
type options struct {
	suppress1      bool      // -1: suppress column 1 (lines unique to file1)
	suppress2      bool      // -2: suppress column 2 (lines unique to file2)
	suppress3      bool      // -3: suppress column 3 (lines common to both)
	order          orderMode // order-checking mode
	outputDelim    string    // column separator (default: tab)
	outputDelimSet bool      // true if --output-delimiter was explicitly given
}

func main() {
	// R1.4: install SIGPIPE handler for graceful exit when piped.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var opts options
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s [OPTION]... FILE1 FILE2\n"+
					"Compare sorted files FILE1 and FILE2 line by line.\n\n"+
					"When FILE1 or FILE2 (not both) is -, read standard input.\n\n"+
					"With no options, produce three-column output.  Column one contains\n"+
					"lines unique to FILE1, column two contains lines unique to FILE2,\n"+
					"and column three contains lines common to both files.\n\n"+
					"  -1                    suppress column 1 (lines unique to FILE1)\n"+
					"  -2                    suppress column 2 (lines unique to FILE2)\n"+
					"  -3                    suppress column 3 (lines that appear in both files)\n"+
					"      --check-order     check that the input is correctly sorted\n"+
					"      --nocheck-order   do not check that the input is correctly sorted\n"+
					"      --output-delimiter=STR  separate columns with STR\n"+
					"      --help            display this help and exit\n"+
					"      --version         output version information and exit\n",
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

		// R2.1: --check-order makes order violations fatal (stop + exit 1).
		if arg == "--check-order" {
			opts.order = orderCheck
			continue
		}
		// R2.2: --nocheck-order suppresses all order checking.
		// D2: last flag wins when both are specified.
		if arg == "--nocheck-order" {
			opts.order = orderNoCheck
			continue
		}

		// R3.4: --output-delimiter=STRING replaces tab as column separator.
		if arg == "--output-delimiter" || strings.HasPrefix(arg, "--output-delimiter=") {
			if strings.Contains(arg, "=") {
				opts.outputDelim = arg[len("--output-delimiter="):]
			} else {
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--output-delimiter' requires an argument\n", progName)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
				opts.outputDelim = args[i]
			}
			opts.outputDelimSet = true
			continue
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

		// Short flags: -1, -2, -3 and combinations like -12, -123.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case '1':
				opts.suppress1 = true
			case '2':
				opts.suppress2 = true
			case '3':
				opts.suppress3 = true
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
		}
	}

	if len(files) < 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
	if len(files) > 2 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, files[2])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// R1.2: accept '-' as a filename to read from stdin.
	r1, closer1, err := openInput(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, files[0], unwrapPathError(err))
		os.Exit(1)
	}
	if closer1 != nil {
		defer closer1.Close()
	}

	r2, closer2, err := openInput(files[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, files[1], unwrapPathError(err))
		os.Exit(1)
	}
	if closer2 != nil {
		defer closer2.Close()
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := commLines(bufio.NewReader(r1), bufio.NewReader(r2), w, opts)

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// openInput opens a file for reading. If name is "-", it returns os.Stdin.
// The returned io.Closer should be closed by the caller; it is nil for stdin.
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

// readLine reads the next line from a buffered reader, stripping the trailing
// newline. Returns the line content, whether a line was read, and any error.
func readLine(br *bufio.Reader) (string, bool, error) {
	line, err := br.ReadString('\n')
	if len(line) > 0 {
		// Strip trailing newline if present.
		if line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		return line, true, err
	}
	return "", false, err
}

// columnPrefix returns the delimiter-based indentation prefix for a given
// column (1, 2, or 3) with the suppression flags applied.
// R2.4: when a column is suppressed, remaining columns shift left.
// R3.4: when --output-delimiter is set, uses that string instead of tab.
func columnPrefix(col int, opts options) string {
	count := 0
	switch col {
	case 1:
		// Column 1 has no indentation.
		count = 0
	case 2:
		// Column 2 normally has 1 delimiter, reduced by 1 for each suppressed earlier column.
		count = 1
		if opts.suppress1 {
			count--
		}
	case 3:
		// Column 3 normally has 2 delimiters, reduced by 1 for each suppressed earlier column.
		count = 2
		if opts.suppress1 {
			count--
		}
		if opts.suppress2 {
			count--
		}
	}
	delim := "\t"
	if opts.outputDelimSet {
		delim = opts.outputDelim
	}
	return strings.Repeat(delim, count)
}

// warnUnsorted emits a diagnostic to stderr if the current line is out of
// sorted order relative to prev. Returns the new warned state.
// D3: at most one diagnostic per file.
// R2.4: format matches GNU comm: "comm: file N is not in sorted order".
func warnUnsorted(current, prev string, fileNum int, hasPrev, alreadyWarned bool) bool {
	if !hasPrev || alreadyWarned {
		return alreadyWarned
	}
	if current < prev {
		fmt.Fprintf(os.Stderr, "%s: file %d is not in sorted order\n", progName, fileNum)
		return true
	}
	return false
}

// commLines performs the three-column comparison of two sorted inputs.
// R1.1: lines unique to file1 go to column 1, unique to file2 go to column 2,
// common lines go to column 3.
// R1.2: comparison is lexicographic byte ordering (LC_ALL=C).
// R1.3: when one file is exhausted, remaining lines go to the appropriate column.
// R2.1-R2.3: order checking per opts.order mode.
func commLines(br1, br2 *bufio.Reader, w *bufio.Writer, opts options) int {
	doCheck := opts.order != orderNoCheck

	// R2.1-R2.3: order tracking state per file.
	var prev1, prev2 string
	var hasPrev1, hasPrev2 bool
	var warned1, warned2 bool

	line1, has1, err1 := readLine(br1)
	if err1 != nil && err1 != io.EOF {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err1)
		return 1
	}

	line2, has2, err2 := readLine(br2)
	if err2 != nil && err2 != io.EOF {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err2)
		return 1
	}

	for has1 && has2 {
		if line1 < line2 {
			// R1.1: line unique to file1 — column 1.
			if !opts.suppress1 {
				if werr := writeLine(w, columnPrefix(1, opts), line1); werr != nil {
					return 1
				}
			}
			prev1 = line1
			hasPrev1 = true
			line1, has1, err1 = readLine(br1)
			if err1 != nil && err1 != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err1)
				return 1
			}
			if doCheck && has1 {
				warned1 = warnUnsorted(line1, prev1, 1, hasPrev1, warned1)
				if warned1 && opts.order == orderCheck {
					break
				}
			}
		} else if line1 > line2 {
			// R1.1: line unique to file2 — column 2.
			if !opts.suppress2 {
				if werr := writeLine(w, columnPrefix(2, opts), line2); werr != nil {
					return 1
				}
			}
			prev2 = line2
			hasPrev2 = true
			line2, has2, err2 = readLine(br2)
			if err2 != nil && err2 != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err2)
				return 1
			}
			if doCheck && has2 {
				warned2 = warnUnsorted(line2, prev2, 2, hasPrev2, warned2)
				if warned2 && opts.order == orderCheck {
					break
				}
			}
		} else {
			// R1.1: common line — column 3.
			if !opts.suppress3 {
				if werr := writeLine(w, columnPrefix(3, opts), line1); werr != nil {
					return 1
				}
			}
			prev1 = line1
			hasPrev1 = true
			line1, has1, err1 = readLine(br1)
			if err1 != nil && err1 != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err1)
				return 1
			}
			if doCheck && has1 {
				warned1 = warnUnsorted(line1, prev1, 1, hasPrev1, warned1)
				if warned1 && opts.order == orderCheck {
					break
				}
			}
			prev2 = line2
			hasPrev2 = true
			line2, has2, err2 = readLine(br2)
			if err2 != nil && err2 != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err2)
				return 1
			}
			if doCheck && has2 {
				warned2 = warnUnsorted(line2, prev2, 2, hasPrev2, warned2)
				if warned2 && opts.order == orderCheck {
					break
				}
			}
		}
	}

	// --check-order: if we detected unsorted input, stop here.
	if opts.order == orderCheck && (warned1 || warned2) {
		return 1
	}

	// R1.3: drain remaining lines from file1.
	for has1 {
		if !opts.suppress1 {
			if werr := writeLine(w, columnPrefix(1, opts), line1); werr != nil {
				return 1
			}
		}
		prev1 = line1
		hasPrev1 = true
		line1, has1, err1 = readLine(br1)
		if err1 != nil && err1 != io.EOF {
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err1)
			return 1
		}
		if doCheck && has1 {
			warned1 = warnUnsorted(line1, prev1, 1, hasPrev1, warned1)
			if warned1 && opts.order == orderCheck {
				return 1
			}
		}
	}

	// R1.3: drain remaining lines from file2.
	for has2 {
		if !opts.suppress2 {
			if werr := writeLine(w, columnPrefix(2, opts), line2); werr != nil {
				return 1
			}
		}
		prev2 = line2
		hasPrev2 = true
		line2, has2, err2 = readLine(br2)
		if err2 != nil && err2 != io.EOF {
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err2)
			return 1
		}
		if doCheck && has2 {
			warned2 = warnUnsorted(line2, prev2, 2, hasPrev2, warned2)
			if warned2 && opts.order == orderCheck {
				return 1
			}
		}
	}

	// R2.3: default mode — if any unsorted input was detected, print final
	// summary and exit 1.
	if opts.order == orderDefault && (warned1 || warned2) {
		fmt.Fprintf(os.Stderr, "%s: input is not in sorted order\n", progName)
		return 1
	}

	return 0
}

// writeLine writes a single output line with the given prefix and a trailing newline.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// unwrapPathError extracts the inner error from an *os.PathError and
// capitalizes the first letter to match GNU error message format.
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
