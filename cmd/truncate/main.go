// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/truncate implements GNU truncate: shrink or extend file size.
//
// Implements prd083-truncate R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2.
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

const programName = "truncate"

const usageText = `Usage: truncate OPTION... FILE...
Shrink or extend the size of each FILE to the specified size.

  -c, --no-create        do not create any files
  -o, --io-blocks        treat SIZE as number of IO blocks instead of bytes
  -r, --reference=RFILE  base size on RFILE
  -s, --size=SIZE        set or adjust the file size by SIZE bytes
      --help             display this help and exit
      --version          output version information and exit

SIZE may be prefixed by: '+' grow, '-' shrink, '<' at most, '>' at least,
'/' round down to multiple, '%' round up to multiple.
SIZE may also have a suffix: K (1024), KB (1000), M, MB, G, GB, T, P, E.
`

const versionText = "truncate (go-unix-utils) 0.1\n"

// truncateOptions holds parsed flag state.
type truncateOptions struct {
	sizeStr  string // -s, --size=SIZE
	refFile  string // -r, --reference=RFILE
	noCreate bool   // -c, --no-create
	ioBlocks bool   // -o, --io-blocks
	showHelp bool   // --help
	showVer  bool   // --version
}

// parsedSize holds the parsed size specification.
type parsedSize struct {
	op    sizeOp
	value int64
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses flags and processes each file argument.
// R3.1: exits 0 on success, non-zero on failure.
// R3.2: prints usage on --help, version on --version.
func run(args []string, stdout *os.File, stderr *os.File) int {
	opts, files := parseArgs(args)
	if opts.showHelp {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if opts.showVer {
		fmt.Fprint(stdout, versionText)
		return 0
	}
	return runTruncate(opts, files, stderr)
}

// runTruncate validates options and processes files.
func runTruncate(opts truncateOptions, files []string, stderr *os.File) int {
	if opts.sizeStr == "" && opts.refFile == "" {
		fmt.Fprintf(stderr, "%s: you must specify either '--size=SIZE' or '--reference=RFILE'\n", programName)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", programName)
		return 1
	}
	baseSize, err := resolveBaseSize(opts)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	ps, err := buildParsedSize(opts, baseSize)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	return processFiles(files, ps, opts, stderr)
}

// resolveBaseSize returns the reference file size if -r is set, or -1 if not.
// R2.1: uses os.Stat to obtain RFILE size without reading contents.
// R2.2: exits 1 if the reference file cannot be accessed.
func resolveBaseSize(opts truncateOptions) (int64, error) {
	if opts.refFile == "" {
		return -1, nil
	}
	fi, err := os.Stat(opts.refFile)
	if err != nil {
		return 0, fmt.Errorf("cannot stat %q: %v", opts.refFile, err)
	}
	return fi.Size(), nil
}

// buildParsedSize constructs a parsedSize from options and optional base size.
// R2.2: when both -r and -s are given, applies -s adjustment relative to ref size.
func buildParsedSize(opts truncateOptions, baseSize int64) (parsedSize, error) {
	if opts.sizeStr == "" {
		// -r only: set to reference file size
		return parsedSize{op: opAbsolute, value: baseSize}, nil
	}
	ps, err := parseSizeSpec(opts.sizeStr)
	if err != nil {
		return parsedSize{}, err
	}
	if baseSize >= 0 && ps.op == opAbsolute {
		// -r and -s without operator: treat -s value as relative grow
		return parsedSize{op: opAbsolute, value: baseSize + ps.value}, nil
	}
	if baseSize >= 0 {
		// -r and -s with operator: compute target from ref size
		target := computeTarget(baseSize, ps.value, ps.op)
		if target < 0 {
			target = 0
		}
		return parsedSize{op: opAbsolute, value: target}, nil
	}
	return ps, nil
}

// processFiles truncates each file, returning 0 on success or 1 on any error.
// R3.1: exit 0 when all succeed, exit 1 when any fail.
func processFiles(files []string, ps parsedSize, opts truncateOptions, stderr *os.File) int {
	exitCode := 0
	for _, f := range files {
		if err := truncateFile(f, ps, opts); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", programName, err)
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
	case arg == "--help":
		opts.showHelp = true
	case arg == "--version":
		opts.showVer = true
	case strings.HasPrefix(arg, "--size="):
		opts.sizeStr = arg[len("--size="):]
	case arg == "--size" && idx+1 < len(args):
		opts.sizeStr = args[idx+1]
		return 1
	case strings.HasPrefix(arg, "--reference="):
		opts.refFile = arg[len("--reference="):]
	case arg == "--reference" && idx+1 < len(args):
		opts.refFile = args[idx+1]
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
			return consumeFlagValue(chars[i+1:], args, idx, &opts.sizeStr)
		case 'r':
			return consumeFlagValue(chars[i+1:], args, idx, &opts.refFile)
		}
	}
	return 0
}

// consumeFlagValue extracts the value for a short flag from either the remaining
// chars or the next argument.
func consumeFlagValue(rest string, args []string, idx int, target *string) int {
	if rest != "" {
		*target = rest
		return 0
	}
	if idx+1 < len(args) {
		*target = args[idx+1]
		return 1
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
