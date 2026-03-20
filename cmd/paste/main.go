// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd027-paste R1.1–R1.4.
// R1.1: Open all files and merge lines side by side with tab delimiter.
// R1.2: Unequal file lengths produce empty fields for exhausted files.
// R1.3: "-" refers to stdin; multiple "-" read stdin sequentially.
// R1.4: No files reads from stdin (passthrough).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultDelimiter = "\t"

// lineReader reads lines one at a time from an io.Reader.
type lineReader struct {
	br   *bufio.Reader
	done bool
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{br: bufio.NewReader(r)}
}

// readLine returns the next line without its trailing newline and true,
// or ("", false) when the underlying reader is exhausted.
func (lr *lineReader) readLine() (string, bool) {
	if lr.done {
		return "", false
	}
	line, err := lr.br.ReadString('\n')
	if err != nil {
		lr.done = true
		if len(line) > 0 {
			return strings.TrimSuffix(line, "\n"), true
		}
		return "", false
	}
	return strings.TrimSuffix(line, "\n"), true
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run dispatches to passthrough or parallel merge based on arguments.
func run(args []string) int {
	w := bufio.NewWriter(os.Stdout)
	var exitCode int
	if len(args) == 0 {
		exitCode = pastePassthrough(w, os.Stdin)
	} else {
		exitCode = pasteParallel(w, args)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// pastePassthrough reads from stdin and writes each line to stdout. R1.4.
func pastePassthrough(w *bufio.Writer, r io.Reader) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		w.WriteString(scanner.Text())
		w.WriteByte('\n')
	}
	return 0
}

// pasteParallel opens all files and merges lines side by side. R1.1–R1.3.
func pasteParallel(w *bufio.Writer, files []string) int {
	readers, closers, err := openLineReaders(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		return 1
	}
	defer closeAll(closers)
	mergeLines(w, readers)
	return 0
}

// openLineReaders opens all files and returns line readers. R1.1, R1.3.
// Multiple "-" entries share a single reader for stdin.
func openLineReaders(files []string) ([]*lineReader, []io.Closer, error) {
	var readers []*lineReader
	var closers []io.Closer
	var stdinReader *lineReader
	for _, name := range files {
		if name == "-" {
			if stdinReader == nil {
				stdinReader = newLineReader(os.Stdin)
			}
			readers = append(readers, stdinReader)
			closers = append(closers, nil)
			continue
		}
		r, c, err := openFileReader(name)
		if err != nil {
			closeAll(closers)
			return nil, nil, err
		}
		readers = append(readers, r)
		closers = append(closers, c)
	}
	return readers, closers, nil
}

// openFileReader opens a single file and returns a line reader and closer.
func openFileReader(name string) (*lineReader, io.Closer, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	return newLineReader(f), f, nil
}

// mergeLines reads one line from each reader per iteration and writes
// merged output until all readers are exhausted. R1.1, R1.2.
func mergeLines(w *bufio.Writer, readers []*lineReader) {
	for {
		lines := make([]string, len(readers))
		anyActive := false
		for i, r := range readers {
			if line, ok := r.readLine(); ok {
				lines[i] = line
				anyActive = true
			}
		}
		if !anyActive {
			break
		}
		writeMergedLine(w, lines)
	}
}

// writeMergedLine writes fields separated by the delimiter, followed by newline.
func writeMergedLine(w *bufio.Writer, lines []string) {
	for i, line := range lines {
		if i > 0 {
			w.WriteString(defaultDelimiter)
		}
		w.WriteString(line)
	}
	w.WriteByte('\n')
}

// closeAll closes all non-nil closers. Best-effort cleanup.
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		if c != nil {
			c.Close() // best-effort cleanup, error ignored
		}
	}
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
