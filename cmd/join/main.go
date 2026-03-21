// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd069-join R1.1–R1.4, R2.1–R2.4, R3.1–R3.4.
// R1.1: Read two sorted files, join on common field (default: field 1).
// R1.2: Default whitespace field splitting, single-space output separator.
// R1.3: Unpaired lines are suppressed by default.
// R1.4: "-" reads from stdin.
// R2.1: -1 FIELD and -2 FIELD set join fields for each file.
// R2.2: -j FIELD sets the join field for both files.
// R2.3: -o FORMAT selects and orders output fields.
// R2.4: -t CHAR sets both input and output field separator.
// R3.1: -a FILENUM prints unpairable lines from specified file.
// R3.2: -v FILENUM prints only unpairable lines, suppressing paired output.
// R3.3: -e STRING replaces missing input fields in -o output.
// R3.4: --header treats first line of each file as header.
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

// outputSpec represents one element in a -o format specification.
type outputSpec struct {
	isJoinField bool // true when spec is "0"
	fileNum     int  // 1 or 2
	fieldIdx    int  // 0-indexed field number
}

// joinConfig holds parsed command-line options.
type joinConfig struct {
	field1      int          // join field for file 1 (0-indexed)
	field2      int          // join field for file 2 (0-indexed)
	sep         string       // field separator; empty means whitespace
	hasSep      bool         // true if -t was specified
	outputFmt   []outputSpec // -o format specs; nil means default output
	empty       string       // -e replacement for missing fields
	hasEmpty    bool         // true if -e was specified
	unpairFile1 bool         // R3.1: -a 1 — also print unpairable lines from file 1
	unpairFile2 bool         // R3.1: -a 2 — also print unpairable lines from file 2
	onlyUnpair1 bool         // R3.2: -v 1 — print only unpairable lines from file 1
	onlyUnpair2 bool         // R3.2: -v 2 — print only unpairable lines from file 2
	header      bool         // R3.4: --header — treat first line as header
}

// outputSep returns the output field separator.
func (c joinConfig) outputSep() string {
	if c.hasSep {
		return c.sep
	}
	return " "
}

// suppressPaired returns true when -v suppresses paired output.
func (c joinConfig) suppressPaired() bool {
	return c.onlyUnpair1 || c.onlyUnpair2
}

// lineReader reads lines one at a time from an io.Reader,
// keeping the current line available for peek-style access.
type lineReader struct {
	scanner *bufio.Scanner
	valid   bool
	line    string
}

func newLineReader(r io.Reader) *lineReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lr := &lineReader{scanner: s}
	lr.advance()
	return lr
}

func (lr *lineReader) advance() {
	lr.valid = lr.scanner.Scan()
	if lr.valid {
		lr.line = lr.scanner.Text()
	}
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, file1, file2, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	return executeJoin(cfg, file1, file2)
}

// executeJoin opens both inputs, performs the join, and flushes output.
func executeJoin(cfg joinConfig, file1, file2 string) int {
	r1, c1, err := openInput(file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	if c1 != nil {
		defer c1.Close()
	}
	r2, c2, err := openInput(file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	if c2 != nil {
		defer c2.Close()
	}
	w := bufio.NewWriter(os.Stdout)
	joinStreams(w, r1, r2, cfg)
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "join: write error: %v\n", err)
		return 1
	}
	return 0
}

// parseArgs extracts flags and two file operands from command-line arguments.
func parseArgs(args []string) (joinConfig, string, string, error) {
	cfg := joinConfig{}
	var operands []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		consumed, err := parseFlag(args[i], args, &i, &cfg)
		if err != nil {
			return cfg, "", "", err
		}
		if !consumed {
			operands = append(operands, args[i])
		}
	}
	if len(operands) != 2 {
		return cfg, "", "", fmt.Errorf("missing operand")
	}
	return cfg, operands[0], operands[1], nil
}

// parseFlag attempts to parse a single flag argument. Returns true if consumed.
func parseFlag(arg string, args []string, i *int, cfg *joinConfig) (bool, error) {
	if arg == "--header" {
		cfg.header = true
		return true, nil
	}
	if arg == "-" || !strings.HasPrefix(arg, "-") || len(arg) < 2 {
		return false, nil
	}
	return parseSingleFlag(arg, args, i, cfg)
}

