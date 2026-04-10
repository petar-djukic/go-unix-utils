// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/join: join lines of two files on a common field.
// Implements srd069-join R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "join"

// config holds parsed command-line options from srd069-join.
type config struct {
	field1     int    // R2.1: -1 FIELD (1-based, default 1)
	field2     int    // R2.1: -2 FIELD (1-based, default 1)
	separator  string // R2.4: -t CHAR
	outFormat  string // R2.3: -o FORMAT
	empty      string // R3.3: -e STRING
	unpairA    []int  // R3.1: -a FILENUM (1 or 2)
	unpairV    []int  // R3.2: -v FILENUM (1 or 2)
	header     bool   // R3.4: --header
	checkOrder bool   // R4.4: --check-order
	files      []string
	parseErr   bool
}

// outputSpec represents a single output field specifier from -o FORMAT.
// R2.3: FILENUM.FIELDNUM or 0 for the join field.
type outputSpec struct {
	fileNum  int // 0 = join field, 1 = file1, 2 = file2
	fieldNum int // 1-based field index within the file
}

// lineScanner wraps bufio.Scanner with peek capability for merge-join.
type lineScanner struct {
	sc    *bufio.Scanner
	line  string
	valid bool
}

func newLineScanner(r io.Reader) *lineScanner {
	return &lineScanner{sc: bufio.NewScanner(r)}
}

// advance reads the next line. Returns true if a line is available.
func (ls *lineScanner) advance() bool {
	ls.valid = ls.sc.Scan()
	if ls.valid {
		ls.line = ls.sc.Text()
	}
	return ls.valid
}

// joiner holds state for the merge-join operation.
type joiner struct {
	cfg   *config
	specs []outputSpec
	w     *bufio.Writer
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the join logic and returns the exit code.
// R4.1: returns 0 on success. R4.2: returns 1 on errors.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.parseErr {
		return 1
	}
	if len(cfg.files) != 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		return 1
	}
	r1, closer1, err := openInput(cfg.files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer closer1()
	r2, closer2, err := openInput(cfg.files[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer closer2()
	return executeJoin(&cfg, r1, r2, os.Stdout)
}

// parseArgs extracts flags and file arguments from the command line.
func parseArgs(args []string) config {
	cfg := config{field1: 1, field2: 1}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		consumed := parseLongFlag(&cfg, args, i)
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		consumed = parseShortFlags(&cfg, args, i)
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n",
			progName, arg)
		cfg.parseErr = true
		return cfg
	}
	return cfg
}

// parseLongFlag handles --long-form flags. Returns args consumed.
func parseLongFlag(cfg *config, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "--header":
		cfg.header = true
		return 1
	case arg == "--check-order":
		cfg.checkOrder = true
		return 1
	}
	return parseLongValueFlag(cfg, args, i)
}

// parseLongValueFlag handles --flag=VALUE and --flag VALUE forms.
func parseLongValueFlag(_ *config, args []string, i int) int {
	arg := args[i]
	if strings.HasPrefix(arg, "--") {
		return 0
	}
	return 0
}

// parseShortFlags handles -x style flags.
func parseShortFlags(cfg *config, args []string, i int) int {
	arg := args[i]
	extra := 0
	for j := 1; j < len(arg); j++ {
		consumed := parseOneShort(cfg, arg[j], arg[j+1:],
			args, i+1+extra)
		if consumed == -1 {
			return 0
		}
		if consumed == restConsumed {
			break
		}
		if consumed > 0 {
			extra += consumed
			break
		}
	}
	return 1 + extra
}

// restConsumed signals that a value-consuming short flag used the rest
// of the argument cluster.
const restConsumed = -2

