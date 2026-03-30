// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shred implements GNU shred: overwrite a file to hide its contents.
//
// Implements prd099-shred R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	// R1.1: default overwrite pass count.
	defaultIterations = 3
	// writeBlockSize is the I/O buffer size for overwrite passes.
	writeBlockSize = 64 * 1024
	progName       = "shred"
)

// shredOptions holds parsed flag state.
type shredOptions struct {
	iterations int   // R1.2: -n/--iterations
	zero       bool  // R1.3: -z/--zero
	remove     bool  // R1.4: -u/--remove
	verbose    bool  // R2.1: -v/--verbose
	force      bool  // -f/--force (parsed, not yet active)
	exact      bool  // -x/--exact (parsed, not yet active)
	size       int64 // R2.2: -s/--size
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags and shreds each file.
// R2.3: processes multiple FILE arguments in order.
// R2.4: continues processing remaining files on error, exits 1.
func run(args []string, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}
	exitCode := 0
	for _, name := range files {
		if err := shredFile(name, opts, stderr); err != nil {
			if isBrokenPipe(err) {
				return 0
			}
			reportError(stderr, name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (shredOptions, []string, error) {
	opts := shredOptions{iterations: defaultIterations}
	var files []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || (arg != "-" && !strings.HasPrefix(arg, "-")) {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			extra, err := parseLongFlag(&opts, arg, args, i)
			if err != nil {
				return opts, nil, err
			}
			i += extra
			continue
		}
		extra, err := parseShortFlags(&opts, arg[1:], args, i)
		if err != nil {
			return opts, nil, err
		}
		i += extra
	}
	return opts, files, nil
}

// parseLongFlag handles a single --key or --key=value argument.
// Returns the number of extra args consumed.
func parseLongFlag(opts *shredOptions, arg string, args []string, idx int) (int, error) {
	key, val, hasVal := splitLongOpt(arg[2:])
	switch key {
	case "iterations":
		return setIntOpt(&opts.iterations, "iterations", val, hasVal, args, idx)
	case "zero":
		opts.zero = true
	case "remove":
		opts.remove = true
	case "verbose":
		opts.verbose = true
	case "force":
		opts.force = true
	case "exact":
		opts.exact = true
	case "size":
		return setSizeOpt(opts, val, hasVal, args, idx)
	default:
		return 0, fmt.Errorf("unrecognized option '--%s'", key)
	}
	return 0, nil
}

// splitLongOpt splits "key=value" into (key, value, true) or ("key", "", false).
func splitLongOpt(s string) (string, string, bool) {
	if key, val, ok := strings.Cut(s, "="); ok {
		return key, val, true
	}
	return s, "", false
}

// setIntOpt parses an integer value from a long option.
func setIntOpt(dest *int, name, val string, hasVal bool, args []string, idx int) (int, error) {
	if !hasVal {
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--%s' requires an argument", name)
		}
		val = args[idx+1]
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("invalid number of passes: %q", val)
		}
		*dest = n
		return 1, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid number of passes: %q", val)
	}
	*dest = n
	return 0, nil
}

// setSizeOpt parses the --size value.
func setSizeOpt(opts *shredOptions, val string, hasVal bool, args []string, idx int) (int, error) {
	if !hasVal {
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--size' requires an argument")
		}
		val = args[idx+1]
		return 1, parseSizeVal(&opts.size, val)
	}
	return 0, parseSizeVal(&opts.size, val)
}

// parseSizeVal parses a size string with unit suffixes into an int64.
// R2.2: supports K, M, G and other suffixes via sizeparse.
func parseSizeVal(dest *int64, val string) error {
	size, err := sizeparse.Parse(val)
	if err != nil {
		return fmt.Errorf("invalid file size: %q", val)
	}
	*dest = size
	return nil
}

// parseShortFlags processes a cluster of short flags (e.g., "-vzn3").
// Returns the number of extra args consumed.
func parseShortFlags(opts *shredOptions, chars string, args []string, idx int) (int, error) {
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'z':
			opts.zero = true
		case 'u':
			opts.remove = true
		case 'v':
			opts.verbose = true
		case 'f':
			opts.force = true
		case 'x':
			opts.exact = true
		case 'n':
			return parseShortIntVal(&opts.iterations, chars[j+1:], args, idx)
		case 's':
			return parseShortSizeVal(opts, chars[j+1:], args, idx)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", chars[j])
		}
	}
	return 0, nil
}

