// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU comm: compare two sorted files line by line.
// Implements prd029-comm R1-R4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// errPrefix is the program name used in error messages.
const errPrefix = "comm"

// options holds parsed command-line flag values.
type options struct {
	suppress1      bool // -1: suppress column 1 (lines unique to file1)
	suppress2      bool // -2: suppress column 2 (lines unique to file2)
	suppress3      bool // -3: suppress column 3 (lines common to both)
	checkOrder     bool // --check-order: fatal on unsorted input
	nocheckOrder   bool // --nocheck-order: disable order checking
	outputDelim    string
	outputDelimSet bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, file1Name, file2Name := parseArgs(os.Args[1:])

	f1, close1 := openInput(file1Name)
	if close1 != nil {
		defer close1()
	}
	f2, close2 := openInput(file2Name)
	if close2 != nil {
		defer close2()
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := runComm(f1, f2, w, opts)

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// openInput opens a file for reading, treating "-" as stdin. (prd029-comm R1.1)
func openInput(name string) (*os.File, func()) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		// R4.2: exit 1 when input file cannot be opened.
		msg := err.Error()
		if pe, ok := err.(*os.PathError); ok {
			msg = pe.Err.Error()
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", errPrefix, name, msg)
		os.Exit(1)
	}
	return f, func() { f.Close() }
}

// columnPrefixes computes the string prefix for each column based on which
// columns are suppressed. (prd029-comm R2.4)
func columnPrefixes(opts options, delim string) (string, string, string) {
	// Each visible column's prefix = delim repeated by count of visible columns before it.
	before2 := 0
	if !opts.suppress1 {
		before2++
	}
	before3 := before2
	if !opts.suppress2 {
		before3++
	}

	col1 := ""
	col2 := strings.Repeat(delim, before2)
	col3 := strings.Repeat(delim, before3)

	return col1, col2, col3
}

// runComm performs the merge comparison of two sorted inputs. (prd029-comm R1, R2, R3)
func runComm(f1, f2 *os.File, w *bufio.Writer, opts options) int {
	s1 := bufio.NewScanner(f1)
	s2 := bufio.NewScanner(f2)
	s1.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	s2.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	delim := "\t"
	if opts.outputDelimSet {
		delim = opts.outputDelim
	}
	prefix1, prefix2, prefix3 := columnPrefixes(opts, delim)

	have1 := s1.Scan()
	have2 := s2.Scan()

	var prev1, prev2 string
	first1, first2 := true, true
	seenUnpairable := false
	warned := [2]bool{}

	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()

		// R1.2: byte-for-byte comparison under LC_ALL=C.
		cmp := strings.Compare(line1, line2)

		switch {
		case cmp < 0:
			// line1 only in file1 → column 1.
			if !first1 {
				if fatal := orderViolation(line1, prev1, 0, opts, seenUnpairable, &warned); fatal {
					return 1
				}
			}
			prev1 = line1
			first1 = false
			seenUnpairable = true
			if !opts.suppress1 {
				if err := writeLine(w, prefix1, line1); err != nil {
					fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
					return 1
				}
			}
			have1 = s1.Scan()

		case cmp > 0:
			// line2 only in file2 → column 2.
			if !first2 {
				if fatal := orderViolation(line2, prev2, 1, opts, seenUnpairable, &warned); fatal {
					return 1
				}
			}
			prev2 = line2
			first2 = false
			seenUnpairable = true
			if !opts.suppress2 {
				if err := writeLine(w, prefix2, line2); err != nil {
					fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
					return 1
				}
			}
			have2 = s2.Scan()

		default:
			// Equal lines → column 3.
			if !first1 {
				if fatal := orderViolation(line1, prev1, 0, opts, seenUnpairable, &warned); fatal {
					return 1
				}
			}
			if !first2 {
				if fatal := orderViolation(line2, prev2, 1, opts, seenUnpairable, &warned); fatal {
					return 1
				}
			}
			prev1 = line1
			prev2 = line2
			first1 = false
			first2 = false
			if !opts.suppress3 {
				if err := writeLine(w, prefix3, line1); err != nil {
					fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
					return 1
				}
			}
			have1 = s1.Scan()
			have2 = s2.Scan()
		}
	}

	// R1.3: drain remaining lines from whichever file is not exhausted.
	for have1 {
		line1 := s1.Text()
		if !first1 {
			if fatal := orderViolation(line1, prev1, 0, opts, seenUnpairable, &warned); fatal {
				return 1
			}
		}
		prev1 = line1
		first1 = false
		seenUnpairable = true
		if !opts.suppress1 {
			if err := writeLine(w, prefix1, line1); err != nil {
				fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
				return 1
			}
		}
		have1 = s1.Scan()
	}

	for have2 {
		line2 := s2.Text()
		if !first2 {
			if fatal := orderViolation(line2, prev2, 1, opts, seenUnpairable, &warned); fatal {
				return 1
			}
		}
		prev2 = line2
		first2 = false
		seenUnpairable = true
		if !opts.suppress2 {
			if err := writeLine(w, prefix2, line2); err != nil {
				fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
				return 1
			}
		}
		have2 = s2.Scan()
	}

	if s1.Err() != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", errPrefix, s1.Err())
		return 1
	}
	if s2.Err() != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", errPrefix, s2.Err())
		return 1
	}

	return 0
}

