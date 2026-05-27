// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd107-dir R1.1-R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		args = []string{"."}
	}

	width := termWidth()
	exitCode := 0
	showHeaders := len(args) > 1

	var files []string
	var dirs []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dir: cannot access '%s': %s\n", arg, sysErrMsg(err))
			exitCode = 2
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, arg)
		} else {
			files = append(files, arg)
		}
	}

	printed := false

	if len(files) > 0 {
		sort.Strings(files)
		escaped := make([]string, len(files))
		for i, f := range files {
			escaped[i] = escapeC(f)
		}
		printColumns(escaped, width)
		printed = true
	}

	for _, d := range dirs {
		if printed {
			fmt.Println()
		}
		if showHeaders {
			fmt.Printf("%s:\n", d)
		}
		names, err := listDir(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dir: cannot open directory '%s': %s\n", d, sysErrMsg(err))
			exitCode = 2
			printed = true
			continue
		}
		printColumns(names, width)
		printed = true
	}

	return exitCode
}

func termWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return 80
	}
	return w
}

func listDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
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
