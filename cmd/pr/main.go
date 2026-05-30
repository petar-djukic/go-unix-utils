// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd110-pr R1.1, R2.1, R2.2, R2.3, R3.1.
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
	opts := options{pageLength: defaultPageLen}
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
		case strings.HasPrefix(a, "--columns="):
			err = setColumns(&opts, a[len("--columns="):])
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
		return paginateNoHeader(scanner, w)
	}
	body := max(opts.pageLength-headerSize-footerSize, 1)
	if noHdr {
		body = opts.pageLength
	}
	date := modTime.Format("2006-01-02 15:04")
	pageNum := 1
	for {
		lines, eof := readBody(scanner, body*cols)
		if len(lines) == 0 && eof {
			break
		}
		if !noHdr {
			if err := writeHeader(w, date, filename, pageNum); err != nil {
				return err
			}
		}
		if cols > 1 {
			if err := writeColumns(w, lines, body, cols, opts.across, !noHdr); err != nil {
				return err
			}
		} else if err := writeBody(w, lines, body); err != nil {
			return err
		}
		if !noHdr {
			if err := writeFooter(w); err != nil {
				return err
			}
		}
		pageNum++
		if eof {
			break
		}
	}
	return scanner.Err()
}

func paginateNoHeader(scanner *bufio.Scanner, w *bufio.Writer) error {
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "%s\n", scanner.Text()); err != nil {
			return err
		}
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

func writeHeader(w *bufio.Writer, date, filename string, pageNum int) error {
	if _, err := fmt.Fprint(w, "\n\n"); err != nil {
		return err
	}
	right := fmt.Sprintf("Page %d", pageNum)
	avail := defaultWidth - len(date) - len(right)
	lpad := avail
	rpad := 0
	if len(filename) < avail {
		lpad = (avail - len(filename)) / 2
		rpad = avail - len(filename) - lpad
	}
	line := date + strings.Repeat(" ", lpad) + filename + strings.Repeat(" ", rpad) + right
	if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, "\n\n")
	return err
}

func writeBody(w *bufio.Writer, lines []string, bodySize int) error {
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return err
		}
	}
	for range bodySize - len(lines) {
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

func writeColumns(w *bufio.Writer, lines []string, bodySize, cols int, across, pad bool) error {
	colWidth := defaultWidth / cols
	n := len(lines)
	rows := (n + cols - 1) / cols
	fullCols := n - (rows-1)*cols
	for r := range rows {
		last := lastDataCol(r, cols, rows, fullCols, n, across)
		for c := 0; c <= last; c++ {
			text := ""
			if across {
				text = lines[r*cols+c]
			} else {
				text = lines[colDownIdx(r, c, rows, fullCols)]
			}
			if c < last {
				text = padToCol(text, c*colWidth, (c+1)*colWidth)
			}
			if _, err := fmt.Fprint(w, text); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	if pad {
		for range bodySize - rows {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
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
