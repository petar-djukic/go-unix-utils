// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd067-split: Split a File into Pieces.
// Covers R1.1-R1.4 (default/line splitting, prefix, stdin),
// R2.1-R2.4 (byte/line-bytes/chunk splitting, conflicting modes),
// R3.1-R3.4 (suffix length, numeric suffixes, additional suffix, filter).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags.
var version = "dev"

const (
	defaultLines     = 1000
	defaultPrefix    = "x"
	defaultSuffixLen = 2
)

// splitMode specifies how input is divided into pieces.
type splitMode int

const (
	modeLines     splitMode = iota // R1.1/R1.3: by line count
	modeBytes                      // R2.1: by byte count
	modeLineBytes                  // R2.2: by bytes at line boundaries
	modeChunks                     // R2.3: by chunk count
)

// config holds parsed flag state.
type config struct {
	mode      splitMode
	lines     int64
	bytes     int64
	chunks    int64
	chunkMode byte // 0=bytes, 'l'=lines, 'r'=round-robin (R2.3)
	file      string
	prefix    string
	suffixLen int
	numeric   bool   // R3.2: numeric suffixes
	addSuffix string // R3.3: additional suffix
	filter    string // R3.4: filter command
}

func main() {
	// Shared protocol: SIGPIPE handler for piped output.
	sys.InstallSIGPIPEHandler()

	cfg, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg))
}

// run opens input and dispatches to the appropriate split function.
// R4.1: exit 0 on success. R4.2: exit 1 on error.
func run(cfg config) int {
	f, err := openInput(cfg.file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	if f != os.Stdin {
		defer f.Close()
	}

	namer := newFileNamer(cfg)
	if err := dispatch(f, cfg, namer); err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	return 0
}

// dispatch calls the split function for the configured mode.
func dispatch(f *os.File, cfg config, namer *fileNamer) error {
	switch cfg.mode {
	case modeBytes:
		return splitByBytes(f, cfg.bytes, namer)
	case modeLineBytes:
		return splitByLineBytes(f, cfg.bytes, namer)
	case modeChunks:
		return dispatchChunks(f, cfg, namer)
	default:
		return splitByLines(f, cfg.lines, namer)
	}
}

// dispatchChunks routes chunk mode to the appropriate handler.
// R2.3: supports N (bytes), l/N (lines), r/N (round-robin).
func dispatchChunks(f *os.File, cfg config, namer *fileNamer) error {
	switch cfg.chunkMode {
	case 'l':
		return splitByChunksLines(f, cfg.chunks, namer)
	case 'r':
		return splitByChunksRoundRobin(f, cfg.chunks, namer)
	default:
		return splitByChunksBytes(f, cfg.chunks, namer)
	}
}

// openInput opens a file or returns stdin for "-".
// R1.4: when FILE is "-", read from stdin.
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("cannot open '%s' for reading: %v", name, err)
	}
	return f, nil
}

// splitByLines divides input into pieces of n lines each.
// R1.1: default 1000 lines. R1.3: custom line count via -l.
func splitByLines(r io.Reader, n int64, namer *fileNamer) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var count int64
	var w io.WriteCloser

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if w == nil {
				var ferr error
				if w, ferr = namer.next(); ferr != nil {
					return ferr
				}
				count = 0
			}
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
			count++
			if count >= n {
				w.Close()
				w = nil
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	closeIfOpen(w)
	return nil
}