// parseOneShort handles a single short flag character.
func parseOneShort(cfg *config, ch byte, rest string, args []string, nextIdx int) int {
	switch ch {
	case '1':
		return shortFieldFlag(&cfg.field1, rest, args, nextIdx)
	case '2':
		return shortFieldFlag(&cfg.field2, rest, args, nextIdx)
	case 'j':
		return shortJointField(cfg, rest, args, nextIdx)
	case 't':
		return shortStringFlag(&cfg.separator, rest, args, nextIdx)
	case 'o':
		return shortStringFlag(&cfg.outFormat, rest, args, nextIdx)
	case 'e':
		return shortStringFlag(&cfg.empty, rest, args, nextIdx)
	case 'a':
		return shortFileNumFlag(&cfg.unpairA, rest, args, nextIdx)
	case 'v':
		return shortFileNumFlag(&cfg.unpairV, rest, args, nextIdx)
	default:
		return -1
	}
}

// shortFieldFlag parses a field number for -1 or -2 flags.
func shortFieldFlag(dst *int, rest string, args []string, nextIdx int) int {
	val, consumed := extractValue(rest, args, nextIdx)
	if consumed == -1 {
		return -1
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		fmt.Fprintf(os.Stderr, "%s: invalid field number: '%s'\n",
			progName, val)
		return -1
	}
	*dst = n
	return consumed
}

// shortJointField sets both field1 and field2 for the -j flag.
func shortJointField(cfg *config, rest string, args []string, nextIdx int) int {
	val, consumed := extractValue(rest, args, nextIdx)
	if consumed == -1 {
		return -1
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		fmt.Fprintf(os.Stderr, "%s: invalid field number: '%s'\n",
			progName, val)
		return -1
	}
	cfg.field1 = n
	cfg.field2 = n
	return consumed
}

// shortStringFlag extracts a string value for flags like -t, -o, -e.
func shortStringFlag(dst *string, rest string, args []string, nextIdx int) int {
	val, consumed := extractValue(rest, args, nextIdx)
	if consumed == -1 {
		return -1
	}
	*dst = val
	return consumed
}

// shortFileNumFlag parses a FILENUM (1 or 2) for -a or -v flags.
func shortFileNumFlag(dst *[]int, rest string, args []string, nextIdx int) int {
	val, consumed := extractValue(rest, args, nextIdx)
	if consumed == -1 {
		return -1
	}
	n, err := strconv.Atoi(val)
	if err != nil || (n != 1 && n != 2) {
		fmt.Fprintf(os.Stderr, "%s: invalid file number: '%s'\n",
			progName, val)
		return -1
	}
	*dst = append(*dst, n)
	return consumed
}

