// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd069-join: Join Lines of Two Files on a Common Field.
// Covers R1.1-R1.4 (default join), R2.1-R2.4 (field selection, output format, separator),
// R3.1-R3.4 (unpairable lines, empty replacement, header), R4.1-R4.2 (exit codes).
// R2.3 (-e empty replacement), R2.4 (-i case-insensitive), R3.1 (--check-order),
// R3.3 (sort-order diagnostics), R3.4 (--version/--help).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// outputSpec represents a single -o format specifier.
type outputSpec struct {
	fileNum  int // 0 for join field, 1 or 2 for file fields
	fieldNum int // 1-indexed field number (only used when fileNum != 0)
}

// config holds parsed flag state for the join command.
type config struct {
	field1      int          // R2.1: join field for file 1 (1-indexed, default 1)
	field2      int          // R2.1: join field for file 2 (1-indexed, default 1)
	sep         string       // R2.4: field separator character
	hasSep      bool         // whether -t was specified
	outSpecs    []outputSpec // R2.3: -o format specifiers
	unpairFile1 bool         // R3.1: -a 1 — also print unpairable from file 1
	unpairFile2 bool         // R3.1: -a 2 — also print unpairable from file 2
	onlyFile1   bool         // R3.2: -v 1 — only unpairable from file 1
	onlyFile2   bool         // R3.2: -v 2 — only unpairable from file 2
	empty       string       // R3.3: -e STRING replacement for missing fields
	header      bool         // R3.4: --header treats first lines as headers
	ignoreCase  bool         // R2.4: -i/--ignore-case for case-insensitive join
}

// lineReader wraps a bufio.Scanner with one-line pushback for merge-join.
type lineReader struct {
	scanner *bufio.Scanner
	pending string // pushed-back line
	hasPend bool   // whether pending is valid
	done    bool   // scanner exhausted
	lineNum int    // 1-indexed line number of last returned line
}

// newLineReader creates a lineReader from an io.Reader.
func newLineReader(r io.Reader) *lineReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	return &lineReader{scanner: s}
}

// next returns the next line and true, or ("", false) at EOF.
func (lr *lineReader) next() (string, bool) {
	if lr.hasPend {
		lr.hasPend = false
		return lr.pending, true
	}
	if lr.done {
		return "", false
	}
	if lr.scanner.Scan() {
		lr.lineNum++
		return lr.scanner.Text(), true
	}
	lr.done = true
	return "", false
}

// pushBack saves a line to be returned by the next call to next().
func (lr *lineReader) pushBack(line string) {
	lr.pending = line
	lr.hasPend = true
}

// err returns any scanner error.
func (lr *lineReader) err() error {
	return lr.scanner.Err()
}

// merger holds state for the merge-join algorithm including order checking.
type merger struct {
	cfg          config
	lr1, lr2     *lineReader
	bw           *bufio.Writer
	name1, name2 string
	prev1, prev2 string
	has1, has2   bool
	unsorted     bool
}

// compareKeys compares two join keys, returning -1, 0, or 1.
// R2.4: when ignoreCase is true, comparison is case-insensitive.
func compareKeys(a, b string, ignoreCase bool) int {
	if ignoreCase {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// checkKey checks sort order for a file's key and updates tracking state.
// R3.1/R3.3: emits "join: FILE:LINE: is not sorted: LINE" diagnostics.
func (m *merger) checkKey(fileNum int, key, line string) {
	var prev *string
	var has *bool
	var name string
	var num int
	if fileNum == 1 {
		prev, has, name, num = &m.prev1, &m.has1, m.name1, m.lr1.lineNum
	} else {
		prev, has, name, num = &m.prev2, &m.has2, m.name2, m.lr2.lineNum
	}
	if *has && compareKeys(key, *prev, m.cfg.ignoreCase) < 0 {
		fmt.Fprintf(os.Stderr, "join: %s:%d: is not sorted: %s\n", name, num, line)
		m.unsorted = true
	}
	*prev = key
	*has = true
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, file1, file2, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, file1, file2))
}

