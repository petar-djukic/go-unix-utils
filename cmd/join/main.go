// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/join implements GNU join: join lines of two files on a common field.
//
// Implements prd069-join R1.1 (default join on first field),
// R1.2 (whitespace field separator, space output separator),
// R1.3 (suppress unpairable lines by default),
// R1.4 (stdin via '-'),
// R2.1 (-1/-2 field selection),
// R2.2 (-j combined field),
// R2.3 (-o output format),
// R2.4 (-t custom separator),
// R3.1 (-a unpairable lines),
// R3.2 (-v unpairable only),
// R3.3 (-e empty field replacement),
// R3.4 (--header).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "join"

// outputSpec represents a single output field specifier.
type outputSpec struct {
	joinField bool // true when specifier is '0'
	fileNum   int  // 1 or 2
	fieldNum  int  // 1-based field number
}

// joinConfig holds parsed flags for a join invocation.
type joinConfig struct {
	file1      string
	file2      string
	field1     int          // R2.1: 1-based join field for file 1 (default 1)
	field2     int          // R2.1: 1-based join field for file 2 (default 1)
	sep        string       // R2.4: field separator ("" means whitespace)
	hasSep     bool         // R2.4: true when -t was specified
	outputFmt  []outputSpec // R2.3: output format specifiers
	hasOutFmt  bool         // R2.3: true when -o was specified
	unpair1    bool         // R3.1: print unpairable lines from file 1
	unpair2    bool         // R3.1: print unpairable lines from file 2
	onlyUnpair bool         // R3.2: suppress paired lines (set when -v used)
	empty      string       // R3.3: replacement for missing fields
	hasEmpty   bool         // R3.3: true when -e was specified
	header     bool         // R3.4: treat first line as header
}

// lineReader wraps a bufio.Scanner with field splitting for join operations.
type lineReader struct {
	scanner *bufio.Scanner
	fields  []string
	rawLine string
	hasLine bool
	joinIdx int    // 0-based index of join field
	sep     string // field separator
	hasSep  bool   // true when using explicit separator
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments, opens files, and performs the join.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	return executeJoin(cfg, stdin, stdout, stderr)
}

// executeJoin opens files and runs the join operation.
func executeJoin(cfg joinConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	r1, c1, err := openFile(cfg.file1, stdin)
	if err != nil {
		printFileError(stderr, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close
	}
	r2, c2, err := openFile(cfg.file2, stdin)
	if err != nil {
		printFileError(stderr, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close
	}
	if err := joinFiles(r1, r2, stdout, cfg); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// parseArgs extracts flags and the two file operands.
func parseArgs(args []string) (joinConfig, error) {
	cfg := joinConfig{field1: 1, field2: 1}
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		var err error
		i, err = parseFlag(args, i, &cfg)
		if err != nil {
			return cfg, err
		}
	}
	if len(files) < 2 {
		return cfg, fmt.Errorf("missing operand")
	}
	if len(files) > 2 {
		return cfg, fmt.Errorf("extra operand '%s'", files[2])
	}
	cfg.file1 = files[0]
	cfg.file2 = files[1]
	return cfg, nil
}

// parseFlag handles a single flag at position i, returning the new index.
func parseFlag(args []string, i int, cfg *joinConfig) (int, error) {
	arg := args[i]
	switch {
	case arg == "-1":
		return parseFlagField(args, i, &cfg.field1)
	case arg == "-2":
		return parseFlagField(args, i, &cfg.field2)
	case arg == "-j":
		idx, err := parseFlagField(args, i, &cfg.field1)
		if err != nil {
			return idx, err
		}
		cfg.field2 = cfg.field1
		return idx, nil
	case arg == "-t":
		return parseFlagSep(args, i, cfg)
	case arg == "-o":
		return parseFlagOutput(args, i, cfg)
	case arg == "-a":
		return parseFlagFileNum(args, i, &cfg.unpair1, &cfg.unpair2)
	case arg == "-v":
		idx, err := parseFlagFileNum(args, i, &cfg.unpair1, &cfg.unpair2)
		if err != nil {
			return idx, err
		}
		cfg.onlyUnpair = true
		return idx, nil
	case arg == "-e":
		return parseFlagEmpty(args, i, cfg)
	case arg == "--header":
		cfg.header = true
		return i, nil
	default:
		return i, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseFlagField parses a field number argument for -1, -2, or -j.
func parseFlagField(args []string, i int, target *int) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("option '%s' requires an argument", args[i])
	}
	n, err := strconv.Atoi(args[i+1])
	if err != nil || n < 1 {
		return i + 1, fmt.Errorf("invalid field number: '%s'", args[i+1])
	}
	*target = n
	return i + 1, nil
}

// parseFlagSep parses the -t separator argument. R2.4.
func parseFlagSep(args []string, i int, cfg *joinConfig) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("option '-t' requires an argument")
	}
	cfg.sep = args[i+1]
	cfg.hasSep = true
	return i + 1, nil
}

