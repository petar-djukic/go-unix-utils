// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/shred: overwrite files to hide contents.
// Implements srd099-shred R1.1-R1.4.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shred"

const tryHelp = "Try 'shred --help' for more information."

// defaultPasses is the number of overwrite iterations per R1.1.
const defaultPasses = 3

// blockSize is the fixed write block size per D3.
const blockSize = 64 * 1024

// helpText is the usage message printed for --help.
const helpText = `Usage: shred [OPTION]... FILE...
Overwrite the specified FILE(s) repeatedly, in order to make it harder
for even very expensive hardware probing to recover the data.

  -n, --iterations=N  overwrite N times instead of the default (3)
  -u, --remove        truncate and remove file after overwriting
  -z, --zero          add a final overwrite with zeros to hide shredding
      --help          display this help and exit
      --version       output version information and exit
`

// versionText is the version string printed for --version.
const versionText = "shred (go-unix-utils) 1.0\n"

// options holds parsed command-line flags.
type options struct {
	iterations int
	zero       bool
	remove     bool
	files      []string
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
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
	size := fi.Size()
	if err := runPasses(path, size, opts); err != nil {
		return err
	}
	if opts.remove {
		return removeFile(path)
	}
	return nil
}

// runPasses executes all overwrite passes: random then optional zero.
func runPasses(path string, size int64, opts *options) error {
	for i := 0; i < opts.iterations; i++ {
		if err := overwritePass(path, size, true); err != nil {
			return err
		}
	}
	if opts.zero {
		if err := overwritePass(path, size, false); err != nil {
			return err
		}
	}
	return nil
}

// overwritePass performs a single overwrite pass on the file.
// D3: writes in fixed-size blocks, syncing after each pass.
// D4: opens with O_WRONLY without truncation.
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
// D2: uses crypto/rand for random data.
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
// R1.4: truncate then remove after overwriting.
func removeFile(path string) error {
	if err := os.Truncate(path, 0); err != nil {
		return fmt.Errorf("%s: %s", path, sysError(err))
	}
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
	if arg == "--help" {
		fmt.Fprint(os.Stdout, helpText)
		return 0, true, nil
	}
	if arg == "--version" {
		fmt.Fprint(os.Stdout, versionText)
		return 0, true, nil
	}
	if arg == "--zero" {
		opts.zero = true
		return 1, false, nil
	}
	if arg == "--remove" {
		opts.remove = true
		return 1, false, nil
	}
	return parseNonBoolArg(arg, args, idx, opts)
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

// parseLongWithValue handles --iterations=N and --iterations N.
func parseLongWithValue(arg string, args []string, idx int, opts *options) (bool, int, error) {
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

// setIterations validates and sets the iteration count.
func setIterations(opts *options, val string) error {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of passes: '%s'", val)
	}
	opts.iterations = n
	return nil
}

// parseShortFlags processes short flag clusters (e.g., "-nzu").
func parseShortFlags(flags string, args []string, idx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'z':
			opts.zero = true
		case 'u':
			opts.remove = true
		case 'n':
			return consumeShortValue(flags, args, idx, j, opts)
		default:
			return 1, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeShortValue extracts the value for the -n flag from a short cluster.
func consumeShortValue(flags string, args []string, idx, j int, opts *options) (int, error) {
	var val string
	var advance int
	if j+1 < len(flags) {
		val = flags[j+1:]
		advance = 1
	} else if idx+1 < len(args) {
		val = args[idx+1]
		advance = 2
	} else {
		return 1, fmt.Errorf("option requires an argument -- 'n'")
	}
	if err := setIterations(opts, val); err != nil {
		return advance, err
	}
	return advance, nil
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