// run opens both files, performs the join, and returns exit code.
func run(cfg config, name1, name2 string) int {
	r1, r2, cleanup, err := openBothInputs(name1, name2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %s\n", err)
		return 1
	}
	defer cleanup()

	bw := bufio.NewWriter(os.Stdout)
	exitCode := joinFiles(cfg, r1, r2, bw, name1, name2)
	if flushErr := bw.Flush(); flushErr != nil {
		fmt.Fprintf(os.Stderr, "join: write error: %s\n", flushErr)
		return 1
	}
	return exitCode
}

// openBothInputs opens both input files, supporting '-' for stdin.
// R1.4: accept '-' to read from stdin for one of the two files.
func openBothInputs(name1, name2 string) (io.Reader, io.Reader, func(), error) {
	r1, c1, err := openInput(name1)
	if err != nil {
		return nil, nil, nil, err
	}
	r2, c2, err := openInput(name2)
	if err != nil {
		c1()
		return nil, nil, nil, err
	}
	return r1, r2, func() { c1(); c2() }, nil
}

// openInput opens a file or returns stdin for "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, formatPathError(name, err)
	}
	return f, func() { f.Close() }, nil
}

// formatPathError produces a GNU-compatible error message.
func formatPathError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// splitFields splits a line into fields based on separator config.
// R1.2: default splits on runs of blanks. R2.4: -t splits on exact char.
func splitFields(line string, cfg config) []string {
	if cfg.hasSep {
		return strings.Split(line, cfg.sep)
	}
	return strings.Fields(line)
}

// getField returns the 1-indexed field from fields, or empty replacement.
func getField(fields []string, idx int, cfg config) string {
	if idx >= 1 && idx <= len(fields) {
		return fields[idx-1]
	}
	return cfg.empty
}

// joinKey extracts the join key from a line's fields.
func joinKey(fields []string, fieldIdx int) string {
	if fieldIdx >= 1 && fieldIdx <= len(fields) {
		return fields[fieldIdx-1]
	}
	return ""
}

// outputSep returns the output field separator.
func outputSep(cfg config) string {
	if cfg.hasSep {
		return cfg.sep
	}
	return " "
}

// suppressPaired returns true if -v suppresses paired output.
func suppressPaired(cfg config) bool {
	return cfg.onlyFile1 || cfg.onlyFile2
}

// showUnpairable returns true if unpairable lines from fileNum should print.
func showUnpairable(cfg config, fileNum int) bool {
	if fileNum == 1 {
		return cfg.unpairFile1 || cfg.onlyFile1
	}
	return cfg.unpairFile2 || cfg.onlyFile2
}

// formatPaired formats a paired output line.
// R1.2: join field first, then remaining fields from file 1, then file 2.
func formatPaired(cfg config, key string, f1, f2 []string) string {
	sep := outputSep(cfg)
	if len(cfg.outSpecs) > 0 {
		return formatWithSpecs(cfg, key, f1, f2, sep)
	}
	return formatDefault(cfg, key, f1, f2, sep)
}

// formatDefault produces default output: join field + remaining from both.
func formatDefault(cfg config, key string, f1, f2 []string, sep string) string {
	parts := make([]string, 0, 1+len(f1)+len(f2))
	parts = append(parts, key)
	parts = appendRemaining(parts, f1, cfg.field1)
	parts = appendRemaining(parts, f2, cfg.field2)
	return strings.Join(parts, sep)
}

// appendRemaining appends all fields except the join field to parts.
func appendRemaining(parts, fields []string, joinIdx int) []string {
	for i, f := range fields {
		if i+1 != joinIdx {
			parts = append(parts, f)
		}
	}
	return parts
}

// formatWithSpecs produces output per -o format specifiers.
func formatWithSpecs(cfg config, key string, f1, f2 []string, sep string) string {
	parts := make([]string, 0, len(cfg.outSpecs))
	for _, spec := range cfg.outSpecs {
		parts = append(parts, resolveSpec(spec, key, f1, f2, cfg))
	}
	return strings.Join(parts, sep)
}

// resolveSpec resolves a single output spec to a field value.
func resolveSpec(spec outputSpec, key string, f1, f2 []string, cfg config) string {
	if spec.fileNum == 0 {
		return key
	}
	if spec.fileNum == 1 {
		return getField(f1, spec.fieldNum, cfg)
	}
	return getField(f2, spec.fieldNum, cfg)
}