// orderViolation checks if the current line is out of order relative to the
// previous line from the same file. Returns true if the violation is fatal
// (--check-order mode). (prd029-comm R3.1, R3.2, R3.3)
func orderViolation(line, prev string, fileIdx int, opts options, seenUnpairable bool, warned *[2]bool) bool {
	if opts.nocheckOrder {
		return false
	}
	if strings.Compare(line, prev) >= 0 {
		return false
	}
	// Out of order detected.
	if opts.checkOrder {
		fmt.Fprintf(os.Stderr, "%s: file %d is not in sorted order\n", errPrefix, fileIdx+1)
		return true
	}
	// Default mode: warn once per file when unpairable lines have been seen.
	if seenUnpairable && !warned[fileIdx] {
		fmt.Fprintf(os.Stderr, "%s: file %d is not in sorted order\n", errPrefix, fileIdx+1)
		warned[fileIdx] = true
	}
	return false
}

// writeLine writes a prefixed line with a newline terminator.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// parseArgs manually parses arguments, supporting GNU short and long forms.
func parseArgs(args []string) (options, string, string) {
	var opts options
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			files = append(files, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--check-order":
				opts.checkOrder = true
			case arg == "--nocheck-order":
				opts.nocheckOrder = true
			case strings.HasPrefix(arg, "--output-delimiter="):
				opts.outputDelim = arg[len("--output-delimiter="):]
				opts.outputDelimSet = true
			case arg == "--output-delimiter":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--output-delimiter' requires an argument\n", errPrefix)
					os.Exit(2)
				}
				opts.outputDelim = args[i]
				opts.outputDelimSet = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", errPrefix, arg)
				os.Exit(2)
			}
			i++
			continue
		}

		// Short options (e.g., -1, -2, -3, -12, -123).
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case '1':
					opts.suppress1 = true
				case '2':
					opts.suppress2 = true
				case '3':
					opts.suppress3 = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", errPrefix, arg[j])
					os.Exit(2)
				}
				j++
			}
			i++
			continue
		}

		// Positional argument.
		files = append(files, arg)
		i++
	}

	if len(files) < 2 {
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\n", errPrefix)
		} else {
			fmt.Fprintf(os.Stderr, "%s: missing operand after '%s'\n", errPrefix, files[0])
		}
		os.Exit(2)
	}
	if len(files) > 2 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", errPrefix, files[2])
		os.Exit(2)
	}

	return opts, files[0], files[1]
}