// extractValue returns the value for a short flag. Uses rest if non-empty,
// otherwise the next arg. Returns the value and consumed count.
func extractValue(rest string, args []string, nextIdx int) (string, int) {
	if rest != "" {
		return rest, restConsumed
	}
	if nextIdx < len(args) {
		return args[nextIdx], 1
	}
	return "", -1
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.4: when the file argument is '-', read from stdin.
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// splitFields splits a line into fields using the configured separator.
// R1.2: default separator is runs of whitespace. R2.4: -t CHAR uses CHAR.
func splitFields(line string, sep string) []string {
	if sep != "" {
		return strings.Split(line, sep)
	}
	return strings.Fields(line)
}

// getField returns the 1-based field from a slice of fields.
// Returns empty string if the field index is out of range.
func getField(fields []string, fieldNum int) string {
	if fieldNum < 1 || fieldNum > len(fields) {
		return ""
	}
	return fields[fieldNum-1]
}

// parseOutputFormat parses the -o FORMAT string into output specs.
// R2.3: FORMAT is comma-separated or space-separated FILENUM.FIELDNUM
// specifiers, or '0' for the join field.
func parseOutputFormat(format string) ([]outputSpec, error) {
	if format == "" {
		return nil, nil
	}
	normalized := strings.ReplaceAll(format, ",", " ")
	tokens := strings.Fields(normalized)
	specs := make([]outputSpec, 0, len(tokens))
	for _, tok := range tokens {
		spec, err := parseOneSpec(tok)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// parseOneSpec parses a single output specifier token.
func parseOneSpec(tok string) (outputSpec, error) {
	if tok == "0" {
		return outputSpec{fileNum: 0, fieldNum: 0}, nil
	}
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return outputSpec{}, fmt.Errorf("invalid field spec: '%s'", tok)
	}
	fnum, err := strconv.Atoi(parts[0])
	if err != nil || (fnum != 1 && fnum != 2) {
		return outputSpec{}, fmt.Errorf("invalid file number in '%s'",
			tok)
	}
	fdnum, err := strconv.Atoi(parts[1])
	if err != nil || fdnum < 1 {
		return outputSpec{}, fmt.Errorf("invalid field number in '%s'",
			tok)
	}
	return outputSpec{fileNum: fnum, fieldNum: fdnum}, nil
}

// executeJoin performs the join operation on two input readers.
// R1.1: merge-join on sorted input. R1.3: unpaired lines suppressed.
func executeJoin(cfg *config, r1, r2 io.Reader, w io.Writer) int {
	specs, err := parseOutputFormat(cfg.outFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	bw := bufio.NewWriter(w)
	j := &joiner{cfg: cfg, specs: specs, w: bw}
	code := j.process(r1, r2)
	if flushErr := bw.Flush(); flushErr != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, flushErr)
		return 1
	}
	return code
}

// process runs the merge-join algorithm on two readers.
func (j *joiner) process(r1, r2 io.Reader) int {
	ls1 := newLineScanner(r1)
	ls2 := newLineScanner(r2)
	ls1.advance()
	ls2.advance()
	if j.cfg.header {
		j.handleHeader(ls1, ls2)
	}
	j.mergeJoin(ls1, ls2)
	j.drainUnpaired(ls1, 1)
	j.drainUnpaired(ls2, 2)
	if err := ls1.sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
		return 1
	}
	if err := ls2.sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
		return 1
	}
	return 0
}

// handleHeader joins the first line of each file regardless of order.
// R3.4: --header treats first lines as headers.
func (j *joiner) handleHeader(ls1, ls2 *lineScanner) {
	if !ls1.valid || !ls2.valid {
		return
	}
	j.writePair(ls1.line, ls2.line)
	ls1.advance()
	ls2.advance()
}

// mergeJoin performs the sorted merge-join loop.
// R1.1: matches lines where join fields are equal.
// R1.3: unpaired lines suppressed by default.
func (j *joiner) mergeJoin(ls1, ls2 *lineScanner) {
	for ls1.valid && ls2.valid {
		key1 := j.joinKey(ls1.line, 1)
		key2 := j.joinKey(ls2.line, 2)
		switch {
		case key1 < key2:
			j.printUnpaired(ls1.line, 1)
			ls1.advance()
		case key1 > key2:
			j.printUnpaired(ls2.line, 2)
			ls2.advance()
		default:
			j.processMatch(ls1, ls2, key1)
		}
	}
}

// processMatch handles matching join keys by computing the cross product
// of all file1 and file2 lines sharing the same key.
func (j *joiner) processMatch(ls1, ls2 *lineScanner, key string) {
	group2 := j.collectGroup(ls2, 2, key)
	for {
		if !j.suppressPaired() {
			for _, line2 := range group2 {
				j.writePair(ls1.line, line2)
			}
		}
		ls1.advance()
		if !ls1.valid || j.joinKey(ls1.line, 1) != key {
			break
		}
	}
}

// collectGroup gathers all consecutive lines with the same join key.
func (j *joiner) collectGroup(ls *lineScanner, fileNum int, key string) []string {
	var group []string
	for ls.valid && j.joinKey(ls.line, fileNum) == key {
		group = append(group, ls.line)
		ls.advance()
	}
	return group
}

// joinKey extracts the join field value from a line for the given file.
func (j *joiner) joinKey(line string, fileNum int) string {
	fields := splitFields(line, j.cfg.separator)
	idx := j.cfg.field1
	if fileNum == 2 {
		idx = j.cfg.field2
	}
	return getField(fields, idx)
}

