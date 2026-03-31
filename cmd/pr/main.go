// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pr implements GNU pr: paginate or columnate files for printing.
//
// Implements prd110-pr R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R4.3, R5.1, R5.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultPageLength  = 66
	headerLineCount    = 5
	footerLineCount    = 5
	defaultPageWidth   = 72
	defaultNumberWidth = 5
	defaultNumberSep   = '\t'
	dateFormat         = "2006-01-02 15:04"
	tabWidth           = 8
)

// prOptions holds parsed flag state.
type prOptions struct {
	pageLength      int
	pageWidth       int
	columns         int
	across          bool
	separator       string
	hasSeparator    bool
	customHeader    string
	hasCustomHeader bool
	omitHeader      bool // -t: suppress header and footer
	omitPagination  bool // -T: suppress header/footer and page padding
	numberLines     bool // -n: number output lines
	numberSep       byte // separator char for numbering
	numberWidth     int  // width of the number field
	doubleSpace     bool // -d: double-space output
	margin          int  // -o: indent each line by margin spaces
	formFeed        bool // -f/-F: use form feeds between pages
	suppressErrors  bool // -r: suppress file-open errors
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and paginates each file to stdout.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "pr: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		if err := processFile(name, stdin, stdout, opts); err != nil {
			if isBrokenPipe(err) {
				return 0
			}
			if !opts.suppressErrors {
				fmt.Fprintf(stderr, "pr: %v\n", err)
			}
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file and paginates it to stdout.
func processFile(
	name string, stdin io.Reader, stdout io.Writer, opts prOptions,
) error {
	r, closer, dateStr, err := openFileForPr(name, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	headerText := resolveHeaderText(name, opts)
	return paginate(r, stdout, opts, dateStr, headerText)
}

// resolveHeaderText determines the header text for a file.
func resolveHeaderText(name string, opts prOptions) string {
	if opts.hasCustomHeader {
		return opts.customHeader
	}
	if name == "-" || name == "" {
		return ""
	}
	return name
}

// openFileForPr opens the input and returns a reader, closer, and date string.
func openFileForPr(
	name string, stdin io.Reader,
) (io.Reader, io.Closer, string, error) {
	if name == "-" {
		return stdin, nil, time.Now().Format(dateFormat), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, "", err
	}
	info, statErr := f.Stat()
	dateStr := time.Now().Format(dateFormat)
	if statErr == nil {
		dateStr = info.ModTime().Format(dateFormat)
	}
	return f, f, dateStr, nil
}

// bodyLineCount calculates the number of body lines per page.
func bodyLineCount(opts prOptions) int {
	if opts.omitHeader {
		return opts.pageLength
	}
	return max(opts.pageLength-headerLineCount-footerLineCount, 1)
}

// paginate reads input and writes paginated output.
func paginate(
	r io.Reader, w io.Writer, opts prOptions,
	dateStr, headerText string,
) error {
	scanner := bufio.NewScanner(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	bodyLines := bodyLineCount(opts)
	effectiveBody := bodyLines
	if opts.doubleSpace {
		effectiveBody = (bodyLines + 1) / 2
	}
	linesPerPage := effectiveBody * opts.columns
	pageNum := 1

	for {
		lines, more := readBodyLines(scanner, linesPerPage)
		if len(lines) == 0 {
			break
		}
		formatted := formatPage(lines, effectiveBody, opts)
		if err := writePage(bw, opts, dateStr, headerText, pageNum, formatted, bodyLines); err != nil {
			return err
		}
		pageNum++
		if !more {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// readBodyLines reads up to n lines from the scanner.
func readBodyLines(scanner *bufio.Scanner, n int) ([]string, bool) {
	var lines []string
	for len(lines) < n {
		if !scanner.Scan() {
			return lines, false
		}
		lines = append(lines, scanner.Text())
	}
	return lines, true
}

// formatPage arranges input lines into formatted output lines.
func formatPage(lines []string, bodyPerCol int, opts prOptions) []string {
	if opts.columns <= 1 {
		return lines
	}
	if opts.across {
		return formatAcross(lines, opts)
	}
	return formatDown(lines, bodyPerCol, opts)
}

// writePage outputs a single page with optional header, body, and footer.
func writePage(
	bw *bufio.Writer, opts prOptions, dateStr, headerText string,
	pageNum int, lines []string, bodyLines int,
) error {
	if opts.omitPagination {
		return writeLines(bw, lines, opts)
	}
	if !opts.omitHeader {
		writeHeader(bw, dateStr, headerText, pageNum, opts.pageWidth, opts.margin)
	}
	linesWritten := writeBodyLines(bw, lines, opts)
	if !opts.omitHeader {
		writePadAndFooter(bw, bodyLines-linesWritten, opts.formFeed)
	}
	return nil
}

// writeLines writes lines with optional margin, numbering, and double-spacing.
func writeLines(bw *bufio.Writer, lines []string, opts prOptions) error {
	for i, line := range lines {
		writeSingleLine(bw, line, i+1, opts)
		if opts.doubleSpace {
			writeMargin(bw, opts.margin)
			bw.WriteByte('\n') //nolint:errcheck
		}
	}
	return nil
}

// writeBodyLines writes body lines and returns the count of output lines used.
func writeBodyLines(bw *bufio.Writer, lines []string, opts prOptions) int {
	count := 0
	for i, line := range lines {
		writeSingleLine(bw, line, i+1, opts)
		count++
		if opts.doubleSpace {
			writeMargin(bw, opts.margin)
			bw.WriteByte('\n') //nolint:errcheck
			count++
		}
	}
	return count
}

// writeSingleLine writes one line with margin and optional numbering.
func writeSingleLine(bw *bufio.Writer, line string, lineNum int, opts prOptions) {
	writeMargin(bw, opts.margin)
	if opts.numberLines && opts.columns <= 1 {
		writeLineNumber(bw, lineNum, opts.numberWidth, opts.numberSep)
	}
	bw.WriteString(line) //nolint:errcheck
	bw.WriteByte('\n')   //nolint:errcheck
}

// writeMargin writes margin spaces.
func writeMargin(bw *bufio.Writer, margin int) {
	for range margin {
		bw.WriteByte(' ') //nolint:errcheck
	}
}

// writeLineNumber writes a right-justified line number followed by the separator.
func writeLineNumber(bw *bufio.Writer, num, width int, sep byte) {
	s := strconv.Itoa(num)
	for range width - len(s) {
		bw.WriteByte(' ') //nolint:errcheck
	}
	bw.WriteString(s) //nolint:errcheck
	bw.WriteByte(sep)  //nolint:errcheck
}

// writeHeader outputs the 5-line page header.
func writeHeader(
	bw *bufio.Writer, dateStr, headerText string, pageNum, pageWidth, margin int,
) {
	writeMargin(bw, margin)
	bw.WriteByte('\n') //nolint:errcheck
	writeMargin(bw, margin)
	bw.WriteByte('\n') //nolint:errcheck

	writeMargin(bw, margin)
	bw.WriteString(formatHeaderLine(dateStr, headerText, pageNum, pageWidth)) //nolint:errcheck
	bw.WriteByte('\n') //nolint:errcheck

	writeMargin(bw, margin)
	bw.WriteByte('\n') //nolint:errcheck
	writeMargin(bw, margin)
	bw.WriteByte('\n') //nolint:errcheck
}

// writePadAndFooter writes padding lines and footer.
func writePadAndFooter(bw *bufio.Writer, padCount int, formFeed bool) {
	if formFeed {
		bw.WriteByte('\f') //nolint:errcheck
		return
	}
	for range padCount {
		bw.WriteByte('\n') //nolint:errcheck
	}
	for range footerLineCount {
		bw.WriteByte('\n') //nolint:errcheck
	}
}

// isBrokenPipe reports whether an error is caused by writing to a broken pipe.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
