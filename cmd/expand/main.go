// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd024-expand.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	stops, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %s\n", err)
		os.Exit(1)
	}
	exitCode := 0
	if len(files) == 0 {
		expand(os.Stdin, w, stops)
	} else {
		for _, name := range files {
			if name == "-" {
				expand(os.Stdin, w, stops)
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "expand: %v\n", err)
				exitCode = 1
				continue
			}
			expand(f, w, stops)
			f.Close()
		}
	}
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

type tabStops struct {
	interval  int
	positions []int
}

func parseArgs(args []string) (tabStops, []string, error) {
	var rawStops []string
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if a == "-t" || strings.HasPrefix(a, "-t") {
			var val string
			if a == "-t" {
				if i+1 >= len(args) {
					return tabStops{}, nil, fmt.Errorf("option requires an argument -- 't'")
				}
				i++
				val = args[i]
			} else {
				val = a[2:]
			}
			rawStops = append(rawStops, val)
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			return tabStops{}, nil, fmt.Errorf("invalid option -- '%s'", a[1:])
		}
		files = append(files, a)
	}
	if len(rawStops) == 0 {
		return tabStops{interval: 8}, files, nil
	}
	stops, err := parseTabStops(strings.Join(rawStops, ","))
	if err != nil {
		return tabStops{}, nil, err
	}
	return stops, files, nil
}

func parseTabStops(val string) (tabStops, error) {
	parts := strings.FieldsFunc(val, func(r rune) bool {
		return r == ',' || r == ' '
	})
	if len(parts) == 0 {
		return tabStops{}, fmt.Errorf("tab size contains invalid character(s): '%s'", val)
	}
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil || n <= 0 {
			return tabStops{}, fmt.Errorf("invalid tab size: '%s'", parts[0])
		}
		return tabStops{interval: n}, nil
	}
	positions := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return tabStops{}, fmt.Errorf("invalid tab size: '%s'", p)
		}
		if i > 0 && n <= positions[i-1] {
			return tabStops{}, fmt.Errorf("tab sizes must be ascending")
		}
		positions[i] = n
	}
	return tabStops{positions: positions}, nil
}

func expand(r io.Reader, w *bufio.Writer, stops tabStops) {
	br := bufio.NewReader(r)
	col := 1
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case '\t':
			spaces := nextTabSpaces(col, stops)
			for range spaces {
				w.WriteByte(' ')
			}
			col += spaces
		case '\n':
			w.WriteByte('\n')
			col = 1
		default:
			w.WriteByte(b)
			col++
		}
	}
}

func nextTabSpaces(col int, stops tabStops) int {
	if stops.positions == nil {
		return stops.interval - (col-1)%stops.interval
	}
	colIdx := col - 1
	for _, p := range stops.positions {
		if p > colIdx {
			return p - colIdx
		}
	}
	return 1
}