// parseShortIntVal parses an int from the rest of a flag cluster or the next arg.
func parseShortIntVal(dest *int, rest string, args []string, idx int) (int, error) {
	if rest != "" {
		n, err := strconv.Atoi(rest)
		if err != nil {
			return 0, fmt.Errorf("invalid number of passes: %q", rest)
		}
		*dest = n
		return 0, nil
	}
	if idx+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'n'")
	}
	n, err := strconv.Atoi(args[idx+1])
	if err != nil {
		return 0, fmt.Errorf("invalid number of passes: %q", args[idx+1])
	}
	*dest = n
	return 1, nil
}

// parseShortSizeVal parses a size from the rest of a flag cluster or the next arg.
func parseShortSizeVal(opts *shredOptions, rest string, args []string, idx int) (int, error) {
	if rest != "" {
		return 0, parseSizeVal(&opts.size, rest)
	}
	if idx+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 's'")
	}
	return 1, parseSizeVal(&opts.size, args[idx+1])
}

// shredFile performs overwrite passes and optional removal for one file.
// R1.1: overwrites with random data for the configured number of passes.
func shredFile(name string, opts shredOptions, stderr io.Writer) error {
	f, err := os.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort close if not already closed by removeAfterShred

	size, err := fileShredSize(f, opts.size)
	if err != nil {
		return err
	}

	if err := performPasses(f, size, opts, name, stderr); err != nil {
		return err
	}

	if opts.remove {
		return removeAfterShred(f, name, opts.verbose, stderr)
	}
	return nil
}

// fileShredSize returns the overwrite size: -s value if set, else file size.
func fileShredSize(f *os.File, optSize int64) (int64, error) {
	if optSize > 0 {
		return optSize, nil
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// totalPasses returns the total number of overwrite passes.
func totalPasses(opts shredOptions) int {
	n := opts.iterations
	if opts.zero {
		n++
	}
	return n
}

// printProgress prints a verbose progress line to stderr.
// R2.1: format matches GNU shred verbose output.
func printProgress(w io.Writer, name string, passNum, total int, passType string) {
	fmt.Fprintf(w, "%s: %s: pass %d/%d (%s)...\n", progName, name, passNum, total, passType)
}

// performPasses runs all overwrite passes: N random plus optional zero.
// R1.1: random data passes. R1.3: optional final zero pass.
// R2.1: prints verbose progress to stderr when enabled.
func performPasses(f *os.File, size int64, opts shredOptions, name string, stderr io.Writer) error {
	total := totalPasses(opts)
	for i := 0; i < opts.iterations; i++ {
		if opts.verbose {
			printProgress(stderr, name, i+1, total, "random")
		}
		if err := overwritePass(f, size, rand.Reader); err != nil {
			return err
		}
	}
	if opts.zero {
		if opts.verbose {
			printProgress(stderr, name, total, total, "000000")
		}
		if err := overwritePass(f, size, zeroReader{}); err != nil {
			return err
		}
	}
	return f.Sync()
}

// overwritePass seeks to the start and writes size bytes from source.
func overwritePass(f *os.File, size int64, source io.Reader) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return writeFromSource(f, size, source)
}

// writeFromSource writes exactly size bytes from source to the file.
func writeFromSource(f *os.File, size int64, source io.Reader) error {
	buf := make([]byte, writeBlockSize)
	remaining := size
	for remaining > 0 {
		n := min(int64(len(buf)), remaining)
		if _, err := io.ReadFull(source, buf[:n]); err != nil {
			return err
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	return nil
}

// zeroReader is an io.Reader that produces zero bytes.
type zeroReader struct{}

// Read fills p with zeros.
func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// removeAfterShred truncates and removes the file.
// R1.4: truncate then unlink.
// R2.1: prints verbose remove progress when enabled.
func removeAfterShred(f *os.File, name string, verbose bool, stderr io.Writer) error {
	if verbose {
		fmt.Fprintf(stderr, "%s: %s: removing\n", progName, name)
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	f.Close() // best-effort close before remove
	if err := os.Remove(name); err != nil {
		return err
	}
	if verbose {
		fmt.Fprintf(stderr, "%s: %s: removed\n", progName, name)
	}
	return nil
}

// reportError prints a shred error message to stderr.
// R2.4: error output includes program name and filename.
func reportError(w io.Writer, name string, err error) {
	fmt.Fprintf(w, "%s: %s: %v\n", progName, name, unwrapErr(err))
}

// unwrapErr extracts the underlying syscall error from os.PathError.
func unwrapErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// isBrokenPipe reports whether an error is caused by writing to a broken pipe.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
