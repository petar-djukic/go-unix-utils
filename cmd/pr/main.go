// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd110-pr: Paginate or Columnate Files for Printing.
// Covers R3.1 (exit codes: 0 on success, 1 on error),
// R3.2 (SIGPIPE handling via pkg/sys.InstallSIGPIPEHandler).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultPageLength = 66
	headerLines       = 5
	footerLines       = 5
	defaultPageWidth  = 72
	defaultNumWidth   = 5
)

// prConfig holds parsed command-line options.
type prConfig struct {
	pageLength     int
	pageWidth      int
	header         string // custom header text (-h)
	omitHeader     bool   // -t: suppress header/footer
	omitPagination bool   // -T: suppress header/footer, no bottom pad
	columns        int    // number of columns (default 1)
	across         bool   // -a: fill across instead of down
	numberLines    bool   // -n: number output lines
	numberChar     byte   // separator for line numbers
	numberWidth    int    // width of line number field
	indent         int    // -o: left margin indent
	doubleSpace    bool   // -d: double-space output
	separator      string // -s: column separator
	separatorSet   bool   // whether -s was explicitly provided
}

func main() {
	// R3.2: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "pr: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(cfg, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses pr flags and returns config and file list.
func parseArgs(args []string) (prConfig, []string, error) {
	cfg := prConfig{
		pageLength:  defaultPageLength,
		pageWidth:   defaultPageWidth,
		columns:     1,
		numberChar:  '\t',
		numberWidth: defaultNumWidth,
		separator:   "\t",
	}
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}
		consumed, err := parseFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, files, nil
}

// parseFlag dispatches flag parsing and returns args consumed.
func parseFlag(arg string, args []string, i int, cfg *prConfig) (int, error) {
	if isColumnFlag(arg) {
		return parseColumnFlag(arg, cfg)
	}
	return parseLongOrShortFlag(arg, args, i, cfg)
}

// isColumnFlag checks if arg is a -N column count flag (e.g., -2, -3).
func isColumnFlag(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	return arg[1] >= '1' && arg[1] <= '9'
}

// parseColumnFlag parses a -N column count flag.
func parseColumnFlag(arg string, cfg *prConfig) (int, error) {
	n := 0
	for _, c := range arg[1:] {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid option -- '%c'", c)
		}
		n = n*10 + int(c-'0')
	}
	cfg.columns = n
	return 1, nil
}

