// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pr implements GNU pr: paginate or columnate files for printing.
//
// Implements prd110-pr R1.1, R2.1, R2.2, R2.3.
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
	defaultPageLength = 66
	headerLineCount   = 5
	footerLineCount   = 5
	defaultPageWidth  = 72
	dateFormat        = "2006-01-02 15:04"
)

// prOptions holds parsed flag state.
type prOptions struct {
	pageLength     int
	pageWidth      int
	customHeader   string
	hasCustomHeader bool
	omitHeader     bool // -t: suppress header and footer
	omitPagination bool // -T: suppress header/footer and page padding
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
			fmt.Fprintf(stderr, "pr: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (prOptions, []string, error) {
	opts := prOptions{
		pageLength: defaultPageLength,
		pageWidth:  defaultPageWidth,
	}
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		consumed, err := parseLongFlag(&opts, args, i)
		if err != nil {
			return opts, nil, err
		}
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		consumed, err = parseShortFlags(&opts, args, i)
		if err != nil {
			return opts, nil, err
		}
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		files = append(files, arg)
	}

	return opts, files, nil
}

// parseLongFlag handles --length=N, --header=H, --omit-header, --omit-pagination.
// Returns number of args consumed (0 if not a long flag).
func parseLongFlag(opts *prOptions, args []string, i int) (int, error) {
	arg := args[i]
	if !strings.HasPrefix(arg, "--") {
		return 0, nil
	}

	if val, ok := cutValue(arg, "--length="); ok {
		return parseLengthValue(opts, val)
	}
	if val, ok := cutValue(arg, "--header="); ok {
		opts.customHeader = val
		opts.hasCustomHeader = true
		return 1, nil
	}
	if arg == "--omit-header" {
		opts.omitHeader = true
		return 1, nil
	}
	if arg == "--omit-pagination" {
		opts.omitPagination = true
		opts.omitHeader = true
		return 1, nil
	}

	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// cutValue extracts the value from a --key=value argument.
func cutValue(arg, prefix string) (string, bool) {
	if strings.HasPrefix(arg, prefix) {
		return arg[len(prefix):], true
	}
	return "", false
}

// parseLengthValue parses and validates the page length value.
func parseLengthValue(opts *prOptions, val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid page length: '%s'", val)
	}
	opts.pageLength = n
	return 1, nil
}

// parseShortFlags handles -l, -h, -t, -T and their argument consumption.
// Returns number of args consumed (0 if not a recognized flag).
func parseShortFlags(opts *prOptions, args []string, i int) (int, error) {
	arg := args[i]
	if len(arg) < 2 || arg[0] != '-' {
		return 0, nil
	}

	chars := arg[1:]
	for j := 0; j < len(chars); j++ {
		ch := chars[j]
		switch ch {
		case 't':
			opts.omitHeader = true
		case 'T':
			opts.omitPagination = true
			opts.omitHeader = true
		case 'l':
			return parseShortWithArg(opts, chars[j+1:], args, i, setPageLength)
		case 'h':
			return parseShortWithArg(opts, chars[j+1:], args, i, setHeader)
		default:
			return 0, fmt.Errorf("unrecognized option '-%c'", ch)
		}
	}
	return 1, nil
}

type optSetter func(opts *prOptions, val string) error

// setPageLength sets the page length from a string value.
func setPageLength(opts *prOptions, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid page length: '%s'", val)
	}
	opts.pageLength = n
	return nil
}

// setHeader sets the custom header text.
func setHeader(opts *prOptions, val string) error {
	opts.customHeader = val
	opts.hasCustomHeader = true
	return nil
}

// parseShortWithArg handles a short flag that takes an argument.
// The value can be the remainder of the current arg or the next arg.
func parseShortWithArg(
	opts *prOptions, rest string, args []string, i int, setter optSetter,
) (int, error) {
	if rest != "" {
		if err := setter(opts, rest); err != nil {
			return 0, err
		}
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument")
	}
	if err := setter(opts, args[i+1]); err != nil {
		return 0, err
	}
	return 2, nil
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

	headerText := name
	if name == "-" || name == "" {
		headerText = ""
	}
	if opts.hasCustomHeader {
		headerText = opts.customHeader
	}

	return paginate(r, stdout, opts, dateStr, headerText)
}

