// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/comm: compare two sorted files line by line.
// Implements srd029-comm R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	programName  = "comm"
	defaultDelim = "\t"
)

// config holds parsed command-line options for comm.
type config struct {
	suppress1     bool   // -1: suppress column 1
	suppress2     bool   // -2: suppress column 2
	suppress3     bool   // -3: suppress column 3
	checkOrder    int    // 0=default(warn), 1=fatal, -1=nocheck
	outputDelim   string // --output-delimiter
	hasOutputDelim bool  // R3.4: true when --output-delimiter was explicitly set
	total         bool   // --total
	zeroTerm      bool   // --zero-terminated
	showHelp      bool
	showVersion   bool
	files         []string
}

// lineReader wraps a bufio.Scanner with a "peek" line buffer so
// the merge loop can test the current line without consuming it.
type lineReader struct {
	sc       *bufio.Scanner
	line     string
	valid    bool
	prevLine string // for order checking
}

// next advances the reader. Returns true if a line is available.
func (lr *lineReader) next() bool {
	lr.valid = lr.sc.Scan()
	if lr.valid {
		lr.line = lr.sc.Text()
	}
	return lr.valid
}

// R4.4: SIGPIPE handler installed at start.
// R1.1: main entry with flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	if cfg.showVersion {
		printVersion()
		return
	}
	if cfg.showHelp {
		printUsage()
		return
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}
}

// printVersion prints version to stdout.
func printVersion() {
	fmt.Println("comm (go-unix-utils) 0.1.0")
}

// printUsage prints the help message to stdout.
func printUsage() {
	fmt.Print(`Usage: comm [OPTION]... FILE1 FILE2
Compare sorted files FILE1 and FILE2 line by line.

  -1              suppress column 1 (lines unique to FILE1)
  -2              suppress column 2 (lines unique to FILE2)
  -3              suppress column 3 (lines common to both)
  --check-order   check that the input is correctly sorted
  --nocheck-order do not check that the input is correctly sorted
  --output-delimiter=STR  separate columns with STR
  --total         output a summary
  --zero-terminated       line delimiter is NUL, not newline
  --help          display this help and exit
  --version       output version information and exit
`)
}

// run executes the comm comparison logic.
// R4.1: returns nil on success (exit 0).
func run(cfg config) error {
	r1, err := openFile(cfg.files[0])
	if err != nil {
		return err
	}
	defer closeFile(r1, cfg.files[0])

	r2, err := openFile(cfg.files[1])
	if err != nil {
		return err
	}
	defer closeFile(r2, cfg.files[1])

	w := bufio.NewWriter(os.Stdout)
	delim := resolveDelim(cfg)
	terminator := resolveTerminator(cfg)

	err = compareFiles(r1, r2, w, cfg, delim, terminator)
	if flushErr := w.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	return err
}

// resolveDelim returns the column delimiter string.
// R3.4: --output-delimiter replaces the default tab, even if empty.
func resolveDelim(cfg config) string {
	if cfg.hasOutputDelim {
		return cfg.outputDelim
	}
	return defaultDelim
}

// resolveTerminator returns the line terminator byte.
func resolveTerminator(cfg config) byte {
	if cfg.zeroTerm {
		return 0
	}
	return '\n'
}

// openFile opens a file for reading; "-" means stdin.
// R4.2: returns error if the file cannot be opened.
func openFile(name string) (io.ReadCloser, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, unwrapPathErr(err))
	}
	return f, nil
}

// closeFile closes a reader if it's not stdin.
func closeFile(r io.ReadCloser, name string) {
	if name != "-" {
		r.Close() // best-effort close
	}
}

