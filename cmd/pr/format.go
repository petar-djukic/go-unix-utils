// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (prOptions, []string, error) {
	opts := prOptions{
		pageLength:  defaultPageLength,
		pageWidth:   defaultPageWidth,
		columns:     1,
		numberWidth: defaultNumberWidth,
		numberSep:   defaultNumberSep,
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
		consumed, err := parseFlag(&opts, args, i)
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

// parseFlag dispatches to long or short flag parsing.
func parseFlag(opts *prOptions, args []string, i int) (int, error) {
	consumed, err := parseLongFlag(opts, args, i)
	if err != nil {
		return 0, err
	}
	if consumed > 0 {
		return consumed, nil
	}
	return parseShortFlags(opts, args, i)
}

// parseLongFlag handles all --long-form flags.
func parseLongFlag(opts *prOptions, args []string, i int) (int, error) {
	arg := args[i]
	if !strings.HasPrefix(arg, "--") {
		return 0, nil
	}
	if val, ok := cutValue(arg, "--length="); ok {
		return parseIntOpt(&opts.pageLength, val, "invalid page length")
	}
	if val, ok := cutValue(arg, "--header="); ok {
		opts.customHeader = val
		opts.hasCustomHeader = true
		return 1, nil
	}
	if val, ok := cutValue(arg, "--columns="); ok {
		return parseColumnsValue(opts, val)
	}
	if val, ok := cutValue(arg, "--width="); ok {
		return parseIntOpt(&opts.pageWidth, val, "invalid page width")
	}
	if val, ok := cutValue(arg, "--separator="); ok {
		opts.separator = val
		opts.hasSeparator = true
		return 1, nil
	}
	if val, ok := cutValue(arg, "--number-lines="); ok {
		opts.numberLines = true
		parseNumberSpec(opts, val)
		return 1, nil
	}
	if val, ok := cutValue(arg, "--indent="); ok {
		return parseIntOpt(&opts.margin, val, "invalid margin")
	}
	return parseLongBoolFlag(opts, arg)
}

// parseLongBoolFlag handles long flags that take no value.
func parseLongBoolFlag(opts *prOptions, arg string) (int, error) {
	switch arg {
	case "--omit-header":
		opts.omitHeader = true
	case "--omit-pagination":
		opts.omitPagination = true
		opts.omitHeader = true
	case "--across":
		opts.across = true
	case "--separator":
		opts.separator = "\t"
		opts.hasSeparator = true
	case "--number-lines":
		opts.numberLines = true
	case "--double-space":
		opts.doubleSpace = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 1, nil
}

// cutValue extracts the value from a --key=value argument.
func cutValue(arg, prefix string) (string, bool) {
	if strings.HasPrefix(arg, prefix) {
		return arg[len(prefix):], true
	}
	return "", false
}

// parseIntOpt parses an integer into the target, returning (1, nil) on success.
func parseIntOpt(target *int, val, errMsg string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%s: '%s'", errMsg, val)
	}
	*target = n
	return 1, nil
}

// parseColumnsValue parses and validates the column count.
func parseColumnsValue(opts *prOptions, val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid number of columns: '%s'", val)
	}
	opts.columns = n
	return 1, nil
}

// parseShortFlags handles short flags and their arguments.
func parseShortFlags(opts *prOptions, args []string, i int) (int, error) {
	arg := args[i]
	if len(arg) < 2 || arg[0] != '-' {
		return 0, nil
	}
	if isColumnShorthand(arg) {
		return parseColumnShorthand(opts, arg)
	}
	return parseShortFlagChars(opts, args, i)
}

// isColumnShorthand returns true if arg is -N where N is all digits.
func isColumnShorthand(arg string) bool {
	for _, ch := range arg[1:] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// parseColumnShorthand parses -N column shorthand.
func parseColumnShorthand(opts *prOptions, arg string) (int, error) {
	n, err := strconv.Atoi(arg[1:])
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid number of columns: '%s'", arg[1:])
	}
	opts.columns = n
	return 1, nil
}

