// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd110-pr R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R4.3, R5.1, R5.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	headerSize     = 5
	footerSize     = 5
	defaultPageLen = 66
	defaultWidth   = 72
)

type options struct {
	pageLength     int
	headerText     string
	headerSet      bool
	omitHeader     bool
	omitPagination bool
	columns        int
	across         bool
	numberLines    bool
	numberSep      byte
	numberWidth    int
	indent         int
	pageWidth      int
	doubleSpace    bool
	separator      string
	separatorSet   bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pr: %s\n", err)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, f := range files {
		if err := processFile(f, opts, w); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "pr: %s\n", err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "pr: %s\n", err)
		exitCode = 1
	}
	return exitCode
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{
		pageLength:  defaultPageLen,
		pageWidth:   defaultWidth,
		numberSep:   '\t',
		numberWidth: 5,
	}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		var err error
		switch {
		case strings.HasPrefix(a, "--length="):
			err = setPageLen(&opts, a[len("--length="):])
		case strings.HasPrefix(a, "--header="):
			opts.headerText, opts.headerSet = a[len("--header="):], true
		case a == "--omit-header" || a == "-t":
			opts.omitHeader = true
		case a == "--omit-pagination" || a == "-T":
			opts.omitPagination = true
		case strings.HasPrefix(a, "-l"):
			err = parseLenFlag(args, &i, &opts)
		case strings.HasPrefix(a, "-h"):
			err = parseHdrFlag(args, &i, &opts)
		case a == "--across" || a == "-a":
			opts.across = true
		case a == "--double-space" || a == "-d":
			opts.doubleSpace = true
		case strings.HasPrefix(a, "--columns="):
			err = setColumns(&opts, a[len("--columns="):])
		case strings.HasPrefix(a, "--number-lines"):
			err = parseNumLong(&opts, a)
		case strings.HasPrefix(a, "-n"):
			parseNumShort(&opts, a)
		case strings.HasPrefix(a, "--indent="):
			err = setIndent(&opts, a[len("--indent="):])
		case strings.HasPrefix(a, "-o"):
			err = parseIndentFlag(args, &i, &opts)
		case strings.HasPrefix(a, "--width="):
			err = setWidth(&opts, a[len("--width="):])
		case strings.HasPrefix(a, "-w"):
			err = parseWidthFlag(args, &i, &opts)
		case strings.HasPrefix(a, "--separator="):
			opts.separator, opts.separatorSet = a[len("--separator="):], true
		case a == "--separator" || a == "-s":
			opts.separator, opts.separatorSet = "\t", true
		case strings.HasPrefix(a, "-s"):
			opts.separator, opts.separatorSet = string(a[2]), true
		case isColumnsFlag(a):
			err = setColumns(&opts, a[1:])
		case strings.HasPrefix(a, "-") && a != "-":
			err = fmt.Errorf("unrecognized option '%s'", a)
		default:
			files = append(files, a)
		}
		if err != nil {
			return opts, nil, err
		}
	}
	if opts.pageLength <= 10 {
		opts.omitHeader = true
	}
	return opts, files, nil
}

func parseLenFlag(args []string, i *int, opts *options) error {
	v, err := valFlag(args, i, "-l")
	if err != nil {
		return err
	}
	return setPageLen(opts, v)
}

func parseHdrFlag(args []string, i *int, opts *options) error {
	v, err := valFlag(args, i, "-h")
	if err != nil {
		return err
	}
	opts.headerText, opts.headerSet = v, true
	return nil
}

func parseIndentFlag(args []string, i *int, opts *options) error {
	v, err := valFlag(args, i, "-o")
	if err != nil {
		return err
	}
	return setIndent(opts, v)
}

func parseWidthFlag(args []string, i *int, opts *options) error {
	v, err := valFlag(args, i, "-w")
	if err != nil {
		return err
	}
	return setWidth(opts, v)
}

