// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/split: split a file into pieces.
// Implements srd067-split R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.2.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "split"

// defaultLines is the number of lines per piece when no mode flag is given.
// R1.1: default is 1000 lines.
const defaultLines int64 = 1000

// defaultPrefix is the output filename prefix when none is specified.
// R1.1: default prefix is "x".
const defaultPrefix = "x"

// defaultSuffixLen is the default suffix length.
// R3.1: default suffix length is 2.
const defaultSuffixLen = 2

// splitMode distinguishes the splitting strategy.
type splitMode int

const (
	modeLineCount splitMode = iota // R1.1, R1.3: split by line count
	modeBytes                      // R2.1: split by byte count
	modeLineBytes                  // R2.2: split by line-bytes
	modeChunks                     // R2.3: split by chunk count
)

// chunkType distinguishes the three forms of -n CHUNKS.
type chunkType int

const (
	chunkByBytes    chunkType = iota // N: split into N byte-sized chunks
	chunkByLines                     // l/N: split into N line-based chunks
	chunkRoundRobin                  // r/N: round-robin lines into N chunks
)

// chunkSpec holds the parsed -n CHUNKS value.
type chunkSpec struct {
	kind  chunkType
	count int64
}

// config holds parsed command-line options.
type config struct {
	mode             splitMode
	modeCount        int
	lineCount        int64
	byteCount        int64
	chunks           chunkSpec
	suffixLen        int
	numericSuffix    bool
	additionalSuffix string
	filter           string
	prefix           string
	inputFile        string
}

// flagVal holds a parsed flag value and the number of args consumed.
type flagVal struct {
	val string
	adv int
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the split logic and returns the exit code.
// R4.1: returns 0 on success.
// R4.2: returns 1 on error.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		reportError(err.Error())
		return 1
	}
	rc, err := openInput(cfg.inputFile)
	if err != nil {
		reportError(err.Error())
		return 1
	}
	defer rc.Close()
	if err := dispatch(rc, &cfg); err != nil {
		reportError(err.Error())
		return 1
	}
	return 0
}

// dispatch selects and runs the appropriate splitting strategy.
func dispatch(r io.Reader, cfg *config) error {
	switch cfg.mode {
	case modeLineCount:
		return splitByLines(r, cfg)
	case modeBytes:
		return splitByBytes(r, cfg)
	case modeLineBytes:
		return splitByLineBytes(r, cfg)
	case modeChunks:
		return splitByChunks(r, cfg)
	}
	return fmt.Errorf("unknown split mode")
}

// parseArgs separates flags from positional arguments and returns a config.
// R2.4: conflicting split options produce an error and exit 1.
func parseArgs(args []string) (config, error) {
	cfg := config{
		mode:      modeLineCount,
		lineCount: defaultLines,
		suffixLen: defaultSuffixLen,
		prefix:    defaultPrefix,
		inputFile: "-",
	}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "--") {
			adv, err := parseLongFlag(&cfg, args, i)
			if err != nil {
				return cfg, err
			}
			i += adv
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			adv, err := parseShortFlags(&cfg, args, i)
			if err != nil {
				return cfg, err
			}
			i += adv
			continue
		}
		break
	}
	return applyPositional(cfg, args[i:])
}

// applyPositional handles [FILE [PREFIX]] after flag parsing.
// R1.2: PREFIX replaces the default 'x'.
// R1.4: absent FILE defaults to stdin ("-").
func applyPositional(cfg config, pos []string) (config, error) {
	if len(pos) > 0 {
		cfg.inputFile = pos[0]
	}
	if len(pos) > 1 {
		cfg.prefix = pos[1]
	}
	if len(pos) > 2 {
		return cfg, fmt.Errorf("extra operand %q", pos[2])
	}
	return cfg, validateConfig(&cfg)
}

// parseLongFlag handles a --key=value or --key value style flag.
func parseLongFlag(cfg *config, args []string, i int) (int, error) {
	key, val, hasVal := splitLongFlag(args[i])
	switch key {
	case "--lines":
		return parseFlagWithSetter(key, val, hasVal, args, i, setLineCount(cfg))
	case "--bytes":
		return parseFlagWithSetter(key, val, hasVal, args, i, setByteCount(cfg))
	case "--line-bytes":
		return parseFlagWithSetter(key, val, hasVal, args, i, setLineByteCount(cfg))
	case "--number":
		return parseFlagWithSetter(key, val, hasVal, args, i, setChunkCount(cfg))
	case "--suffix-length":
		return parseFlagWithSetter(key, val, hasVal, args, i, setSuffixLen(cfg))
	case "--numeric-suffixes":
		cfg.numericSuffix = true
		return 1, nil
	case "--additional-suffix":
		return parseFlagWithSetter(key, val, hasVal, args, i, setString(&cfg.additionalSuffix))
	case "--filter":
		return parseFlagWithSetter(key, val, hasVal, args, i, setString(&cfg.filter))
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", args[i])
	}
}