// parseSingleFlag handles short flags (-1, -2, -j, -t, -o, -e, -a, -v).
func parseSingleFlag(arg string, args []string, i *int, cfg *joinConfig) (bool, error) {
	flag := arg[:2]
	rest := arg[2:]
	switch flag {
	case "-1":
		return parseFlagField(rest, args, i, "1", &cfg.field1)
	case "-2":
		return parseFlagField(rest, args, i, "2", &cfg.field2)
	case "-j":
		return parseJFlag(rest, args, i, cfg)
	case "-t":
		return true, parseSepValue(rest, args, i, cfg)
	case "-o":
		return true, parseOValue(rest, args, i, cfg)
	case "-e":
		return true, parseEmptyValue(rest, args, i, cfg)
	case "-a":
		return true, parseFileNumFlag(rest, args, i, "a", cfg, true)
	case "-v":
		return true, parseFileNumFlag(rest, args, i, "v", cfg, false)
	default:
		return false, fmt.Errorf("invalid option -- '%c'", arg[1])
	}
}

// parseFileNumFlag parses -a or -v FILENUM flags.
// R3.1: -a sets unpairFile1/unpairFile2.
// R3.2: -v sets onlyUnpair1/onlyUnpair2.
func parseFileNumFlag(rest string, args []string, i *int, name string, cfg *joinConfig, isA bool) error {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- '%s'", name)
		}
		*i++
		val = args[*i]
	}
	return applyFileNumFlag(val, name, cfg, isA)
}

// applyFileNumFlag sets the appropriate config field for -a or -v.
func applyFileNumFlag(val, _ string, cfg *joinConfig, isA bool) error {
	switch val {
	case "1":
		if isA {
			cfg.unpairFile1 = true
		} else {
			cfg.onlyUnpair1 = true
		}
	case "2":
		if isA {
			cfg.unpairFile2 = true
		} else {
			cfg.onlyUnpair2 = true
		}
	default:
		return fmt.Errorf("invalid file number: '%s'", val)
	}
	return nil
}

// parseFlagField parses -1 or -2 field selection flag.
func parseFlagField(rest string, args []string, i *int, name string, dest *int) (bool, error) {
	n, err := parseFieldValue(rest, args, i, name)
	if err != nil {
		return true, err
	}
	*dest = n
	return true, nil
}

// parseJFlag handles -j FIELD, setting both join fields.
func parseJFlag(rest string, args []string, i *int, cfg *joinConfig) (bool, error) {
	n, err := parseFieldValue(rest, args, i, "j")
	if err != nil {
		return true, err
	}
	cfg.field1 = n
	cfg.field2 = n
	return true, nil
}

// parseFieldValue extracts a 1-indexed field number from inline or next arg.
func parseFieldValue(rest string, args []string, i *int, flagChar string) (int, error) {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- '%s'", flagChar)
		}
		*i++
		val = args[*i]
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid field number: '%s'", val)
	}
	return n - 1, nil // convert to 0-indexed
}

// parseSepValue extracts the separator character from inline or next arg.
func parseSepValue(rest string, args []string, i *int, cfg *joinConfig) error {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- 't'")
		}
		*i++
		val = args[*i]
	}
	cfg.sep = val
	cfg.hasSep = true
	return nil
}

// parseOValue parses -o FORMAT, consuming comma/space-separated specs.
// R2.3: Supports both comma-separated and space-separated specifiers.
func parseOValue(rest string, args []string, i *int, cfg *joinConfig) error {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- 'o'")
		}
		*i++
		val = args[*i]
	}
	if err := addOutputSpecs(val, cfg); err != nil {
		return err
	}
	for *i+1 < len(args) && looksLikeSpec(args[*i+1]) {
		*i++
		if err := addOutputSpecs(args[*i], cfg); err != nil {
			return err
		}
	}
	return nil
}

// addOutputSpecs parses a comma/space-separated string of output specs.
func addOutputSpecs(val string, cfg *joinConfig) error {
	parts := strings.FieldsFunc(val, func(r rune) bool {
		return r == ',' || r == ' '
	})
	for _, p := range parts {
		spec, err := parseOneSpec(p)
		if err != nil {
			return err
		}
		cfg.outputFmt = append(cfg.outputFmt, spec)
	}
	return nil
}

// parseOneSpec parses a single output specifier: "0" or "FILENUM.FIELDNUM".
func parseOneSpec(s string) (outputSpec, error) {
	if s == "0" {
		return outputSpec{isJoinField: true}, nil
	}
	before, after, found := strings.Cut(s, ".")
	if !found {
		return outputSpec{}, fmt.Errorf("invalid field specification: '%s'", s)
	}
	fileNum, err := strconv.Atoi(before)
	if err != nil || (fileNum != 1 && fileNum != 2) {
		return outputSpec{}, fmt.Errorf("invalid file number in field spec: '%s'", s)
	}
	fieldNum, err := strconv.Atoi(after)
	if err != nil || fieldNum < 1 {
		return outputSpec{}, fmt.Errorf("invalid field number in field spec: '%s'", s)
	}
	return outputSpec{fileNum: fileNum, fieldIdx: fieldNum - 1}, nil
}