// splitByBytes divides input into pieces of exactly n bytes each.
// R2.1: -b N byte-based splitting.
func splitByBytes(r io.Reader, n int64, namer *fileNamer) error {
	buf := make([]byte, 64*1024)
	var remaining int64
	var w io.WriteCloser

	for {
		nr, err := r.Read(buf)
		if nr > 0 {
			var werr error
			w, remaining, werr = writeByteChunks(
				w, buf[:nr], remaining, n, namer,
			)
			if werr != nil {
				return werr
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	closeIfOpen(w)
	return nil
}

// writeByteChunks writes data across piece boundaries, opening new
// files as each piece fills up.
func writeByteChunks(
	w io.WriteCloser, data []byte, remaining, chunkSize int64,
	namer *fileNamer,
) (io.WriteCloser, int64, error) {
	for len(data) > 0 {
		if w == nil {
			var err error
			if w, err = namer.next(); err != nil {
				return nil, 0, err
			}
			remaining = chunkSize
		}
		toWrite := int64(len(data))
		if toWrite > remaining {
			toWrite = remaining
		}
		if _, err := w.Write(data[:toWrite]); err != nil {
			return w, remaining, err
		}
		data = data[toWrite:]
		remaining -= toWrite
		if remaining <= 0 {
			w.Close()
			w = nil
		}
	}
	return w, remaining, nil
}

// splitByLineBytes divides input into pieces of at most n bytes,
// breaking only at line boundaries.
// R2.2: -C N line-bytes mode. Lines longer than N get their own piece.
func splitByLineBytes(r io.Reader, maxBytes int64, namer *fileNamer) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var size int64
	var w io.WriteCloser

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			var werr error
			w, size, werr = writeLineBounded(
				w, line, size, maxBytes, namer,
			)
			if werr != nil {
				return werr
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	closeIfOpen(w)
	return nil
}

// writeLineBounded writes a line, starting a new piece when adding the
// line would exceed maxBytes. Lines exceeding maxBytes are split into
// maxBytes-sized chunks across multiple pieces.
func writeLineBounded(
	w io.WriteCloser, line []byte, size, maxBytes int64,
	namer *fileNamer,
) (io.WriteCloser, int64, error) {
	if w != nil && size > 0 && size+int64(len(line)) > maxBytes {
		w.Close()
		w = nil
		size = 0
	}
	for len(line) > 0 {
		if w == nil {
			var err error
			if w, err = namer.next(); err != nil {
				return nil, 0, err
			}
			size = 0
		}
		room := maxBytes - size
		chunk := int64(len(line))
		if chunk > room {
			chunk = room
		}
		if _, err := w.Write(line[:chunk]); err != nil {
			return w, size, err
		}
		line = line[chunk:]
		size += chunk
		if size >= maxBytes {
			w.Close()
			w = nil
			size = 0
		}
	}
	return w, size, nil
}

// closeIfOpen closes a writer if it is non-nil.
func closeIfOpen(w io.WriteCloser) {
	if w != nil {
		w.Close() // best-effort close at end of split
	}
}

// fileNamer generates sequential output filenames.
// R3.1: configurable suffix length. R3.2: numeric suffixes.
// R3.3: additional suffix. R3.4: filter command.
type fileNamer struct {
	prefix    string
	suffix    []byte
	index     int64
	suffixLen int
	numeric   bool
	addSuffix string
	filter    string
	first     bool
}

// newFileNamer creates a namer from config.
func newFileNamer(cfg config) *fileNamer {
	n := &fileNamer{
		prefix:    cfg.prefix,
		suffixLen: cfg.suffixLen,
		numeric:   cfg.numeric,
		addSuffix: cfg.addSuffix,
		filter:    cfg.filter,
		first:     true,
	}
	if !n.numeric {
		n.suffix = make([]byte, cfg.suffixLen)
		for i := range n.suffix {
			n.suffix[i] = 'a'
		}
	}
	return n
}

// next creates and returns the next output writer.
func (n *fileNamer) next() (io.WriteCloser, error) {
	name, err := n.nextName()
	if err != nil {
		return nil, err
	}
	if n.filter != "" {
		return startFilter(n.filter, name)
	}
	return os.Create(name)
}

// nextName advances the suffix and returns the next filename.
func (n *fileNamer) nextName() (string, error) {
	if !n.first {
		if err := n.advance(); err != nil {
			return "", err
		}
	}
	n.first = false
	return n.currentName(), nil
}

// advance increments the suffix counter.
func (n *fileNamer) advance() error {
	if n.numeric {
		n.index++
		if n.index >= numericMax(n.suffixLen) {
			return fmt.Errorf("output file suffixes exhausted")
		}
	} else if !incrementSuffix(n.suffix) {
		return fmt.Errorf("output file suffixes exhausted")
	}
	return nil
}

// currentName returns the filename for the current suffix.
func (n *fileNamer) currentName() string {
	var suf string
	if n.numeric {
		suf = fmt.Sprintf("%0*d", n.suffixLen, n.index)
	} else {
		suf = string(n.suffix)
	}
	return n.prefix + suf + n.addSuffix
}

// numericMax returns 10^suffixLen.
func numericMax(suffixLen int) int64 {
	result := int64(1)
	for i := 0; i < suffixLen; i++ {
		result *= 10
	}
	return result
}

// incrementSuffix advances an alphabetic suffix by one position.
// Returns false when all positions have wrapped past 'z'.
func incrementSuffix(suffix []byte) bool {
	for i := len(suffix) - 1; i >= 0; i-- {
		if suffix[i] < 'z' {
			suffix[i]++
			return true
		}
		suffix[i] = 'a'
	}
	return false
}

// filterWriter pipes data to a shell command.
// R3.4: --filter=COMMAND support.
type filterWriter struct {
	cmd  *exec.Cmd
	pipe io.WriteCloser
}

// Write sends data to the filter command's stdin.
func (fw *filterWriter) Write(p []byte) (int, error) {
	return fw.pipe.Write(p)
}

// Close closes the pipe and waits for the command to finish.
func (fw *filterWriter) Close() error {
	if err := fw.pipe.Close(); err != nil {
		return err
	}
	return fw.cmd.Wait()
}

// startFilter launches a shell command for filter mode.
func startFilter(command, filename string) (io.WriteCloser, error) {
	cmd := exec.Command("sh", "-c", command)
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
	return &filterWriter{cmd: cmd, pipe: pipe}, nil
}

// parseArgs processes command-line arguments and returns configuration.
// Returns exit code -1 to continue, >= 0 for early exit.
func parseArgs(args []string) (config, int) {
	cfg := config{
		lines:     defaultLines,
		file:      "-",
		prefix:    defaultPrefix,
		suffixLen: defaultSuffixLen,
	}
	var positional []string
	modeSet := false

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		exit := parseOneArg(args, &i, &cfg, &positional, &modeSet)
		if exit >= 0 {
			return config{}, exit
		}
	}

	return applyPositional(cfg, positional)
}

// parseOneArg handles a single argument.
// Returns -1 to continue, >= 0 to exit.
func parseOneArg(
	args []string, i *int, cfg *config, positional *[]string,
	modeSet *bool,
) int {
	arg := args[*i]
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "-d" || arg == "--numeric-suffixes":
		cfg.numeric = true
		return -1
	default:
		return parseFlagWithValue(
			args, i, cfg, positional, modeSet, arg,
		)
	}
}

// parseFlagWithValue handles flags that take values or positional args.
func parseFlagWithValue(
	args []string, i *int, cfg *config, positional *[]string,
	modeSet *bool, arg string,
) int {
	switch {
	case arg == "-l" || arg == "--lines":
		return consumeMode(args, i, cfg, modeSet, modeLines)
	case strings.HasPrefix(arg, "-l"):
		return applyMode(cfg, modeSet, modeLines, arg[2:])
	case strings.HasPrefix(arg, "--lines="):
		return applyMode(cfg, modeSet, modeLines, arg[8:])
	case arg == "-b" || arg == "--bytes":
		return consumeMode(args, i, cfg, modeSet, modeBytes)
	case strings.HasPrefix(arg, "-b"):
		return applyMode(cfg, modeSet, modeBytes, arg[2:])
	case strings.HasPrefix(arg, "--bytes="):
		return applyMode(cfg, modeSet, modeBytes, arg[8:])
	case arg == "-C" || arg == "--line-bytes":
		return consumeMode(args, i, cfg, modeSet, modeLineBytes)
	case strings.HasPrefix(arg, "-C"):
		return applyMode(cfg, modeSet, modeLineBytes, arg[2:])
	case strings.HasPrefix(arg, "--line-bytes="):
		return applyMode(cfg, modeSet, modeLineBytes, arg[13:])
	case arg == "-n" || arg == "--number":
		return consumeChunkMode(args, i, cfg, modeSet)
	case strings.HasPrefix(arg, "-n"):
		return applyChunkMode(cfg, modeSet, arg[2:])
	case strings.HasPrefix(arg, "--number="):
		return applyChunkMode(cfg, modeSet, arg[9:])
	default:
		return parseOptionFlag(args, i, cfg, positional, arg)
	}
}

// parseOptionFlag handles non-mode option flags.
func parseOptionFlag(
	args []string, i *int, cfg *config, positional *[]string,
	arg string,
) int {
	switch {
	case arg == "-a" || arg == "--suffix-length":
		return consumeSuffixLen(args, i, cfg)
	case strings.HasPrefix(arg, "-a"):
		return applySuffixLen(cfg, arg[2:])
	case strings.HasPrefix(arg, "--suffix-length="):
		return applySuffixLen(cfg, arg[16:])
	case strings.HasPrefix(arg, "--additional-suffix="):
		cfg.addSuffix = arg[20:]
		return -1
	case arg == "--additional-suffix":
		return consumeStringVal(args, i, &cfg.addSuffix)
	case strings.HasPrefix(arg, "--filter="):
		cfg.filter = arg[9:]
		return -1
	case arg == "--filter":
		return consumeStringVal(args, i, &cfg.filter)
	default:
		return handleDefault(arg, positional)
	}
}

// handleDefault handles non-flag arguments or reports unrecognized options.
func handleDefault(arg string, positional *[]string) int {
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		fmt.Fprintf(os.Stderr, "split: unrecognized option '%s'\n", arg)
		return 1
	}
	*positional = append(*positional, arg)
	return -1
}

