// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd023-fold: Wrap Long Lines to a Specified Width.
// Covers R1.1-R1.4 (core line wrapping, stdin/file reading),
// R2.1 (-w width flag), R2.2 (column-mode tab handling),
// R2.3 (-b byte-mode flag), R3.1-R3.4 (-s space-break mode),
// R4.1-R4.4 (exit codes and SIGPIPE).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const tabSize = 8

// foldConfig holds parsed command-line options.
type foldConfig struct {
	width    int  // R2.1: fold width, default 80
	byteMode bool // R2.3: count bytes instead of columns
	spaces   bool // R3.1: break at last space within width
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fold: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(cfg, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses fold flags and returns config and file list.
// R1.2: stdin when no files or "-" given.
func parseArgs(args []string) (foldConfig, []string, error) {
	cfg := foldConfig{width: 80}
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
		consumed, err := parseFoldFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, files, nil
}

// parseFoldFlag parses a single flag and returns args consumed.
// R3.4: rejects unrecognized options.
func parseFoldFlag(arg string, args []string, i int, cfg *foldConfig) (int, error) {
	switch {
	case arg == "-b" || arg == "--bytes":
		cfg.byteMode = true
		return 1, nil
	case arg == "-s" || arg == "--spaces":
		// R3.1: enable space-break mode.
		cfg.spaces = true
		return 1, nil
	case strings.HasPrefix(arg, "-w"):
		return parseShortWidth(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--width"):
		return parseLongWidth(arg, args, i, cfg)
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

// parseShortWidth parses -w N or -wN form.
func parseShortWidth(arg string, args []string, i int, cfg *foldConfig) (int, error) {
	if len(arg) > 2 {
		return 1, applyWidth(arg[2:], cfg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'w'")
	}
	return 2, applyWidth(args[i+1], cfg)
}

// parseLongWidth parses --width N or --width=N form.
func parseLongWidth(arg string, args []string, i int, cfg *foldConfig) (int, error) {
	if strings.HasPrefix(arg, "--width=") {
		return 1, applyWidth(arg[len("--width="):], cfg)
	}
	if arg != "--width" {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '--width' requires an argument")
	}
	return 2, applyWidth(args[i+1], cfg)
}

// applyWidth validates and sets the fold width.
// R3.1 (task): invalid width values produce an error to stderr and exit 1.
func applyWidth(val string, cfg *foldConfig) error {
	w, err := strconv.Atoi(val)
	if err != nil || w <= 0 {
		return fmt.Errorf("invalid number of columns: '%s'", val)
	}
	cfg.width = w
	return nil
}

// run processes all files and returns the exit code.
// R1.1: reads stdin when no files given.
// R1.3: concatenates output from multiple files.
// R4.1/R4.2: exit 0 on success, 1 on any error.
func run(cfg foldConfig, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := processOneFile(name, stdin, bw, cfg); err != nil {
			fmt.Fprintf(stderr, "fold: %s\n", err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processOneFile opens and folds a single file.
// R3.2 (task): prints error and continues for nonexistent files.
func processOneFile(name string, stdin io.Reader, bw *bufio.Writer, cfg foldConfig) error {
	r, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return foldInput(r, bw, cfg)
}

// openInput opens a file or returns stdin for "-".
// R1.2: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, unwrapOSError(err))
	}
	return f, nil
}

// unwrapOSError extracts the underlying error message from an os.PathError.
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// foldInput dispatches to the appropriate folding strategy.
func foldInput(r io.Reader, bw *bufio.Writer, cfg foldConfig) error {
	br := bufio.NewReader(r)
	switch {
	case cfg.byteMode && cfg.spaces:
		return foldBytesSpaces(br, bw, cfg.width)
	case cfg.byteMode:
		return foldBytes(br, bw, cfg.width)
	case cfg.spaces:
		return foldColumnsSpaces(br, bw, cfg.width)
	default:
		return foldColumns(br, bw, cfg.width)
	}
}

// --- Non-space-break fold functions ---

// foldBytes folds input counting bytes.
// R2.3: each byte counts as 1 regardless of character type.
func foldBytes(br *bufio.Reader, bw *bufio.Writer, width int) error {
	col := 0
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if c == '\n' {
			if wErr := bw.WriteByte('\n'); wErr != nil {
				return wErr
			}
			col = 0
			continue
		}
		if col >= width {
			if wErr := bw.WriteByte('\n'); wErr != nil {
				return wErr
			}
			col = 0
		}
		if wErr := bw.WriteByte(c); wErr != nil {
			return wErr
		}
		col++
	}
}

// foldColumns folds input counting display columns.
// R2.2: tabs expand to next 8-column tab stop.
func foldColumns(br *bufio.Reader, bw *bufio.Writer, width int) error {
	col := 0
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		col, err = foldOneColumn(c, col, width, bw)
		if err != nil {
			return err
		}
	}
}

// foldOneColumn processes one byte in column mode and returns updated column.
// R2.2: handles tab stops, backspace, carriage return per GNU fold behavior.
func foldOneColumn(c byte, col, width int, bw *bufio.Writer) (int, error) {
	switch c {
	case '\n':
		err := bw.WriteByte('\n')
		return 0, err
	case '\t':
		return foldTab(col, width, bw)
	case '\b':
		err := bw.WriteByte(c)
		if col > 0 {
			col--
		}
		return col, err
	case '\r':
		err := bw.WriteByte(c)
		return 0, err
	default:
		return foldRegularByte(c, col, width, bw)
	}
}

// foldRegularByte handles a non-special byte in column mode.
func foldRegularByte(c byte, col, width int, bw *bufio.Writer) (int, error) {
	if col == width {
		if err := bw.WriteByte('\n'); err != nil {
			return 0, err
		}
		col = 0
	}
	if err := bw.WriteByte(c); err != nil {
		return 0, err
	}
	return col + 1, nil
}

// foldTab handles a tab character in column mode.
// R2.2: tab advances to next 8-column tab stop.
func foldTab(col, width int, bw *bufio.Writer) (int, error) {
	newCol := nextTabStop(col)
	if newCol > width {
		if err := bw.WriteByte('\n'); err != nil {
			return 0, err
		}
		newCol = tabSize
	}
	if err := bw.WriteByte('\t'); err != nil {
		return 0, err
	}
	return newCol, nil
}

// nextTabStop returns the column position of the next tab stop after col.
func nextTabStop(col int) int {
	return (col/tabSize + 1) * tabSize
}

// --- Space-break mode (-s) functions ---

// foldBytesSpaces folds input counting bytes with space-breaking.
// R3.1: breaks at last space within width.
// R3.4: compatible with -b byte counting.
func foldBytesSpaces(br *bufio.Reader, bw *bufio.Writer, width int) error {
	var buf []byte
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				_, wErr := bw.Write(buf)
				return wErr
			}
			return err
		}
		if c == '\n' {
			if wErr := writeBufNewline(buf, bw); wErr != nil {
				return wErr
			}
			buf = buf[:0]
			continue
		}
		buf = append(buf, c)
		for len(buf) >= width {
			var wErr error
			buf, wErr = breakBytesAtSpace(buf, width, bw)
			if wErr != nil {
				return wErr
			}
		}
	}
}

// breakBytesAtSpace breaks a byte buffer at the last space within width.
// R3.2: falls back to hard break if no space found.
// R3.3: space is written as last character before the newline.
func breakBytesAtSpace(buf []byte, width int, bw *bufio.Writer) ([]byte, error) {
	breakIdx := lastSpaceIn(buf, width)
	if breakIdx >= 0 {
		return writeBreak(buf, breakIdx+1, bw)
	}
	return writeBreak(buf, width, bw)
}

// foldColumnsSpaces folds input counting columns with space-breaking.
// R3.1: breaks at last space within width.
func foldColumnsSpaces(br *bufio.Reader, bw *bufio.Writer, width int) error {
	var buf []byte
	col := 0
	lastSpIdx := -1
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				_, wErr := bw.Write(buf)
				return wErr
			}
			return err
		}
		if c == '\n' {
			if wErr := writeBufNewline(buf, bw); wErr != nil {
				return wErr
			}
			buf, col, lastSpIdx = nil, 0, -1
			continue
		}
		newCol := advanceColumn(c, col)
		buf, col, lastSpIdx, err = handleColOverflow(
			buf, col, lastSpIdx, c, newCol, width, bw,
		)
		if err != nil {
			return err
		}
		buf = append(buf, c)
		col = advanceColumn(c, col)
		if c == ' ' {
			lastSpIdx = len(buf) - 1
		}
	}
}

