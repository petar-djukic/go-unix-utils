// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd027-paste R1.1–R1.4, R2.1–R2.3, R3.1–R3.3.
// R1.1: Open all files and merge lines side by side with tab delimiter.
// R1.2: Unequal file lengths produce empty fields for exhausted files.
// R1.3: "-" refers to stdin; multiple "-" read stdin sequentially.
// R1.4: No files reads from stdin (passthrough).
// R2.1: -d DELIM configures the separator; delimiter list cycles across fields.
// R2.2: Escape sequences \n, \t, \\, \0 are recognized in DELIM.
// R2.3: Delimiter cycling resets from the first delimiter for each new output line.
// R3.1: -s processes files one at a time, joining all lines with delimiter.
// R3.2: Delimiter list cycles across fields within each serial output line.
// R3.3: -s overrides parallel mode.
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

// run parses flags and dispatches to passthrough, serial, or parallel merge.
func run(args []string) int {
	delims, serial, remaining := parseArgs(args)
	w := bufio.NewWriter(os.Stdout)
	var exitCode int
	if serial {
		exitCode = pasteSerial(w, remaining, delims)
	} else if len(remaining) == 0 {
		exitCode = pastePassthrough(w, os.Stdin)
	} else {
		exitCode = pasteParallel(w, remaining, delims)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// parseArgs extracts -d and -s flags, returns delimiters, serial flag, and remaining args.
func parseArgs(args []string) ([]string, bool, []string) {
	delims := []string{defaultDelimiter}
	serial := false
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			remaining = append(remaining, args[i+1:]...)
			break
		}
		if args[i] == "-s" {
			serial = true
			continue
		}
		if args[i] == "-d" && i+1 < len(args) {
			i++
			delims = parseDelimList(args[i])
			continue
		}
		if strings.HasPrefix(args[i], "-d") && len(args[i]) > 2 {
			delims = parseDelimList(args[i][2:])
			continue
		}
		// Handle combined flags like -sd or -ds
		if handleCombinedFlags(args[i], &delims, &serial, args, &i) {
			continue
		}
		remaining = append(remaining, args[i])
	}
	return delims, serial, remaining
}

// handleCombinedFlags handles flags like -sd<delim> or -ds<delim>.
// Returns true if the argument was consumed as a combined flag.
func handleCombinedFlags(arg string, delims *[]string, serial *bool, args []string, i *int) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	rest := arg[1:]
	if rest == "sd" && *i+1 < len(args) {
		*serial = true
		*i++
		*delims = parseDelimList(args[*i])
		return true
	}
	if strings.HasPrefix(rest, "sd") && len(rest) > 2 {
		*serial = true
		*delims = parseDelimList(rest[2:])
		return true
	}
	return false
}

// parseDelimList parses DELIM into a list of delimiter strings. R2.2.
// Recognizes \n, \t, \\, and \0 (empty string).
func parseDelimList(s string) []string {
	var delims []string
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			delims = append(delims, parseEscape(s[i+1]))
			i++
			continue
		}
		delims = append(delims, string(s[i]))
	}
	if len(delims) == 0 {
		return []string{""}
	}
	return delims
}

// parseEscape converts an escape character to its string value. R2.2.
func parseEscape(c byte) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case '\\':
		return "\\"
	case '0':
		return ""
	default:
		return string(c)
	}
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

// pasteSerial processes files one at a time, joining all lines of each file
// with cycling delimiters into a single output line. R3.1, R3.2, R3.3.
func pasteSerial(w *bufio.Writer, files []string, delims []string) int {
	if len(files) == 0 {
		return pasteSerialReader(w, os.Stdin, delims)
	}
	for _, name := range files {
		var code int
		if name == "-" {
			code = pasteSerialReader(w, os.Stdin, delims)
		} else {
			code = pasteSerialFile(w, name, delims)
		}
		if code != 0 {
			return code
		}
	}
	return 0
}

// pasteSerialFile opens a file and joins all its lines on one output line.
func pasteSerialFile(w *bufio.Writer, name string, delims []string) int {
	f, err := os.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %s: %s\n", name, osErrorMessage(err))
		return 1
	}
	defer f.Close() // best-effort cleanup, error ignored
	return pasteSerialReader(w, f, delims)
}

// pasteSerialReader joins all lines from a reader into one output line. R3.2.
func pasteSerialReader(w *bufio.Writer, r io.Reader, delims []string) int {
	lr := newLineReader(r)
	fieldIdx := 0
	for {
		line, ok := lr.readLine()
		if !ok {
			break
		}
		if fieldIdx > 0 {
			w.WriteString(delims[(fieldIdx-1)%len(delims)])
		}
		w.WriteString(line)
		fieldIdx++
	}
	w.WriteByte('\n')
	return 0
}

// pasteParallel opens all files and merges lines side by side. R1.1–R1.3.
func pasteParallel(w *bufio.Writer, files []string, delims []string) int {
	readers, closers, err := openLineReaders(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		return 1
	}
	defer closeAll(closers)
	mergeLines(w, readers, delims)
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
// merged output until all readers are exhausted. R1.1, R1.2, R2.3.
func mergeLines(w *bufio.Writer, readers []*lineReader, delims []string) {
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
		writeMergedLine(w, lines, delims)
	}
}

// writeMergedLine writes fields separated by cycling delimiters. R2.1, R2.3.
func writeMergedLine(w *bufio.Writer, lines []string, delims []string) {
	for i, line := range lines {
		if i > 0 {
			w.WriteString(delims[(i-1)%len(delims)])
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
