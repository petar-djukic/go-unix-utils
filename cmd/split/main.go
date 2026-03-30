// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the split utility.
// Implements prd067-split R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultLines     = 1000
	defaultPrefix    = "x"
	defaultSuffixLen = 2
)

// splitMode identifies the splitting strategy.
type splitMode int

const (
	modeUnset     splitMode = iota
	modeLines               // -l (default)
	modeBytes               // -b
	modeLineBytes           // -C
	modeChunks              // -n
)

// chunkType identifies the sub-mode for -n CHUNKS.
type chunkType int

const (
	chunkByBytes    chunkType = iota // N
	chunkByLines                     // l/N
	chunkRoundRobin                  // r/N
)

// chunkSpec holds the parsed -n argument.
type chunkSpec struct {
	typ   chunkType
	count int
}

// config holds parsed command-line options for split.
type config struct {
	mode      splitMode
	lines     int
	byteCount int64
	chunk     chunkSpec
	file      string
	prefix    string
	suffixLen int
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments, opens input, and dispatches to the split mode.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	r, err := openInput(cfg.file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	defer r.Close()
	if err := dispatch(r, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	return 0
}

// dispatch routes to the correct splitting function based on cfg.mode.
func dispatch(r io.Reader, cfg config) error {
	switch cfg.mode {
	case modeBytes:
		return splitByBytes(r, cfg)
	case modeLineBytes:
		return splitByLineBytes(r, cfg)
	case modeChunks:
		return splitByChunks(r, cfg)
	default:
		return splitByLines(r, cfg)
	}
}

// parseArgs parses CLI arguments into a config.
func parseArgs(args []string) (config, error) {
	cfg := config{
		lines:     defaultLines,
		prefix:    defaultPrefix,
		suffixLen: defaultSuffixLen,
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		var err error
		i, err = parseFlag(&cfg, args, i)
		if err != nil {
			return cfg, err
		}
	}
	return cfg, applyPositional(&cfg, positional)
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, args []string, i int) (int, error) {
	if strings.HasPrefix(args[i], "--") {
		return parseLongFlag(cfg, args, i)
	}
	return parseShortFlag(cfg, args, i)
}

// parseLongFlag handles --lines, --bytes, --line-bytes, --number.
func parseLongFlag(cfg *config, args []string, i int) (int, error) {
	key, val, hasEq := strings.Cut(args[i], "=")
	if key != "--lines" && key != "--bytes" && key != "--line-bytes" && key != "--number" {
		return i, fmt.Errorf("unrecognized option '%s'", args[i])
	}
	if !hasEq {
		if i+1 >= len(args) {
			return i, fmt.Errorf("option '%s' requires an argument", key)
		}
		i++
		val = args[i]
	}
	switch key {
	case "--lines":
		return i, setLines(cfg, val)
	case "--bytes":
		return i, setByteMode(cfg, modeBytes, val)
	case "--line-bytes":
		return i, setByteMode(cfg, modeLineBytes, val)
	default:
		return i, setChunks(cfg, val)
	}
}

// parseShortFlag handles -l, -b, -C, -n with optional attached values.
func parseShortFlag(cfg *config, args []string, i int) (int, error) {
	flag := args[i][1]
	if flag != 'l' && flag != 'b' && flag != 'C' && flag != 'n' {
		return i, fmt.Errorf("invalid option -- '%c'", flag)
	}
	val := args[i][2:]
	if val == "" {
		if i+1 >= len(args) {
			return i, fmt.Errorf("option '-%c' requires an argument", flag)
		}
		i++
		val = args[i]
	}
	switch flag {
	case 'l':
		return i, setLines(cfg, val)
	case 'b':
		return i, setByteMode(cfg, modeBytes, val)
	case 'C':
		return i, setByteMode(cfg, modeLineBytes, val)
	default:
		return i, setChunks(cfg, val)
	}
}

// setMode sets the split mode, enforcing R2.4 conflict detection.
func setMode(cfg *config, mode splitMode) error {
	if cfg.mode != modeUnset && cfg.mode != mode {
		return fmt.Errorf("cannot split in more than one way")
	}
	cfg.mode = mode
	return nil
}

// setLines configures line-based splitting.
func setLines(cfg *config, val string) error {
	if err := setMode(cfg, modeLines); err != nil {
		return err
	}
	n, err := parseLinesValue(val)
	if err != nil {
		return err
	}
	cfg.lines = n
	return nil
}

// setByteMode configures byte-based or line-bytes splitting.
// R2.1, R2.2: shared setter for -b and -C modes.
func setByteMode(cfg *config, mode splitMode, val string) error {
	if err := setMode(cfg, mode); err != nil {
		return err
	}
	n, err := parseByteValue(val)
	if err != nil {
		return err
	}
	cfg.byteCount = n
	return nil
}

// setChunks configures chunk-based splitting.
func setChunks(cfg *config, val string) error {
	if err := setMode(cfg, modeChunks); err != nil {
		return err
	}
	spec, err := parseChunkSpec(val)
	if err != nil {
		return err
	}
	cfg.chunk = spec
	return nil
}

// parseLinesValue parses a positive integer line count string.
func parseLinesValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, nil
}

// parseByteValue parses a size string with optional unit suffix.
// R2.1: supports K, M, G, T, P, E and KB, MB, etc.
func parseByteValue(s string) (int64, error) {
	n, err := sizeparse.Parse(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of bytes: '%s'", s)
	}
	return n, nil
}

// parseChunkSpec parses a -n CHUNKS argument into a chunkSpec.
// R2.3: supports N, l/N, and r/N forms.
func parseChunkSpec(s string) (chunkSpec, error) {
	if strings.HasPrefix(s, "l/") {
		return parseChunkCount(s[2:], chunkByLines)
	}
	if strings.HasPrefix(s, "r/") {
		return parseChunkCount(s[2:], chunkRoundRobin)
	}
	return parseChunkCount(s, chunkByBytes)
}

// parseChunkCount parses a positive integer chunk count.
func parseChunkCount(s string, typ chunkType) (chunkSpec, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return chunkSpec{}, fmt.Errorf("invalid number of chunks: '%s'", s)
	}
	return chunkSpec{typ: typ, count: n}, nil
}

