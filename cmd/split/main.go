// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/split: split a file into pieces.
// Implements srd067-split R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.2.
package main

import (
	"fmt"
	"io"
	"os"
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
	chunkByBytes      chunkType = iota // N: split into N byte-sized chunks
	chunkByLines                       // l/N: split into N line-based chunks
	chunkRoundRobin                    // r/N: round-robin lines into N chunks
)

// chunkSpec holds the parsed -n CHUNKS value.
type chunkSpec struct {
	kind  chunkType
	count int64
}

// config holds parsed command-line options.
type config struct {
	mode            splitMode
	lineCount       int64
	byteCount       int64
	chunks          chunkSpec
	suffixLen       int
	numericSuffix   bool
	additionalSuffix string
	filter          string
	prefix          string
	inputFile       string
}

// parseArgs separates flags from positional arguments and returns a config.
// R2.4: conflicting split options produce an error and exit 1.
func parseArgs(args []string) (config, error) {
	panic("not implemented")
}

// parseLongFlag handles a --key=value or --key value style flag.
func parseLongFlag(cfg *config, args []string, i int) (int, error) {
	panic("not implemented")
}

// parseShortFlags handles single-character flags, possibly combined.
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	panic("not implemented")
}

// parseChunkSpec parses a CHUNKS argument into a chunkSpec.
// R2.3: supports N, l/N, and r/N forms.
func parseChunkSpec(s string) (chunkSpec, error) {
	panic("not implemented")
}

// parseByteSize parses a byte count with optional suffix via sizeparse.
// R2.1: supports K, M, G, T, P, E, Z, Y and KB, MB, etc.
func parseByteSize(s string) (int64, error) {
	// Ensure sizeparse is used so the import is not unused.
	_ = sizeparse.Parse
	panic("not implemented")
}

// validateConfig checks for conflicting options.
// R2.4: conflicting split options must produce an error.
func validateConfig(cfg *config) error {
	panic("not implemented")
}

// run executes the split logic and returns the exit code.
// R4.1: returns 0 on success.
// R4.2: returns 1 on error.
func run(args []string) int {
	panic("not implemented")
}

// openInput returns the input reader for the given file argument.
// R1.4: reads from stdin when FILE is "-" or absent.
func openInput(name string) (io.ReadCloser, error) {
	panic("not implemented")
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
	panic("not implemented")
}

// next returns the next suffix string and advances the counter.
func (g *suffixGenerator) next() (string, error) {
	panic("not implemented")
}

// outputFilename builds the full output filename.
// R1.2: prefix + suffix + additional-suffix.
// R3.3: appends --additional-suffix after the generated suffix.
func outputFilename(prefix, suffix, additional string) string {
	panic("not implemented")
}

// splitByLines splits input into pieces of n lines each.
// R1.1, R1.3: line-count splitting mode.
func splitByLines(r io.Reader, cfg *config) error {
	panic("not implemented")
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
	panic("not implemented")
}

// reportError prints a GNU-compatible diagnostic to stderr.
func reportError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// Ensure imports are used so the file compiles as a contract stub.
var _ = strings.HasPrefix
var _ = fmt.Sprintf
var _ io.Reader
var _ = os.Stdin