// formatUnpairable formats an unpairable line for output.
func formatUnpairable(cfg config, fields []string, fieldIdx, fileNum int) string {
	sep := outputSep(cfg)
	key := joinKey(fields, fieldIdx)
	if len(cfg.outSpecs) > 0 {
		return formatUnpairSpecs(cfg, key, fields, fileNum, sep)
	}
	parts := make([]string, 0, len(fields))
	parts = append(parts, key)
	parts = appendRemaining(parts, fields, fieldIdx)
	return strings.Join(parts, sep)
}

// formatUnpairSpecs formats unpairable line with -o specs.
func formatUnpairSpecs(cfg config, key string, fields []string, fileNum int, sep string) string {
	parts := make([]string, 0, len(cfg.outSpecs))
	for _, spec := range cfg.outSpecs {
		parts = append(parts, resolveUnpairSpec(spec, key, fields, cfg, fileNum))
	}
	return strings.Join(parts, sep)
}

// resolveUnpairSpec resolves a spec for an unpairable line.
func resolveUnpairSpec(spec outputSpec, key string, fields []string, cfg config, fileNum int) string {
	if spec.fileNum == 0 {
		return key
	}
	if spec.fileNum == fileNum {
		return getField(fields, spec.fieldNum, cfg)
	}
	return cfg.empty
}

// joinFiles performs the join algorithm on two sorted inputs.
func joinFiles(cfg config, r1, r2 io.Reader, bw *bufio.Writer, name1, name2 string) int {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)

	if cfg.header {
		if code := processHeader(cfg, lr1, lr2, bw); code != 0 {
			return code
		}
	}
	m := &merger{
		cfg: cfg, lr1: lr1, lr2: lr2, bw: bw,
		name1: name1, name2: name2,
	}
	mergeCode := m.run()
	if errCode := checkReaderErrors(lr1, lr2); errCode != 0 {
		return errCode
	}
	return mergeCode
}

// processHeader reads and joins the first line of each file as a header.
// R3.4: header lines are joined regardless of sort order.
func processHeader(cfg config, lr1, lr2 *lineReader, bw *bufio.Writer) int {
	line1, ok1 := lr1.next()
	line2, ok2 := lr2.next()
	if !ok1 || !ok2 {
		return 0
	}
	f1 := splitFields(line1, cfg)
	f2 := splitFields(line2, cfg)
	key := joinKey(f1, cfg.field1)
	line := formatPaired(cfg, key, f1, f2)
	if err := writeLine(bw, line); err != nil {
		fmt.Fprintf(os.Stderr, "join: write error: %s\n", err)
		return 1
	}
	return 0
}

// run performs the merge-join with sort-order checking.
// R3.1: checks input order and returns 1 if unsorted input is detected.
func (m *merger) run() int {
	line1, ok1 := m.lr1.next()
	line2, ok2 := m.lr2.next()

	for ok1 && ok2 {
		var code int
		code, line1, ok1, line2, ok2 = m.step(line1, line2)
		if code != 0 {
			return code
		}
	}
	if code := drainUnpaired(m.cfg, m.lr1, line1, ok1, m.cfg.field1, 1, m.bw); code != 0 {
		return code
	}
	if code := drainUnpaired(m.cfg, m.lr2, line2, ok2, m.cfg.field2, 2, m.bw); code != 0 {
		return code
	}
	if m.unsorted {
		fmt.Fprintln(os.Stderr, "join: input is not in sorted order")
		return 1
	}
	return 0
}

// step processes one step of the merge-join with order checking.
func (m *merger) step(line1, line2 string) (int, string, bool, string, bool) {
	f1 := splitFields(line1, m.cfg)
	f2 := splitFields(line2, m.cfg)
	key1 := joinKey(f1, m.cfg.field1)
	key2 := joinKey(f2, m.cfg.field2)
	m.checkKey(1, key1, line1)
	m.checkKey(2, key2, line2)

	cmp := compareKeys(key1, key2, m.cfg.ignoreCase)
	if cmp < 0 {
		code := emitUnpair(m.cfg, f1, m.cfg.field1, 1, m.bw)
		next, ok := m.lr1.next()
		return code, next, ok, line2, true
	}
	if cmp > 0 {
		code := emitUnpair(m.cfg, f2, m.cfg.field2, 2, m.bw)
		next, ok := m.lr2.next()
		return code, line1, true, next, ok
	}
	return m.emitMatches(key1, f1, f2)
}

