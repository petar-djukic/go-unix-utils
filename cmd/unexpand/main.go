// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd025-unexpand.
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
	stops, allMode, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: %s\n", err)
		os.Exit(1)
	}
	exitCode := 0
	if len(files) == 0 {
		unexpand(os.Stdin, w, stops, allMode)
	} else {
		for _, name := range files {
			if name == "-" {
				unexpand(os.Stdin, w, stops, allMode)
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
				exitCode = 1
				continue
			}
			unexpand(f, w, stops, allMode)
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

func parseArgs(args []string) (tabStops, bool, []string, error) {
	var rawStops []string
	var files []string
	allMode := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if a == "-a" {
			allMode = true
			continue
		}
		if a == "-t" || strings.HasPrefix(a, "-t") {
			var val string
			if a == "-t" {
				if i+1 >= len(args) {
					return tabStops{}, false, nil, fmt.Errorf("option requires an argument -- 't'")
				}
				i++
				val = args[i]
			} else {
				val = a[2:]
			}
			rawStops = append(rawStops, val)
			allMode = true
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			return tabStops{}, false, nil, fmt.Errorf("invalid option -- '%s'", a[1:])
		}
		files = append(files, a)
	}
	if len(rawStops) == 0 {
		return tabStops{interval: 8}, allMode, files, nil
	}
	stops, err := parseTabStops(strings.Join(rawStops, ","))
	if err != nil {
		return tabStops{}, false, nil, err
	}
	return stops, allMode, files, nil
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

func unexpand(r io.Reader, w *bufio.Writer, stops tabStops, allMode bool) {
	br := bufio.NewReader(r)
	col := 1
	leading := true
	spaceRun := 0
	spaceRunStart := 0
	flush := func() {
		if !leading && spaceRun < 2 {
			for i := 0; i < spaceRun; i++ {
				w.WriteByte(' ')
			}
		} else {
			emitCompacted(w, spaceRunStart, spaceRun, stops)
		}
		spaceRun = 0
		spaceRunStart = 0
	}
	for {
		b, err := br.ReadByte()
		if err != nil {
			if spaceRun > 0 {
				flush()
			}
			return
		}
		switch b {
		case ' ':
			if leading || allMode {
				if spaceRun == 0 {
					spaceRunStart = col
				}
				spaceRun++
				col++
			} else {
				w.WriteByte(' ')
				col++
			}
		case '\t':
			if leading || allMode {
				nextCol := nextTabStop(col+spaceRun, stops)
				spaceRun = 0
				spaceRunStart = 0
				w.WriteByte('\t')
				col = nextCol
			} else {
				w.WriteByte('\t')
				col = nextTabStop(col, stops)
			}
		case '\n':
			if spaceRun > 0 {
				flush()
			}
			w.WriteByte('\n')
			col = 1
			leading = true
		default:
			if spaceRun > 0 {
				flush()
			}
			w.WriteByte(b)
			col++
			leading = false
		}
	}
}

func nextTabStop(col int, stops tabStops) int {
	if stops.positions == nil {
		return col + stops.interval - (col-1)%stops.interval
	}
	colIdx := col - 1
	for _, p := range stops.positions {
		if p > colIdx {
			return p + 1
		}
	}
	return col + 1
}

func pastLastStop(col int, stops tabStops) bool {
	if stops.positions == nil {
		return false
	}
	return col-1 >= stops.positions[len(stops.positions)-1]
}

func emitCompacted(w *bufio.Writer, startCol int, count int, stops tabStops) {
	col := startCol
	end := startCol + count
	for col < end {
		if !pastLastStop(col, stops) {
			next := nextTabStop(col, stops)
			if next <= end {
				w.WriteByte('\t')
				col = next
				continue
			}
		}
		w.WriteByte(' ')
		col++
	}
}