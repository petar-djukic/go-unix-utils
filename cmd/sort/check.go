// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R4.2:
// check mode (-c/-C) reads lines and verifies sorted order.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
)

// runCheck reads input and verifies that it is sorted.
// R4.2: -c reports first disorder; -C exits silently.
func runCheck(cfg config) int {
	files := cfg.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	var prev []byte
	cmp := makeCompareResult(cfg)
	for _, name := range files {
		code := checkFile(name, &prev, cmp, cfg)
		if code != 0 {
			return code
		}
	}
	return 0
}

// checkFile verifies sorted order for a single input source.
func checkFile(
	name string, prev *[]byte, cmp func([]byte, []byte) int, cfg config,
) int {
	r, closer, err := openInput(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	scanner := makeScanner(r, cfg.zeroTerminated)
	return scanForDisorder(scanner, name, prev, cmp, cfg)
}

// scanForDisorder reads lines from a scanner and checks order.
func scanForDisorder(
	scanner *bufio.Scanner, name string,
	prev *[]byte, cmp func([]byte, []byte) int, cfg config,
) int {
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := copyBytes(scanner.Bytes())
		if *prev != nil && isDisordered(cmp(*prev, line), cfg.unique) {
			return reportDisorder(name, lineNum, line, cfg.checkQuiet)
		}
		*prev = line
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "sort: read error: %v\n", err)
		return 2
	}
	return 0
}

// isDisordered returns true if the comparison result indicates disorder.
// With -u, equal lines are also considered disordered.
func isDisordered(cmpResult int, unique bool) bool {
	if unique {
		return cmpResult >= 0
	}
	return cmpResult > 0
}

// reportDisorder prints a diagnostic (unless quiet) and returns exit 1.
func reportDisorder(name string, lineNum int, line []byte, quiet bool) int {
	if !quiet {
		fmt.Fprintf(os.Stderr, "sort: %s:%d: disorder: %s\n",
			name, lineNum, string(line))
	}
	return 1
}

// makeCompareResult returns a comparison function that returns an int.
// Used by check mode to detect disorder.
func makeCompareResult(cfg config) func(a, b []byte) int {
	if len(cfg.keys) > 0 {
		return func(a, b []byte) int {
			return compareKeys(a, b, cfg)
		}
	}
	return makeWholeLineCompare(cfg)
}

// makeWholeLineCompare returns a whole-line int comparison function.
func makeWholeLineCompare(cfg config) func(a, b []byte) int {
	mods := globalMods(cfg)
	return func(a, b []byte) int {
		cmp := compareByMods(a, b, mods)
		if cmp != 0 {
			if cfg.reverse {
				return -cmp
			}
			return cmp
		}
		if cfg.stable {
			return 0
		}
		lr := bytes.Compare(a, b)
		if cfg.reverse {
			return -lr
		}
		return lr
	}
}