// handleColOverflow breaks the buffer when adding c would exceed width.
// R3.2: falls back to hard break when no space exists in the buffer.
func handleColOverflow(
	buf []byte, col, lastSpIdx int, c byte, newCol, width int,
	bw *bufio.Writer,
) ([]byte, int, int, error) {
	for newCol > width && len(buf) > 0 {
		var err error
		if lastSpIdx >= 0 {
			buf, err = writeBreak(buf, lastSpIdx+1, bw)
		} else {
			buf, err = writeBreak(buf, len(buf), bw)
		}
		if err != nil {
			return nil, 0, 0, err
		}
		col = computeColumns(buf)
		lastSpIdx = findLastSpace(buf)
		newCol = advanceColumn(c, col)
	}
	return buf, col, lastSpIdx, nil
}

// --- Shared helpers ---

// writeBreak writes buf[:n] followed by a newline and returns the remainder.
func writeBreak(buf []byte, n int, bw *bufio.Writer) ([]byte, error) {
	if _, err := bw.Write(buf[:n]); err != nil {
		return nil, err
	}
	if err := bw.WriteByte('\n'); err != nil {
		return nil, err
	}
	remaining := make([]byte, len(buf)-n)
	copy(remaining, buf[n:])
	return remaining, nil
}

// writeBufNewline writes buf contents followed by a newline.
func writeBufNewline(buf []byte, bw *bufio.Writer) error {
	if _, err := bw.Write(buf); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// advanceColumn returns the column position after byte c at column col.
func advanceColumn(c byte, col int) int {
	switch c {
	case '\t':
		return nextTabStop(col)
	case '\b':
		if col > 0 {
			return col - 1
		}
		return 0
	case '\r':
		return 0
	default:
		return col + 1
	}
}

// computeColumns calculates the total column width of a byte buffer.
func computeColumns(buf []byte) int {
	col := 0
	for _, c := range buf {
		col = advanceColumn(c, col)
	}
	return col
}

// lastSpaceIn returns the index of the last space in buf[0:limit], or -1.
func lastSpaceIn(buf []byte, limit int) int {
	for j := limit - 1; j >= 0; j-- {
		if buf[j] == ' ' {
			return j
		}
	}
	return -1
}

// findLastSpace returns the index of the last space in buf, or -1.
func findLastSpace(buf []byte) int {
	for j := len(buf) - 1; j >= 0; j-- {
		if buf[j] == ' ' {
			return j
		}
	}
	return -1
}