func parseNumShort(opts *options, a string) {
	opts.numberLines = true
	rest := a[2:]
	if len(rest) == 0 {
		return
	}
	if rest[0] < '0' || rest[0] > '9' {
		opts.numberSep = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 0 {
		if w, err := strconv.Atoi(rest); err == nil && w >= 1 {
			opts.numberWidth = w
		}
	}
}

func parseNumLong(opts *options, a string) error {
	opts.numberLines = true
	if a == "--number-lines" {
		return nil
	}
	if !strings.HasPrefix(a, "--number-lines=") {
		return fmt.Errorf("unrecognized option '%s'", a)
	}
	rest := a[len("--number-lines="):]
	if len(rest) == 0 {
		return nil
	}
	if rest[0] < '0' || rest[0] > '9' {
		opts.numberSep = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 0 {
		w, err := strconv.Atoi(rest)
		if err != nil || w < 1 {
			return fmt.Errorf("invalid number width: %q", rest)
		}
		opts.numberWidth = w
	}
	return nil
}

func valFlag(args []string, i *int, prefix string) (string, error) {
	a := args[*i]
	if len(a) > len(prefix) {
		return a[len(prefix):], nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option requires an argument -- '%c'", prefix[1])
	}
	return args[*i], nil
}

func setPageLen(opts *options, s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fmt.Errorf("invalid page length: %q", s)
	}
	opts.pageLength = n
	return nil
}

func setColumns(opts *options, s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fmt.Errorf("invalid number of columns: %q", s)
	}
	opts.columns = n
	return nil
}

func setIndent(opts *options, s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid indent: %q", s)
	}
	opts.indent = n
	return nil
}

func setWidth(opts *options, s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fmt.Errorf("invalid page width: %q", s)
	}
	opts.pageWidth = n
	return nil
}

func isColumnsFlag(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func processFile(path string, opts options, w *bufio.Writer) error {
	var r io.Reader
	var filename string
	var modTime time.Time
	if path == "-" {
		r = os.Stdin
		filename = ""
		modTime = time.Now()
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		r = f
		filename = path
		modTime = info.ModTime()
	}
	if opts.headerSet {
		filename = opts.headerText
	}
	return paginate(r, filename, modTime, opts, w)
}

func paginate(r io.Reader, filename string, modTime time.Time, opts options, w *bufio.Writer) error {
	scanner := bufio.NewScanner(r)
	cols := max(opts.columns, 1)
	noHdr := opts.omitHeader || opts.omitPagination
	if noHdr && cols <= 1 {
		return paginateNoHeader(scanner, w, opts)
	}
	body := max(opts.pageLength-headerSize-footerSize, 1)
	if noHdr {
		body = opts.pageLength
	}
	bodyLines := body
	if opts.doubleSpace {
		bodyLines = max(body/2, 1)
	}
	date := modTime.Format("2006-01-02 15:04")
	pageNum := 1
	lineNum := 1
	for {
		lines, eof := readBody(scanner, bodyLines*cols)
		if len(lines) == 0 && eof {
			break
		}
		if !noHdr {
			if err := writeHeader(w, date, filename, pageNum, opts); err != nil {
				return err
			}
		}
		if cols > 1 {
			if err := writeColumns(w, lines, body, cols, opts, !noHdr, lineNum); err != nil {
				return err
			}
		} else if err := writeBody(w, lines, body, opts, lineNum); err != nil {
			return err
		}
		if !noHdr {
			if err := writeFooter(w); err != nil {
				return err
			}
		}
		lineNum += len(lines)
		pageNum++
		if eof {
			break
		}
	}
	return scanner.Err()
}

func paginateNoHeader(scanner *bufio.Scanner, w *bufio.Writer, opts options) error {
	ind := strings.Repeat(" ", opts.indent)
	lineNum := 1
	for scanner.Scan() {
		prefix := ""
		if opts.numberLines {
			prefix = fmtNum(lineNum, opts.numberSep, opts.numberWidth)
		}
		if _, err := fmt.Fprintf(w, "%s%s%s\n", ind, prefix, scanner.Text()); err != nil {
			return err
		}
		if opts.doubleSpace {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
		}
		lineNum++
	}
	return scanner.Err()
}

func readBody(scanner *bufio.Scanner, n int) ([]string, bool) {
	var lines []string
	for range n {
		if !scanner.Scan() {
			return lines, true
		}
		lines = append(lines, scanner.Text())
	}
	return lines, false
}

func writeHeader(w *bufio.Writer, date, filename string, pageNum int, opts options) error {
	if _, err := fmt.Fprint(w, "\n\n"); err != nil {
		return err
	}
	right := fmt.Sprintf("Page %d", pageNum)
	avail := opts.pageWidth - len(date) - len(right)
	lpad := avail
	rpad := 0
	if len(filename) < avail {
		lpad = (avail - len(filename)) / 2
		rpad = avail - len(filename) - lpad
	}
	ind := strings.Repeat(" ", opts.indent)
	line := date + strings.Repeat(" ", lpad) + filename + strings.Repeat(" ", rpad) + right
	if _, err := fmt.Fprintf(w, "%s%s\n", ind, line); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, "\n\n")
	return err
}

