// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd067-split R1.1–R1.4: basic file splitting by line count,
// R2.1–R2.4: byte-based, line-bytes, and chunk-based splitting modes,
// R3.1–R3.4: suffix control, numeric suffixes, additional suffix, and filter,
// R4.1–R4.2: exit codes (0 on success, 1 on error).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName         = "split"
	defaultLines     = 1000
	defaultPrefix    = "x"
	defaultSuffixLen = 2
)

// splitMode identifies the active splitting strategy.
type splitMode int

const (
	modeLines     splitMode = iota // -l / default
	modeBytes                      // -b
	modeLineBytes                  // -C
	modeChunks                     // -n
)

// chunkStrategy identifies the sub-mode for -n CHUNKS.
type chunkStrategy int

const (
	chunkBytes      chunkStrategy = iota // N — split by byte position
	chunkLines                           // l/N — split by line count
	chunkRoundRobin                      // r/N — round-robin lines
)

// config holds parsed command-line options for split.
type config struct {
	mode             splitMode
	lines            int
	byteCount        int64
	chunks           int
	chunkMode        chunkStrategy
	prefix           string
	suffixLen        int
	numericSuffix    bool   // R3.2: use numeric suffixes
	additionalSuffix string // R3.3: appended after suffix
	filterCmd        string // R3.4: pipe output to command
	inputFile        string
	modeSet          bool // true after first mode flag parsed
}

// filterWriter pipes data to a shell command for filter mode. R3.4.
type filterWriter struct {
	pipe io.WriteCloser
	cmd  *exec.Cmd
}

func (fw *filterWriter) Write(p []byte) (int, error) {
	return fw.pipe.Write(p)
}

// Close closes the pipe and waits for the command to finish.
func (fw *filterWriter) Close() error {
	if err := fw.pipe.Close(); err != nil {
		fw.cmd.Wait() // best-effort wait
		return err
	}
	return fw.cmd.Wait()
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes the split operation.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			return printHelp(stdout)
		case "--version":
			return printVersion(stdout)
		}
	}
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := executeSplit(cfg, stdin); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// printHelp writes usage information to stdout.
func printHelp(w io.Writer) int {
	fmt.Fprintln(w, "Usage: split [OPTION]... [FILE [PREFIX]]")
	fmt.Fprintln(w, "Output pieces of FILE to PREFIXaa, PREFIXab, ...;")
	fmt.Fprintln(w, "default size is 1000 lines, and default PREFIX is 'x'.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -a, --suffix-length=N  generate suffixes of length N (default 2)")
	fmt.Fprintln(w, "  -d, --numeric-suffixes  use numeric suffixes starting at 0")
	fmt.Fprintln(w, "      --additional-suffix=SUFF  append SUFF to output file names")
	fmt.Fprintln(w, "  -l, --lines=NUMBER   put NUMBER lines/records per output file")
	fmt.Fprintln(w, "  -b, --bytes=SIZE     put SIZE bytes per output file")
	fmt.Fprintln(w, "  -C, --line-bytes=SIZE  put at most SIZE bytes, breaking at lines")
	fmt.Fprintln(w, "  -n, --number=CHUNKS  generate CHUNKS output files")
	fmt.Fprintln(w, "      --filter=COMMAND  write to shell COMMAND; $FILE is filename")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
	return 0
}

// printVersion writes version information to stdout.
func printVersion(w io.Writer) int {
	fmt.Fprintln(w, "split (go-unix-utils)")
	return 0
}

// parseArgs extracts configuration from command-line arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{
		mode:      modeLines,
		lines:     defaultLines,
		prefix:    defaultPrefix,
		suffixLen: defaultSuffixLen,
		inputFile: "-",
	}
	positional, err := parseOptions(args, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, applyPositional(cfg, positional)
}

// parseOptions processes flag arguments and returns remaining positional args.
func parseOptions(args []string, cfg *config) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return positional, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		extra, err := parseSingleOption(args, i, cfg)
		if err != nil {
			return nil, err
		}
		i += extra
	}
	return positional, nil
}