// consumeMode reads the next argument as the value for a mode flag.
func consumeMode(
	args []string, i *int, cfg *config, modeSet *bool, mode splitMode,
) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"split: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applyMode(cfg, modeSet, mode, args[*i])
}

// applyMode validates and sets the split mode and its parameter.
// R2.4: conflicting split modes produce an error.
func applyMode(cfg *config, modeSet *bool, mode splitMode, val string) int {
	if *modeSet && cfg.mode != mode {
		fmt.Fprintln(os.Stderr, "split: cannot split in more than one way")
		return 1
	}
	*modeSet = true
	cfg.mode = mode

	switch mode {
	case modeLines:
		return parseLineCount(cfg, val)
	default:
		return parseByteCount(cfg, val)
	}
}

// consumeChunkMode reads the next argument for -n/--number.
func consumeChunkMode(
	args []string, i *int, cfg *config, modeSet *bool,
) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"split: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applyChunkMode(cfg, modeSet, args[*i])
}

// applyChunkMode validates and sets chunk mode.
// R2.4: conflicting modes produce an error.
func applyChunkMode(cfg *config, modeSet *bool, val string) int {
	if *modeSet && cfg.mode != modeChunks {
		fmt.Fprintln(os.Stderr, "split: cannot split in more than one way")
		return 1
	}
	*modeSet = true
	cfg.mode = modeChunks
	return parseChunkSpec(cfg, val)
}

