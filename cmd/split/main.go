// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type splitMode int

const (
	modeLines splitMode = iota
	modeBytes
	modeLineBytes
	modeChunks
)

type options struct {
	mode             splitMode
	modeSet          bool
	lines            int
	bytesVal         int64
	lineBytes        int64
	chunks           string
	prefix           string
	suffixLen        int
	numeric          bool
	additionalSuffix string
	filter           string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, file, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}

	r, closer, err := openInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}

	if err := run(r, opts); err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (options, string, error) {
	opts := options{lines: 1000, prefix: "x", suffixLen: 2}
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i:], &opts)
			if err != nil {
				return opts, "", err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, "", err
			}
			i += 1 + n
			continue
		}
		positional = append(positional, arg)
		i++
	}

	return assignPositional(opts, positional)
}

func assignPositional(opts options, pos []string) (options, string, error) {
	file := "-"
	if len(pos) >= 1 {
		file = pos[0]
	}
	if len(pos) >= 2 {
		opts.prefix = pos[1]
	}
	if len(pos) > 2 {
		return opts, "", fmt.Errorf("extra operand '%s'", pos[2])
	}
	return opts, file, nil
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--lines" || strings.HasPrefix(flag, "--lines="):
		val, n, err := longOptValue(flag, "--lines", remaining)
		if err != nil {
			return 0, err
		}
		return n, setLineMode(opts, val)
	case flag == "--bytes" || strings.HasPrefix(flag, "--bytes="):
		val, n, err := longOptValue(flag, "--bytes", remaining)
		if err != nil {
			return 0, err
		}
		return n, setBytesMode(opts, val)
	case flag == "--line-bytes" || strings.HasPrefix(flag, "--line-bytes="):
		val, n, err := longOptValue(flag, "--line-bytes", remaining)
		if err != nil {
			return 0, err
		}
		return n, setLineBytesMode(opts, val)
	case flag == "--number" || strings.HasPrefix(flag, "--number="):
		val, n, err := longOptValue(flag, "--number", remaining)
		if err != nil {
			return 0, err
		}
		return n, setChunksMode(opts, val)
	case flag == "--suffix-length" || strings.HasPrefix(flag, "--suffix-length="):
		val, n, err := longOptValue(flag, "--suffix-length", remaining)
		if err != nil {
			return 0, err
		}
		return n, setSuffixLen(opts, val)
	case flag == "--numeric-suffixes":
		opts.numeric = true
		return 1, nil
	case flag == "--additional-suffix" || strings.HasPrefix(flag, "--additional-suffix="):
		val, n, err := longOptValue(flag, "--additional-suffix", remaining)
		if err != nil {
			return 0, err
		}
		opts.additionalSuffix = val
		return n, nil
	case flag == "--filter" || strings.HasPrefix(flag, "--filter="):
		val, n, err := longOptValue(flag, "--filter", remaining)
		if err != nil {
			return 0, err
		}
		opts.filter = val
		return n, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		if isBoolFlag(flags[j]) {
			if err := setBoolFlag(opts, flags[j]); err != nil {
				return 0, err
			}
			continue
		}
		setter, ok := shortFlagSetter(flags[j])
		if !ok {
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
		val, extra, err := extractFlagValue(flags[j+1:], remaining, consumed, flags[j])
		if err != nil {
			return 0, err
		}
		if err := setter(opts, val); err != nil {
			return 0, err
		}
		return consumed + extra, nil
	}
	return consumed, nil
}