// parseSingleOption handles one flag and returns extra args consumed.
func parseSingleOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if n, err := parseLongModeOption(args, i, cfg); err != nil || n >= 0 {
		return n, err
	}
	if n, err := parseLongSuffixOption(args, i, cfg); err != nil || n >= 0 {
		return n, err
	}
	if arg == "--numeric-suffixes" || strings.HasPrefix(arg, "--numeric-suffixes=") {
		cfg.numericSuffix = true
		return 0, nil
	}
	return parseShortFlags(args, i, cfg)
}

// parseLongModeOption handles long-form mode flags (--lines, --bytes, etc.).
// Returns -1 if the flag was not matched.
func parseLongModeOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--lines=") {
		return 0, setModeLines(arg[len("--lines="):], cfg)
	}
	if arg == "--lines" {
		return requireNextArg(args, i, "--lines", func(v string) error {
			return setModeLines(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "--bytes=") {
		return 0, setModeBytes(arg[len("--bytes="):], cfg)
	}
	if arg == "--bytes" {
		return requireNextArg(args, i, "--bytes", func(v string) error {
			return setModeBytes(v, cfg)
		})
	}
	return parseLongModeOptionCont(args, i, cfg)
}

// parseLongModeOptionCont handles remaining long-form mode flags.
// Returns -1 if the flag was not matched.
func parseLongModeOptionCont(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--line-bytes=") {
		return 0, setModeLineBytes(arg[len("--line-bytes="):], cfg)
	}
	if arg == "--line-bytes" {
		return requireNextArg(args, i, "--line-bytes", func(v string) error {
			return setModeLineBytes(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "--number=") {
		return 0, setModeChunks(arg[len("--number="):], cfg)
	}
	if arg == "--number" {
		return requireNextArg(args, i, "--number", func(v string) error {
			return setModeChunks(v, cfg)
		})
	}
	return -1, nil
}

// parseLongSuffixOption handles long-form suffix and filter flags.
// Returns -1 if the flag was not matched.
func parseLongSuffixOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--suffix-length=") {
		return 0, setSuffixLength(arg[len("--suffix-length="):], cfg)
	}
	if arg == "--suffix-length" {
		return requireNextArg(args, i, "--suffix-length", func(v string) error {
			return setSuffixLength(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "--additional-suffix=") {
		cfg.additionalSuffix = arg[len("--additional-suffix="):]
		return 0, nil
	}
	if arg == "--additional-suffix" {
		return requireNextArg(args, i, "--additional-suffix", func(v string) error {
			cfg.additionalSuffix = v
			return nil
		})
	}
	return parseLongFilterOption(args, i, cfg)
}

// parseLongFilterOption handles the --filter flag.
// Returns -1 if the flag was not matched.
func parseLongFilterOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--filter=") {
		cfg.filterCmd = arg[len("--filter="):]
		return 0, nil
	}
	if arg == "--filter" {
		return requireNextArg(args, i, "--filter", func(v string) error {
			cfg.filterCmd = v
			return nil
		})
	}
	return -1, nil
}

// requireNextArg validates that a next argument exists and calls the setter.
func requireNextArg(args []string, i int, name string, setter func(string) error) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", name)
	}
	return 1, setter(args[i+1])
}

// parseShortFlags handles short-form flags (-l, -b, -C, -n, -a, -d).
func parseShortFlags(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if arg == "-d" {
		cfg.numericSuffix = true
		return 0, nil
	}
	if strings.HasPrefix(arg, "-a") {
		return parseShortWithValue(args, i, 'a', func(v string) error {
			return setSuffixLength(v, cfg)
		})
	}
	return parseShortModeFlags(args, i, cfg)
}

// parseShortModeFlags handles short-form mode flags (-l, -b, -C, -n).
func parseShortModeFlags(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "-l") {
		return parseShortWithValue(args, i, 'l', func(v string) error {
			return setModeLines(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "-b") {
		return parseShortWithValue(args, i, 'b', func(v string) error {
			return setModeBytes(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "-C") {
		return parseShortWithValue(args, i, 'C', func(v string) error {
			return setModeLineBytes(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "-n") {
		return parseShortWithValue(args, i, 'n', func(v string) error {
			return setModeChunks(v, cfg)
		})
	}
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseShortWithValue handles a short flag with attached or separate value.
func parseShortWithValue(args []string, i int, flag byte, setter func(string) error) (int, error) {
	val := args[i][2:]
	if val == "" {
		if i+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
		}
		return 1, setter(args[i+1])
	}
	return 0, setter(val)
}

// setModeLines sets line-splitting mode. R2.4: conflicts checked.
func setModeLines(s string, cfg *config) error {
	if err := checkModeConflict(cfg, modeLines); err != nil {
		return err
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid number of lines: '%s'", s)
	}
	cfg.mode = modeLines
	cfg.lines = n
	cfg.modeSet = true
	return nil
}

// setModeBytes sets byte-splitting mode. R2.1: parses size with suffixes.
func setModeBytes(s string, cfg *config) error {
	if err := checkModeConflict(cfg, modeBytes); err != nil {
		return err
	}
	n, err := parseByteSize(s)
	if err != nil {
		return fmt.Errorf("invalid number of bytes: '%s'", s)
	}
	cfg.mode = modeBytes
	cfg.byteCount = n
	cfg.modeSet = true
	return nil
}

// setModeLineBytes sets line-bytes mode. R2.2: parses size with suffixes.
func setModeLineBytes(s string, cfg *config) error {
	if err := checkModeConflict(cfg, modeLineBytes); err != nil {
		return err
	}
	n, err := parseByteSize(s)
	if err != nil {
		return fmt.Errorf("invalid number of bytes: '%s'", s)
	}
	cfg.mode = modeLineBytes
	cfg.byteCount = n
	cfg.modeSet = true
	return nil
}

// setModeChunks sets chunk-splitting mode. R2.3: parses N, l/N, r/N forms.
func setModeChunks(s string, cfg *config) error {
	if err := checkModeConflict(cfg, modeChunks); err != nil {
		return err
	}
	strategy, n, err := parseChunkSpec(s)
	if err != nil {
		return err
	}
	cfg.mode = modeChunks
	cfg.chunkMode = strategy
	cfg.chunks = n
	cfg.modeSet = true
	return nil
}

// setSuffixLength parses and sets the suffix length. R3.1.
func setSuffixLength(s string, cfg *config) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid suffix length: '%s'", s)
	}
	cfg.suffixLen = n
	return nil
}

// checkModeConflict returns an error if a different mode was already set.
// R2.4: conflicting split options must produce an error.
func checkModeConflict(cfg *config, newMode splitMode) error {
	if cfg.modeSet && cfg.mode != newMode {
		return fmt.Errorf("cannot split in more than one way")
	}
	return nil
}

// parseByteSize parses a byte count with optional suffix.
// R2.1: K=1024, M=1024^2, etc. KB=1000, MB=1000^2, etc.
func parseByteSize(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	multiplier, numStr := extractSuffix(s)
	if multiplier == 0 {
		return 0, fmt.Errorf("invalid suffix")
	}
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number")
	}
	return n * multiplier, nil
}

// extractSuffix separates the numeric part from the suffix and returns
// the multiplier. Returns 0 multiplier for invalid suffixes.
func extractSuffix(s string) (int64, string) {
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"KB", 1000},
		{"MB", 1000 * 1000},
		{"GB", 1000 * 1000 * 1000},
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"PB", 1000 * 1000 * 1000 * 1000 * 1000},
		{"EB", 1000 * 1000 * 1000 * 1000 * 1000 * 1000},
		{"K", 1024},
		{"M", 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"T", 1024 * 1024 * 1024 * 1024},
		{"P", 1024 * 1024 * 1024 * 1024 * 1024},
		{"E", 1024 * 1024 * 1024 * 1024 * 1024 * 1024},
	}
	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.suffix) {
			return sf.mult, s[:len(s)-len(sf.suffix)]
		}
	}
	return 1, s
}

// parseChunkSpec parses a -n CHUNKS argument into strategy and count.
// R2.3: supports N, l/N, and r/N forms.
func parseChunkSpec(s string) (chunkStrategy, int, error) {
	if strings.HasPrefix(s, "l/") {
		n, err := parseChunkCount(s[2:])
		return chunkLines, n, err
	}
	if strings.HasPrefix(s, "r/") {
		n, err := parseChunkCount(s[2:])
		return chunkRoundRobin, n, err
	}
	n, err := parseChunkCount(s)
	return chunkBytes, n, err
}

// parseChunkCount parses and validates a positive integer chunk count.
func parseChunkCount(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of chunks: '%s'", s)
	}
	return n, nil
}