// parseShortFlagChars processes individual short flag characters.
func parseShortFlagChars(opts *prOptions, args []string, i int) (int, error) {
	chars := args[i][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 't':
			opts.omitHeader = true
		case 'T':
			opts.omitPagination = true
			opts.omitHeader = true
		case 'a':
			opts.across = true
		case 'd':
			opts.doubleSpace = true
		case 'r':
			opts.suppressErrors = true
		case 'f', 'F':
			opts.formFeed = true
		case 'n':
			return parseShortNumberLines(opts, chars[j+1:])
		case 's':
			return parseShortSep(opts, chars[j+1:])
		case 'l':
			return parseShortWithArg(opts, chars[j+1:], args, i, setPageLength)
		case 'h':
			return parseShortWithArg(opts, chars[j+1:], args, i, setHeader)
		case 'w':
			return parseShortWithArg(opts, chars[j+1:], args, i, setWidth)
		case 'o':
			return parseShortWithArg(opts, chars[j+1:], args, i, setMargin)
		default:
			return 0, fmt.Errorf("unrecognized option '-%c'", chars[j])
		}
	}
	return 1, nil
}

type optSetter func(opts *prOptions, val string) error

func setPageLength(opts *prOptions, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid page length: '%s'", val)
	}
	opts.pageLength = n
	return nil
}

func setHeader(opts *prOptions, val string) error {
	opts.customHeader = val
	opts.hasCustomHeader = true
	return nil
}

func setWidth(opts *prOptions, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid page width: '%s'", val)
	}
	opts.pageWidth = n
	return nil
}

func setMargin(opts *prOptions, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid margin: '%s'", val)
	}
	opts.margin = n
	return nil
}

// parseShortSep handles -s with optional inline character.
func parseShortSep(opts *prOptions, rest string) (int, error) {
	opts.hasSeparator = true
	if rest != "" {
		opts.separator = string(rest[0])
	} else {
		opts.separator = "\t"
	}
	return 1, nil
}

// parseShortNumberLines handles -n with optional inline SEP and WIDTH.
func parseShortNumberLines(opts *prOptions, rest string) (int, error) {
	opts.numberLines = true
	parseNumberSpec(opts, rest)
	return 1, nil
}

// parseNumberSpec parses the optional CHAR[WIDTH] spec for line numbering.
func parseNumberSpec(opts *prOptions, spec string) {
	if spec == "" {
		return
	}
	if spec[0] < '0' || spec[0] > '9' {
		opts.numberSep = spec[0]
		spec = spec[1:]
	}
	if spec != "" {
		if n, err := strconv.Atoi(spec); err == nil && n > 0 {
			opts.numberWidth = n
		}
	}
}

// parseShortWithArg handles a short flag that takes an argument.
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

// formatHeaderLine formats the three-part header: date (left), filename (center), Page N (right).
// R1.1: centers header text within the page width.
func formatHeaderLine(dateStr, headerText string, pageNum, pageWidth int) string {
	right := fmt.Sprintf("Page %d", pageNum)
	avail := pageWidth - len(dateStr) - len(right)
	if avail < len(headerText)+2 {
		return dateStr + " " + headerText + " " + right
	}
	leftPad := (avail - len(headerText)) / 2
	rightPad := avail - len(headerText) - leftPad
	return dateStr + strings.Repeat(" ", leftPad) + headerText +
		strings.Repeat(" ", rightPad) + right
}

// colWidth returns the visual width for each column.
func colWidth(opts prOptions) int {
	return opts.pageWidth / opts.columns
}

// formatDown arranges lines filling down each column before the next.
func formatDown(lines []string, _ int, opts prOptions) []string {
	rowCount := (len(lines) + opts.columns - 1) / opts.columns
	result := make([]string, rowCount)
	for row := range rowCount {
		result[row] = buildRowDown(lines, row, rowCount, opts)
	}
	return result
}

// buildRowDown builds a single output row in fill-down mode.
func buildRowDown(lines []string, row, rowCount int, opts prOptions) string {
	var sb strings.Builder
	cw := colWidth(opts)
	lastCol := lastNonEmptyCol(lines, row, rowCount, opts.columns)
	pos := 0
	for col := 0; col <= lastCol; col++ {
		idx := col*rowCount + row
		cell := cellAt(lines, idx)
		pos = writeFormattedCell(&sb, cell, idx+1, col, col == lastCol, cw, pos, opts)
	}
	return sb.String()
}

