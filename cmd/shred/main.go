// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/shred: overwrite files to hide contents.
// Implements srd099-shred R1.1-R1.4, R2.1-R2.4.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shred"

const tryHelp = "Try 'shred --help' for more information."

// defaultPasses is the number of overwrite iterations per R1.1.
const defaultPasses = 3

// blockSize is the fixed write block size.
const blockSize = 64 * 1024

// helpText is the usage message printed for --help.
const helpText = `Usage: shred [OPTION]... FILE...
Overwrite the specified FILE(s) repeatedly, in order to make it harder
for even very expensive hardware probing to recover the data.

  -n, --iterations=N  overwrite N times instead of the default (3)
  -s, --size=N        shred this many bytes (suffixes like K, M, G accepted)
  -u, --remove[=HOW]  truncate and remove file after overwriting; See below
  -v, --verbose        show progress
  -x, --exact          do not round file sizes up to the next full block
  -z, --zero           add a final overwrite with zeros to hide shredding
      --help           display this help and exit
      --version        output version information and exit

HOW for --remove (default: wipesync):
  unlink    use a standard unlink call
  wipe      obfuscate the name before unlinking
  wipesync  like wipe but also sync each obfuscated name
`

// versionText is the version string printed for --version.
const versionText = "shred (go-unix-utils) 1.0\n"

// options holds parsed command-line flags.
type options struct {
	iterations   int
	zero         bool
	remove       bool
	removeMethod string // "unlink", "wipe", "wipesync"; empty defaults to "wipesync"
	size         int64  // 0 means use file size
	exact        bool
	verbose      bool
	files        []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the shred logic and returns the exit code.
// R3.1: exit 0 on success. R3.2: exit 1 on error.
func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", progName, err, tryHelp)
		return 1
	}
	if opts == nil {
		return 0 // --help or --version handled
	}
	if len(opts.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n%s\n", progName, tryHelp)
		return 1
	}
	return processFiles(opts)
}

// processFiles shreds each file in order.
// R2.3: process multiple files. R2.4: continue on error.
func processFiles(opts *options) int {
	exitCode := 0
	for _, f := range opts.files {
		if err := shredFile(f, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// shredFile overwrites a single file with random data.
// R1.1: overwrite with random data for configured iterations.
// R1.3: -z adds a final zero pass.
// R1.4: -u truncates and removes after overwriting.
func shredFile(path string, opts *options) error {
	size, err := resolveSize(path, opts)
	if err != nil {
		return err
	}
	if err := runPasses(path, size, opts); err != nil {
		return err
	}
	if opts.remove {
		return removeFile(path, opts)
	}
	return nil
}

// resolveSize determines how many bytes to overwrite.
// R2.2: --size overrides file size.
// --exact avoids rounding up to file block boundary.
func resolveSize(path string, opts *options) (int64, error) {
	if opts.size > 0 {
		return opts.size, nil
	}
	fi, err := sys.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("%s: %s", path, sysError(err))
	}
	size := fi.Size
	if !opts.exact && fi.Blksize > 0 {
		size = roundUp(size, fi.Blksize)
	}
	return size, nil
}

// roundUp rounds n up to the next multiple of block.
func roundUp(n, block int64) int64 {
	if block <= 0 || n%block == 0 {
		return n
	}
	return n + block - n%block
}

// totalPasses returns the total number of overwrite passes.
func totalPasses(opts *options) int {
	total := opts.iterations
	if opts.zero {
		total++
	}
	return total
}

// runPasses executes all overwrite passes: random then optional zero.
// R2.4: verbose output shows each pass.
func runPasses(path string, size int64, opts *options) error {
	total := totalPasses(opts)
	for i := 0; i < opts.iterations; i++ {
		printVerbose(opts, path, i+1, total, "random")
		if err := overwritePass(path, size, true); err != nil {
			return err
		}
	}
	if opts.zero {
		printVerbose(opts, path, total, total, "000000")
		if err := overwritePass(path, size, false); err != nil {
			return err
		}
	}
	return nil
}

// printVerbose prints progress info to stderr when verbose mode is on.
// R2.1/R2.4: format matches GNU shred verbose output.
func printVerbose(opts *options, path string, pass, total int, pattern string) {
	if !opts.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: pass %d/%d (%s)...\n",
		progName, path, pass, total, pattern)
}

// overwritePass performs a single overwrite pass on the file.
// Opens with O_WRONLY without truncation and syncs after writing.
func overwritePass(path string, size int64, random bool) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	defer f.Close()
	if err := writeBlocks(f, size, random); err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	return nil
}

