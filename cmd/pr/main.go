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
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	// defaultPageLength is the default number of lines per page (R1.1).
	defaultPageLength = 66
	// defaultHeaderLines is the number of header lines per page (R1.1).
	defaultHeaderLines = 5
	// defaultFooterLines is the number of footer lines per page (R1.1).
	defaultFooterLines = 5
	// defaultPageWidth is the default page width in columns (R4.2).
	defaultPageWidth = 72
	// defaultNumberWidth is the default line number field width (R4.1).
	defaultNumberWidth = 5
	// defaultNumberChar is the default line number separator (R4.1).
	defaultNumberChar = '\t'
	// progName is the program name for error messages.
	progName = "pr"
)

// config holds all parsed command-line options for pr.
type config struct {
	pageLength     int    // R2.1: -l N, page length in lines (default 66)
	header         string // R2.1: -h HEADER, custom header text
	omitHeader     bool   // R2.2: -t, suppress header and footer
	omitPagination bool   // R2.2: -T, suppress header/footer and page padding
	columns        int    // R3.1: -COLUMN, number of columns (default 1)
	across         bool   // R3.1: -a, fill columns across instead of down
	numberLines    bool   // R4.1: -n, number output lines
	numberChar     byte   // R4.1: separator char after number (default tab)
	numberWidth    int    // R4.1: number field width (default 5)
	indent         int    // R4.2: -o MARGIN, left margin indent
	pageWidth      int    // R4.2: -w WIDTH, page width (default 72)
	doubleSpace    bool   // R4.3: -d, double-space output
	separator      string // R4.3: -s CHAR, column separator (default tab)
}

// defaultConfig returns the default pr configuration.
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

	// R2.1: page length and header
	pageLength := fs.Int("l", defaultPageLength, "page length")
	header := fs.String("h", "", "header string")

	// R2.2: omit header/pagination
	omitHeader := fs.Bool("t", false, "omit header and footer")
	omitPagination := fs.Bool("T", false, "omit pagination")

	// R3.1: columns and across
	columns := fs.Int("columns", 1, "number of columns")
	across := fs.Bool("a", false, "fill columns across")

	// R4.1: line numbering
	numberLines := fs.Bool("n", false, "number lines")

	// R4.2: indent and width
	indent := fs.Int("o", 0, "indent margin")
	pageWidth := fs.Int("w", defaultPageWidth, "page width")

	// R4.3: double-space and separator
	doubleSpace := fs.Bool("d", false, "double-space output")
	separator := fs.String("s", "\t", "column separator")

	// R3.1: check for -COLUMN numeric flag before parsing
	args := preprocessColumnFlag(os.Args[1:], &cfg)

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	cfg.pageLength = *pageLength
	cfg.header = *header
	cfg.omitHeader = *omitHeader
	cfg.omitPagination = *omitPagination
	if *columns > 1 {
		cfg.columns = *columns
	}
	cfg.across = *across
	cfg.numberLines = *numberLines
	cfg.indent = *indent
	cfg.pageWidth = *pageWidth
	cfg.doubleSpace = *doubleSpace
	cfg.separator = *separator

	return cfg, fs.Args()
}

// preprocessColumnFlag extracts -N column flags from args before flag parsing.
// R3.1: -COLUMN syntax sets the number of columns.
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

// bodyLines returns the number of body lines per page given the config.
// R1.1: body = pageLength - headerLines - footerLines.
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

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R2.3: stdin when filename is "-" or no files given.
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

// formatOpenError extracts the underlying error for GNU-compatible messages.
func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// formatHeader formats a page header line.
// R1.1: header shows date, filename, and page number.
func formatHeader(_ config, _ string, _ int) string {
	log.Fatal("not implemented")
	return ""
}

// writeHeader writes the page header to the output.
// R1.1: 5-line header block.
func writeHeader(_ *bufio.Writer, _ config, _ string, _ int) error {
	log.Fatal("not implemented")
	return nil
}

// writeFooter writes the page footer to the output.
// R1.1: 5-line footer block.
func writeFooter(_ *bufio.Writer, _ config) error {
	log.Fatal("not implemented")
	return nil
}

// paginateFile reads a file and writes paginated output.
// R1.1: formats input into pages with header, body, footer.
func paginateFile(_ io.Reader, _ *bufio.Writer, _ config, _ string) error {
	log.Fatal("not implemented")
	return nil
}

// formatColumns arranges lines into multi-column layout.
// R3.1: columns filled down by default, across with -a.
func formatColumns(_ []string, _ config) []string {
	log.Fatal("not implemented")
	return nil
}

// numberLine prepends a line number to a line.
// R4.1: number with configurable separator and width.
func numberLine(_ string, _ int, _ config) string {
	log.Fatal("not implemented")
	return ""
}

// indentLine prepends margin spaces to a line.
// R4.2: -o MARGIN indents each line.
func indentLine(_ string, _ int) string {
	log.Fatal("not implemented")
	return ""
}

// doubleSpaceLine adds an extra newline after a line.
// R4.3: -d double-spaces output.
func doubleSpaceLine(line string) string {
	log.Fatal("not implemented")
	_ = line
	return ""
}

// prFile processes a single file through the pr pipeline.
func prFile(name string, w *bufio.Writer, cfg config) error {
	log.Fatal("not implemented")
	_ = name
	_ = w
	_ = cfg
	return nil
}

// run processes all files and returns the exit code.
// R5.1: exit 0 on success, exit 1 on any error.
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
	// R5.2: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	cfg, args := parseFlags()

	// R2.3: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	os.Exit(run(cfg, args))
}

// init suppresses unused import warnings.
var _ = strings.TrimSpace