// parseLongOrShortFlag handles non-column flags.
func parseLongOrShortFlag(arg string, args []string, i int, cfg *prConfig) (int, error) {
	switch {
	case arg == "-t" || arg == "--omit-header":
		cfg.omitHeader = true
		return 1, nil
	case arg == "-T" || arg == "--omit-pagination":
		cfg.omitPagination = true
		cfg.omitHeader = true
		return 1, nil
	case arg == "-a" || arg == "--across":
		cfg.across = true
		return 1, nil
	case arg == "-d" || arg == "--double-space":
		cfg.doubleSpace = true
		return 1, nil
	case strings.HasPrefix(arg, "-l"):
		return parseIntOpt(arg, args, i, "-l", &cfg.pageLength)
	case strings.HasPrefix(arg, "--length"):
		return parseLongIntOpt(arg, args, i, "--length", &cfg.pageLength)
	case strings.HasPrefix(arg, "-w"):
		return parseIntOpt(arg, args, i, "-w", &cfg.pageWidth)
	case strings.HasPrefix(arg, "--width"):
		return parseLongIntOpt(arg, args, i, "--width", &cfg.pageWidth)
	case strings.HasPrefix(arg, "-o"):
		return parseIntOpt(arg, args, i, "-o", &cfg.indent)
	case strings.HasPrefix(arg, "--indent"):
		return parseLongIntOpt(arg, args, i, "--indent", &cfg.indent)
	case strings.HasPrefix(arg, "-h"):
		return parseStringOpt(arg, args, i, "-h", &cfg.header)
	case strings.HasPrefix(arg, "--header"):
		return parseLongStringOpt(arg, args, i, "--header", &cfg.header)
	case strings.HasPrefix(arg, "-n"):
		return parseNumberFlag(arg, cfg)
	case strings.HasPrefix(arg, "-s"):
		return parseSeparatorFlag(arg, cfg)
	case strings.HasPrefix(arg, "--separator"):
		return parseLongSeparatorFlag(arg, args, i, cfg)
	case arg == "--number-lines":
		cfg.numberLines = true
		return 1, nil
	case strings.HasPrefix(arg, "--number-lines="):
		return parseNumberLongFlag(arg, cfg)
	case strings.HasPrefix(arg, "--columns"):
		return parseLongColumnsFlag(arg, args, i, cfg)
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

// parseIntOpt parses -X N or -XN short int options.
func parseIntOpt(arg string, args []string, i int, flag string, target *int) (int, error) {
	if len(arg) > len(flag) {
		return 1, parsePositiveInt(arg[len(flag):], target)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%s'", flag[1:])
	}
	return 2, parsePositiveInt(args[i+1], target)
}

// parseLongIntOpt parses --flag=N or --flag N long int options.
func parseLongIntOpt(arg string, args []string, i int, flag string, target *int) (int, error) {
	eqForm := flag + "="
	if strings.HasPrefix(arg, eqForm) {
		return 1, parsePositiveInt(arg[len(eqForm):], target)
	}
	if arg != flag {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", flag)
	}
	return 2, parsePositiveInt(args[i+1], target)
}

// parsePositiveInt parses and validates a positive integer value.
func parsePositiveInt(val string, target *int) error {
	n := 0
	for _, c := range val {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid number '%s'", val)
		}
		n = n*10 + int(c-'0')
	}
	*target = n
	return nil
}

// parseStringOpt parses -X VALUE or -XVALUE short string options.
func parseStringOpt(arg string, args []string, i int, flag string, target *string) (int, error) {
	if len(arg) > len(flag) {
		*target = arg[len(flag):]
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%s'", flag[1:])
	}
	*target = args[i+1]
	return 2, nil
}

// parseLongStringOpt parses --flag=VALUE or --flag VALUE long string options.
func parseLongStringOpt(arg string, args []string, i int, flag string, target *string) (int, error) {
	eqForm := flag + "="
	if strings.HasPrefix(arg, eqForm) {
		*target = arg[len(eqForm):]
		return 1, nil
	}
	if arg != flag {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", flag)
	}
	*target = args[i+1]
	return 2, nil
}

// parseNumberFlag parses -n[CHAR[WIDTH]] flag.
func parseNumberFlag(arg string, cfg *prConfig) (int, error) {
	cfg.numberLines = true
	rest := arg[2:]
	if len(rest) == 0 {
		return 1, nil
	}
	cfg.numberChar = rest[0]
	if len(rest) > 1 {
		w := 0
		for _, c := range rest[1:] {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("invalid number '%s'", rest[1:])
			}
			w = w*10 + int(c-'0')
		}
		cfg.numberWidth = w
	}
	return 1, nil
}

// parseNumberLongFlag parses --number-lines=CHAR[WIDTH].
func parseNumberLongFlag(arg string, cfg *prConfig) (int, error) {
	cfg.numberLines = true
	val := arg[len("--number-lines="):]
	if len(val) == 0 {
		return 1, nil
	}
	cfg.numberChar = val[0]
	if len(val) > 1 {
		w := 0
		for _, c := range val[1:] {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("invalid number '%s'", val[1:])
			}
			w = w*10 + int(c-'0')
		}
		cfg.numberWidth = w
	}
	return 1, nil
}

// parseSeparatorFlag parses -s[CHAR] flag.
func parseSeparatorFlag(arg string, cfg *prConfig) (int, error) {
	cfg.separatorSet = true
	if len(arg) > 2 {
		cfg.separator = arg[2:]
		return 1, nil
	}
	cfg.separator = "\t"
	return 1, nil
}

// parseLongSeparatorFlag parses --separator=CHAR flag.
func parseLongSeparatorFlag(arg string, args []string, i int, cfg *prConfig) (int, error) {
	cfg.separatorSet = true
	eqForm := "--separator="
	if strings.HasPrefix(arg, eqForm) {
		cfg.separator = arg[len(eqForm):]
		return 1, nil
	}
	if arg != "--separator" {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '--separator' requires an argument")
	}
	cfg.separator = args[i+1]
	return 2, nil
}

// parseLongColumnsFlag parses --columns=N flag.
func parseLongColumnsFlag(arg string, args []string, i int, cfg *prConfig) (int, error) {
	eqForm := "--columns="
	if strings.HasPrefix(arg, eqForm) {
		return 1, parsePositiveInt(arg[len(eqForm):], &cfg.columns)
	}
	if arg != "--columns" {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '--columns' requires an argument")
	}
	return 2, parsePositiveInt(args[i+1], &cfg.columns)
}