// parseFlagOutput parses the -o output format argument. R2.3.
func parseFlagOutput(args []string, i int, cfg *joinConfig) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("option '-o' requires an argument")
	}
	i++
	specs, err := parseOutputSpecs(args[i])
	if err != nil {
		return i, err
	}
	cfg.outputFmt = append(cfg.outputFmt, specs...)
	cfg.hasOutFmt = true
	// Consume additional space-separated specifiers (non-flag args).
	for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
		specs, err = parseOutputSpecs(args[i+1])
		if err != nil {
			break
		}
		cfg.outputFmt = append(cfg.outputFmt, specs...)
		i++
	}
	return i, nil
}

// parseFlagFileNum parses -a or -v FILENUM argument. R3.1, R3.2.
func parseFlagFileNum(args []string, i int, flag1, flag2 *bool) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("option '%s' requires an argument", args[i])
	}
	switch args[i+1] {
	case "1":
		*flag1 = true
	case "2":
		*flag2 = true
	default:
		return i + 1, fmt.Errorf("invalid file number: '%s'", args[i+1])
	}
	return i + 1, nil
}

// parseFlagEmpty parses the -e STRING argument. R3.3.
func parseFlagEmpty(args []string, i int, cfg *joinConfig) (int, error) {
	if i+1 >= len(args) {
		return i, fmt.Errorf("option '-e' requires an argument")
	}
	cfg.empty = args[i+1]
	cfg.hasEmpty = true
	return i + 1, nil
}

// parseOutputSpecs parses a comma-separated list of output specifiers.
func parseOutputSpecs(s string) ([]outputSpec, error) {
	parts := strings.Split(s, ",")
	specs := make([]outputSpec, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		spec, err := parseOneSpec(p)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// parseOneSpec parses a single FILENUM.FIELDNUM or '0' specifier.
func parseOneSpec(s string) (outputSpec, error) {
	if s == "0" {
		return outputSpec{joinField: true}, nil
	}
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		return outputSpec{}, fmt.Errorf("invalid field spec: '%s'", s)
	}
	fnum, err := strconv.Atoi(s[:dotIdx])
	if err != nil || (fnum != 1 && fnum != 2) {
		return outputSpec{}, fmt.Errorf("invalid file number in spec: '%s'", s)
	}
	fdnum, err := strconv.Atoi(s[dotIdx+1:])
	if err != nil || fdnum < 1 {
		return outputSpec{}, fmt.Errorf("invalid field number in spec: '%s'", s)
	}
	return outputSpec{fileNum: fnum, fieldNum: fdnum}, nil
}

// openFile opens a file for reading. "-" means stdin. R1.4.
func openFile(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// printFileError writes a GNU-compatible file error message to stderr.
func printFileError(stderr io.Writer, err error) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(stderr, "%s: %s: %v\n", programName, pe.Path, pe.Err)
		return
	}
	fmt.Fprintf(stderr, "%s: %v\n", programName, err)
}

// newLineReader creates a lineReader and reads the first line.
func newLineReader(r io.Reader, joinIdx int, sep string, hasSep bool) *lineReader {
	lr := &lineReader{
		scanner: bufio.NewScanner(r),
		joinIdx: joinIdx,
		sep:     sep,
		hasSep:  hasSep,
	}
	lr.advance()
	return lr
}

// advance reads the next line and splits it into fields.
func (lr *lineReader) advance() {
	lr.hasLine = lr.scanner.Scan()
	if lr.hasLine {
		lr.rawLine = lr.scanner.Text()
		lr.splitFields()
	}
}

