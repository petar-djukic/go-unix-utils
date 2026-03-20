// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm R1.1–R1.4: three-column comparison of two sorted
// files, byte-for-byte under LC_ALL=C, with proper exhaustion handling.
// R2.1–R2.4: column suppression flags (-1, -2, -3) with indentation adjustment.
// R3.1–R3.4: order checking (--check-order, --nocheck-order) and --output-delimiter.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "comm"

// config holds parsed command-line options for comm.
type config struct {
	suppress1    bool   // R2.1: suppress column 1 (file1-only)
	suppress2    bool   // R2.2: suppress column 2 (file2-only)
	suppress3    bool   // R2.3: suppress column 3 (common)
	checkOrder   bool   // R3.2: fatal on unsorted input
	noCheckOrder bool   // R3.3: disable order checking entirely
	delimiter    string // R3.4: column separator (default "\t")
	prefix1      string // computed prefix for column 1 output
	prefix2      string // computed prefix for column 2 output
	prefix3      string // computed prefix for column 3 output
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes the comm comparison, returning exit code.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cfg, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	r1, c1, err := openFile(files[0], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close on read-only file
	}
	r2, c2, err := openFile(files[1], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close on read-only file
	}
	return compareFiles(r1, r2, stdout, stderr, cfg)
}

// parseArgs extracts flags and file operands from args.
// R2.1–R2.4: supports -1, -2, -3 flags for column suppression.
// R3.2–R3.4: supports --check-order, --nocheck-order, --output-delimiter.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{delimiter: "\t"}
	files := make([]string, 0, 2)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if err := parseFlag(arg, &cfg); err != nil {
			return config{}, nil, err
		}
	}
	if len(files) != 2 {
		return config{}, nil, fmt.Errorf("missing operand")
	}
	computePrefixes(&cfg)
	return cfg, files, nil
}