// applyPositional maps positional arguments to config fields.
// R1.2: [FILE [PREFIX]] positional arguments.
func applyPositional(cfg *config, args []string) error {
	switch len(args) {
	case 0:
		cfg.file = "-"
	case 1:
		cfg.file = args[0]
	case 2:
		cfg.file = args[0]
		cfg.prefix = args[1]
	default:
		return fmt.Errorf("extra operand '%s'", args[2])
	}
	return nil
}

// openInput opens the input file, or returns stdin for "-".
// R1.4: read from stdin when FILE is "-" or absent.
func openInput(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}

// splitByLines reads from r and writes chunks of cfg.lines lines each.
// R1.1: default 1000 lines. R1.3: configurable via -l.
func splitByLines(r io.Reader, cfg config) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for fileIndex := 0; ; fileIndex++ {
		suffix, err := generateSuffix(fileIndex, cfg.suffixLen)
		if err != nil {
			return err
		}
		done, werr := writeLineChunk(br, cfg.prefix+suffix, cfg.lines)
		if werr != nil {
			return werr
		}
		if done {
			return nil
		}
	}
}

// writeLineChunk reads up to maxLines lines and writes them to filename.
// Returns true when EOF is reached (no more data to read).
func writeLineChunk(br *bufio.Reader, filename string, maxLines int) (bool, error) {
	var buf bytes.Buffer
	eof := false
	for range maxLines {
		line, err := br.ReadBytes('\n')
		buf.Write(line)
		if err == io.EOF {
			eof = true
			break
		}
		if err != nil {
			return false, err
		}
	}
	if buf.Len() > 0 {
		if err := os.WriteFile(filename, buf.Bytes(), 0o666); err != nil {
			return false, err
		}
	}
	return eof, nil
}

// splitByBytes reads from r and writes chunks of cfg.byteCount bytes each.
// R2.1: byte-based splitting.
func splitByBytes(r io.Reader, cfg config) error {
	for fileIndex := 0; ; fileIndex++ {
		suffix, err := generateSuffix(fileIndex, cfg.suffixLen)
		if err != nil {
			return err
		}
		done, werr := writeByteChunk(r, cfg.prefix+suffix, cfg.byteCount)
		if werr != nil {
			return werr
		}
		if done {
			return nil
		}
	}
}

// writeByteChunk reads up to maxBytes bytes and writes them to filename.
// Returns true when EOF is reached.
func writeByteChunk(r io.Reader, filename string, maxBytes int64) (bool, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes))
	if len(data) == 0 {
		return true, nil
	}
	if werr := os.WriteFile(filename, data, 0o666); werr != nil {
		return false, werr
	}
	return int64(len(data)) < maxBytes, err
}

// splitByLineBytes splits input into pieces of at most cfg.byteCount bytes,
// breaking only at line boundaries. R2.2.
func splitByLineBytes(r io.Reader, cfg config) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var pending []byte
	fileIndex := 0
	for {
		chunk, leftover, eof, err := collectLineBytes(br, pending, cfg.byteCount)
		if err != nil {
			return err
		}
		pending = leftover
		if len(chunk) == 0 {
			return nil
		}
		suffix, serr := generateSuffix(fileIndex, cfg.suffixLen)
		if serr != nil {
			return serr
		}
		if werr := os.WriteFile(cfg.prefix+suffix, chunk, 0o666); werr != nil {
			return werr
		}
		fileIndex++
		if eof && len(pending) == 0 {
			return nil
		}
	}
}