// splitLongFlag splits --key=value into key, value, hasValue.
func splitLongFlag(arg string) (string, string, bool) {
	key, val, found := strings.Cut(arg, "=")
	return key, val, found
}

// parseFlagWithSetter resolves a flag value and applies it via setter.
func parseFlagWithSetter(key, val string, hasVal bool, args []string, i int, apply func(string) error) (int, error) {
	fv, err := resolveFlagVal(val, hasVal, args, i)
	if err != nil {
		return 0, fmt.Errorf("option '%s' requires an argument", key)
	}
	if err := apply(fv.val); err != nil {
		return 0, err
	}
	return fv.adv, nil
}

// resolveFlagVal extracts a flag value from inline or next arg.
func resolveFlagVal(val string, hasVal bool, args []string, i int) (flagVal, error) {
	if hasVal {
		return flagVal{val: val, adv: 1}, nil
	}
	if i+1 >= len(args) {
		return flagVal{}, fmt.Errorf("requires an argument")
	}
	return flagVal{val: args[i+1], adv: 2}, nil
}

// parseShortFlags handles single-character flags, possibly combined.
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	flags := args[i][1:]
	j := 0
	for j < len(flags) {
		switch flags[j] {
		case 'l':
			return shortWithArg(flags[j+1:], args, i, setLineCount(cfg))
		case 'b':
			return shortWithArg(flags[j+1:], args, i, setByteCount(cfg))
		case 'C':
			return shortWithArg(flags[j+1:], args, i, setLineByteCount(cfg))
		case 'n':
			return shortWithArg(flags[j+1:], args, i, setChunkCount(cfg))
		case 'a':
			return shortWithArg(flags[j+1:], args, i, setSuffixLen(cfg))
		case 'd':
			cfg.numericSuffix = true
			j++
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// shortWithArg extracts the value for a short flag that takes an argument.
func shortWithArg(rest string, args []string, i int, apply func(string) error) (int, error) {
	if rest != "" {
		return 1, apply(rest)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument")
	}
	return 2, apply(args[i+1])
}

// setLineCount returns a setter for the -l/--lines value.
func setLineCount(cfg *config) func(string) error {
	return func(s string) error {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid number of lines: %q", s)
		}
		cfg.mode = modeLineCount
		cfg.lineCount = n
		cfg.modeCount++
		return nil
	}
}

// setByteCount returns a setter for the -b/--bytes value.
func setByteCount(cfg *config) func(string) error {
	return func(s string) error {
		n, err := parseByteSize(s)
		if err != nil {
			return err
		}
		cfg.mode = modeBytes
		cfg.byteCount = n
		cfg.modeCount++
		return nil
	}
}

// setLineByteCount returns a setter for the -C/--line-bytes value.
func setLineByteCount(cfg *config) func(string) error {
	return func(s string) error {
		n, err := parseByteSize(s)
		if err != nil {
			return err
		}
		cfg.mode = modeLineBytes
		cfg.byteCount = n
		cfg.modeCount++
		return nil
	}
}

// setChunkCount returns a setter for the -n/--number value.
func setChunkCount(cfg *config) func(string) error {
	return func(s string) error {
		cs, err := parseChunkSpec(s)
		if err != nil {
			return err
		}
		cfg.mode = modeChunks
		cfg.chunks = cs
		cfg.modeCount++
		return nil
	}
}

// setSuffixLen returns a setter for the -a/--suffix-length value.
func setSuffixLen(cfg *config) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid suffix length: %q", s)
		}
		cfg.suffixLen = n
		return nil
	}
}

// setString returns a setter that stores a string value.
func setString(dst *string) func(string) error {
	return func(s string) error {
		*dst = s
		return nil
	}
}

// parseChunkSpec parses a CHUNKS argument into a chunkSpec.
// R2.3: supports N, l/N, and r/N forms.
func parseChunkSpec(s string) (chunkSpec, error) {
	if strings.HasPrefix(s, "l/") {
		return parseChunkNum(s[2:], chunkByLines)
	}
	if strings.HasPrefix(s, "r/") {
		return parseChunkNum(s[2:], chunkRoundRobin)
	}
	return parseChunkNum(s, chunkByBytes)
}

// parseChunkNum parses the numeric part of a chunk specification.
func parseChunkNum(s string, kind chunkType) (chunkSpec, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return chunkSpec{}, fmt.Errorf("invalid number of chunks: %q", s)
	}
	return chunkSpec{kind: kind, count: n}, nil
}

