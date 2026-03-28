// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R3.4:
// merge mode (-m) merges pre-sorted input files without re-sorting.
// D4: reads one line at a time from each input using a heap-based k-way merge.
package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"io"
	"os"
)

// mergeEntry holds a buffered line and its source scanner.
type mergeEntry struct {
	line    []byte
	scanner *bufio.Scanner
	index   int // source file index for stable ordering
}

// mergeHeap implements heap.Interface for k-way merge.
type mergeHeap struct {
	entries []mergeEntry
	less    func(a, b []byte) bool
}

// Len returns the number of entries in the heap.
func (h *mergeHeap) Len() int { return len(h.entries) }

// Less compares two entries by sort key, breaking ties by file index.
func (h *mergeHeap) Less(i, j int) bool {
	if h.less(h.entries[i].line, h.entries[j].line) {
		return true
	}
	if h.less(h.entries[j].line, h.entries[i].line) {
		return false
	}
	return h.entries[i].index < h.entries[j].index
}

// Swap exchanges two entries in the heap.
func (h *mergeHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
}

// Push adds an entry to the heap (required by heap.Interface).
func (h *mergeHeap) Push(x any) {
	h.entries = append(h.entries, x.(mergeEntry))
}

// Pop removes and returns the smallest entry (required by heap.Interface).
func (h *mergeHeap) Pop() any {
	old := h.entries
	n := len(old)
	entry := old[n-1]
	h.entries = old[:n-1]
	return entry
}

// runMerge merges pre-sorted input files using a heap-based k-way merge.
func runMerge(cfg config) int {
	scanners, closers, err := openMergeSources(cfg.files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	defer closeMergeSources(closers)
	h := initMergeHeap(scanners, cfg)
	return writeMergeOutput(h, cfg)
}

// openMergeSources opens all input files for merge.
func openMergeSources(files []string) ([]*bufio.Scanner, []io.Closer, error) {
	if len(files) == 0 {
		s := bufio.NewScanner(os.Stdin)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		return []*bufio.Scanner{s}, nil, nil
	}
	var scanners []*bufio.Scanner
	var closers []io.Closer
	for _, name := range files {
		s, c, err := openMergeFile(name)
		if err != nil {
			closeMergeSources(closers)
			return nil, nil, err
		}
		scanners = append(scanners, s)
		if c != nil {
			closers = append(closers, c)
		}
	}
	return scanners, closers, nil
}

// openMergeFile opens a single file for merge scanning.
func openMergeFile(name string) (*bufio.Scanner, io.Closer, error) {
	if name == "-" {
		s := bufio.NewScanner(os.Stdin)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		return s, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("open failed: %s: %w", name, err)
	}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	return s, f, nil
}

// closeMergeSources closes all opened files.
func closeMergeSources(closers []io.Closer) {
	for _, c := range closers {
		c.Close() // best-effort close
	}
}

// initMergeHeap creates and initializes the merge heap.
func initMergeHeap(scanners []*bufio.Scanner, cfg config) *mergeHeap {
	cmp := makeCompare(cfg)
	h := &mergeHeap{less: cmp}
	heap.Init(h)
	for i, s := range scanners {
		if s.Scan() {
			line := make([]byte, len(s.Bytes()))
			copy(line, s.Bytes())
			heap.Push(h, mergeEntry{line: line, scanner: s, index: i})
		}
	}
	return h
}

// writeMergeOutput drains the heap and writes merged lines.
func writeMergeOutput(h *mergeHeap, cfg config) int {
	w, closer, err := openOutput(cfg.outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	bw := bufio.NewWriter(w)
	code := drainMergeHeap(h, bw, cfg)
	if code != 0 {
		return code
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
		return 2
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "sort: close error: %v\n", err)
			return 2
		}
	}
	return 0
}

// drainMergeHeap pops entries from the heap and writes them.
func drainMergeHeap(h *mergeHeap, bw *bufio.Writer, cfg config) int {
	var prev []byte
	eq := makeEqual(cfg)
	for h.Len() > 0 {
		entry := heap.Pop(h).(mergeEntry)
		if cfg.unique && prev != nil && eq(prev, entry.line) {
			advanceMergeScanner(h, entry)
			continue
		}
		if err := writeMergeLine(bw, entry.line); err != nil {
			return 2
		}
		prev = entry.line
		advanceMergeScanner(h, entry)
	}
	return 0
}

// writeMergeLine writes a single line with newline.
func writeMergeLine(bw *bufio.Writer, line []byte) error {
	if _, err := bw.Write(line); err != nil {
		fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
		return err
	}
	if err := bw.WriteByte('\n'); err != nil {
		fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
		return err
	}
	return nil
}

// advanceMergeScanner reads the next line from the entry's source.
func advanceMergeScanner(h *mergeHeap, entry mergeEntry) {
	if entry.scanner.Scan() {
		line := make([]byte, len(entry.scanner.Bytes()))
		copy(line, entry.scanner.Bytes())
		heap.Push(h, mergeEntry{
			line:    line,
			scanner: entry.scanner,
			index:   entry.index,
		})
	}
}