func writeBody(w *bufio.Writer, lines []string, bodySize int, opts options, lineNum int) error {
	ind := strings.Repeat(" ", opts.indent)
	outputLines := 0
	for i, line := range lines {
		prefix := ""
		if opts.numberLines {
			prefix = fmtNum(lineNum+i, opts.numberSep, opts.numberWidth)
		}
		if _, err := fmt.Fprintf(w, "%s%s%s\n", ind, prefix, line); err != nil {
			return err
		}
		outputLines++
		if opts.doubleSpace {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
			outputLines++
		}
	}
	for range bodySize - outputLines {
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeFooter(w *bufio.Writer) error {
	for range footerSize {
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeColumns(w *bufio.Writer, lines []string, bodySize, cols int, opts options, pad bool, lineNum int) error {
	colWidth := opts.pageWidth / cols
	n := len(lines)
	rows := (n + cols - 1) / cols
	fullCols := n - (rows-1)*cols
	ind := strings.Repeat(" ", opts.indent)
	outputRows := 0
	for r := range rows {
		if _, err := fmt.Fprint(w, ind); err != nil {
			return err
		}
		last := lastDataCol(r, cols, rows, fullCols, n, opts.across)
		for c := 0; c <= last; c++ {
			idx := colIdx(r, c, cols, rows, fullCols, opts.across)
			cell := fmtCell(lines[idx], idx, c, last, colWidth, lineNum, opts)
			if _, err := fmt.Fprint(w, cell); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
		outputRows++
		if opts.doubleSpace {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
			outputRows++
		}
	}
	if pad {
		for range bodySize - outputRows {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func fmtCell(text string, idx, c, last, colWidth, lineNum int, opts options) string {
	prefix := ""
	if opts.numberLines {
		prefix = fmtNum(lineNum+idx, opts.numberSep, opts.numberWidth)
	}
	cell := prefix + text
	if c < last {
		if opts.separatorSet {
			return cell + opts.separator
		}
		return padToCol(cell, opts.indent+c*colWidth, opts.indent+(c+1)*colWidth)
	}
	return cell
}

func fmtNum(n int, sep byte, width int) string {
	s := strconv.Itoa(n)
	if len(s) < width {
		s = strings.Repeat(" ", width-len(s)) + s
	}
	return s + string(sep)
}

func lastDataCol(r, cols, rows, fullCols, n int, across bool) int {
	if across {
		if remaining := n - r*cols; remaining >= cols {
			return cols - 1
		} else {
			return remaining - 1
		}
	}
	if r < rows-1 || fullCols >= cols {
		return cols - 1
	}
	return fullCols - 1
}

func colIdx(r, c, cols, rows, fullCols int, across bool) int {
	if across {
		return r*cols + c
	}
	return colDownIdx(r, c, rows, fullCols)
}

func colDownIdx(r, c, rows, fullCols int) int {
	if c < fullCols {
		return c*rows + r
	}
	return fullCols*rows + (c-fullCols)*(rows-1) + r
}

func padToCol(s string, absStart, absTarget int) string {
	if absStart+len(s) >= absTarget {
		return s[:absTarget-absStart]
	}
	var buf strings.Builder
	buf.WriteString(s)
	pos := absStart + len(s)
	for pos < absTarget {
		nextTab := (pos/8 + 1) * 8
		if nextTab <= absTarget {
			buf.WriteByte('\t')
			pos = nextTab
		} else {
			buf.WriteString(strings.Repeat(" ", absTarget-pos))
			pos = absTarget
		}
	}
	return buf.String()
}