// emitMatches handles matching keys with cartesian product for duplicates.
// Checks file-1 sort order for each additional line read.
func (m *merger) emitMatches(
	key string, f1 []string, f2Init []string,
) (int, string, bool, string, bool) {
	group2 := m.collectGroup2(key, f2Init)
	for {
		if code := emitPairs(m.cfg, key, f1, group2, m.bw); code != 0 {
			return code, "", false, "", false
		}
		next1, ok1 := m.lr1.next()
		if !ok1 {
			next2, ok2 := m.lr2.next()
			return 0, "", false, next2, ok2
		}
		nf1 := splitFields(next1, m.cfg)
		nk1 := joinKey(nf1, m.cfg.field1)
		m.checkKey(1, nk1, next1)
		if compareKeys(nk1, key, m.cfg.ignoreCase) != 0 {
			next2, ok2 := m.lr2.next()
			return 0, next1, true, next2, ok2
		}
		f1 = nf1
	}
}

// collectGroup2 collects file-2 lines matching key, with order checking.
func (m *merger) collectGroup2(key string, initial []string) [][]string {
	group := [][]string{initial}
	for {
		line, ok := m.lr2.next()
		if !ok {
			return group
		}
		fields := splitFields(line, m.cfg)
		fk := joinKey(fields, m.cfg.field2)
		m.checkKey(2, fk, line)
		if compareKeys(fk, key, m.cfg.ignoreCase) != 0 {
			m.lr2.pushBack(line)
			return group
		}
		group = append(group, fields)
	}
}

// emitPairs emits all pairings of one file-1 line with the file-2 group.
func emitPairs(cfg config, key string, f1 []string, group [][]string, bw *bufio.Writer) int {
	if suppressPaired(cfg) {
		return 0
	}
	for _, f2 := range group {
		line := formatPaired(cfg, key, f1, f2)
		if err := writeLine(bw, line); err != nil {
			fmt.Fprintf(os.Stderr, "join: write error: %s\n", err)
			return 1
		}
	}
	return 0
}

// emitUnpair handles an unpairable line.
func emitUnpair(cfg config, fields []string, fieldIdx, fileNum int, bw *bufio.Writer) int {
	if !showUnpairable(cfg, fileNum) {
		return 0
	}
	line := formatUnpairable(cfg, fields, fieldIdx, fileNum)
	if err := writeLine(bw, line); err != nil {
		fmt.Fprintf(os.Stderr, "join: write error: %s\n", err)
		return 1
	}
	return 0
}

// drainUnpaired outputs remaining unpairable lines from a lineReader.
func drainUnpaired(cfg config, lr *lineReader, cur string, has bool, fieldIdx, fileNum int, bw *bufio.Writer) int {
	if !showUnpairable(cfg, fileNum) {
		return 0
	}
	for has {
		fields := splitFields(cur, cfg)
		line := formatUnpairable(cfg, fields, fieldIdx, fileNum)
		if err := writeLine(bw, line); err != nil {
			fmt.Fprintf(os.Stderr, "join: write error: %s\n", err)
			return 1
		}
		cur, has = lr.next()
	}
	return 0
}

// checkReaderErrors checks both readers for scan errors.
func checkReaderErrors(lr1, lr2 *lineReader) int {
	if err := lr1.err(); err != nil {
		fmt.Fprintf(os.Stderr, "join: read error: %s\n", err)
		return 1
	}
	if err := lr2.err(); err != nil {
		fmt.Fprintf(os.Stderr, "join: read error: %s\n", err)
		return 1
	}
	return 0
}

// writeLine writes a line followed by a newline to the writer.
func writeLine(bw *bufio.Writer, line string) error {
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// --- Flag parsing ---

// parseArgs processes command-line flags and returns config, file names, exit code.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (config, string, string, int) {
	cfg := config{field1: 1, field2: 1}
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		exit := parseFlag(arg, args, &i, &cfg)
		if exit >= 0 {
			return config{}, "", "", exit
		}
	}
	return validatePositionals(cfg, positionals)
}

