// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the unexpand utility (prd025-unexpand R1-R4).
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

const defaultTabStop = 8

func main() {
	sys.InstallSIGPIPEHandler() // R4.4

	var tabStops []int
	allMode, firstOnlyExplicit := false, false
	args := os.Args[1:]
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case arg == "--help":
			fmt.Print("Usage: unexpand [OPTION]... [FILE]...\nConvert blanks in each FILE to tabs, writing to standard output.\n\n  -a, --all           convert all blanks, instead of just initial blanks\n      --first-only    convert only leading sequences of blanks (default)\n  -t, --tabs=N        have tabs N characters apart, not 8\n  -t, --tabs=LIST     use comma separated list of tab positions\n      --help          display this help and exit\n      --version       output version information and exit\n")
			os.Exit(0)
		case arg == "--version":
			fmt.Println("unexpand (go-unix-utils) 0.1")
			os.Exit(0)
		case arg == "--all":
			allMode = true
			i++
		case arg == "--first-only":
			firstOnlyExplicit = true
			i++
		case strings.HasPrefix(arg, "--tabs="):
			tabStops = parseTabStops(arg[len("--tabs="):])
			i++
		case arg == "--tabs":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "unexpand: option '--tabs' requires an argument\n")
				os.Exit(1)
			}
			tabStops = parseTabStops(args[i])
			i++
		case len(arg) == 0 || arg[0] != '-' || arg == "-":
			files = append(files, arg)
			i++
		default:
			for j := 1; j < len(arg); {
				switch arg[j] {
				case 'a':
					allMode = true
					j++
				case 't':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "unexpand: option requires an argument -- 't'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					tabStops = parseTabStops(val)
					j = len(arg)
				default:
					fmt.Fprintf(os.Stderr, "unexpand: invalid option -- '%c'\n", arg[j])
					os.Exit(1)
				}
			}
			i++
		}
	}

	if len(tabStops) > 0 && !firstOnlyExplicit {
		allMode = true // R3.3: -t implies -a
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	if len(tabStops) == 0 {
		tabStops = []int{defaultTabStop}
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	for _, file := range files {
		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unexpand: %v\n", err) // R4.2
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}
		if err := unexpandInput(r, w, tabStops, allMode); err != nil {
			os.Exit(1) // R4.3
		}
	}
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}
func parseTabStops(s string) []int {
	parts := strings.Split(s, ",")
	stops := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "unexpand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		stops = append(stops, n)
	}
	return stops
}

// nextTabStop returns the next tab stop column (0-indexed). Returns -1 past last explicit stop.
func nextTabStop(col int, stops []int) int {
	if len(stops) == 1 {
		interval := stops[0]
		return col + (interval - col%interval)
	}
	for _, s := range stops {
		if s-1 > col {
			return s - 1
		}
	}
	return -1
}

// unexpandInput converts spaces to tabs in r, writing to w.
func unexpandInput(r io.Reader, w *bufio.Writer, stops []int, allMode bool) error {
	br := bufio.NewReader(r)
	col, inLeading, spaceRun, spaceStartCol := 0, true, 0, 0

	emitSpaces := func(converting bool) error {
		if !converting {
			for range spaceRun {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
			spaceRun = 0
			return nil
		}
		curCol, target := spaceStartCol, spaceStartCol+spaceRun
		for curCol < target {
			next := nextTabStop(curCol, stops)
			if next == -1 || next > target {
				for curCol < target {
					if err := w.WriteByte(' '); err != nil {
						return err
					}
					curCol++
				}
			} else {
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				curCol = next
			}
		}
		spaceRun = 0
		return nil
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				if spaceRun > 0 {
					return emitSpaces(allMode || inLeading)
				}
				return nil
			}
			return err
		}
		converting := allMode || inLeading
		switch b {
		case '\n':
			if spaceRun > 0 {
				if err := emitSpaces(converting); err != nil {
					return err
				}
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			col, inLeading = 0, true
		case ' ':
			if converting {
				if spaceRun == 0 {
					spaceStartCol = col
				}
				spaceRun++
			} else {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
			col++
		case '\t':
			if converting && spaceRun > 0 {
				if err := emitSpaces(true); err != nil {
					return err
				}
			}
			if err := w.WriteByte('\t'); err != nil {
				return err
			}
			if next := nextTabStop(col, stops); next == -1 {
				col++
			} else {
				col = next
			}
		default:
			if spaceRun > 0 {
				if err := emitSpaces(converting); err != nil {
					return err
				}
			}
			if err := w.WriteByte(b); err != nil {
				return err
			}
			col++
			inLeading = false
		}
	}
}
