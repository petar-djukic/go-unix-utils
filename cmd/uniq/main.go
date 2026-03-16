// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4:
// cmd/uniq reads input line by line and suppresses adjacent duplicate lines,
// writing unique lines to stdout or an output file. Supports counting (-c),
// duplicate-only (-d), unique-only (-u), and all-repeated (-D) output modes.
// Supports field skipping (-f), character skipping (-s), check-chars (-w),
// and case-insensitive comparison (-i). Supports -z/--zero-terminated for
// NUL-delimited I/O and --group for group separator insertion. Reads from
// stdin when no file argument is given, or from a named file. A second
// positional argument specifies the output file. Installs SIGPIPE handler
// per shared protocol.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU uniq format.
const progName = "uniq"

// options holds the parsed command-line flags for output selection and
// comparison control.
// R2.1-R2.4: counting and duplicate filtering modes.
// R3.1-R3.4: field/character skip, check-chars, and case-insensitive options.
// R4.1-R4.4: zero-terminated I/O and group separator modes.
type options struct {
	count          bool   // -c/--count: prefix lines with occurrence count
	repeated       bool   // -d/--repeated: print only duplicate lines (one per group)
	unique         bool   // -u/--unique: print only unique lines
	allRepeated    string // -D/--all-repeated: print all duplicate lines; "none", "prepend", or "separate"
	skipFields     int    // -f N/--skip-fields=N: skip first N fields before comparing
	skipChars      int    // -s N/--skip-chars=N: skip first N characters (after field skip)
	checkChars     int    // -w N/--check-chars=N: compare at most N characters (0 = unlimited)
	ignoreCase     bool   // -i/--ignore-case: fold case when comparing
	zeroTerminated bool   // -z/--zero-terminated: use NUL as line delimiter
	group          string // --group: insert separators around groups; "prepend", "append", "separate", or "both"
}