// parseFlag dispatches a single flag argument.
func parseFlag(arg string, args []string, i *int, cfg *config) int {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, cfg)
	}
	return parseShortFlag(arg, args, i, cfg)
}

// parseLongFlag handles --flag arguments.
func parseLongFlag(arg string, cfg *config) int {
	switch arg {
	case "--header":
		cfg.header = true
		return -1
	case "--ignore-case":
		cfg.ignoreCase = true
		return -1
	case "--check-order":
		// R3.1: --check-order is the default behavior; recognized as no-op.
		return -1
	// TODO: --nocheck-order not implemented per prd069-join non_goals (E6).
	case "--help":
		return printHelp()
	case "--version":
		return printVersion()
	default:
		fmt.Fprintf(os.Stderr, "join: unrecognized option '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'join --help' for more information.")
		return 1
	}
}

// parseShortFlag handles short flags like -1, -2, -j, -t, -o, -a, -v, -e, -i.
func parseShortFlag(arg string, args []string, i *int, cfg *config) int {
	flag := arg[1:]
	if len(flag) == 0 {
		return -1
	}
	switch flag[0] {
	case '1':
		return parseFieldFlag(flag[1:], args, i, &cfg.field1)
	case '2':
		return parseFieldFlag(flag[1:], args, i, &cfg.field2)
	case 'j':
		return parseJointField(flag[1:], args, i, cfg)
	case 't':
		return parseSepFlag(flag[1:], args, i, cfg)
	case 'o':
		return parseOutputFlag(flag[1:], args, i, cfg)
	case 'a':
		return parseFileNumFlag(flag[1:], args, i, cfg, setUnpair)
	case 'v':
		return parseFileNumFlag(flag[1:], args, i, cfg, setOnly)
	case 'e':
		return parseEmptyFlag(flag[1:], args, i, cfg)
	case 'i':
		// R2.4: -i for case-insensitive join field comparison.
		cfg.ignoreCase = true
		return -1
	default:
		fmt.Fprintf(os.Stderr, "join: invalid option -- '%c'\n", flag[0])
		fmt.Fprintln(os.Stderr, "Try 'join --help' for more information.")
		return 1
	}
}

// parseFieldFlag parses -1 FIELD or -2 FIELD.
func parseFieldFlag(rest string, args []string, i *int, target *int) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		fmt.Fprintf(os.Stderr, "join: invalid field number: '%s'\n", val)
		return 1
	}
	*target = n
	return -1
}

// parseJointField parses -j FIELD and sets both field1 and field2.
func parseJointField(rest string, args []string, i *int, cfg *config) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		fmt.Fprintf(os.Stderr, "join: invalid field number: '%s'\n", val)
		return 1
	}
	cfg.field1 = n
	cfg.field2 = n
	return -1
}

// parseSepFlag parses -t CHAR.
func parseSepFlag(rest string, args []string, i *int, cfg *config) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	if len(val) != 1 {
		fmt.Fprintf(os.Stderr, "join: multi-character tab '%s'\n", val)
		return 1
	}
	cfg.sep = val
	cfg.hasSep = true
	return -1
}

// parseOutputFlag parses -o FORMAT.
func parseOutputFlag(rest string, args []string, i *int, cfg *config) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	return parseOutputFormat(val, args, i, cfg)
}

// parseOutputFormat parses -o format specifiers, consuming additional space-separated args.
func parseOutputFormat(first string, args []string, i *int, cfg *config) int {
	if code := parseSpecList(first, cfg); code != 0 {
		return code
	}
	// R2.3: GNU join allows space-separated specs: -o 1.1 2.1 0
	for *i+1 < len(args) {
		next := args[*i+1]
		if strings.HasPrefix(next, "-") && next != "-" {
			break
		}
		if !looksLikeSpec(next) {
			break
		}
		*i++
		if code := parseSpecList(next, cfg); code != 0 {
			return code
		}
	}
	return -1
}

// looksLikeSpec returns true if the string looks like an output spec.
func looksLikeSpec(s string) bool {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "0" {
			continue
		}
		if !strings.Contains(part, ".") {
			return false
		}
	}
	return true
}

// parseSpecList parses a comma-separated list of output specs.
func parseSpecList(s string, cfg *config) int {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec, err := parseOneSpec(part)
		if err != nil {
			fmt.Fprintf(os.Stderr, "join: invalid field spec: '%s'\n", part)
			return 1
		}
		cfg.outSpecs = append(cfg.outSpecs, spec)
	}
	return -1
}