// openFileForPr opens the input and returns a reader, closer, and date string.
func openFileForPr(
	name string, stdin io.Reader,
) (io.Reader, io.Closer, string, error) {
	if name == "-" {
		dateStr := time.Now().Format(dateFormat)
		return stdin, nil, dateStr, nil
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
	body := opts.pageLength - headerLineCount - footerLineCount
	if body < 1 {
		body = 1
	}
	return body
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
	pageNum := 1
	hasMore := true

	for hasMore {
		lines, more := readBodyLines(scanner, bodyLines)
		hasMore = more
		if len(lines) == 0 {
			break
		}
		if err := writePage(bw, opts, dateStr, headerText, pageNum, lines, bodyLines); err != nil {
			return err
		}
		pageNum++
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// readBodyLines reads up to n lines from the scanner.
// Returns the lines and whether more input is available.
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

// writePage outputs a single page with optional header, body, and footer.
func writePage(
	bw *bufio.Writer, opts prOptions, dateStr, headerText string,
	pageNum int, lines []string, bodyLines int,
) error {
	if opts.omitPagination {
		return writeRawLines(bw, lines)
	}

	if !opts.omitHeader {
		writeHeader(bw, dateStr, headerText, pageNum, opts.pageWidth)
	}

	writeBodyLines(bw, lines, bodyLines, !opts.omitPagination)

	if !opts.omitHeader {
		writeFooter(bw)
	}

	return nil
}

// writeRawLines writes lines without any pagination structure.
func writeRawLines(bw *bufio.Writer, lines []string) error {
	for _, line := range lines {
		bw.WriteString(line)   //nolint:errcheck
		bw.WriteByte('\n')     //nolint:errcheck
	}
	return nil
}

// writeHeader outputs the 5-line page header.
// R2.1: 2 blank lines + header text line + 2 blank lines.
func writeHeader(
	bw *bufio.Writer, dateStr, headerText string, pageNum, pageWidth int,
) {
	// Two blank lines
	bw.WriteByte('\n') //nolint:errcheck
	bw.WriteByte('\n') //nolint:errcheck

	// Header text line
	headerLine := formatHeaderLine(dateStr, headerText, pageNum, pageWidth)
	bw.WriteString(headerLine) //nolint:errcheck
	bw.WriteByte('\n')         //nolint:errcheck

	// Two blank lines
	bw.WriteByte('\n') //nolint:errcheck
	bw.WriteByte('\n') //nolint:errcheck
}

// formatHeaderLine formats the three-column header: date, filename, Page N.
// R1.1: header line contains date, filename, and page number.
func formatHeaderLine(
	dateStr, headerText string, pageNum, pageWidth int,
) string {
	colWidth := pageWidth / 3
	rightPart := fmt.Sprintf("Page %d", pageNum)

	left := padRight(dateStr, colWidth)
	center := padRight(headerText, colWidth)

	return left + center + rightPart
}

// padRight pads a string with trailing spaces to reach the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// writeBodyLines writes body content lines and pads short pages.
// R2.3: fill body area between header and trailer.
func writeBodyLines(bw *bufio.Writer, lines []string, bodyLines int, pad bool) {
	for _, line := range lines {
		bw.WriteString(line) //nolint:errcheck
		bw.WriteByte('\n')   //nolint:errcheck
	}

	if pad {
		for i := len(lines); i < bodyLines; i++ {
			bw.WriteByte('\n') //nolint:errcheck
		}
	}
}

// writeFooter outputs the 5-line page footer (blank lines).
// R2.2: each page ends with a 5-line trailer block.
func writeFooter(bw *bufio.Writer) {
	for i := 0; i < footerLineCount; i++ {
		bw.WriteByte('\n') //nolint:errcheck
	}
}

// isBrokenPipe reports whether an error is caused by writing to a broken pipe.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