func main() {
	// R1.4: install SIGPIPE handler for graceful exit when piped.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var files []string
	var opts options

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
					"  -c, --count           prefix lines by the number of occurrences\n"+
					"  -d, --repeated        only print duplicate lines, one for each group\n"+
					"  -D                    print all duplicate lines\n"+
					"      --all-repeated[=METHOD]  like -D, but allow separating groups\n"+
					"                                 with an empty line;\n"+
					"                                 METHOD={none(default),prepend,separate}\n"+
					"  -f, --skip-fields=N   avoid comparing the first N fields\n"+
					"      --group[=METHOD]  show all items, separating groups with an empty line;\n"+
					"                          METHOD={separate(default),prepend,append,both}\n"+
					"  -i, --ignore-case     ignore differences in case when comparing\n"+
					"  -s, --skip-chars=N    avoid comparing the first N characters\n"+
					"  -u, --unique          only print unique lines\n"+
					"  -w, --check-chars=N   compare no more than N characters in lines\n"+
					"  -z, --zero-terminated  line delimiter is NUL, not newline\n"+
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
		if arg == "--count" {
			opts.count = true
			continue
		}
		if arg == "--repeated" {
			opts.repeated = true
			continue
		}
		if arg == "--unique" {
			opts.unique = true
			continue
		}
		// R4.1: --zero-terminated.
		if arg == "--zero-terminated" {
			opts.zeroTerminated = true
			continue
		}
		// R4.2: --group with optional method.
		if arg == "--group" {
			opts.group = "separate"
			continue
		}
		if strings.HasPrefix(arg, "--group=") {
			method := arg[len("--group="):]
			switch method {
			case "prepend", "append", "separate", "both":
				opts.group = method
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--group'\n", progName, method)
				fmt.Fprintf(os.Stderr, "Valid arguments are:\n  - 'prepend'\n  - 'append'\n  - 'separate'\n  - 'both'\n")
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}
		// R2.4: --all-repeated with optional method.
		if arg == "--all-repeated" {
			opts.allRepeated = "none"
			continue
		}
		if strings.HasPrefix(arg, "--all-repeated=") {
			method := arg[len("--all-repeated="):]
			switch method {
			case "none", "prepend", "separate":
				opts.allRepeated = method
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--all-repeated'\n", progName, method)
				fmt.Fprintf(os.Stderr, "Valid arguments are:\n  - 'none'\n  - 'prepend'\n  - 'separate'\n")
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}

		// R3.4: -i/--ignore-case.
		if arg == "--ignore-case" {
			opts.ignoreCase = true
			continue
		}
		// R3.1: --skip-fields=N.
		if strings.HasPrefix(arg, "--skip-fields=") {
			n, perr := strconv.Atoi(arg[len("--skip-fields="):])
			if perr != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of fields to skip: '%s'\n", progName, arg[len("--skip-fields="):])
				os.Exit(1)
			}
			opts.skipFields = n
			continue
		}
		// R3.2: --skip-chars=N.
		if strings.HasPrefix(arg, "--skip-chars=") {
			n, perr := strconv.Atoi(arg[len("--skip-chars="):])
			if perr != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of bytes to skip: '%s'\n", progName, arg[len("--skip-chars="):])
				os.Exit(1)
			}
			opts.skipChars = n
			continue
		}
		// R3.3: --check-chars=N.
		if strings.HasPrefix(arg, "--check-chars=") {
			n, perr := strconv.Atoi(arg[len("--check-chars="):])
			if perr != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of bytes to compare: '%s'\n", progName, arg[len("--check-chars="):])
				os.Exit(1)
			}
			opts.checkChars = n
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

		// Short flags: -c, -d, -u, -D, -f, -s, -w, -i and combinations.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case 'c':
				opts.count = true
			case 'd':
				opts.repeated = true
			case 'u':
				opts.unique = true
			case 'i':
				// R3.4: case-insensitive comparison.
				opts.ignoreCase = true
			case 'z':
				// R4.1: zero-terminated lines.
				opts.zeroTerminated = true
			case 'D':
				// R2.4: -D defaults to "none" delimiter method.
				opts.allRepeated = "none"
			case 'f':
				// R3.1: -f N skip fields. Value is rest of this arg or next arg.
				val := flags[j+1:]
				if len(val) == 0 {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'f'\n", progName)
						os.Exit(1)
					}
					val = args[i]
				}
				n, perr := strconv.Atoi(val)
				if perr != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "%s: invalid number of fields to skip: '%s'\n", progName, val)
					os.Exit(1)
				}
				opts.skipFields = n
				j = len(flags) // consumed rest of flag cluster
			case 's':
				// R3.2: -s N skip characters. Value is rest of this arg or next arg.
				val := flags[j+1:]
				if len(val) == 0 {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\n", progName)
						os.Exit(1)
					}
					val = args[i]
				}
				n, perr := strconv.Atoi(val)
				if perr != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "%s: invalid number of bytes to skip: '%s'\n", progName, val)
					os.Exit(1)
				}
				opts.skipChars = n
				j = len(flags) // consumed rest of flag cluster
			case 'w':
				// R3.3: -w N check-chars. Value is rest of this arg or next arg.
				val := flags[j+1:]
				if len(val) == 0 {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
						os.Exit(1)
					}
					val = args[i]
				}
				n, perr := strconv.Atoi(val)
				if perr != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "%s: invalid number of bytes to compare: '%s'\n", progName, val)
					os.Exit(1)
				}
				opts.checkChars = n
				j = len(flags) // consumed rest of flag cluster
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
		}
	}

	// R4.3: --group is incompatible with -c, -d, -D, and -u.
	if opts.group != "" && (opts.count || opts.repeated || opts.allRepeated != "" || opts.unique) {
		fmt.Fprintf(os.Stderr, "%s: --group is mutually exclusive with -c/-d/-D/-u\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
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
	var exitCode int
	if opts.group != "" {
		exitCode = uniqGroup(bufio.NewReader(input), w, opts)
	} else {
		exitCode = uniqLines(input, w, opts)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// lineDelimiter returns the line delimiter byte based on -z flag.
// R4.1: NUL when zero-terminated, newline otherwise.
func lineDelimiter(opts options) byte {
	if opts.zeroTerminated {
		return 0
	}
	return '\n'
}

// compareKey extracts the comparison substring from a line after applying
// field skip (-f), character skip (-s), and check-chars (-w) options, then
// optionally folds case (-i).
// R3.1: skip N fields. R3.2: skip N chars. R3.3: limit to N chars. R3.4: fold case.
func compareKey(line string, opts options) string {
	s := line
	// R3.1: skip fields (whitespace-delimited).
	for f := 0; f < opts.skipFields && len(s) > 0; f++ {
		// Skip leading whitespace before the field.
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
			s = s[1:]
		}
		// Skip the non-whitespace field content.
		for len(s) > 0 && s[0] != ' ' && s[0] != '\t' {
			s = s[1:]
		}
	}
	// R3.2: skip characters (after field skip).
	if opts.skipChars > 0 {
		if opts.skipChars >= len(s) {
			s = ""
		} else {
			s = s[opts.skipChars:]
		}
	}
	// R3.3: limit comparison to N characters.
	if opts.checkChars > 0 && opts.checkChars < len(s) {
		s = s[:opts.checkChars]
	}
	// R3.4: fold case for case-insensitive comparison.
	if opts.ignoreCase {
		s = strings.ToUpper(s)
	}
	return s
}

// uniqLines reads from r and writes deduplicated adjacent lines to w,
// applying the output selection modes from opts.
// R1.1: suppresses all but the first occurrence of any run of identical
// adjacent lines. R1.2: lines appearing only once are written through.
// R1.4: comparison is case-sensitive and includes the full line content.
// R2.1-R2.4: counting and filtering via opts.
// R3.1-R3.4: comparison key extraction via compareKey.
func uniqLines(r io.Reader, w *bufio.Writer, opts options) int {
	br := bufio.NewReader(r)
	delim := lineDelimiter(opts)

	// R2.4: -D mode uses different output logic (all lines in duplicate groups).
	if opts.allRepeated != "" {
		return uniqAllRepeated(br, w, opts)
	}

	var prevLine string
	var prevKey string
	hasPrev := false
	count := 0

	for {
		line, err := br.ReadString(delim)
		if len(line) > 0 {
			content := line
			if line[len(line)-1] == delim {
				content = line[:len(line)-1]
			}

			key := compareKey(content, opts)
			if !hasPrev {
				prevLine = content
				prevKey = key
				hasPrev = true
				count = 1
			} else if key == prevKey {
				count++
			} else {
				if shouldOutput(count, opts) {
					if werr := writeLine(w, prevLine, count, opts.count, delim); werr != nil {
						return 1
					}
				}
				prevLine = content
				prevKey = key
				count = 1
			}
		}
		if err != nil {
			if err == io.EOF {
				// Flush last group.
				if hasPrev && shouldOutput(count, opts) {
					if werr := writeLine(w, prevLine, count, opts.count, delim); werr != nil {
						return 1
					}
				}
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// uniqAllRepeated handles -D/--all-repeated mode, printing every line in
// duplicate groups. R2.4: the delimiter method controls blank line insertion.
// R3.1-R3.4: uses compareKey for line comparison.
func uniqAllRepeated(br *bufio.Reader, w *bufio.Writer, opts options) int {
	delim := lineDelimiter(opts)
	// In -D mode we need to keep all lines in the current group, not just count.
	var group []string
	var prevKey string
	hasPrev := false
	firstDupGroup := true

	flushGroup := func() error {
		if len(group) <= 1 {
			return nil
		}
		// R2.4: delimiter method controls blank lines between groups.
		if opts.allRepeated == "prepend" || (opts.allRepeated == "separate" && !firstDupGroup) {
			if err := w.WriteByte(delim); err != nil {
				return err
			}
		}
		firstDupGroup = false
		for _, l := range group {
			if _, err := w.WriteString(l); err != nil {
				return err
			}
			if err := w.WriteByte(delim); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		line, err := br.ReadString(delim)
		if len(line) > 0 {
			content := line
			if line[len(line)-1] == delim {
				content = line[:len(line)-1]
			}

			key := compareKey(content, opts)
			if !hasPrev {
				group = []string{content}
				prevKey = key
				hasPrev = true
			} else if key == prevKey {
				group = append(group, content)
			} else {
				if ferr := flushGroup(); ferr != nil {
					return 1
				}
				group = []string{content}
				prevKey = key
			}
		}
		if err != nil {
			if err == io.EOF {
				if hasPrev {
					if ferr := flushGroup(); ferr != nil {
						return 1
					}
				}
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// uniqGroup handles --group mode, printing all lines and inserting empty
// line separators between groups of identical adjacent lines.
// R4.2: --group=prepend|append|separate|both controls separator placement.
// Separator logic: "separate" inserts between groups only. "prepend" adds
// before each group (including first). "append" adds after each group
// (including last). "both" adds before first, between groups, and after last
// — but inter-group separators are single (not doubled).
func uniqGroup(br *bufio.Reader, w *bufio.Writer, opts options) int {
	delim := lineDelimiter(opts)
	var prevKey string
	hasPrev := false
	firstGroup := true

	writeSep := func() error {
		return w.WriteByte(delim)
	}

	for {
		line, err := br.ReadString(delim)
		if len(line) > 0 {
			content := line
			if line[len(line)-1] == delim {
				content = line[:len(line)-1]
			}

			key := compareKey(content, opts)
			newGroup := !hasPrev || key != prevKey

			if newGroup {
				if firstGroup {
					// Before first group: prepend and both emit a separator.
					if opts.group == "prepend" || opts.group == "both" {
						if werr := writeSep(); werr != nil {
							return 1
						}
					}
				} else {
					// Between groups: all modes emit exactly one separator.
					if werr := writeSep(); werr != nil {
						return 1
					}
				}
				firstGroup = false
			}

			hasPrev = true
			prevKey = key

			if _, werr := w.WriteString(content); werr != nil {
				return 1
			}
			if werr := w.WriteByte(delim); werr != nil {
				return 1
			}
		}
		if err != nil {
			if err == io.EOF {
				// After last group: append and both emit a trailing separator.
				if hasPrev && (opts.group == "append" || opts.group == "both") {
					if werr := writeSep(); werr != nil {
						return 1
					}
				}
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// shouldOutput returns true if a group with the given count should produce
// output based on the filtering options.
// R2.1: -d filters to count > 1. R2.3: -u filters to count == 1.
func shouldOutput(count int, opts options) bool {
	if opts.repeated {
		return count > 1
	}
	if opts.unique {
		return count == 1
	}
	return true
}

// writeLine writes a single output line, optionally prefixed with count.
// R2.4: -c format uses %7d to match GNU uniq right-justified count.
// R4.1: uses delim as the line terminator (NUL or newline).
func writeLine(w *bufio.Writer, line string, count int, showCount bool, delim byte) error {
	if showCount {
		if _, err := fmt.Fprintf(w, "%7d %s", count, line); err != nil {
			return err
		}
		return w.WriteByte(delim)
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte(delim)
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