// parseChunkSpec parses a chunk specification: N, l/N, or r/N.
// R2.3: three forms of chunk splitting.
func parseChunkSpec(cfg *config, val string) int {
	spec := val
	if strings.HasPrefix(spec, "l/") {
		cfg.chunkMode = 'l'
		spec = spec[2:]
	} else if strings.HasPrefix(spec, "r/") {
		cfg.chunkMode = 'r'
		spec = spec[2:]
	}
	n, err := strconv.ParseInt(spec, 10, 64)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr,
			"split: invalid number of chunks: '%s'\n", val)
		return 1
	}
	cfg.chunks = n
	return -1
}

// consumeSuffixLen reads the next argument as suffix length.
func consumeSuffixLen(args []string, i *int, cfg *config) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"split: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applySuffixLen(cfg, args[*i])
}

// applySuffixLen parses and sets the suffix length.
// R3.1: -a N sets suffix length.
func applySuffixLen(cfg *config, val string) int {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr,
			"split: invalid suffix length: '%s'\n", val)
		return 1
	}
	cfg.suffixLen = n
	return -1
}

// consumeStringVal reads the next argument as a string value.
func consumeStringVal(args []string, i *int, target *string) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"split: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	*target = args[*i]
	return -1
}

// parseLineCount parses and validates a line count value.
func parseLineCount(cfg *config, val string) int {
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr,
			"split: invalid number of lines: '%s'\n", val)
		return 1
	}
	cfg.lines = n
	return -1
}

// parseByteCount parses a byte count with optional size suffix.
// R2.1: supports K, M, G, T, P, E, Z, Y and KB, MB, etc. suffixes.
func parseByteCount(cfg *config, val string) int {
	n, err := sizeparse.Parse(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr,
			"split: invalid number of bytes: '%s'\n", val)
		return 1
	}
	cfg.bytes = n
	return -1
}

// applyPositional maps positional arguments to FILE and PREFIX.
// R1.2: PREFIX argument customizes output filename prefix.
// R1.4: FILE defaults to "-" (stdin).
func applyPositional(cfg config, positional []string) (config, int) {
	if len(positional) > 0 {
		cfg.file = positional[0]
	}
	if len(positional) > 1 {
		cfg.prefix = positional[1]
	}
	if len(positional) > 2 {
		fmt.Fprintf(os.Stderr,
			"split: extra operand '%s'\n", positional[2])
		return config{}, 1
	}
	return cfg, -1
}

// printHelp writes usage information and returns exit code 0.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: split [OPTION]... [FILE [PREFIX]]
Output pieces of FILE to PREFIXaa, PREFIXab, ...;
default size is 1000 lines, and default PREFIX is 'x'.

With no FILE, or when FILE is -, read standard input.

  -a, --suffix-length=N    generate suffixes of length N (default 2)
  -b, --bytes=SIZE         put SIZE bytes per output file
  -C, --line-bytes=SIZE    put at most SIZE bytes of records per output file
  -d, --numeric-suffixes   use numeric suffixes starting at 0
  -l, --lines=NUMBER       put NUMBER lines/records per output file
  -n, --number=CHUNKS      generate CHUNKS output files; see below
      --additional-suffix=SUFF  append SUFF to output file names
      --filter=COMMAND     write to shell COMMAND; file name is $FILE

      --help     display this help and exit
      --version  output version information and exit

CHUNKS may be:
  N       split into N files based on size of input
  l/N     split into N files without splitting lines
  r/N     like 'l' but use round robin distribution

SIZE may be (or may be an integer optionally followed by) one of following:
KB 1000, K 1024, MB 1000*1000, M 1024*1024, and so on for G, T, P, E, Z, Y.
`)
	return 0
}

// printVersion writes version information and returns exit code 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "split (go-unix-utils) %s\n", version)
	return 0
}