// collectLineBytes accumulates lines up to maxBytes, returning the chunk,
// any leftover data, EOF status, and any error.
// R2.2: lines longer than maxBytes are split across chunks.
func collectLineBytes(
	br *bufio.Reader, pending []byte, maxBytes int64,
) ([]byte, []byte, bool, error) {
	var buf bytes.Buffer
	if len(pending) > 0 {
		if int64(len(pending)) > maxBytes {
			return pending[:maxBytes], pending[maxBytes:], false, nil
		}
		buf.Write(pending)
		if int64(buf.Len()) >= maxBytes {
			return buf.Bytes(), nil, false, nil
		}
	}
	for {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && err == io.EOF {
			return buf.Bytes(), nil, true, nil
		}
		if err != nil && err != io.EOF {
			return nil, nil, false, err
		}
		isEOF := err == io.EOF
		if buf.Len() == 0 && int64(len(line)) > maxBytes {
			return line[:maxBytes], line[maxBytes:], isEOF, nil
		}
		if buf.Len() > 0 && int64(buf.Len()+len(line)) > maxBytes {
			return buf.Bytes(), line, isEOF, nil
		}
		buf.Write(line)
		if isEOF {
			return buf.Bytes(), nil, true, nil
		}
		if int64(buf.Len()) >= maxBytes {
			return buf.Bytes(), nil, false, nil
		}
	}
}

// splitByChunks reads all input and splits into cfg.chunk.count pieces.
// R2.3: supports byte chunks (N), line chunks (l/N), round-robin (r/N).
func splitByChunks(r io.Reader, cfg config) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	switch cfg.chunk.typ {
	case chunkByLines:
		return splitChunkLines(data, cfg)
	case chunkRoundRobin:
		return splitChunkRoundRobin(data, cfg)
	default:
		return splitChunkBytes(data, cfg)
	}
}

// splitChunkBytes divides data into cfg.chunk.count byte-sized chunks.
func splitChunkBytes(data []byte, cfg config) error {
	n := cfg.chunk.count
	total := len(data)
	base := total / n
	remainder := total % n
	offset := 0
	for i := range n {
		size := base
		if i < remainder {
			size++
		}
		if err := writeChunkFile(cfg, i, data[offset:offset+size]); err != nil {
			return err
		}
		offset += size
	}
	return nil
}

// splitChunkLines divides lines into cfg.chunk.count groups, balanced by
// byte size with line-boundary breaks. R2.3: l/N form.
func splitChunkLines(data []byte, cfg config) error {
	lines := splitIntoLines(data)
	n := cfg.chunk.count
	total := len(data)
	byteOff := 0
	lineOff := 0
	for i := range n {
		lineEnd := len(lines)
		if i < n-1 {
			target := total * (i + 1) / n
			lineEnd = lineOff
			for lineEnd < len(lines) && byteOff < target {
				byteOff += len(lines[lineEnd])
				lineEnd++
			}
		}
		if err := writeChunkFile(cfg, i, joinLines(lines, lineOff, lineEnd)); err != nil {
			return err
		}
		lineOff = lineEnd
	}
	return nil
}

// splitChunkRoundRobin distributes lines across cfg.chunk.count files
// in round-robin order. R2.3: r/N form.
func splitChunkRoundRobin(data []byte, cfg config) error {
	lines := splitIntoLines(data)
	n := cfg.chunk.count
	bufs := make([]bytes.Buffer, n)
	for i, line := range lines {
		bufs[i%n].Write(line)
	}
	for i := range n {
		if err := writeChunkFile(cfg, i, bufs[i].Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// writeChunkFile writes chunk data to the output file for chunk index i.
func writeChunkFile(cfg config, i int, data []byte) error {
	suffix, err := generateSuffix(i, cfg.suffixLen)
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.prefix+suffix, data, 0o666)
}

// joinLines concatenates a sub-slice of lines into a single byte slice.
func joinLines(lines [][]byte, from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i < to; i++ {
		buf.Write(lines[i])
	}
	return buf.Bytes()
}

// splitIntoLines splits data into lines, each including its trailing newline.
func splitIntoLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	var lines [][]byte
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			lines = append(lines, data)
			break
		}
		lines = append(lines, data[:idx+1])
		data = data[idx+1:]
	}
	return lines
}

// generateSuffix returns the alphabetic suffix for the given index and length.
// R1.1: suffix pattern aa, ab, ..., az, ba, ..., zz for length 2.
func generateSuffix(index, length int) (string, error) {
	suffix := make([]byte, length)
	idx := index
	for i := length - 1; i >= 0; i-- {
		suffix[i] = 'a' + byte(idx%26)
		idx /= 26
	}
	if idx > 0 {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return string(suffix), nil
}