func setBoolFlag(opts *options, ch byte) error {
	switch ch {
	case 'd':
		opts.numeric = true
		return nil
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
}

func shortFlagSetter(ch byte) (func(*options, string) error, bool) {
	switch ch {
	case 'l':
		return setLineMode, true
	case 'b':
		return setBytesMode, true
	case 'C':
		return setLineBytesMode, true
	case 'n':
		return setChunksMode, true
	case 'a':
		return setSuffixLen, true
	default:
		return nil, false
	}
}

func isBoolFlag(ch byte) bool {
	return ch == 'd'
}

func extractFlagValue(rest string, remaining []string, consumed int, flag byte) (string, int, error) {
	if rest != "" {
		return rest, 0, nil
	}
	if len(remaining) > consumed {
		return remaining[consumed], 1, nil
	}
	return "", 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func parseLineCount(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, nil
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func splitLines(r io.Reader, so suffixOpts, linesPerPiece int) error {
	br := bufio.NewReader(r)
	lineCount := 0
	pieceIndex := 0
	var pw *pieceWriter

	for {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			if lineCount%linesPerPiece == 0 {
				if pw != nil {
					pw.Close()
				}
				var err error
				pw, err = newPieceWriter(so, pieceIndex)
				if err != nil {
					return err
				}
				pieceIndex++
			}
			if _, err := pw.Write(line); err != nil {
				pw.Close()
				return err
			}
			lineCount++
		}
		if readErr != nil {
			if pw != nil {
				pw.Close()
			}
			if readErr != io.EOF {
				return readErr
			}
			return nil
		}
	}
}

type suffixOpts struct {
	prefix           string
	suffixLen        int
	numeric          bool
	additionalSuffix string
	filter           string
}

func newSuffixOpts(opts options) suffixOpts {
	return suffixOpts{
		prefix:           opts.prefix,
		suffixLen:        opts.suffixLen,
		numeric:          opts.numeric,
		additionalSuffix: opts.additionalSuffix,
		filter:           opts.filter,
	}
}

func pieceName(so suffixOpts, index int) (string, error) {
	var suffix string
	var err error
	if so.numeric {
		suffix, err = numericSuffix(index, so.suffixLen)
	} else {
		suffix, err = alphabeticSuffix(index, so.suffixLen)
	}
	if err != nil {
		return "", err
	}
	return so.prefix + suffix + so.additionalSuffix, nil
}

func numericSuffix(index, length int) (string, error) {
	s := strconv.Itoa(index)
	if len(s) > length {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return strings.Repeat("0", length-len(s)) + s, nil
}

func setSuffixLen(opts *options, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid suffix length: '%s'", val)
	}
	opts.suffixLen = n
	return nil
}

func alphabeticSuffix(index, length int) (string, error) {
	maxPieces := 1
	for range length {
		maxPieces *= 26
	}
	if index >= maxPieces {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	buf := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		buf[i] = 'a' + byte(index%26)
		index /= 26
	}
	return string(buf), nil
}

func run(r io.Reader, opts options) error {
	so := newSuffixOpts(opts)
	switch opts.mode {
	case modeBytes:
		return splitBytes(r, so, opts.bytesVal)
	case modeLineBytes:
		return splitLineBytes(r, so, opts.lineBytes)
	case modeChunks:
		return splitChunks(r, so, opts.chunks)
	default:
		return splitLines(r, so, opts.lines)
	}
}

func setMode(opts *options, mode splitMode) error {
	if opts.modeSet && opts.mode != mode {
		return fmt.Errorf("cannot split in more than one way")
	}
	opts.mode = mode
	opts.modeSet = true
	return nil
}

func setLineMode(opts *options, val string) error {
	n, err := parseLineCount(val)
	if err != nil {
		return err
	}
	if err := setMode(opts, modeLines); err != nil {
		return err
	}
	opts.lines = n
	return nil
}

func setBytesMode(opts *options, val string) error {
	n, err := parseByteCount(val)
	if err != nil {
		return err
	}
	if err := setMode(opts, modeBytes); err != nil {
		return err
	}
	opts.bytesVal = n
	return nil
}

func setLineBytesMode(opts *options, val string) error {
	n, err := parseByteCount(val)
	if err != nil {
		return err
	}
	if err := setMode(opts, modeLineBytes); err != nil {
		return err
	}
	opts.lineBytes = n
	return nil
}

func setChunksMode(opts *options, val string) error {
	if err := setMode(opts, modeChunks); err != nil {
		return err
	}
	opts.chunks = val
	return nil
}

func longOptValue(flag, name string, remaining []string) (string, int, error) {
	if strings.HasPrefix(flag, name+"=") {
		return flag[len(name)+1:], 1, nil
	}
	if len(remaining) < 2 {
		return "", 0, fmt.Errorf("option '%s' requires an argument", name)
	}
	return remaining[1], 2, nil
}

func parseByteCount(s string) (int64, error) {
	n, err := sizeparse.Parse(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of bytes: '%s'", s)
	}
	return n, nil
}

func splitBytes(r io.Reader, so suffixOpts, size int64) error {
	pieceIndex := 0
	for {
		pw, err := newPieceWriter(so, pieceIndex)
		if err != nil {
			return err
		}
		written, copyErr := io.CopyN(pw, r, size)
		closeErr := pw.Close()
		if written == 0 {
			pw.Remove()
		}
		if copyErr != nil {
			if copyErr == io.EOF {
				return nil
			}
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		pieceIndex++
	}
}

func splitLineBytes(r io.Reader, so suffixOpts, maxBytes int64) error {
	sz := int(maxBytes)
	br := bufio.NewReaderSize(r, sz)
	pieceIndex := 0

	for {
		data, err := readLineBytesChunk(br, sz)
		if len(data) > 0 {
			if werr := writePiece(so, pieceIndex, data); werr != nil {
				return werr
			}
			pieceIndex++
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func readLineBytesChunk(br *bufio.Reader, sz int) ([]byte, error) {
	buf, _ := br.Peek(sz)
	if len(buf) == 0 {
		return nil, io.EOF
	}
	writeLen := len(buf)
	if len(buf) >= sz {
		if lastNL := bytes.LastIndexByte(buf, '\n'); lastNL >= 0 {
			writeLen = lastNL + 1
		}
	}
	data := make([]byte, writeLen)
	_, err := br.Read(data)
	return data, err
}

func splitChunks(r io.Reader, so suffixOpts, spec string) error {
	mode, n, err := parseChunkSpec(spec)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	switch mode {
	case "":
		return splitByteChunks(data, so, n)
	case "l":
		return splitLineChunks(data, so, n)
	case "r":
		return splitRoundRobin(data, so, n)
	default:
		return fmt.Errorf("invalid number of chunks: '%s'", spec)
	}
}

func parseChunkSpec(spec string) (string, int, error) {
	numStr := spec
	mode := ""
	if strings.HasPrefix(spec, "l/") {
		mode = "l"
		numStr = spec[2:]
	} else if strings.HasPrefix(spec, "r/") {
		mode = "r"
		numStr = spec[2:]
	}
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid number of chunks: '%s'", spec)
	}
	return mode, n, nil
}

func splitByteChunks(data []byte, so suffixOpts, n int) error {
	total := len(data)
	baseSize := total / n
	remainder := total % n
	offset := 0
	for i := range n {
		size := baseSize
		if i < remainder {
			size++
		}
		if err := writePiece(so, i, data[offset:offset+size]); err != nil {
			return err
		}
		offset += size
	}
	return nil
}

func splitLineChunks(data []byte, so suffixOpts, n int) error {
	total := int64(len(data))
	offset := int64(0)
	for i := range n {
		target := total * int64(i+1) / int64(n)
		end := offset
		if target > offset {
			end = findLineEnd(data, target)
		}
		if err := writePiece(so, i, data[offset:end]); err != nil {
			return err
		}
		offset = end
	}
	return nil
}

func findLineEnd(data []byte, pos int64) int64 {
	if pos <= 0 {
		return 0
	}
	if pos >= int64(len(data)) {
		return int64(len(data))
	}
	if data[pos-1] == '\n' {
		return pos
	}
	idx := bytes.IndexByte(data[pos:], '\n')
	if idx < 0 {
		return int64(len(data))
	}
	return pos + int64(idx) + 1
}

func splitRoundRobin(data []byte, so suffixOpts, n int) error {
	lines := splitIntoLines(data)
	bufs := make([]bytes.Buffer, n)
	for i, line := range lines {
		bufs[i%n].Write(line)
	}
	for i := range n {
		if err := writePiece(so, i, bufs[i].Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func writePiece(so suffixOpts, index int, data []byte) error {
	pw, err := newPieceWriter(so, index)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := pw.Write(data); err != nil {
			pw.Close()
			return err
		}
	}
	return pw.Close()
}

type pieceWriter struct {
	file *os.File
	cmd  *exec.Cmd
	pipe io.WriteCloser
	name string
}

func newPieceWriter(so suffixOpts, index int) (*pieceWriter, error) {
	name, err := pieceName(so, index)
	if err != nil {
		return nil, err
	}
	if so.filter != "" {
		return newFilterWriter(so.filter, name)
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	return &pieceWriter{file: f, name: name}, nil
}

func newFilterWriter(command, filename string) (*pieceWriter, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(os.Environ(), "FILE="+filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &pieceWriter{cmd: cmd, pipe: pipe, name: filename}, nil
}

func (pw *pieceWriter) Write(p []byte) (int, error) {
	if pw.pipe != nil {
		return pw.pipe.Write(p)
	}
	return pw.file.Write(p)
}

func (pw *pieceWriter) Close() error {
	if pw.pipe != nil {
		pw.pipe.Close()
		return pw.cmd.Wait()
	}
	return pw.file.Close()
}

func (pw *pieceWriter) Remove() {
	if pw.file != nil {
		os.Remove(pw.name)
	}
}

func splitIntoLines(data []byte) [][]byte {
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