// run processes all files and returns the exit code.
// R3.1: exit 0 on success, exit 1 on any error.
func run(cfg prConfig, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := processFile(name, stdin, bw, stderr, cfg); err != nil {
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processFile opens a file and formats it as paginated output.
func processFile(name string, stdin io.Reader, bw *bufio.Writer, stderr io.Writer, cfg prConfig) error {
	r, displayName, err := openInput(name, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "pr: %s\n", formatOpenError(name, err))
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return paginateFile(r, bw, displayName, cfg)
}

// openInput opens a file or returns stdin for "-".
func openInput(name string, stdin io.Reader) (io.ReadCloser, string, error) {
	if name == "-" {
		return io.NopCloser(stdin), "", nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, "", err
	}
	return f, name, nil
}

// formatOpenError formats an os.Open error for display matching GNU pr format.
func formatOpenError(name string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("%s: %s", name, pe.Err.Error())
	}
	return fmt.Sprintf("%s: %s", name, err.Error())
}

// paginateFile reads all lines and formats them into pages.
func paginateFile(r io.Reader, bw *bufio.Writer, displayName string, cfg prConfig) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	if cfg.columns > 1 {
		return writeMultiColumn(bw, lines, displayName, cfg)
	}
	return writeSingleColumn(bw, lines, displayName, cfg)
}

// readAllLines reads all lines from a reader.
func readAllLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// bodyLinesPerPage returns the number of body lines per page.
func bodyLinesPerPage(cfg prConfig) int {
	if cfg.omitHeader {
		return cfg.pageLength
	}
	return max(cfg.pageLength-headerLines-footerLines, 1)
}

// writeSingleColumn writes single-column paginated output.
func writeSingleColumn(bw *bufio.Writer, lines []string, displayName string, cfg prConfig) error {
	bodyLines := bodyLinesPerPage(cfg)
	page := 1
	lineIdx := 0
	lineNum := 1
	for lineIdx < len(lines) || page == 1 {
		if err := writePageHeader(bw, displayName, page, cfg); err != nil {
			return err
		}
		bodyWritten := 0
		for bodyWritten < bodyLines && lineIdx < len(lines) {
			if err := writeBodyLine(bw, lines[lineIdx], &lineNum, cfg); err != nil {
				return err
			}
			lineIdx++
			bodyWritten++
			if cfg.doubleSpace && bodyWritten < bodyLines {
				if _, err := bw.WriteString("\n"); err != nil {
					return err
				}
				bodyWritten++
			}
		}
		if err := writePageFooter(bw, bodyWritten, bodyLines, cfg); err != nil {
			return err
		}
		page++
		if lineIdx >= len(lines) {
			break
		}
	}
	return nil
}

// writePageHeader writes the page header (5 lines).
func writePageHeader(bw *bufio.Writer, displayName string, page int, cfg prConfig) error {
	if cfg.omitHeader {
		return nil
	}
	// Two blank lines before header.
	if _, err := bw.WriteString("\n\n"); err != nil {
		return err
	}
	dateStr := formatDate()
	headerText := displayName
	if cfg.header != "" {
		headerText = cfg.header
	}
	indent := strings.Repeat(" ", cfg.indent)
	line := fmt.Sprintf("%s%s  %s  Page %d", indent, dateStr, headerText, page)
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	// Two blank lines after header.
	if _, err := bw.WriteString("\n\n\n"); err != nil {
		return err
	}
	return nil
}

// formatDate returns the current date formatted for pr headers.
func formatDate() string {
	now := time.Now()
	return now.Format("2006-01-02 15:04")
}

