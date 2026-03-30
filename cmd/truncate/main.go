// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/truncate implements GNU truncate: shrink or extend file size.
//
// Implements prd083-truncate R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sizeOp represents the size operation to apply.
type sizeOp int

const (
	opAbsolute  sizeOp = iota // set to exact size
	opGrow                    // +N: grow by N bytes
	opShrink                  // -N: shrink by N bytes
	opAtMost                  // <N: set to min(current, N)
	opAtLeast                 // >N: set to max(current, N)
	opRoundDown               // /N: round down to multiple of N
	opRoundUp                 // %N: round up to multiple of N
)

// truncateOptions holds parsed flag state.
type truncateOptions struct {
	sizeStr  string // -s, --size=SIZE
	noCreate bool   // -c, --no-create
	ioBlocks bool   // -o, --io-blocks
}

// parsedSize holds the parsed size specification.
type parsedSize struct {
	op    sizeOp
	value int64
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags and processes each file argument.
// R1.1, R1.3: processes multiple FILE arguments with the same size operation.
func run(args []string, stderr *os.File) int {
	opts, files := parseArgs(args)
	if opts.sizeStr == "" {
		fmt.Fprintln(stderr, "truncate: you must specify '--size=SIZE'")
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "truncate: missing file operand")
		return 1
	}
	ps, err := parseSizeSpec(opts.sizeStr)
	if err != nil {
		fmt.Fprintf(stderr, "truncate: %v\n", err)
		return 1
	}
	return processFiles(files, ps, opts, stderr)
}

// processFiles truncates each file, returning 0 on success or 1 on any error.
func processFiles(files []string, ps parsedSize, opts truncateOptions, stderr *os.File) int {
	exitCode := 0
	for _, f := range files {
		if err := truncateFile(f, ps, opts); err != nil {
			fmt.Fprintf(stderr, "truncate: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (truncateOptions, []string) {
	var opts truncateOptions
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			i += parseLongFlag(&opts, arg, args, i)
			continue
		}
		if len(arg) >= 2 && arg[0] == '-' {
			i += parseShortFlags(&opts, arg[1:], args, i)
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// parseLongFlag handles --flag and --flag=VALUE forms.
// Returns the number of additional args consumed.
func parseLongFlag(opts *truncateOptions, arg string, args []string, idx int) int {
	switch {
	case arg == "--no-create":
		opts.noCreate = true
	case arg == "--io-blocks":
		opts.ioBlocks = true
	case strings.HasPrefix(arg, "--size="):
		opts.sizeStr = arg[len("--size="):]
	case arg == "--size" && idx+1 < len(args):
		opts.sizeStr = args[idx+1]
		return 1
	}
	return 0
}

// parseShortFlags processes short flag characters.
// Returns the number of additional args consumed.
func parseShortFlags(opts *truncateOptions, chars string, args []string, idx int) int {
	for i, ch := range chars {
		switch ch {
		case 'c':
			opts.noCreate = true
		case 'o':
			opts.ioBlocks = true
		case 's':
			rest := chars[i+1:]
			if rest != "" {
				opts.sizeStr = rest
				return 0
			}
			if idx+1 < len(args) {
				opts.sizeStr = args[idx+1]
				return 1
			}
			return 0
		}
	}
	return 0
}

// parseSizeSpec parses a size specification with optional operator prefix.
// R1.2: supports +, -, <, >, /, % prefixes before the numeric value.
func parseSizeSpec(s string) (parsedSize, error) {
	if s == "" {
		return parsedSize{}, fmt.Errorf("invalid number: %q", s)
	}
	op, rest := extractOp(s)
	value, err := sizeparse.Parse(rest)
	if err != nil {
		return parsedSize{}, fmt.Errorf("invalid number: %q", s)
	}
	return parsedSize{op: op, value: value}, nil
}

// extractOp separates the operator prefix from the size string.
func extractOp(s string) (sizeOp, string) {
	switch s[0] {
	case '+':
		return opGrow, s[1:]
	case '-':
		return opShrink, s[1:]
	case '<':
		return opAtMost, s[1:]
	case '>':
		return opAtLeast, s[1:]
	case '/':
		return opRoundDown, s[1:]
	case '%':
		return opRoundUp, s[1:]
	default:
		return opAbsolute, s
	}
}

// truncateFile applies the size operation to a single file.
// R1.1: truncates or extends the file to the computed size.
// R1.4: -c suppresses creation of nonexistent files.
func truncateFile(path string, ps parsedSize, opts truncateOptions) error {
	fi, statErr := os.Stat(path)
	if os.IsNotExist(statErr) {
		if opts.noCreate {
			return nil
		}
		if err := createFile(path); err != nil {
			return err
		}
		fi, statErr = os.Stat(path)
	}
	if statErr != nil {
		return fmt.Errorf("cannot open %q for writing: %v", path, statErr)
	}
	return applyTruncate(path, fi.Size(), ps, opts)
}

// createFile creates an empty file.
func createFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("cannot open %q for writing: %v", path, err)
	}
	f.Close() // best-effort close; truncation is the critical op
	return nil
}

// applyTruncate computes the target size and truncates the file.
// R1.4: multiplies by IO block size when --io-blocks is set.
func applyTruncate(path string, current int64, ps parsedSize, opts truncateOptions) error {
	size := ps.value
	if opts.ioBlocks {
		size *= getBlockSize(path)
	}
	target := computeTarget(current, size, ps.op)
	if target < 0 {
		target = 0
	}
	if err := os.Truncate(path, target); err != nil {
		return fmt.Errorf("failed to truncate %q at %d bytes: %v", path, target, err)
	}
	return nil
}

// getBlockSize returns the IO block size for the file or its parent directory.
// R1.4: used with --io-blocks to convert block count to bytes.
func getBlockSize(path string) int64 {
	fi, err := sys.Stat(path)
	if err == nil && fi.Blksize > 0 {
		return fi.Blksize
	}
	fi, err = sys.Stat(filepath.Dir(path))
	if err == nil && fi.Blksize > 0 {
		return fi.Blksize
	}
	return 512
}

// computeTarget applies the size operation to produce the final file size.
// R1.2: handles absolute, relative (+/-), at-most (<), at-least (>),
// round-down (/), and round-up (%) operations.
func computeTarget(current, size int64, op sizeOp) int64 {
	switch op {
	case opGrow:
		return current + size
	case opShrink:
		return current - size
	case opAtMost:
		return min(current, size)
	case opAtLeast:
		return max(current, size)
	case opRoundDown:
		if size == 0 {
			return current
		}
		return (current / size) * size
	case opRoundUp:
		if size == 0 {
			return current
		}
		return ((current + size - 1) / size) * size
	default:
		return size
	}
}