// splitFields splits the raw line into fields using the configured separator.
func (lr *lineReader) splitFields() {
	if lr.hasSep {
		lr.fields = strings.Split(lr.rawLine, lr.sep)
	} else {
		lr.fields = strings.Fields(lr.rawLine)
	}
}

// key returns the join field value.
func (lr *lineReader) key() string {
	if lr.joinIdx >= len(lr.fields) {
		return ""
	}
	return lr.fields[lr.joinIdx]
}

// joinFiles reads two sorted inputs and writes joined output.
// R1.1, R1.2, R1.3, R2.1-R2.4, R3.1-R3.4.
func joinFiles(r1, r2 io.Reader, w io.Writer, cfg joinConfig) error {
	lr1 := newLineReader(r1, cfg.field1-1, cfg.sep, cfg.hasSep)
	lr2 := newLineReader(r2, cfg.field2-1, cfg.sep, cfg.hasSep)
	bw := bufio.NewWriter(w)
	// R3.4: handle header lines before main join loop.
	if cfg.header {
		if err := writeHeader(lr1, lr2, bw, cfg); err != nil {
			bw.Flush() // best-effort flush
			return err
		}
	}
	for lr1.hasLine && lr2.hasLine {
		if err := joinStep(lr1, lr2, bw, cfg); err != nil {
			bw.Flush() // best-effort flush
			return err
		}
	}
	// R3.1/R3.2: drain remaining lines from either file.
	if err := drainRemaining(lr1, lr2, bw, cfg); err != nil {
		bw.Flush() // best-effort flush
		return err
	}
	if err := lr1.scanner.Err(); err != nil {
		return err
	}
	if err := lr2.scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// writeHeader joins and prints the first line of each file as a header. R3.4.
func writeHeader(lr1, lr2 *lineReader, bw *bufio.Writer, cfg joinConfig) error {
	var f1, f2 []string
	key := ""
	if lr1.hasLine {
		f1 = lr1.fields
		key = lr1.key()
		lr1.advance()
	}
	if lr2.hasLine {
		f2 = lr2.fields
		if key == "" {
			key = lr2.key()
		}
		lr2.advance()
	}
	return writePair(bw, key, f1, f2, cfg)
}

// joinStep compares keys and dispatches to match or skip.
func joinStep(lr1, lr2 *lineReader, bw *bufio.Writer, cfg joinConfig) error {
	k1 := lr1.key()
	k2 := lr2.key()
	if k1 < k2 {
		// R3.1: print unpairable from file 1 if requested.
		if cfg.unpair1 {
			if err := writeUnpairable(bw, lr1.fields, 1, cfg); err != nil {
				return err
			}
		}
		lr1.advance()
		return nil
	}
	if k1 > k2 {
		// R3.1: print unpairable from file 2 if requested.
		if cfg.unpair2 {
			if err := writeUnpairable(bw, lr2.fields, 2, cfg); err != nil {
				return err
			}
		}
		lr2.advance()
		return nil
	}
	return processMatch(lr1, lr2, bw, cfg)
}

// processMatch handles matching keys by collecting file2 group and pairing.
func processMatch(lr1, lr2 *lineReader, bw *bufio.Writer, cfg joinConfig) error {
	key := lr1.key()
	group2 := collectGroup(lr2, key)
	for lr1.hasLine && lr1.key() == key {
		// R3.2: suppress paired lines when -v is used.
		if !cfg.onlyUnpair {
			for _, f2 := range group2 {
				if err := writePair(bw, key, lr1.fields, f2, cfg); err != nil {
					return err
				}
			}
		}
		lr1.advance()
	}
	return nil
}

// collectGroup gathers all consecutive lines with the given key from lr.
func collectGroup(lr *lineReader, key string) [][]string {
	var group [][]string
	for lr.hasLine && lr.key() == key {
		fields := make([]string, len(lr.fields))
		copy(fields, lr.fields)
		group = append(group, fields)
		lr.advance()
	}
	return group
}

// drainRemaining outputs remaining unpairable lines after the main loop.
func drainRemaining(lr1, lr2 *lineReader, bw *bufio.Writer, cfg joinConfig) error {
	if cfg.unpair1 {
		for lr1.hasLine {
			if err := writeUnpairable(bw, lr1.fields, 1, cfg); err != nil {
				return err
			}
			lr1.advance()
		}
	}
	if cfg.unpair2 {
		for lr2.hasLine {
			if err := writeUnpairable(bw, lr2.fields, 2, cfg); err != nil {
				return err
			}
			lr2.advance()
		}
	}
	return nil
}

// writeUnpairable writes an unpairable line with empty replacement. R3.1, R3.3.
func writeUnpairable(bw *bufio.Writer, fields []string, fileNum int, cfg joinConfig) error {
	sep := outputSep(cfg)
	joinIdx := cfg.field1 - 1
	if fileNum == 2 {
		joinIdx = cfg.field2 - 1
	}
	key := ""
	if joinIdx < len(fields) {
		key = fields[joinIdx]
	}
	if cfg.hasOutFmt {
		return writeUnpairFormatted(bw, key, fields, fileNum, cfg, sep)
	}
	// Default output: key followed by non-join fields.
	parts := []string{key}
	parts = appendNonJoinFields(parts, fields, joinIdx)
	if _, err := bw.WriteString(strings.Join(parts, sep)); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// writeUnpairFormatted writes formatted output for an unpairable line. R3.1, R3.3.
func writeUnpairFormatted(bw *bufio.Writer, key string, fields []string, fileNum int, cfg joinConfig, sep string) error {
	parts := make([]string, 0, len(cfg.outputFmt))
	for _, spec := range cfg.outputFmt {
		parts = append(parts, resolveUnpairSpec(spec, key, fields, fileNum, cfg))
	}
	if _, err := bw.WriteString(strings.Join(parts, sep)); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// resolveUnpairSpec resolves a spec for an unpairable line. R3.3.
func resolveUnpairSpec(spec outputSpec, key string, fields []string, fileNum int, cfg joinConfig) string {
	if spec.joinField {
		return key
	}
	if spec.fileNum != fileNum {
		return emptyVal(cfg)
	}
	idx := spec.fieldNum - 1
	if idx < len(fields) {
		return fields[idx]
	}
	return emptyVal(cfg)
}

// emptyVal returns the -e replacement string or empty. R3.3.
func emptyVal(cfg joinConfig) string {
	if cfg.hasEmpty {
		return cfg.empty
	}
	return ""
}

// outputSep returns the output field separator. R2.4.
func outputSep(cfg joinConfig) string {
	if cfg.hasSep {
		return cfg.sep
	}
	return " "
}

// writePair writes one joined output line. R1.2, R2.3, R2.4.
func writePair(bw *bufio.Writer, key string, f1, f2 []string, cfg joinConfig) error {
	sep := outputSep(cfg)
	if cfg.hasOutFmt {
		return writeFormatted(bw, key, f1, f2, cfg, sep)
	}
	return writeDefault(bw, key, f1, f2, cfg, sep)
}

// writeDefault writes the default output: join field, file1 rest, file2 rest.
func writeDefault(bw *bufio.Writer, key string, f1, f2 []string, cfg joinConfig, sep string) error {
	parts := []string{key}
	parts = appendNonJoinFields(parts, f1, cfg.field1-1)
	parts = appendNonJoinFields(parts, f2, cfg.field2-1)
	if _, err := bw.WriteString(strings.Join(parts, sep)); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// appendNonJoinFields appends all fields except the join field.
func appendNonJoinFields(parts, fields []string, joinIdx int) []string {
	for i, f := range fields {
		if i != joinIdx {
			parts = append(parts, f)
		}
	}
	return parts
}

// writeFormatted writes output using -o format specifiers. R2.3.
func writeFormatted(bw *bufio.Writer, key string, f1, f2 []string, cfg joinConfig, sep string) error {
	parts := make([]string, 0, len(cfg.outputFmt))
	for _, spec := range cfg.outputFmt {
		parts = append(parts, resolveSpec(spec, key, f1, f2, cfg))
	}
	if _, err := bw.WriteString(strings.Join(parts, sep)); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// resolveSpec resolves a single output specifier to a field value. R3.3.
func resolveSpec(spec outputSpec, key string, f1, f2 []string, cfg joinConfig) string {
	if spec.joinField {
		return key
	}
	var fields []string
	if spec.fileNum == 1 {
		fields = f1
	} else {
		fields = f2
	}
	idx := spec.fieldNum - 1
	if idx < len(fields) {
		return fields[idx]
	}
	return emptyVal(cfg)
}
