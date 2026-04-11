// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/pr: paginate or columnate files for printing.
// Implements srd110-pr R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R4.3, R5.1, R5.2.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultPageLength  = 66 // R1.1: lines per page
	defaultHeaderLines = 5  // R1.1: header lines
	defaultFooterLines = 5  // R1.1: footer lines
	defaultPageWidth   = 72 // R4.2: page width
	defaultNumberWidth = 5  // R4.1: number field width
	defaultNumberChar  = '\t'
	tabWidth           = 8
	progName           = "pr"
)

// config holds all parsed command-line options for pr.
type config struct {
	pageLength     int
	header         string
	omitHeader     bool
	omitPagination bool
	columns        int
	across         bool
	numberLines    bool
	numberChar     byte
	numberWidth    int
	indent         int
	pageWidth      int
	doubleSpace    bool
	separator      string
}

func defaultConfig() config {
	return config{
		pageLength:  defaultPageLength,
		columns:     1,
		numberChar:  defaultNumberChar,
		numberWidth: defaultNumberWidth,
		pageWidth:   defaultPageWidth,
		separator:   "\t",
	}
}

// parseFlags parses command-line flags and returns config and file arguments.
func parseFlags() (config, []string) {
	cfg := defaultConfig()
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {}

	pageLength := fs.Int("l", defaultPageLength, "page length")
	header := fs.String("h", "", "header string")
	omitHeader := fs.Bool("t", false, "omit header and footer")
	omitPagination := fs.Bool("T", false, "omit pagination")
	columns := fs.Int("columns", 1, "number of columns")
	across := fs.Bool("a", false, "fill columns across")
	// R4.1: -n handled by preprocessNumberFlag (not registered here)
	indent := fs.Int("o", 0, "indent margin")
	pageWidth := fs.Int("w", defaultPageWidth, "page width")
	doubleSpace := fs.Bool("d", false, "double-space output")
	separator := fs.String("s", "\t", "column separator")

	args := preprocessColumnFlag(os.Args[1:], &cfg)
	args = preprocessNumberFlag(args, &cfg)

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	applyParsedFlags(&cfg, *pageLength, *header, *omitHeader, *omitPagination,
		*columns, *across, *indent, *pageWidth, *doubleSpace, *separator)
	return cfg, fs.Args()
}

func applyParsedFlags(cfg *config, pageLength int, header string, omitHeader, omitPagination bool,
	columns int, across bool, indent, pageWidth int, doubleSpace bool, separator string) {
	cfg.pageLength = pageLength
	cfg.header = header
	cfg.omitHeader = omitHeader
	cfg.omitPagination = omitPagination
	if columns > 1 {
		cfg.columns = columns
	}
	cfg.across = across
	cfg.indent = indent
	cfg.pageWidth = pageWidth
	cfg.doubleSpace = doubleSpace
	cfg.separator = separator
}

