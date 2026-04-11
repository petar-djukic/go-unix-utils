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
	separatorSet   bool // R4.3: true when -s was explicitly provided
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
	// R4.3: -s handled by preprocessSeparatorFlag (not registered here)

	args := preprocessColumnFlag(os.Args[1:], &cfg)
	args = preprocessNumberFlag(args, &cfg)
	args = preprocessSeparatorFlag(args, &cfg)
	args = preprocessLongFlags(args)

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	applyParsedFlags(&cfg, *pageLength, *header, *omitHeader, *omitPagination,
		*columns, *across, *indent, *pageWidth, *doubleSpace)
	return cfg, fs.Args()
}

func applyParsedFlags(cfg *config, pageLength int, header string, omitHeader, omitPagination bool,
	columns int, across bool, indent, pageWidth int, doubleSpace bool) {
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

// preprocessSeparatorFlag extracts -s[CHAR] and --separator[=CHAR].
// R4.3: sets separatorSet to distinguish explicit -s from default behavior.
func preprocessSeparatorFlag(args []string, cfg *config) []string {
	var filtered []string
	for _, arg := range args {
		if parseSepShort(arg, cfg) || parseSepLong(arg, cfg) {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func parseSepShort(arg string, cfg *config) bool {
	if len(arg) < 2 || arg[0] != '-' || arg[1] != 's' {
		return false
	}
	cfg.separatorSet = true
	if len(arg) > 2 {
		cfg.separator = string(arg[2])
	}
	return true
}

func parseSepLong(arg string, cfg *config) bool {
	if arg == "--separator" {
		cfg.separatorSet = true
		return true
	}
	const prefix = "--separator="
	if strings.HasPrefix(arg, prefix) {
		cfg.separatorSet = true
		val := arg[len(prefix):]
		if len(val) > 0 {
			cfg.separator = string(val[0])
		}
		return true
	}
	return false
}

// preprocessLongFlags converts long-form flags to short-form equivalents.
// R4.2: --indent=MARGIN, --width=WIDTH. R4.3: --double-space.
func preprocessLongFlags(args []string) []string {
	var filtered []string
	for _, arg := range args {
		if replacement, ok := mapLongFlag(arg); ok {
			filtered = append(filtered, replacement...)
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func mapLongFlag(arg string) ([]string, bool) {
	if strings.HasPrefix(arg, "--indent=") {
		return []string{"-o", arg[len("--indent="):]}, true
	}
	if strings.HasPrefix(arg, "--width=") {
		return []string{"-w", arg[len("--width="):]}, true
	}
	if arg == "--double-space" {
		return []string{"-d"}, true
	}
	return nil, false
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

// contentBodyCount returns the number of content lines per page,
// accounting for double-spacing. R4.3: double-space halves capacity.
func contentBodyCount(cfg config, body int) int {
	if cfg.doubleSpace {
		return body / 2
	}
	return body
}

func indentString(cfg config) string {
	if cfg.indent <= 0 {
		return ""
	}
	return strings.Repeat(" ", cfg.indent)
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

// writeHeader writes the 5-line page header with indent.
// R4.2: each line is prefixed with indent spaces.
func writeHeader(w *bufio.Writer, cfg config, hdr string, pg int, date time.Time) error {
	indent := indentString(cfg)
	line := formatHeader(cfg, hdr, pg, date)
	_, err := fmt.Fprintf(w, "%s\n%s\n%s%s\n%s\n%s\n",
		indent, indent, indent, line, indent, indent)
	return err
}

func writeFooter(w *bufio.Writer, cfg config) error {
	indent := indentString(cfg)
	_, err := fmt.Fprintf(w, "%s\n%s\n%s\n%s\n%s\n",
		indent, indent, indent, indent, indent)
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

// writeBodyLines writes body content lines with indent and double-spacing.
// R4.2: indent prefix on every line. R4.3: double-space inserts blank lines.
func writeBodyLines(w *bufio.Writer, cfg config, indent string, lines []string, bodyCount int) error {
	written := 0
	lineIdx := 0
	for written < bodyCount {
		if lineIdx < len(lines) {
			if err := writeIndentedLine(w, indent, lines[lineIdx]); err != nil {
				return err
			}
			lineIdx++
			written++
			if cfg.doubleSpace && written < bodyCount {
				if err := writeIndentedBlank(w, indent); err != nil {
					return err
				}
				written++
			}
		} else {
			if err := writeIndentedBlank(w, indent); err != nil {
				return err
			}
			written++
		}
	}
	return nil
}

func writeIndentedLine(w *bufio.Writer, indent, text string) error {
	if indent != "" {
		if _, err := w.WriteString(indent); err != nil {
			return err
		}
	}
	if _, err := w.WriteString(text); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func writeIndentedBlank(w *bufio.Writer, indent string) error {
	return writeIndentedLine(w, indent, "")
}

func writePage(w *bufio.Writer, cfg config, hdr string, pg int, date time.Time, lines []string, bodyCount int) error {
	indent := indentString(cfg)
	if err := writeHeader(w, cfg, hdr, pg, date); err != nil {
		return err
	}
	if err := writeBodyLines(w, cfg, indent, lines, bodyCount); err != nil {
		return err
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
// R4.1: numbering applies before column layout; lineNum increments across pages.
// R4.3: double-space reduces content lines per page.
func writePages(w *bufio.Writer, cfg config, hdr string, date time.Time, lines []string, body int) error {
	content := contentBodyCount(cfg, body)
	linesPerPage := linesPerPageCount(cfg, content)
	pageNum := 1
	lineNum := 1
	for i := 0; ; i += linesPerPage {
		end := min(i+linesPerPage, len(lines))
		pageLines := lines[i:end]
		pageLines = formatPageLines(pageLines, cfg, content, &lineNum)
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
// R4.3: when separatorSet, columns are separated by separator only (no padding).
func formatColumnRow(lines []string, cfg config, rowIdx, rowCount, colWidth, baseNum int) string {
	lastCol := lastPresentCol(lines, cfg, rowIdx, rowCount)
	if lastCol < 0 {
		return ""
	}
	var buf strings.Builder
	for c := 0; c <= lastCol; c++ {
		idx := columnLineIndex(cfg, rowIdx, c, rowCount)
		text := cellText(lines, idx)
		isLast := c == lastCol
		if cfg.separatorSet {
			writeSepCell(&buf, text, idx, cfg, baseNum, c > 0)
		} else {
			writeStdCell(&buf, text, idx, cfg, c*colWidth, (c+1)*colWidth, baseNum, isLast)
		}
	}
	return buf.String()
}

// writeSepCell writes a column cell in separator mode (no padding).
// R4.3: columns separated by separator character only.
func writeSepCell(buf *strings.Builder, text string, idx int, cfg config, baseNum int, prependSep bool) {
	if prependSep {
		buf.WriteString(cfg.separator)
	}
	if cfg.numberLines && baseNum > 0 {
		fmt.Fprintf(buf, "%*d%c", cfg.numberWidth, baseNum+idx, cfg.numberChar)
	}
	buf.WriteString(text)
}

// writeStdCell writes a column cell in standard mode (padded to column width).
func writeStdCell(buf *strings.Builder, text string, idx int, cfg config, colStart, colEnd, baseNum int, isLast bool) {
	if cfg.numberLines && baseNum > 0 {
		writeNumberedCell(buf, text, baseNum+idx, cfg, colStart, colEnd, isLast)
	} else {
		writeColumnCell(buf, text, colStart, colEnd, isLast)
	}
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
func writeNumberedCell(buf *strings.Builder, text string, num int, cfg config, colStart, colEnd int, isLast bool) {
	digits := strconv.Itoa(num)
	digitStart := colStart + cfg.numberWidth - len(digits)

	// R4.1: right-justify number in field
	padWithTabs(buf, colStart, digitStart)
	buf.WriteString(digits)
	pos := digitStart + len(digits)

	// Write separator and advance to text position
	pos = writeNumberSep(buf, pos, colStart, cfg)

	buf.WriteString(text)
	if !isLast {
		pos += visualWidth(text, pos)
		padWithTabs(buf, pos, colEnd)
	}
}

// writeNumberSep writes the number separator and returns the new visual position.
// For TAB separator, pads to colStart+tabWidth using tab-aware spacing.
// For non-TAB, writes the literal separator byte.
func writeNumberSep(buf *strings.Builder, pos, colStart int, cfg config) int {
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