// parseOneSpec parses a single FILENUM.FIELDNUM or '0' specifier.
func parseOneSpec(s string) (outputSpec, error) {
	if s == "0" {
		return outputSpec{fileNum: 0}, nil
	}
	before, after, found := strings.Cut(s, ".")
	if !found {
		return outputSpec{}, fmt.Errorf("missing dot in spec")
	}
	fileNum, err := strconv.Atoi(before)
	if err != nil || (fileNum != 1 && fileNum != 2) {
		return outputSpec{}, fmt.Errorf("invalid file number")
	}
	fieldNum, err := strconv.Atoi(after)
	if err != nil || fieldNum < 1 {
		return outputSpec{}, fmt.Errorf("invalid field number")
	}
	return outputSpec{fileNum: fileNum, fieldNum: fieldNum}, nil
}

// setFunc is used to set unpair/only flags by file number.
type setFunc func(cfg *config, fileNum int)

// setUnpair sets the -a flag for a file number.
func setUnpair(cfg *config, fileNum int) {
	if fileNum == 1 {
		cfg.unpairFile1 = true
	} else {
		cfg.unpairFile2 = true
	}
}

// setOnly sets the -v flag for a file number.
func setOnly(cfg *config, fileNum int) {
	if fileNum == 1 {
		cfg.onlyFile1 = true
	} else {
		cfg.onlyFile2 = true
	}
}

// parseFileNumFlag parses -a FILENUM or -v FILENUM.
func parseFileNumFlag(rest string, args []string, i *int, cfg *config, setter setFunc) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	n, err := strconv.Atoi(val)
	if err != nil || (n != 1 && n != 2) {
		fmt.Fprintf(os.Stderr, "join: invalid file number: '%s'\n", val)
		return 1
	}
	setter(cfg, n)
	return -1
}

// parseEmptyFlag parses -e STRING.
func parseEmptyFlag(rest string, args []string, i *int, cfg *config) int {
	val, exit := getArgValue(rest, args, i)
	if exit >= 0 {
		return exit
	}
	cfg.empty = val
	return -1
}

// getArgValue returns the value attached to a flag or from the next argument.
func getArgValue(rest string, args []string, i *int) (string, int) {
	if rest != "" {
		return rest, -1
	}
	*i++
	if *i >= len(args) {
		fmt.Fprintln(os.Stderr, "join: option requires an argument")
		fmt.Fprintln(os.Stderr, "Try 'join --help' for more information.")
		return "", 1
	}
	return args[*i], -1
}

// validatePositionals checks that exactly two file arguments are provided.
func validatePositionals(cfg config, pos []string) (config, string, string, int) {
	if len(pos) < 2 {
		fmt.Fprintln(os.Stderr, "join: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'join --help' for more information.")
		return config{}, "", "", 1
	}
	if len(pos) > 2 {
		fmt.Fprintf(os.Stderr, "join: extra operand '%s'\n", pos[2])
		fmt.Fprintln(os.Stderr, "Try 'join --help' for more information.")
		return config{}, "", "", 1
	}
	return cfg, pos[0], pos[1], -1
}

// --- Help and version ---

// printHelp writes usage information to stdout.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: join [OPTION]... FILE1 FILE2
Join lines of two sorted files on a common field.

  -a FILENUM        also print unpairable lines from file FILENUM
  -e STRING         replace missing (empty) input fields with STRING
  -i, --ignore-case ignore differences in case when comparing fields
  -j FIELD          equivalent to '-1 FIELD -2 FIELD'
  -o FORMAT         output FORMAT is a comma or space separated list
  -t CHAR           use CHAR as input and output field separator
  -v FILENUM        like -a FILENUM, but suppress joined output lines
  -1 FIELD          join on this FIELD of file 1
  -2 FIELD          join on this FIELD of file 2
      --check-order check that input is sorted (default)
      --header      treat the first line in each file as a header
      --help        display this help and exit
      --version     output version information and exit
`)
	return 0
}

// printVersion writes version information to stdout.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "join (go-unix-utils) %s\n", version)
	return 0
}
