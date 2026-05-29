// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd110-pr R1.1, R2.1, R2.2, R2.3.
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
	if opts.omitHeader || opts.omitPagination {
		return paginateNoHeader(scanner, w)
	}
	date := modTime.Format("2006-01-02 15:04")
	body := max(opts.pageLength-headerSize-footerSize, 1)
	pageNum := 1
	for {
		lines, eof := readBody(scanner, body)
		if len(lines) == 0 && eof {
			break
		}
		if err := writeHeader(w, date, filename, pageNum); err != nil {
			return err
		}
		if err := writeBody(w, lines, body); err != nil {
			return err
		}
		if err := writeFooter(w); err != nil {
			return err
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