// parseByteSize parses a byte count with optional suffix via sizeparse.
// R2.1: supports K, M, G, T, P, E, Z, Y and KB, MB, etc.
func parseByteSize(s string) (int64, error) {
	n, err := sizeparse.Parse(s)
	if err != nil {
		return 0, fmt.Errorf("invalid number of bytes: %q", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("invalid number of bytes: %q", s)
	}
	return n, nil
}

// validateConfig checks for conflicting options.
// R2.4: conflicting split options must produce an error.
func validateConfig(cfg *config) error {
	if cfg.modeCount > 1 {
		return fmt.Errorf("cannot split in more than one way")
	}
	return nil
}

// openInput returns the input reader for the given file argument.
// R1.4: reads from stdin when FILE is "-" or absent.
func openInput(name string) (io.ReadCloser, error) {
	if name == "" || name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(name)
}

// suffixGenerator produces sequential suffixes for output files.
type suffixGenerator struct {
	length  int
	numeric bool
	current int
}

// newSuffixGenerator creates a suffix generator.
// R3.1: suffix length is configurable via -a.
// R3.2: -d uses numeric suffixes.
func newSuffixGenerator(length int, numeric bool) *suffixGenerator {
	return &suffixGenerator{length: length, numeric: numeric}
}

// next returns the next suffix string and advances the counter.
func (g *suffixGenerator) next() (string, error) {
	limit := g.maxCount()
	if g.current >= limit {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	var s string
	if g.numeric {
		s = fmt.Sprintf("%0*d", g.length, g.current)
	} else {
		s = alphaEncode(g.current, g.length)
	}
	g.current++
	return s, nil
}

// maxCount returns the total number of suffixes available.
func (g *suffixGenerator) maxCount() int {
	base := 26
	if g.numeric {
		base = 10
	}
	result := 1
	for range g.length {
		result *= base
	}
	return result
}

// alphaEncode converts a number to an alphabetic suffix of given length.
func alphaEncode(n, length int) string {
	buf := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		buf[i] = 'a' + byte(n%26)
		n /= 26
	}
	return string(buf)
}

// outputFilename builds the full output filename.
// R1.2: prefix + suffix + additional-suffix.
// R3.3: appends --additional-suffix after the generated suffix.
func outputFilename(prefix, suffix, additional string) string {
	return prefix + suffix + additional
}

// splitByLines splits input into pieces of n lines each.
// R1.1, R1.3: line-count splitting mode.
func splitByLines(r io.Reader, cfg *config) error {
	gen := newSuffixGenerator(cfg.suffixLen, cfg.numericSuffix)
	br := bufio.NewReader(r)
	var w io.WriteCloser
	var count int64
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			w, count, err = writeLine(w, count, line, gen, cfg)
			if err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			closeIfOpen(w)
			return err
		}
	}
	return closeIfOpen(w)
}

// writeLine writes a line to the current piece, opening a new piece if needed.
func writeLine(w io.WriteCloser, count int64, line []byte, gen *suffixGenerator, cfg *config) (io.WriteCloser, int64, error) {
	if w == nil {
		var err error
		w, err = openNextPiece(gen, cfg)
		if err != nil {
			return nil, 0, err
		}
		count = 0
	}
	if _, err := w.Write(line); err != nil {
		closeIfOpen(w)
		return nil, 0, err
	}
	count++
	if count >= cfg.lineCount {
		if err := w.Close(); err != nil {
			return nil, 0, err
		}
		return nil, 0, nil
	}
	return w, count, nil
}

// openNextPiece opens the next output piece file.
func openNextPiece(gen *suffixGenerator, cfg *config) (io.WriteCloser, error) {
	suffix, err := gen.next()
	if err != nil {
		return nil, err
	}
	fname := outputFilename(cfg.prefix, suffix, cfg.additionalSuffix)
	return openOutputPiece(fname, cfg.filter)
}

// splitByBytes splits input into pieces of n bytes each.
// R2.1: byte-count splitting mode.
func splitByBytes(r io.Reader, cfg *config) error {
	panic("not implemented")
}

// splitByLineBytes splits input into pieces of at most n bytes,
// breaking only at line boundaries.
// R2.2: line-bytes splitting mode.
func splitByLineBytes(r io.Reader, cfg *config) error {
	panic("not implemented")
}

// splitByChunks splits input into a fixed number of chunks.
// R2.3: chunk splitting mode with N, l/N, r/N forms.
func splitByChunks(r io.Reader, cfg *config) error {
	panic("not implemented")
}

// openOutputPiece opens or creates an output file for writing.
// R3.4: when --filter is set, pipes to a shell command instead.
func openOutputPiece(filename, filter string) (io.WriteCloser, error) {
	if filter != "" {
		panic("not implemented") // R3.4: filter mode
	}
	return os.Create(filename)
}

// closeIfOpen closes w if it is non-nil.
func closeIfOpen(w io.WriteCloser) error {
	if w != nil {
		return w.Close()
	}
	return nil
}

// reportError prints a GNU-compatible diagnostic to stderr.
func reportError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
}