// writeBodyLine writes a single body line with optional numbering and indent.
func writeBodyLine(bw *bufio.Writer, line string, lineNum *int, cfg prConfig) error {
	indent := strings.Repeat(" ", cfg.indent)
	if _, err := bw.WriteString(indent); err != nil {
		return err
	}
	if cfg.numberLines {
		numStr := fmt.Sprintf("%*d%c", cfg.numberWidth, *lineNum, cfg.numberChar)
		if _, err := bw.WriteString(numStr); err != nil {
			return err
		}
		*lineNum++
	}
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// writePageFooter writes the page footer (padding + 5 blank lines).
func writePageFooter(bw *bufio.Writer, bodyWritten, bodyLines int, cfg prConfig) error {
	if cfg.omitPagination || cfg.omitHeader {
		return nil
	}
	// Pad remaining body lines.
	for bodyWritten < bodyLines {
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
		bodyWritten++
	}
	// Five footer blank lines.
	if _, err := bw.WriteString("\n\n\n\n\n"); err != nil {
		return err
	}
	return nil
}

// writeMultiColumn writes multi-column paginated output.
func writeMultiColumn(bw *bufio.Writer, lines []string, displayName string, cfg prConfig) error {
	bodyLines := bodyLinesPerPage(cfg)
	page := 1
	lineIdx := 0
	lineNum := 1
	colWidth := columnWidth(cfg)
	for lineIdx < len(lines) || page == 1 {
		if err := writePageHeader(bw, displayName, page, cfg); err != nil {
			return err
		}
		pageEnd := min(lineIdx+bodyLines*cfg.columns, len(lines))
		pageLines := lines[lineIdx:pageEnd]
		if cfg.across {
			if err := writeAcross(bw, pageLines, colWidth, &lineNum, cfg); err != nil {
				return err
			}
		} else {
			if err := writeDown(bw, pageLines, bodyLines, colWidth, &lineNum, cfg); err != nil {
				return err
			}
		}
		rowsWritten := bodyLines
		if len(pageLines) < bodyLines*cfg.columns {
			rowsWritten = (len(pageLines) + cfg.columns - 1) / cfg.columns
		}
		if err := writePageFooter(bw, rowsWritten, bodyLines, cfg); err != nil {
			return err
		}
		lineIdx = pageEnd
		page++
		if lineIdx >= len(lines) {
			break
		}
	}
	return nil
}

// columnWidth calculates the width of each column.
func columnWidth(cfg prConfig) int {
	sepLen := len(cfg.separator)
	if !cfg.separatorSet {
		sepLen = 1
	}
	usable := cfg.pageWidth - cfg.indent
	if cfg.numberLines {
		usable -= cfg.numberWidth + 1
	}
	return max((usable-sepLen*(cfg.columns-1))/cfg.columns, 1)
}

// writeAcross writes multi-column output filling across (row-major).
func writeAcross(bw *bufio.Writer, pageLines []string, colWidth int, lineNum *int, cfg prConfig) error {
	for i := 0; i < len(pageLines); i += cfg.columns {
		indent := strings.Repeat(" ", cfg.indent)
		if _, err := bw.WriteString(indent); err != nil {
			return err
		}
		if cfg.numberLines {
			numStr := fmt.Sprintf("%*d%c", cfg.numberWidth, *lineNum, cfg.numberChar)
			if _, err := bw.WriteString(numStr); err != nil {
				return err
			}
			*lineNum++
		}
		for c := 0; c < cfg.columns; c++ {
			idx := i + c
			text := ""
			if idx < len(pageLines) {
				text = pageLines[idx]
			}
			if c > 0 {
				if _, err := bw.WriteString(cfg.separator); err != nil {
					return err
				}
			}
			if err := writeColumnText(bw, text, colWidth, c == cfg.columns-1); err != nil {
				return err
			}
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

// writeDown writes multi-column output filling down (column-major).
func writeDown(bw *bufio.Writer, pageLines []string, bodyLines, colWidth int, lineNum *int, cfg prConfig) error {
	rows := min(bodyLines, len(pageLines))
	for row := range rows {
		indent := strings.Repeat(" ", cfg.indent)
		if _, err := bw.WriteString(indent); err != nil {
			return err
		}
		if cfg.numberLines {
			numStr := fmt.Sprintf("%*d%c", cfg.numberWidth, *lineNum, cfg.numberChar)
			if _, err := bw.WriteString(numStr); err != nil {
				return err
			}
			*lineNum++
		}
		for c := 0; c < cfg.columns; c++ {
			idx := row + c*bodyLines
			text := ""
			if idx < len(pageLines) {
				text = pageLines[idx]
			}
			if c > 0 {
				if _, err := bw.WriteString(cfg.separator); err != nil {
					return err
				}
			}
			isLast := c == cfg.columns-1
			if err := writeColumnText(bw, text, colWidth, isLast); err != nil {
				return err
			}
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

// writeColumnText writes text padded to colWidth, unless it's the last column.
func writeColumnText(bw *bufio.Writer, text string, colWidth int, isLast bool) error {
	if len(text) > colWidth {
		text = text[:colWidth]
	}
	if _, err := bw.WriteString(text); err != nil {
		return err
	}
	if !isLast && len(text) < colWidth {
		padding := strings.Repeat(" ", colWidth-len(text))
		if _, err := bw.WriteString(padding); err != nil {
			return err
		}
	}
	return nil
}
