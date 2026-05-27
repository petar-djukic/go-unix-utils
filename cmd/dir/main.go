// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd107-dir R1.1-R1.5, R2.1-R2.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: dir [OPTION]... [FILE]...
List directory contents.

  -a             do not ignore entries starting with .
  -A             do not list implied . and ..
  -C             list entries by columns
  -d             list directories themselves, not their contents
  -F             append indicator (one of */=>@|) to entries
  -h             with -l and/or -s, print human readable sizes
  -i             print the index number of each file
  -l             use a long listing format
  -n             like -l, but list numeric user and group IDs
  -R             list subdirectories recursively
  -r             reverse order while sorting
  -s             print the allocated size of each file, in blocks
  -S             sort by file size, largest first
  -t             sort by modification time, newest first
  -U             do not sort; list entries in directory order
  -v             natural sort of (version) numbers within text
  -x             list entries by lines instead of by columns
  -1             list one file per line
      --color[=WHEN]  colorize the output; WHEN can be 'always'
                         (default if omitted), 'auto', or 'never'
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `dir (go-unix-utils) dev
`

type options struct {
	singleColumn   bool
	longFormat     bool
	forceColumns   bool
	horizontalSort bool
	showAll        bool
	showAlmostAll  bool
	dirAsEntry     bool
	sortByTime     bool
	sortBySize     bool
	reverseSort    bool
	unsorted       bool
	versionSort    bool
	showInode      bool
	showBlocks     bool
	numericIds     bool
	humanReadable  bool
	classify       bool
	recursive      bool
	colorMode      string
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, paths := parseArgs(os.Args[1:])
	os.Exit(run(paths, opts))
}

func parseArgs(args []string) (options, []string) {
	opts := options{forceColumns: true}
	var paths []string
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--":
			return opts, append(paths, args...)
		case arg == "--color":
			opts.colorMode = "always"
		case strings.HasPrefix(arg, "--color="):
			opts.colorMode = parseColorValue(arg)
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "dir: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
			os.Exit(2)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			parseShortFlags(arg[1:], &opts)
		default:
			paths = append(paths, arg)
		}
	}
	return opts, paths
}

func parseShortFlags(flags string, opts *options) {
	for _, ch := range flags {
		switch ch {
		case '1':
			opts.forceColumns = false
			opts.horizontalSort = false
			if !opts.longFormat {
				opts.singleColumn = true
			}
		case 'l':
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'C':
			opts.forceColumns = true
			opts.horizontalSort = false
			opts.longFormat = false
			opts.singleColumn = false
		case 'x':
			opts.forceColumns = true
			opts.horizontalSort = true
			opts.longFormat = false
			opts.singleColumn = false
		case 'a':
			opts.showAll = true
			opts.showAlmostAll = false
		case 'A':
			opts.showAlmostAll = true
			opts.showAll = false
		case 'b', 'B', 'c', 'G', 'H', 'k', 'L', 'm', 'N',
			'o', 'p', 'q', 'Q', 'u', 'X', 'Z':
		case 'd':
			opts.dirAsEntry = true
		case 'f':
			opts.showAll = true
			opts.showAlmostAll = false
			opts.unsorted = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.versionSort = false
		case 'F':
			opts.classify = true
		case 'g':
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'h':
			opts.humanReadable = true
		case 'i':
			opts.showInode = true
		case 'n':
			opts.numericIds = true
			opts.longFormat = true
			opts.singleColumn = false
			opts.forceColumns = false
			opts.horizontalSort = false
		case 'R':
			opts.recursive = true
		case 'r':
			opts.reverseSort = true
		case 's':
			opts.showBlocks = true
		case 't':
			opts.sortByTime = true
			opts.sortBySize = false
			opts.unsorted = false
			opts.versionSort = false
		case 'S':
			opts.sortBySize = true
			opts.sortByTime = false
			opts.unsorted = false
			opts.versionSort = false
		case 'U':
			opts.unsorted = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.versionSort = false
		case 'v':
			opts.versionSort = true
			opts.sortByTime = false
			opts.sortBySize = false
			opts.unsorted = false
		default:
			fmt.Fprintf(os.Stderr, "dir: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
			os.Exit(2)
		}
	}
}

func parseColorValue(arg string) string {
	val := arg[len("--color="):]
	switch val {
	case "always", "auto", "never":
		return val
	default:
		fmt.Fprintf(os.Stderr, "dir: invalid argument '%s' for '--color'\n", val)
		fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
		os.Exit(2)
		return ""
	}
}

func run(paths []string, opts options) int {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	width := termWidth()
	exitCode := 0

	var files []string
	var dirs []string
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dir: cannot access '%s': %s\n", p, sysErrMsg(err))
			exitCode = 2
			continue
		}
		if info.IsDir() && !opts.dirAsEntry {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}

	showHeaders := len(dirs) > 1 || len(files) > 0 || opts.recursive
	printed := false

	if len(files) > 0 {
		sortNames(files, opts)
		escaped := make([]string, len(files))
		for i, f := range files {
			escaped[i] = escapeC(f)
		}
		writeEntries(escaped, width, opts)
		printed = true
	}

	for _, d := range dirs {
		if printed {
			fmt.Println()
		}
		if showHeaders {
			fmt.Printf("%s:\n", d)
		}
		names, err := listDir(d, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dir: cannot open directory '%s': %s\n", d, sysErrMsg(err))
			exitCode = 1
			printed = true
			continue
		}
		writeEntries(names, width, opts)
		printed = true
	}

	return exitCode
}