// looksLikeSpec returns true if the string resembles an output spec
// ("0" or "N.M" where N and M are integers).
func looksLikeSpec(s string) bool {
	if s == "0" {
		return true
	}
	before, after, found := strings.Cut(s, ".")
	if !found || before == "" || after == "" {
		return false
	}
	_, err1 := strconv.Atoi(before)
	_, err2 := strconv.Atoi(after)
	return err1 == nil && err2 == nil
}

// parseEmptyValue parses -e STRING for missing field replacement.
func parseEmptyValue(rest string, args []string, i *int, cfg *joinConfig) error {
	val := rest
	if val == "" {
		if *i+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- 'e'")
		}
		*i++
		val = args[*i]
	}
	cfg.empty = val
	cfg.hasEmpty = true
	return nil
}

// openInput opens a file for reading, or returns stdin for "-".
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	return f, f, nil
}

// splitLine splits a line into fields based on separator configuration.
// With -t: exact single-character split. Without: whitespace runs.
func splitLine(line string, cfg joinConfig) []string {
	if cfg.hasSep {
		return strings.Split(line, cfg.sep)
	}
	return strings.Fields(line)
}

// getKey extracts the join field value from a fields slice.
func getKey(fields []string, fieldIdx int) string {
	if fieldIdx < len(fields) {
		return fields[fieldIdx]
	}
	return ""
}

// joinStreams performs the merge-join of two sorted inputs.
// R3.4: When --header is set, consume and join the first lines as headers.
func joinStreams(w *bufio.Writer, r1, r2 io.Reader, cfg joinConfig) {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)
	if cfg.header {
		writeHeaderLine(w, lr1, lr2, cfg)
	}
	mergeJoin(w, lr1, lr2, cfg)
	drainRemaining(w, lr1, lr2, cfg)
}

// writeHeaderLine joins and prints the header lines from both files.
// R3.4: Headers are joined regardless of sort order.
func writeHeaderLine(w *bufio.Writer, lr1, lr2 *lineReader, cfg joinConfig) {
	var f1, f2 []string
	if lr1.valid {
		f1 = splitLine(lr1.line, cfg)
		lr1.advance()
	}
	if lr2.valid {
		f2 = splitLine(lr2.line, cfg)
		lr2.advance()
	}
	key := headerKey(f1, f2, cfg)
	writeJoinLine(w, key, f1, f2, cfg)
}

// headerKey returns the join field for the header line.
func headerKey(f1, f2 []string, cfg joinConfig) string {
	if k := getKey(f1, cfg.field1); k != "" {
		return k
	}
	return getKey(f2, cfg.field2)
}

// mergeJoin performs the core merge-join loop over two sorted readers.
func mergeJoin(w *bufio.Writer, lr1, lr2 *lineReader, cfg joinConfig) {
	for lr1.valid && lr2.valid {
		f1 := splitLine(lr1.line, cfg)
		f2 := splitLine(lr2.line, cfg)
		key1 := getKey(f1, cfg.field1)
		key2 := getKey(f2, cfg.field2)
		cmp := strings.Compare(key1, key2)
		if cmp < 0 {
			writeUnpairable1(w, f1, key1, cfg)
			lr1.advance()
			continue
		}
		if cmp > 0 {
			writeUnpairable2(w, f2, key2, cfg)
			lr2.advance()
			continue
		}
		joinGroup(w, lr1, lr2, cfg, key1)
	}
}

// drainRemaining outputs any remaining unpairable lines after merge.
func drainRemaining(w *bufio.Writer, lr1, lr2 *lineReader, cfg joinConfig) {
	drainUnpairable1(w, lr1, cfg)
	drainUnpairable2(w, lr2, cfg)
}

// drainUnpairable1 outputs remaining file1 lines if -a 1 or -v 1.
func drainUnpairable1(w *bufio.Writer, lr1 *lineReader, cfg joinConfig) {
	if !cfg.unpairFile1 && !cfg.onlyUnpair1 {
		return
	}
	for lr1.valid {
		f1 := splitLine(lr1.line, cfg)
		key1 := getKey(f1, cfg.field1)
		writeUnpairableLine(w, f1, nil, key1, cfg)
		lr1.advance()
	}
}

// drainUnpairable2 outputs remaining file2 lines if -a 2 or -v 2.
func drainUnpairable2(w *bufio.Writer, lr2 *lineReader, cfg joinConfig) {
	if !cfg.unpairFile2 && !cfg.onlyUnpair2 {
		return
	}
	for lr2.valid {
		f2 := splitLine(lr2.line, cfg)
		key2 := getKey(f2, cfg.field2)
		writeUnpairableLine(w, nil, f2, key2, cfg)
		lr2.advance()
	}
}