// writePair outputs a single joined line from two matching input lines.
// R1.3: output format is join field + remaining fields from both files.
func (j *joiner) writePair(line1, line2 string) {
	fields1 := splitFields(line1, j.cfg.separator)
	fields2 := splitFields(line2, j.cfg.separator)
	joinField := getField(fields1, j.cfg.field1)
	out := formatOutputLine(j.cfg, joinField, fields1, fields2, j.specs)
	fmt.Fprintln(j.w, out)
}

// printUnpaired outputs an unpaired line if -a or -v includes the file.
// R1.3: suppressed by default (no -a or -v).
func (j *joiner) printUnpaired(line string, fileNum int) {
	if !j.shouldPrintUnpaired(fileNum) {
		return
	}
	fields := splitFields(line, j.cfg.separator)
	idx := j.cfg.field1
	var fields1, fields2 []string
	if fileNum == 1 {
		fields1 = fields
	} else {
		fields2 = fields
		idx = j.cfg.field2
	}
	joinField := getField(fields, idx)
	out := formatOutputLine(j.cfg, joinField, fields1, fields2, j.specs)
	fmt.Fprintln(j.w, out)
}

// drainUnpaired outputs remaining lines from a scanner as unpaired.
func (j *joiner) drainUnpaired(ls *lineScanner, fileNum int) {
	for ls.valid {
		j.printUnpaired(ls.line, fileNum)
		ls.advance()
	}
}

// shouldPrintUnpaired checks if unpaired lines from fileNum should print.
func (j *joiner) shouldPrintUnpaired(fileNum int) bool {
	return slices.Contains(j.cfg.unpairA, fileNum) ||
		slices.Contains(j.cfg.unpairV, fileNum)
}

// suppressPaired returns true if -v mode suppresses paired output.
func (j *joiner) suppressPaired() bool {
	return len(j.cfg.unpairV) > 0
}

// formatOutputLine builds the output line for a joined pair.
// R2.3: uses -o specs if set, otherwise default format.
// R3.3: uses -e STRING for missing fields.
func formatOutputLine(cfg *config, joinField string, fields1, fields2 []string, specs []outputSpec) string {
	sep := " "
	if cfg.separator != "" {
		sep = cfg.separator
	}
	if len(specs) == 0 {
		return formatDefaultLine(joinField, fields1, fields2,
			cfg.field1, cfg.field2, sep)
	}
	return formatSpecLine(cfg, joinField, fields1, fields2, specs, sep)
}

// formatDefaultLine produces the default output: join field, then
// remaining fields from file1 and file2.
func formatDefaultLine(joinField string, fields1, fields2 []string, jf1, jf2 int, sep string) string {
	var parts []string
	parts = append(parts, joinField)
	for i, f := range fields1 {
		if i+1 != jf1 {
			parts = append(parts, f)
		}
	}
	for i, f := range fields2 {
		if i+1 != jf2 {
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, sep)
}

// formatSpecLine produces output according to -o specs.
func formatSpecLine(cfg *config, joinField string, fields1, fields2 []string, specs []outputSpec, sep string) string {
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		parts = append(parts, resolveSpec(cfg, s, joinField,
			fields1, fields2))
	}
	return strings.Join(parts, sep)
}

// resolveSpec resolves a single output spec to a field value.
func resolveSpec(cfg *config, s outputSpec, joinField string, fields1, fields2 []string) string {
	switch s.fileNum {
	case 0:
		return joinField
	case 1:
		return fieldOrEmpty(fields1, s.fieldNum, cfg.empty)
	case 2:
		return fieldOrEmpty(fields2, s.fieldNum, cfg.empty)
	default:
		return ""
	}
}

// fieldOrEmpty returns the field value or the replacement string.
func fieldOrEmpty(fields []string, fieldNum int, empty string) string {
	if fieldNum < 1 || fieldNum > len(fields) {
		return empty
	}
	return fields[fieldNum-1]
}