// writeBlocks writes size bytes to f using fixed-size blocks.
// Uses crypto/rand for random data; zero-initialized buffer for zero passes.
func writeBlocks(f *os.File, size int64, random bool) error {
	buf := make([]byte, blockSize)
	remaining := size
	for remaining > 0 {
		n := int64(blockSize)
		if n > remaining {
			n = remaining
		}
		if random {
			if _, err := io.ReadFull(rand.Reader, buf[:n]); err != nil {
				return err
			}
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	return nil
}

// removeFile truncates and removes a file.
// R1.4/R2.2: supports unlink methods (unlink, wipe, wipesync).
func removeFile(path string, opts *options) error {
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "%s: %s: removing\n", progName, path)
	}
	if err := os.Truncate(path, 0); err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	method := opts.removeMethod
	if method == "" {
		method = "wipesync"
	}
	if method == "wipe" || method == "wipesync" {
		return wipeAndRemove(path, method == "wipesync")
	}
	return unlinkFile(path)
}

// wipeAndRemove renames a file to obscure its name, then unlinks.
func wipeAndRemove(path string, doSync bool) error {
	renamed, err := renameToDots(path, doSync)
	if err != nil {
		// best-effort: fall back to direct unlink
		return unlinkFile(path)
	}
	return unlinkFile(renamed)
}

// renameToDots renames a file by replacing basename chars with zeros.
func renameToDots(path string, doSync bool) (string, error) {
	dir, base := splitDirBase(path)
	newBase := strings.Repeat("0", len(base))
	newPath := dir + newBase
	if err := os.Rename(path, newPath); err != nil {
		return "", err
	}
	if doSync {
		syncDir(dir)
	}
	return newPath, nil
}

// splitDirBase splits a path into directory (with trailing slash) and base.
func splitDirBase(path string) (string, string) {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "", path
	}
	return path[:idx+1], path[idx+1:]
}

// syncDir syncs the directory to flush renames to disk.
func syncDir(dir string) {
	if dir == "" {
		dir = "."
	}
	d, err := os.Open(dir)
	if err != nil {
		return // best-effort
	}
	d.Sync() // best-effort: ignore sync error
	d.Close()
}

// unlinkFile removes a file via os.Remove.
func unlinkFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	return nil
}

// parseArgs parses command-line arguments.
// Returns nil options when --help or --version was handled.
func parseArgs(args []string) (*options, error) {
	opts := &options{iterations: defaultPasses}
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			i++
			opts.files = append(opts.files, args[i:]...)
			return opts, nil
		}
		advance, done, err := parseOneArg(args[i], args, i, opts)
		if err != nil {
			return nil, err
		}
		if done {
			return nil, nil
		}
		i += advance
	}
	return opts, nil
}

// parseOneArg dispatches a single argument to the appropriate handler.
func parseOneArg(arg string, args []string, idx int, opts *options) (int, bool, error) {
	if handled, advance, done, err := parseBoolLong(arg, opts); handled {
		return advance, done, err
	}
	return parseNonBoolArg(arg, args, idx, opts)
}

// parseBoolLong handles long boolean flags and --remove with optional value.
func parseBoolLong(arg string, opts *options) (bool, int, bool, error) {
	switch {
	case arg == "--help":
		fmt.Fprint(os.Stdout, helpText)
		return true, 0, true, nil
	case arg == "--version":
		fmt.Fprint(os.Stdout, versionText)
		return true, 0, true, nil
	case arg == "--zero":
		opts.zero = true
		return true, 1, false, nil
	case arg == "--exact":
		opts.exact = true
		return true, 1, false, nil
	case arg == "--verbose":
		opts.verbose = true
		return true, 1, false, nil
	case arg == "--remove":
		opts.remove = true
		return true, 1, false, nil
	case strings.HasPrefix(arg, "--remove="):
		return true, 1, false, parseRemoveMethod(opts, arg[len("--remove="):])
	}
	return false, 0, false, nil
}