// preprocessColumnFlag extracts -N column flags from args before flag parsing.
func preprocessColumnFlag(args []string, cfg *config) []string {
	var filtered []string
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			if n, err := strconv.Atoi(arg[1:]); err == nil && n > 0 {
				cfg.columns = n
				continue
			}
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

// preprocessNumberFlag extracts -n[SEP[DIGITS]] and --number-lines[=SEP[DIGITS]].
// R4.1: line numbering with optional separator and width.
func preprocessNumberFlag(args []string, cfg *config) []string {
	var filtered []string
	for _, arg := range args {
		if matchesNumberFlag(arg) {
			cfg.numberLines = true
			if len(arg) > 2 {
				parseNumberSuffix(arg[2:], cfg)
			}
			continue
		}
		if matchesNumberLongFlag(arg, cfg) {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func matchesNumberFlag(arg string) bool {
	return arg == "-n" || (len(arg) > 2 && arg[0] == '-' && arg[1] == 'n')
}

func matchesNumberLongFlag(arg string, cfg *config) bool {
	if arg == "--number-lines" {
		cfg.numberLines = true
		return true
	}
	const prefix = "--number-lines="
	if strings.HasPrefix(arg, prefix) {
		cfg.numberLines = true
		parseNumberSuffix(arg[len(prefix):], cfg)
		return true
	}
	return false
}

// parseNumberSuffix parses optional SEP[DIGITS] from -n or --number-lines value.
func parseNumberSuffix(suffix string, cfg *config) {
	if len(suffix) == 0 {
		return
	}
	cfg.numberChar = suffix[0]
	if len(suffix) > 1 {
		if w, err := strconv.Atoi(suffix[1:]); err == nil && w > 0 {
			cfg.numberWidth = w
		}
	}
}

func bodyLines(cfg config) int {
	if cfg.omitPagination || cfg.omitHeader {
		return cfg.pageLength
	}
	body := cfg.pageLength - defaultHeaderLines - defaultFooterLines
	if body < 0 {
		return 0
	}
	return body
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

func formatHeader(cfg config, headerText string, pageNum int, date time.Time) string {
	dateStr := date.Format("2006-01-02 15:04")
	pageStr := fmt.Sprintf("Page %d", pageNum)
	centerStart := max((cfg.pageWidth-len(headerText))/2, len(dateStr))
	leftPad := centerStart - len(dateStr)
	pagePos := cfg.pageWidth - len(pageStr)
	rightPad := max(pagePos-centerStart-len(headerText), 0)
	return dateStr + strings.Repeat(" ", leftPad) +
		headerText + strings.Repeat(" ", rightPad) + pageStr
}

func writeHeader(w *bufio.Writer, cfg config, hdr string, pg int, date time.Time) error {
	line := formatHeader(cfg, hdr, pg, date)
	_, err := fmt.Fprintf(w, "\n\n%s\n\n\n", line)
	return err
}

func writeFooter(w *bufio.Writer, _ config) error {
	_, err := fmt.Fprint(w, "\n\n\n\n\n")
	return err
}

// numberLine prepends a line number for single-column mode.
// R4.1: right-justified number in numberWidth field, then separator, then text.
func numberLine(line string, num int, cfg config) string {
	return fmt.Sprintf("%*d%c%s", cfg.numberWidth, num, cfg.numberChar, line)
}

// numberPageLines numbers each input line for single-column output.
func numberPageLines(lines []string, lineNum *int, cfg config) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = numberLine(line, *lineNum, cfg)
		*lineNum++
	}
	return result
}

// visualWidth computes visual width of text starting at a given visual position,
// accounting for tab expansion to 8-char tab stops.
func visualWidth(text string, startPos int) int {
	pos := startPos
	for i := 0; i < len(text); i++ {
		if text[i] == '\t' {
			pos = ((pos / tabWidth) + 1) * tabWidth
		} else {
			pos++
		}
	}
	return pos - startPos
}

func writePage(w *bufio.Writer, cfg config, hdr string, pg int, date time.Time, lines []string, bodyCount int) error {
	if err := writeHeader(w, cfg, hdr, pg, date); err != nil {
		return err
	}
	for i := range bodyCount {
		if i < len(lines) {
			if _, err := fmt.Fprintln(w, lines[i]); err != nil {
				return err
			}
		} else {
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
	}
	return writeFooter(w, cfg)
}

func paginateFile(r io.Reader, w *bufio.Writer, cfg config, hdr string, date time.Time) error {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	body := bodyLines(cfg)
	if body <= 0 {
		body = 1
	}
	return writePages(w, cfg, hdr, date, lines, body)
}

// writePages outputs all pages from the collected lines.
// R4.1: numbering applies before column layout for multi-column mode;
// lineNum increments across pages (D3).
func writePages(w *bufio.Writer, cfg config, hdr string, date time.Time, lines []string, body int) error {
	linesPerPage := linesPerPageCount(cfg, body)
	pageNum := 1
	lineNum := 1
	for i := 0; ; i += linesPerPage {
		end := min(i+linesPerPage, len(lines))
		pageLines := lines[i:end]
		pageLines = formatPageLines(pageLines, cfg, body, &lineNum)
		if err := writePage(w, cfg, hdr, pageNum, date, pageLines, body); err != nil {
			return err
		}
		pageNum++
		if end >= len(lines) {
			break
		}
	}
	return nil
}

// formatPageLines applies column layout and/or numbering to page lines.
func formatPageLines(lines []string, cfg config, body int, lineNum *int) []string {
	if cfg.columns > 1 && cfg.numberLines {
		result := formatColumns(lines, cfg, body, *lineNum)
		*lineNum += len(lines)
		return result
	}
	if cfg.columns > 1 {
		return formatColumns(lines, cfg, body, 0)
	}
	if cfg.numberLines {
		return numberPageLines(lines, lineNum, cfg)
	}
	return lines
}

func linesPerPageCount(cfg config, body int) int {
	if cfg.columns > 1 {
		return body * cfg.columns
	}
	return body
}

func columnWidth(cfg config) int {
	return cfg.pageWidth / cfg.columns
}

// formatColumns arranges lines into multi-column layout.
// baseNum > 0 enables line numbering within columns.
func formatColumns(lines []string, cfg config, bodyRows, baseNum int) []string {
	colWidth := columnWidth(cfg)
	nlines := len(lines)
	rows := (nlines + cfg.columns - 1) / cfg.columns
	if rows > bodyRows {
		rows = bodyRows
	}
	result := make([]string, rows)
	for j := range rows {
		result[j] = formatColumnRow(lines, cfg, j, rows, colWidth, baseNum)
	}
	return result
}

// formatColumnRow formats a single output row for multi-column layout.
// When baseNum > 0 and cfg.numberLines, each cell includes a position-aware number.
func formatColumnRow(lines []string, cfg config, rowIdx, rowCount, colWidth, baseNum int) string {
	lastCol := lastPresentCol(lines, cfg, rowIdx, rowCount)
	if lastCol < 0 {
		return ""
	}
	var buf strings.Builder
	for c := 0; c <= lastCol; c++ {
		idx := columnLineIndex(cfg, rowIdx, c, rowCount)
		text := cellText(lines, idx)
		colStart := c * colWidth
		colEnd := colStart + colWidth
		isLast := c == lastCol
		if cfg.numberLines && baseNum > 0 {
			writeNumberedCell(&buf, text, baseNum+idx, cfg, colStart, colEnd, isLast)
		} else {
			writeColumnCell(&buf, text, colStart, colEnd, isLast)
		}
	}
	return buf.String()
}

func cellText(lines []string, idx int) string {
	if idx < len(lines) {
		return lines[idx]
	}
	return ""
}

// writeColumnCell writes a plain (unnumbered) column cell with padding.
func writeColumnCell(buf *strings.Builder, text string, colStart, colEnd int, isLast bool) {
	if isLast {
		buf.WriteString(text)
		return
	}
	writeColumnField(buf, text, colStart, colEnd)
}

// writeNumberedCell writes a column cell with position-aware line numbering.
// R4.1: the number is right-justified using tab-aware padding from colStart.
// The separator (TAB or char) bridges from number to text start position.
func writeNumberedCell(buf *strings.Builder, text string, num int, cfg config, colStart, colEnd int, isLast bool) {
	digits := strconv.Itoa(num)
	digitStart := colStart + cfg.numberWidth - len(digits)

	// R4.1: right-justify number in field
	padWithTabs(buf, colStart, digitStart)
	buf.WriteString(digits)
	pos := digitStart + len(digits)

	// Write separator and advance to text position
	pos = writeSeparator(buf, pos, colStart, cfg)

	buf.WriteString(text)
	if !isLast {
		pos += visualWidth(text, pos)
		padWithTabs(buf, pos, colEnd)
	}
}

// writeSeparator writes the number separator and returns the new visual position.
// For TAB separator, pads to colStart+tabWidth using tab-aware spacing.
// For non-TAB, writes the literal separator byte.
func writeSeparator(buf *strings.Builder, pos, colStart int, cfg config) int {
	if cfg.numberChar == '\t' {
		target := colStart + tabWidth
		if target <= pos {
			target = ((pos / tabWidth) + 1) * tabWidth
		}
		padWithTabs(buf, pos, target)
		return target
	}
	buf.WriteByte(cfg.numberChar)
	return pos + 1
}

func lastPresentCol(lines []string, cfg config, rowIdx, rowCount int) int {
	for c := cfg.columns - 1; c >= 0; c-- {
		if columnLineIndex(cfg, rowIdx, c, rowCount) < len(lines) {
			return c
		}
	}
	return -1
}

func columnLineIndex(cfg config, row, col, rowCount int) int {
	if cfg.across {
		return row*cfg.columns + col
	}
	return row + col*rowCount
}

// writeColumnField writes text and pads to the column end using tabs and spaces.
// Uses visual width (accounting for tab expansion) for correct padding.
func writeColumnField(buf *strings.Builder, text string, colStart, colEnd int) {
	colWidth := colEnd - colStart
	vw := visualWidth(text, colStart)
	if vw >= colWidth {
		buf.WriteString(text[:colWidth])
		return
	}
	buf.WriteString(text)
	padWithTabs(buf, colStart+vw, colEnd)
}

func padWithTabs(buf *strings.Builder, pos, target int) {
	for pos < target {
		nextTab := ((pos / tabWidth) + 1) * tabWidth
		if nextTab <= target {
			buf.WriteByte('\t')
			pos = nextTab
		} else {
			buf.WriteByte(' ')
			pos++
		}
	}
}

func fileHeaderInfo(f *os.File, name string, cfg config) (string, time.Time) {
	hdr := cfg.header
	if hdr == "" && name != "-" {
		hdr = name
	}
	date := time.Now()
	if name != "-" {
		if info, err := f.Stat(); err == nil {
			date = info.ModTime()
		}
	}
	return hdr, date
}

func prFile(name string, w *bufio.Writer, cfg config) error {
	f, err := openInput(name)
	if err != nil {
		return err
	}
	if f != os.Stdin {
		defer f.Close()
	}
	hdr, date := fileHeaderInfo(f, name, cfg)
	return paginateFile(f, w, cfg, hdr, date)
}

func run(cfg config, files []string) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range files {
		if err := prFile(name, w, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error\n", progName)
		exitCode = 1
	}
	return exitCode
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args := parseFlags()
	if len(args) == 0 {
		args = []string{"-"}
	}
	os.Exit(run(cfg, args))
}