// unwrapPathErr extracts the inner error from a PathError.
func unwrapPathErr(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// compareState tracks the state of the two-file comparison.
type compareState struct {
	w              *bufio.Writer
	cfg            config
	delim          string
	terminator     byte
	total1         int
	total2         int
	total3         int
	orderViolated  bool // R3.1: tracks if any order violation was detected
}

// compareFiles performs the merge-like comparison of two sorted readers.
// R1.2: merge-compares lines from both files.
// R1.3: remaining lines go to the appropriate column when one file exhausts.
func compareFiles(r1, r2 io.ReadCloser, w *bufio.Writer, cfg config, delim string, terminator byte) error {
	s := &compareState{w: w, cfg: cfg, delim: delim, terminator: terminator}
	lr1 := newLineReader(r1, terminator)
	lr2 := newLineReader(r2, terminator)

	lr1.next()
	lr2.next()

	for lr1.valid && lr2.valid {
		if err := s.checkOrders(lr1, lr2); err != nil {
			return err
		}
		if err := s.mergePair(lr1, lr2); err != nil {
			return err
		}
	}
	if err := s.drainReader(lr1, 1); err != nil {
		return err
	}
	if err := s.drainReader(lr2, 2); err != nil {
		return err
	}
	if err := lr1.sc.Err(); err != nil {
		return err
	}
	if err := lr2.sc.Err(); err != nil {
		return err
	}
	if err := s.writeTotal(); err != nil {
		return err
	}
	// R3.1: default mode exits non-zero after all processing if violations found
	if s.orderViolated {
		return fmt.Errorf("input is not in sorted order")
	}
	return nil
}

// newLineReader creates a lineReader with the appropriate split function.
func newLineReader(r io.Reader, terminator byte) *lineReader {
	sc := bufio.NewScanner(r)
	if terminator == 0 {
		sc.Split(scanNullTerminated)
	}
	return &lineReader{sc: sc}
}

// mergePair compares current lines and advances the appropriate reader(s).
// R1.2: less = col1, greater = col2, equal = col3.
func (s *compareState) mergePair(lr1, lr2 *lineReader) error {
	cmp := strings.Compare(lr1.line, lr2.line)
	switch {
	case cmp < 0:
		if err := s.writeColumn(1, lr1.line); err != nil {
			return err
		}
		lr1.prevLine = lr1.line
		lr1.next()
	case cmp > 0:
		if err := s.writeColumn(2, lr2.line); err != nil {
			return err
		}
		lr2.prevLine = lr2.line
		lr2.next()
	default:
		if err := s.writeColumn(3, lr1.line); err != nil {
			return err
		}
		lr1.prevLine = lr1.line
		lr2.prevLine = lr2.line
		lr1.next()
		lr2.next()
	}
	return nil
}

// checkOrders validates sort order for both files.
// R3.3: --nocheck-order disables checking entirely.
// R3.1: default warns on out-of-order.
// R3.2: --check-order is fatal.
func (s *compareState) checkOrders(lr1, lr2 *lineReader) error {
	if s.cfg.checkOrder == -1 {
		return nil
	}
	if err := s.checkOneOrder(lr1, 1); err != nil {
		return err
	}
	return s.checkOneOrder(lr2, 2)
}

// checkOneOrder checks if the current line is in order relative to prev.
func (s *compareState) checkOneOrder(lr *lineReader, fileNum int) error {
	if lr.prevLine == "" || lr.line >= lr.prevLine {
		return nil
	}
	msg := fmt.Sprintf("file %d is not in sorted order", fileNum)
	if s.cfg.checkOrder == 1 {
		return fmt.Errorf("%s", msg)
	}
	// R3.1: default mode prints warning to stderr, continues, exits non-zero at end
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	s.orderViolated = true
	return nil
}

// drainReader outputs all remaining lines from a reader.
// R1.3: when one file is exhausted, remaining lines go to the right column.
func (s *compareState) drainReader(lr *lineReader, col int) error {
	for lr.valid {
		if err := s.writeColumn(col, lr.line); err != nil {
			return err
		}
		lr.next()
	}
	return nil
}

// writeColumn writes a line to the appropriate column with indentation.
// R1.3: col1=no indent, col2=one delim, col3=two delims.
// R2.4: suppressed columns shift indentation left.
func (s *compareState) writeColumn(col int, line string) error {
	switch col {
	case 1:
		s.total1++
		if s.cfg.suppress1 {
			return nil
		}
	case 2:
		s.total2++
		if s.cfg.suppress2 {
			return nil
		}
	case 3:
		s.total3++
		if s.cfg.suppress3 {
			return nil
		}
	}
	prefix := s.columnPrefix(col)
	_, err := fmt.Fprintf(s.w, "%s%s%c", prefix, line, s.terminator)
	return err
}

// columnPrefix returns the delimiter prefix for a given column.
// R2.4: indentation adjusts based on which columns are suppressed.
func (s *compareState) columnPrefix(col int) string {
	n := 0
	switch col {
	case 2:
		if !s.cfg.suppress1 {
			n = 1
		}
	case 3:
		if !s.cfg.suppress1 {
			n++
		}
		if !s.cfg.suppress2 {
			n++
		}
	}
	return strings.Repeat(s.delim, n)
}

// writeTotal outputs the --total summary line if requested.
func (s *compareState) writeTotal() error {
	if !s.cfg.total {
		return nil
	}
	_, err := fmt.Fprintf(s.w, "%d%s%d%s%d%stotal%c",
		s.total1, s.delim, s.total2, s.delim, s.total3, s.delim, s.terminator)
	return err
}

// scanNullTerminated is a bufio.SplitFunc for NUL-terminated lines.
func scanNullTerminated(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == 0 {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// --- Flag parsing ---

// parseArgs parses command-line arguments into config.
// R1.1: parses -1, -2, -3, --check-order, --nocheck-order,
// --output-delimiter, --total, --zero-terminated, --help, --version.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") && arg != "-") {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		skip, err := parseFlag(&cfg, arg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	return cfg, validateArgs(cfg)
}

// validateArgs checks that exactly two file arguments were given.
// R1.1: exit 1 if fewer or more than two files.
func validateArgs(cfg config) error {
	if cfg.showHelp || cfg.showVersion {
		return nil
	}
	if len(cfg.files) < 2 {
		if len(cfg.files) == 0 {
			return fmt.Errorf("missing operand")
		}
		return fmt.Errorf("missing operand after '%s'", cfg.files[0])
	}
	if len(cfg.files) > 2 {
		return fmt.Errorf("extra operand '%s'", cfg.files[2])
	}
	return nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, arg string, args []string, i int) (int, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, arg, args, i)
	}
	return parseShortFlags(cfg, arg[1:])
}