// parseRemoveMethod validates and sets the remove method.
func parseRemoveMethod(opts *options, method string) error {
	switch method {
	case "unlink", "wipe", "wipesync":
		opts.remove = true
		opts.removeMethod = method
		return nil
	default:
		return fmt.Errorf("invalid --remove argument: '%s'", method)
	}
}

// parseNonBoolArg handles long value flags, short flags, and file args.
func parseNonBoolArg(arg string, args []string, idx int, opts *options) (int, bool, error) {
	if handled, advance, err := parseLongWithValue(arg, args, idx, opts); handled {
		return advance, false, err
	}
	if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
		advance, err := parseShortFlags(arg[1:], args, idx, opts)
		return advance, false, err
	}
	opts.files = append(opts.files, arg)
	return 1, false, nil
}

// parseLongWithValue handles --iterations=N and --size=N long flags.
func parseLongWithValue(arg string, args []string, idx int, opts *options) (bool, int, error) {
	if handled, adv, err := parseLongIterations(arg, args, idx, opts); handled {
		return true, adv, err
	}
	return parseLongSize(arg, args, idx, opts)
}

// parseLongIterations handles --iterations=N and --iterations N.
func parseLongIterations(arg string, args []string, idx int, opts *options) (bool, int, error) {
	if strings.HasPrefix(arg, "--iterations=") {
		val := arg[len("--iterations="):]
		return true, 1, setIterations(opts, val)
	}
	if arg == "--iterations" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		return true, 2, setIterations(opts, args[idx+1])
	}
	return false, 0, nil
}

// parseLongSize handles --size=N and --size N.
func parseLongSize(arg string, args []string, idx int, opts *options) (bool, int, error) {
	if strings.HasPrefix(arg, "--size=") {
		val := arg[len("--size="):]
		return true, 1, setSize(opts, val)
	}
	if arg == "--size" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		return true, 2, setSize(opts, args[idx+1])
	}
	return false, 0, nil
}

// setIterations validates and sets the iteration count.
func setIterations(opts *options, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of passes: '%s'", val)
	}
	opts.iterations = n
	return nil
}

// setSize parses a size string using sizeparse and stores it.
// R2.2: supports K, M, G suffixes via pkg/sizeparse.
func setSize(opts *options, val string) error {
	n, err := sizeparse.Parse(val)
	if err != nil {
		return fmt.Errorf("invalid file size: '%s'", val)
	}
	if n <= 0 {
		return fmt.Errorf("invalid file size: '%s'", val)
	}
	opts.size = n
	return nil
}

// parseShortFlags processes short flag clusters (e.g., "-vzun3").
func parseShortFlags(flags string, args []string, idx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'z':
			opts.zero = true
		case 'u':
			opts.remove = true
		case 'v':
			opts.verbose = true
		case 'x':
			opts.exact = true
		case 'n':
			return consumeShortValue(flags, args, idx, j, "n", func(v string) error {
				return setIterations(opts, v)
			})
		case 's':
			return consumeShortValue(flags, args, idx, j, "s", func(v string) error {
				return setSize(opts, v)
			})
		default:
			return 1, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeShortValue extracts the value for a short flag requiring an argument.
func consumeShortValue(
	flags string, args []string, idx, j int,
	flag string, setter func(string) error,
) (int, error) {
	if j+1 < len(flags) {
		return 1, setter(flags[j+1:])
	}
	if idx+1 < len(args) {
		return 2, setter(args[idx+1])
	}
	return 1, fmt.Errorf("option requires an argument -- '%s'", flag)
}

// sysError extracts the underlying OS error message.
func sysError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst uppercases the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