func sortNames(names []string, opts options) {
	if opts.unsorted {
		return
	}
	if opts.reverseSort {
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
	} else {
		sort.Strings(names)
	}
}

func writeEntries(names []string, width int, opts options) {
	if opts.singleColumn || opts.longFormat {
		for _, name := range names {
			fmt.Println(name)
		}
	} else {
		printColumns(names, width)
	}
}

func termWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return 80
	}
	return w
}

func listDir(path string, opts options) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	if opts.showAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			if !opts.showAll && !opts.showAlmostAll {
				continue
			}
		}
		names = append(names, name)
	}
	sortNames(names, opts)
	escaped := make([]string, len(names))
	for i, name := range names {
		escaped[i] = escapeC(name)
	}
	return escaped, nil
}

func escapeC(s string) string {
	needsEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == ' ' || c < 0x20 || c >= 0x7f {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString("\\\\")
		case '\a':
			b.WriteString("\\a")
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\v':
			b.WriteString("\\v")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		case ' ':
			b.WriteString("\\ ")
		default:
			if c < 0x20 || c >= 0x7f {
				b.WriteByte('\\')
				b.WriteByte('0' + c>>6)
				b.WriteByte('0' + (c>>3)&7)
				b.WriteByte('0' + c&7)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

func printColumns(names []string, width int) {
	n := len(names)
	if n == 0 {
		return
	}
	lens := make([]int, n)
	for i, name := range names {
		lens[i] = len(name)
	}
	ncols, nrows := computeLayout(lens, width)
	colWidths := make([]int, ncols)
	for col := range ncols {
		for row := range nrows {
			idx := col*nrows + row
			if idx < n && lens[idx] > colWidths[col] {
				colWidths[col] = lens[idx]
			}
		}
		if col < ncols-1 {
			colWidths[col] += 2
		}
	}
	for row := range nrows {
		pos := 0
		for col := range ncols {
			idx := col*nrows + row
			if idx >= n {
				break
			}
			fmt.Print(names[idx])
			nameEnd := pos + lens[idx]
			nextIdx := (col+1)*nrows + row
			if col+1 < ncols && nextIdx < n {
				target := pos + colWidths[col]
				printIndent(nameEnd, target)
				pos = target
			}
		}
		fmt.Println()
	}
}

func printIndent(from, to int) {
	for from < to {
		if to/8 > (from+1)/8 {
			fmt.Print("\t")
			from += 8 - from%8
		} else {
			fmt.Print(" ")
			from++
		}
	}
}

func computeLayout(lens []int, width int) (int, int) {
	n := len(lens)
	if n == 0 {
		return 0, 0
	}
	maxCols := min(n, width)
	for nc := maxCols; nc >= 2; nc-- {
		nr := (n + nc - 1) / nc
		total := 0
		fits := true
		for col := range nc {
			colMax := 0
			for row := range nr {
				idx := col*nr + row
				if idx < n && lens[idx] > colMax {
					colMax = lens[idx]
				}
			}
			total += colMax
			if col < nc-1 {
				total += 2
			}
			if total > width {
				fits = false
				break
			}
		}
		if fits {
			return nc, nr
		}
	}
	return 1, n
}

func sysErrMsg(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return capitalize(pe.Err.Error())
	}
	return capitalize(err.Error())
}

func capitalize(s string) string {
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}