// parseFlag parses a single flag argument like "-1", "-2", "-3",
// combined forms like "-12", "-123", or long flags --check-order,
// --nocheck-order, --output-delimiter=STRING.
func parseFlag(arg string, cfg *config) error {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, cfg)
	}
	for _, ch := range arg[1:] {
		switch ch {
		case '1':
			cfg.suppress1 = true
		case '2':
			cfg.suppress2 = true
		case '3':
			cfg.suppress3 = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// parseLongFlag parses --check-order, --nocheck-order, and
// --output-delimiter=STRING. R3.2, R3.3, R3.4.
func parseLongFlag(arg string, cfg *config) error {
	switch {
	case arg == "--check-order":
		cfg.checkOrder = true
		return nil
	case arg == "--nocheck-order":
		cfg.noCheckOrder = true
		return nil
	case strings.HasPrefix(arg, "--output-delimiter="):
		cfg.delimiter = arg[len("--output-delimiter="):]
		return nil
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// computePrefixes sets the delimiter prefix for each column based on which
// columns are suppressed. R2.4: leftmost visible column has no delimiter;
// each subsequent visible column adds one delimiter.
func computePrefixes(cfg *config) {
	col := 0
	if !cfg.suppress1 {
		cfg.prefix1 = strings.Repeat(cfg.delimiter, col)
		col++
	}
	if !cfg.suppress2 {
		cfg.prefix2 = strings.Repeat(cfg.delimiter, col)
		col++
	}
	if !cfg.suppress3 {
		cfg.prefix3 = strings.Repeat(cfg.delimiter, col)
	}
}

// orderState tracks order checking state across file comparison.
// R3.1: default warns and exits 1 at end. R3.2: fatal immediately.
// R3.3: disabled entirely.
type orderState struct {
	prev1    string
	prev2    string
	hasPrev1 bool
	hasPrev2 bool
	violated bool // any order violation seen (for default mode exit code)
}

// compareFiles reads two sorted files and writes three-column output.
// R1.1–R1.4: column assignment by lexicographic comparison.
// R2.1–R2.4: suppressed columns are skipped; prefixes are pre-computed.
// R3.1–R3.3: order checking with configurable severity.
func compareFiles(r1, r2 io.Reader, stdout io.Writer, stderr io.Writer, cfg config) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	w := bufio.NewWriter(stdout)
	os := &orderState{}

	have1 := s1.Scan()
	have2 := s2.Scan()
	file1Adv, file2Adv := true, true

	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		if fatal := checkMainOrder(os, cfg, stderr, line1, line2, file1Adv, file2Adv); fatal {
			return flushAndExit(w, 1)
		}
		if err := emitPair(w, line1, line2, cfg); err != nil {
			return writeError(stderr, err)
		}
		cmp := compareLine(line1, line2)
		trackPrev(os, line1, line2, cmp)
		file1Adv = cmp <= 0
		file2Adv = cmp >= 0
		have1, have2 = advance(s1, s2, cmp)
	}
	if rc := drainWithOrder(w, s1, have1, os, 1, cfg, stderr); rc != 0 {
		return rc
	}
	if rc := drainWithOrder(w, s2, have2, os, 2, cfg, stderr); rc != 0 {
		return rc
	}
	if err := checkScanErr(s1, s2, stderr); err != nil {
		return 1
	}
	if err := w.Flush(); err != nil {
		return writeError(stderr, err)
	}
	return finalExitCode(os, cfg, stderr)
}

// checkMainOrder checks order for files that just advanced in the main loop.
// Returns true if processing should stop (--check-order mode).
func checkMainOrder(os *orderState, cfg config, stderr io.Writer, line1, line2 string, adv1, adv2 bool) bool {
	if cfg.noCheckOrder {
		return false
	}
	if adv1 && os.hasPrev1 && compareLine(os.prev1, line1) > 0 {
		reportOrderViolation(os, stderr, 1)
		if cfg.checkOrder {
			return true
		}
	}
	if adv2 && os.hasPrev2 && compareLine(os.prev2, line2) > 0 {
		reportOrderViolation(os, stderr, 2)
		if cfg.checkOrder {
			return true
		}
	}
	return false
}

// reportOrderViolation prints the file-specific order warning to stderr.
func reportOrderViolation(os *orderState, stderr io.Writer, fileNum int) {
	os.violated = true
	fmt.Fprintf(stderr, "%s: file %d is not in sorted order\n", progName, fileNum)
}

// trackPrev records current lines as previous based on which files will advance.
func trackPrev(os *orderState, line1, line2 string, cmp int) {
	if cmp <= 0 {
		os.prev1 = line1
		os.hasPrev1 = true
	}
	if cmp >= 0 {
		os.prev2 = line2
		os.hasPrev2 = true
	}
}

// finalExitCode returns the exit code after all processing.
// R3.1: default mode exits 1 if any order violation was detected.
func finalExitCode(os *orderState, cfg config, stderr io.Writer) int {
	if os.violated && !cfg.noCheckOrder {
		fmt.Fprintf(stderr, "%s: input is not in sorted order\n", progName)
		return 1
	}
	return 0
}

// drainWithOrder drains remaining lines from a scanner with order checking.
func drainWithOrder(w *bufio.Writer, s *bufio.Scanner, haveLine bool, os *orderState, fileNum int, cfg config, stderr io.Writer) int {
	suppress := (fileNum == 1 && cfg.suppress1) || (fileNum == 2 && cfg.suppress2)
	prefix := cfg.prefix1
	if fileNum == 2 {
		prefix = cfg.prefix2
	}
	prev, hasPrev := drainPrev(os, fileNum)
	if haveLine {
		if rc := drainOneLine(w, s.Text(), prev, hasPrev, suppress, prefix, os, fileNum, cfg, stderr); rc != 0 {
			return rc
		}
		prev = s.Text()
		hasPrev = true
	}
	for s.Scan() {
		if rc := drainOneLine(w, s.Text(), prev, hasPrev, suppress, prefix, os, fileNum, cfg, stderr); rc != 0 {
			return rc
		}
		prev = s.Text()
		hasPrev = true
	}
	return 0
}

// drainPrev returns the previous line and hasPrev flag for a file.
func drainPrev(os *orderState, fileNum int) (string, bool) {
	if fileNum == 1 {
		return os.prev1, os.hasPrev1
	}
	return os.prev2, os.hasPrev2
}

// drainOneLine processes one line during drain: checks order, then emits.
// For --check-order, stops before emitting the violating line.
// For default, emits the line and sets the violation flag.
func drainOneLine(w *bufio.Writer, line, prev string, hasPrev, suppress bool, prefix string, os *orderState, fileNum int, cfg config, stderr io.Writer) int {
	if !cfg.noCheckOrder && hasPrev && compareLine(prev, line) > 0 {
		reportOrderViolation(os, stderr, fileNum)
		if cfg.checkOrder {
			return flushAndExit(w, 1)
		}
	}
	if suppress {
		return 0
	}
	if err := writeLine(w, prefix, line); err != nil {
		return writeError(stderr, err)
	}
	return 0
}

// flushAndExit flushes the writer and returns the given exit code.
func flushAndExit(w *bufio.Writer, code int) int {
	w.Flush() // best-effort flush before exit
	return code
}

// emitPair writes the appropriate column output for two current lines.
// R2.1–R2.3: suppressed columns produce no output.
func emitPair(w *bufio.Writer, line1, line2 string, cfg config) error {
	cmp := compareLine(line1, line2)
	switch {
	case cmp < 0:
		if cfg.suppress1 {
			return nil
		}
		return writeLine(w, cfg.prefix1, line1)
	case cmp > 0:
		if cfg.suppress2 {
			return nil
		}
		return writeLine(w, cfg.prefix2, line2)
	default:
		if cfg.suppress3 {
			return nil
		}
		return writeLine(w, cfg.prefix3, line1)
	}
}

// advance moves the appropriate scanner(s) forward based on comparison.
func advance(s1, s2 *bufio.Scanner, cmp int) (bool, bool) {
	switch {
	case cmp < 0:
		return s1.Scan(), true
	case cmp > 0:
		return true, s2.Scan()
	default:
		return s1.Scan(), s2.Scan()
	}
}

// compareLine returns <0 if a<b, 0 if a==b, >0 if a>b (byte-for-byte).
func compareLine(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// writeLine writes a prefix and line followed by a newline.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// checkScanErr checks both scanners for read errors.
func checkScanErr(s1, s2 *bufio.Scanner, stderr io.Writer) error {
	if err := s1.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return err
	}
	if err := s2.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return err
	}
	return nil
}

// openFile opens a file for reading; "-" means stdin.
func openFile(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, unwrapPathError(err)
	}
	return f, f, nil
}

// writeError reports a write error and returns exit code 1.
func writeError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
	return 1
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