// parseLongFlag handles --name and --name=value flags.
func parseLongFlag(cfg *config, arg string, args []string, i int) (int, error) {
	switch {
	case arg == "--check-order":
		cfg.checkOrder = 1
		return 0, nil
	case arg == "--nocheck-order":
		cfg.checkOrder = -1
		return 0, nil
	case arg == "--total":
		cfg.total = true
		return 0, nil
	case arg == "--zero-terminated":
		cfg.zeroTerm = true
		return 0, nil
	case arg == "--help":
		cfg.showHelp = true
		return 0, nil
	case arg == "--version":
		cfg.showVersion = true
		return 0, nil
	case strings.HasPrefix(arg, "--output-delimiter"):
		return parseOutputDelim(cfg, arg, args, i)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseOutputDelim parses --output-delimiter=STR or --output-delimiter STR.
// R3.4: replaces tab with custom delimiter.
func parseOutputDelim(cfg *config, arg string, args []string, i int) (int, error) {
	if strings.HasPrefix(arg, "--output-delimiter=") {
		cfg.outputDelim = arg[len("--output-delimiter="):]
		cfg.hasOutputDelim = true
		return 0, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '--output-delimiter' requires an argument")
	}
	cfg.outputDelim = args[i+1]
	cfg.hasOutputDelim = true
	return 1, nil
}

// parseShortFlags processes bundled short flags like -12.
// R1.1: -1, -2, -3 can be combined.
func parseShortFlags(cfg *config, flags string) (int, error) {
	for _, ch := range flags {
		switch ch {
		case '1':
			cfg.suppress1 = true
		case '2':
			cfg.suppress2 = true
		case '3':
			cfg.suppress3 = true
		case 'z':
			cfg.zeroTerm = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