// applyPositional sets inputFile and prefix from positional arguments.
func applyPositional(cfg *config, positional []string) error {
	if len(positional) > 2 {
		return fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) > 0 {
		cfg.inputFile = positional[0]
	}
	if len(positional) > 1 {
		cfg.prefix = positional[1]
	}
	return nil
}

// makeFilename constructs the output filename for the given index.
// Applies prefix, suffix (alphabetic or numeric), and additional suffix.
func makeFilename(index int, cfg *config) (string, error) {
	suffix, err := generateSuffix(index, cfg.suffixLen, cfg.numericSuffix)
	if err != nil {
		return "", err
	}
	return cfg.prefix + suffix + cfg.additionalSuffix, nil
}

// generateSuffix returns a suffix for the given index.
// R3.2: when numeric is true, produces "00", "01", etc.
func generateSuffix(index, length int, numeric bool) (string, error) {
	if numeric {
		return generateNumericSuffix(index, length)
	}
	return generateAlphaSuffix(index, length)
}

// generateAlphaSuffix returns an alphabetic suffix for the given index.
// R1.1: suffixes follow the pattern aa, ab, ..., az, ba, ..., zz.
func generateAlphaSuffix(index, length int) (string, error) {
	suffix := make([]byte, length)
	n := index
	for i := length - 1; i >= 0; i-- {
		suffix[i] = 'a' + byte(n%26)
		n /= 26
	}
	if n > 0 {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return string(suffix), nil
}

// generateNumericSuffix returns a zero-padded numeric suffix. R3.2.
func generateNumericSuffix(index, length int) (string, error) {
	s := fmt.Sprintf("%0*d", length, index)
	if len(s) > length {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return s, nil
}

// openOutput creates an output writer for the given filename.
// R3.4: when filterCmd is set, pipes to the shell command instead.
func openOutput(filename string, cfg *config) (io.WriteCloser, error) {
	if cfg.filterCmd != "" {
		return openFilterOutput(filename, cfg.filterCmd)
	}
	return os.Create(filename)
}

// openFilterOutput starts a shell command and returns a writer to its stdin.
// The filename is available as $FILE in the command. R3.4.
func openFilterOutput(filename, filterCmd string) (io.WriteCloser, error) {
	cmd := exec.Command("sh", "-c", filterCmd)
	cmd.Env = append(os.Environ(), "FILE="+filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("filter pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("filter start: %w", err)
	}
	return &filterWriter{pipe: pipe, cmd: cmd}, nil
}

// executeSplit opens the input and dispatches to the appropriate mode.
func executeSplit(cfg *config, stdin io.Reader) error {
	reader, closer, err := openInput(cfg.inputFile, stdin)
	if err != nil {
		return err
	}
	defer closer()
	switch cfg.mode {
	case modeBytes:
		return splitByBytes(reader, cfg)
	case modeLineBytes:
		return splitByLineBytes(reader, cfg)
	case modeChunks:
		return splitByChunks(reader, cfg)
	default:
		return splitByLines(reader, cfg)
	}
}

// openInput returns a reader for the specified file or stdin.
// R1.4: when path is "-", reads from stdin.
func openInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// splitByLines reads from r and writes chunks of cfg.lines lines each.
// R1.1: generates output files with alphabetic suffixes (xaa, xab, ...).
func splitByLines(r io.Reader, cfg *config) error {
	br := bufio.NewReader(r)
	for fileIdx := 0; ; fileIdx++ {
		filename, err := makeFilename(fileIdx, cfg)
		if err != nil {
			return err
		}
		wrote, writeErr := writeLineChunk(br, filename, cfg.lines, cfg)
		if writeErr != nil {
			return writeErr
		}
		if !wrote {
			break
		}
	}
	return nil
}

// writeLineChunk checks for remaining data and writes up to maxLines lines.
// Returns false when input is exhausted.
func writeLineChunk(br *bufio.Reader, filename string, maxLines int, cfg *config) (bool, error) {
	if _, err := br.Peek(1); err == io.EOF {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return writeLineChunkData(br, filename, maxLines, cfg)
}

// writeLineChunkData creates a file and writes up to maxLines lines to it.
func writeLineChunkData(br *bufio.Reader, filename string, maxLines int, cfg *config) (bool, error) {
	f, err := openOutput(filename, cfg)
	if err != nil {
		return false, err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for lineCount := 0; lineCount < maxLines; {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := bw.Write(line); werr != nil {
				return true, werr
			}
			if line[len(line)-1] == '\n' {
				lineCount++
			}
		}
		if readErr == io.EOF {
			return true, bw.Flush()
		}
		if readErr != nil {
			return true, readErr
		}
	}
	return true, bw.Flush()
}

// splitByBytes reads from r and writes chunks of cfg.byteCount bytes each.
// R2.1: byte-count splitting.
func splitByBytes(r io.Reader, cfg *config) error {
	br := bufio.NewReader(r)
	buf := make([]byte, cfg.byteCount)
	for fileIdx := 0; ; fileIdx++ {
		filename, err := makeFilename(fileIdx, cfg)
		if err != nil {
			return err
		}
		n, readErr := io.ReadFull(br, buf)
		if n == 0 {
			break
		}
		if writeErr := writeByteOutput(filename, buf[:n], cfg); writeErr != nil {
			return writeErr
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// writeByteOutput writes data to a file or filter output.
func writeByteOutput(filename string, data []byte, cfg *config) error {
	w, err := openOutput(filename, cfg)
	if err != nil {
		return err
	}
	_, werr := w.Write(data)
	cerr := w.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// splitByLineBytes reads from r and writes chunks of at most cfg.byteCount
// bytes each, breaking at line boundaries. R2.2: lines longer than the
// limit are written to their own piece.
func splitByLineBytes(r io.Reader, cfg *config) error {
	br := bufio.NewReader(r)
	var pending []byte
	for fileIdx := 0; ; fileIdx++ {
		if pending == nil {
			if _, err := br.Peek(1); err == io.EOF {
				break
			} else if err != nil {
				return err
			}
		}
		filename, err := makeFilename(fileIdx, cfg)
		if err != nil {
			return err
		}
		var writeErr error
		pending, writeErr = writeLineBytesChunk(br, filename, cfg.byteCount, pending, cfg)
		if writeErr != nil {
			return writeErr
		}
	}
	return nil
}

// writeLineBytesChunk writes one chunk of at most maxBytes bytes, breaking
// at line boundaries. Long lines are broken at the byte limit.
// Returns any pending data that didn't fit.
func writeLineBytesChunk(br *bufio.Reader, filename string, maxBytes int64, pending []byte, cfg *config) ([]byte, error) {
	f, err := openOutput(filename, cfg)
	if err != nil {
		return pending, err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	var written int64
	if pending != nil {
		n := writePending(bw, pending, maxBytes)
		written = int64(n)
		if n < len(pending) {
			return pending[n:], bw.Flush()
		}
	}
	return fillLineBytesChunk(br, bw, written, maxBytes)
}

// writePending writes as much of pending data as fits in maxBytes.
func writePending(bw *bufio.Writer, data []byte, maxBytes int64) int {
	toWrite := len(data)
	if int64(toWrite) > maxBytes {
		toWrite = int(maxBytes)
	}
	bw.Write(data[:toWrite]) //nolint:errcheck — checked at flush
	return toWrite
}

// fillLineBytesChunk reads lines and fills the chunk up to maxBytes.
// Returns pending data that didn't fit.
func fillLineBytesChunk(br *bufio.Reader, bw *bufio.Writer, written, maxBytes int64) ([]byte, error) {
	for {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			pending, werr := writeLine(bw, line, &written, maxBytes)
			if werr != nil {
				return nil, werr
			}
			if pending != nil {
				return pending, bw.Flush()
			}
		}
		if readErr == io.EOF {
			return nil, bw.Flush()
		}
		if readErr != nil {
			return nil, readErr
		}
		if written >= maxBytes {
			return nil, bw.Flush()
		}
	}
}

// writeLine writes a line to bw respecting the byte limit. If the line
// doesn't fit, returns the unwritten portion as pending.
func writeLine(bw *bufio.Writer, line []byte, written *int64, maxBytes int64) ([]byte, error) {
	lineLen := int64(len(line))
	remaining := maxBytes - *written
	if *written > 0 && lineLen > remaining {
		return line, nil
	}
	if lineLen <= remaining {
		if _, err := bw.Write(line); err != nil {
			return nil, err
		}
		*written += lineLen
		return nil, nil
	}
	// Line longer than limit with nothing written yet: write up to limit
	if _, err := bw.Write(line[:remaining]); err != nil {
		return nil, err
	}
	*written += remaining
	return line[remaining:], nil
}

// splitByChunks dispatches to the appropriate chunk strategy. R2.3.
func splitByChunks(r io.Reader, cfg *config) error {
	switch cfg.chunkMode {
	case chunkLines:
		return splitChunksByLines(r, cfg)
	case chunkRoundRobin:
		return splitChunksByRoundRobin(r, cfg)
	default:
		return splitChunksByBytes(r, cfg)
	}
}

// splitChunksByBytes splits the input into N chunks of roughly equal byte size.
// R2.3: N form — requires seekable input or buffers all data.
func splitChunksByBytes(r io.Reader, cfg *config) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	totalSize := int64(len(data))
	chunkSize := totalSize / int64(cfg.chunks)
	remainder := totalSize % int64(cfg.chunks)
	var offset int64
	for i := 0; i < cfg.chunks; i++ {
		filename, serr := makeFilename(i, cfg)
		if serr != nil {
			return serr
		}
		size := chunkSize
		if int64(i) < remainder {
			size++
		}
		chunk := data[offset : offset+size]
		if err := writeByteOutput(filename, chunk, cfg); err != nil {
			return err
		}
		offset += size
	}
	return nil
}

// splitChunksByLines splits the input into N chunks by line count.
// R2.3: l/N form.
func splitChunksByLines(r io.Reader, cfg *config) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	totalLines := len(lines)
	chunkSize := totalLines / cfg.chunks
	remainder := totalLines % cfg.chunks
	lineIdx := 0
	for i := 0; i < cfg.chunks; i++ {
		filename, serr := makeFilename(i, cfg)
		if serr != nil {
			return serr
		}
		count := chunkSize
		if i < remainder {
			count++
		}
		if err := writeLinesOutput(filename, lines, lineIdx, count, cfg); err != nil {
			return err
		}
		lineIdx += count
	}
	return nil
}

// splitChunksByRoundRobin distributes lines round-robin across N chunks.
// R2.3: r/N form.
func splitChunksByRoundRobin(r io.Reader, cfg *config) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	writers, closers, err := openChunkOutputs(cfg)
	if err != nil {
		return err
	}
	defer closeAll(closers)
	for i, line := range lines {
		chunkIdx := i % cfg.chunks
		if _, werr := writers[chunkIdx].Write(line); werr != nil {
			return werr
		}
	}
	return flushAll(writers)
}

// readAllLines reads all lines from r, preserving line endings.
func readAllLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReader(r)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return lines, nil
}

// openChunkOutputs creates all chunk output writers for round-robin mode.
func openChunkOutputs(cfg *config) ([]*bufio.Writer, []io.Closer, error) {
	writers := make([]*bufio.Writer, cfg.chunks)
	closers := make([]io.Closer, cfg.chunks)
	for i := 0; i < cfg.chunks; i++ {
		filename, err := makeFilename(i, cfg)
		if err != nil {
			closeAll(closers[:i])
			return nil, nil, err
		}
		w, err := openOutput(filename, cfg)
		if err != nil {
			closeAll(closers[:i])
			return nil, nil, err
		}
		writers[i] = bufio.NewWriter(w)
		closers[i] = w
	}
	return writers, closers, nil
}

// closeAll closes all non-nil closers.
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		if c != nil {
			c.Close() // best-effort cleanup
		}
	}
}

// flushAll flushes all buffered writers.
func flushAll(writers []*bufio.Writer) error {
	for _, w := range writers {
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// writeLinesOutput writes count lines starting at startIdx to an output.
func writeLinesOutput(filename string, lines [][]byte, startIdx, count int, cfg *config) error {
	w, err := openOutput(filename, cfg)
	if err != nil {
		return err
	}
	defer w.Close()
	bw := bufio.NewWriter(w)
	end := min(startIdx+count, len(lines))
	for i := startIdx; i < end; i++ {
		if _, werr := bw.Write(lines[i]); werr != nil {
			return werr
		}
	}
	return bw.Flush()
}