// writeUnpairable1 outputs a file1 line that has no pair, if configured.
// R3.1: -a 1 prints unpairable file1 lines alongside paired lines.
// R3.2: -v 1 prints only unpairable file1 lines.
func writeUnpairable1(w *bufio.Writer, f1 []string, key string, cfg joinConfig) {
	if !cfg.unpairFile1 && !cfg.onlyUnpair1 {
		return
	}
	writeUnpairableLine(w, f1, nil, key, cfg)
}

// writeUnpairable2 outputs a file2 line that has no pair, if configured.
// R3.1: -a 2 prints unpairable file2 lines alongside paired lines.
// R3.2: -v 2 prints only unpairable file2 lines.
func writeUnpairable2(w *bufio.Writer, f2 []string, key string, cfg joinConfig) {
	if !cfg.unpairFile2 && !cfg.onlyUnpair2 {
		return
	}
	writeUnpairableLine(w, nil, f2, key, cfg)
}

// writeUnpairableLine writes an unpairable line using -o format or default.
func writeUnpairableLine(w *bufio.Writer, f1, f2 []string, key string, cfg joinConfig) {
	if cfg.outputFmt != nil {
		writeFormatLine(w, key, f1, f2, cfg)
		return
	}
	writeDefaultUnpairLine(w, f1, f2, key, cfg)
}

// writeDefaultUnpairLine writes the default output for an unpairable line.
func writeDefaultUnpairLine(w *bufio.Writer, f1, f2 []string, key string, cfg joinConfig) {
	sep := cfg.outputSep()
	w.WriteString(key)
	if f1 != nil {
		writeRemainingFields(w, f1, cfg.field1, sep)
	}
	if f2 != nil {
		writeRemainingFields(w, f2, cfg.field2, sep)
	}
	w.WriteByte('\n')
}

// joinGroup joins all file1 and file2 lines sharing the current key.
// Buffers file2 lines with the matching key, then cross-joins with
// all file1 lines that also match.
func joinGroup(w *bufio.Writer, lr1, lr2 *lineReader, cfg joinConfig, key string) {
	var group2 [][]string
	for lr2.valid {
		f2 := splitLine(lr2.line, cfg)
		if getKey(f2, cfg.field2) != key {
			break
		}
		group2 = append(group2, f2)
		lr2.advance()
	}
	for lr1.valid {
		f1 := splitLine(lr1.line, cfg)
		if getKey(f1, cfg.field1) != key {
			break
		}
		if !cfg.suppressPaired() {
			for _, f2 := range group2 {
				writeJoinLine(w, key, f1, f2, cfg)
			}
		}
		lr1.advance()
	}
}

// writeJoinLine writes one joined output line using either -o format
// or the default layout (join field, then remaining fields from each file).
func writeJoinLine(w *bufio.Writer, key string, f1, f2 []string, cfg joinConfig) {
	if cfg.outputFmt != nil {
		writeFormatLine(w, key, f1, f2, cfg)
		return
	}
	writeDefaultLine(w, key, f1, f2, cfg)
}

// writeDefaultLine writes the default output: join field followed by
// remaining fields from file1 then file2.
func writeDefaultLine(w *bufio.Writer, key string, f1, f2 []string, cfg joinConfig) {
	sep := cfg.outputSep()
	w.WriteString(key)
	writeRemainingFields(w, f1, cfg.field1, sep)
	writeRemainingFields(w, f2, cfg.field2, sep)
	w.WriteByte('\n')
}

// writeFormatLine writes output according to the -o format specification.
// R2.3: Each spec is resolved and separated by the output separator.
func writeFormatLine(w *bufio.Writer, key string, f1, f2 []string, cfg joinConfig) {
	sep := cfg.outputSep()
	for i, spec := range cfg.outputFmt {
		if i > 0 {
			w.WriteString(sep)
		}
		w.WriteString(resolveSpec(spec, key, f1, f2, cfg))
	}
	w.WriteByte('\n')
}

// resolveSpec returns the field value for a single output spec.
// R3.3: Uses -e replacement when the field index is out of range.
func resolveSpec(spec outputSpec, key string, f1, f2 []string, cfg joinConfig) string {
	if spec.isJoinField {
		return key
	}
	fields := f1
	if spec.fileNum == 2 {
		fields = f2
	}
	return resolveField(fields, spec.fieldIdx, cfg)
}

// resolveField returns the field value or the -e replacement if missing.
func resolveField(fields []string, idx int, cfg joinConfig) string {
	if fields != nil && idx < len(fields) {
		return fields[idx]
	}
	if cfg.hasEmpty {
		return cfg.empty
	}
	return ""
}

// writeRemainingFields writes all fields except the join field,
// each preceded by the separator.
func writeRemainingFields(w *bufio.Writer, fields []string, joinIdx int, sep string) {
	for i, f := range fields {
		if i == joinIdx {
			continue
		}
		w.WriteString(sep)
		w.WriteString(f)
	}
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