// formatAcross arranges lines filling across (left to right) each row.
func formatAcross(lines []string, opts prOptions) []string {
	rowCount := (len(lines) + opts.columns - 1) / opts.columns
	result := make([]string, rowCount)
	for row := range rowCount {
		result[row] = buildRowAcross(lines, row, opts)
	}
	return result
}

// buildRowAcross builds a single output row in across mode.
func buildRowAcross(lines []string, row int, opts prOptions) string {
	var sb strings.Builder
	cw := colWidth(opts)
	lastCol := lastNonEmptyColAcross(lines, row, opts.columns)
	pos := 0
	for col := 0; col <= lastCol; col++ {
		idx := row*opts.columns + col
		cell := cellAt(lines, idx)
		pos = writeFormattedCell(&sb, cell, idx+1, col, col == lastCol, cw, pos, opts)
	}
	return sb.String()
}

// writeFormattedCell writes a cell with optional numbering and column padding.
// Returns the new visual position.
func writeFormattedCell(
	sb *strings.Builder, cell string, lineNum, col int,
	isLast bool, cw, pos int, opts prOptions,
) int {
	if col > 0 && opts.hasSeparator {
		sb.WriteString(opts.separator)
		pos += len(opts.separator)
	}
	if opts.numberLines {
		pos = writeColNumber(sb, lineNum, col, pos, opts)
	}
	sb.WriteString(cell)
	pos += expandedWidth(cell, pos)
	if isLast || opts.hasSeparator {
		return pos
	}
	return tabPadTo(sb, pos, (col+1)*cw)
}

// writeColNumber writes a line number prefix for a column cell.
// Column 0 uses a literal tab separator; other columns expand tab to spaces
// using tab stops relative to the column start position.
func writeColNumber(
	sb *strings.Builder, lineNum, col, pos int, opts prOptions,
) int {
	numStr := rightJustify(strconv.Itoa(lineNum), opts.numberWidth)
	sb.WriteString(numStr)
	pos += opts.numberWidth
	if col == 0 && opts.numberSep == '\t' {
		sb.WriteByte('\t')
		pos = nextTabStop(pos)
	} else if opts.numberSep == '\t' {
		// Tab stops relative to column start
		colStart := col * colWidth(opts)
		relPos := pos - colStart
		target := colStart + ((relPos/tabWidth)+1)*tabWidth
		for pos < target {
			sb.WriteByte(' ')
			pos++
		}
	} else {
		sb.WriteByte(opts.numberSep)
		pos++
	}
	return pos
}

// rightJustify right-justifies s in a field of the given width.
func rightJustify(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// lastNonEmptyCol finds the last column with content in fill-down mode.
func lastNonEmptyCol(lines []string, row, rowCount, columns int) int {
	last := 0
	for col := range columns {
		if col*rowCount+row < len(lines) {
			last = col
		}
	}
	return last
}

// lastNonEmptyColAcross finds the last column with content in across mode.
func lastNonEmptyColAcross(lines []string, row, columns int) int {
	last := 0
	for col := range columns {
		if row*columns+col < len(lines) {
			last = col
		}
	}
	return last
}

// cellAt returns the line at index, or "" if out of bounds.
func cellAt(lines []string, idx int) string {
	if idx < len(lines) {
		return lines[idx]
	}
	return ""
}

// expandedWidth computes the visual width of s starting at visual position pos.
func expandedWidth(s string, pos int) int {
	start := pos
	for _, b := range []byte(s) {
		if b == '\t' {
			pos = nextTabStop(pos)
		} else {
			pos++
		}
	}
	return pos - start
}

// nextTabStop returns the next tab stop position after pos.
func nextTabStop(pos int) int {
	return ((pos / tabWidth) + 1) * tabWidth
}

// tabPadTo pads from current position to target using tab bytes and spaces.
func tabPadTo(sb *strings.Builder, pos, target int) int {
	for {
		next := nextTabStop(pos)
		if next > target {
			break
		}
		sb.WriteByte('\t')
		pos = next
	}
	for pos < target {
		sb.WriteByte(' ')
		pos++
	}
	return pos
}
