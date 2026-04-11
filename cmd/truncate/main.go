// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/truncate: shrink or extend file size.
// Implements srd083 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "truncate"

// sizeOp represents the type of size adjustment operation.
type sizeOp int

const (
	opSet       sizeOp = iota // set to exact value
	opGrow                    // add to current/ref size
	opShrink                  // subtract from current/ref size
	opAtMost                  // min(current, value)
	opAtLeast                 // max(current, value)
	opRoundDown               // round down to multiple
	opRoundUp                 // round up to multiple
)

// parsedSize holds a parsed size specification with its operation type.
type parsedSize struct {
	op    sizeOp
	value int64
}

// config holds parsed command-line options.
type config struct {
	size     *parsedSize
	refFile  string
	noCreate bool
	files    []string
}

// usageError indicates a command-line usage error that warrants a "Try --help" hint.
type usageError struct {
	msg string
}

// Error implements the error interface.
func (e *usageError) Error() string { return e.msg }

// main entry point with SIGPIPE handler.
// R3.3: InstallSIGPIPEHandler for graceful SIGPIPE exit.
func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		if _, ok := err.(*usageError); ok {
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		}
		os.Exit(1)
	}
	if !execute(cfg) {
		os.Exit(1)
	}
}

// execute processes all files and returns true if all succeed.
// R1.3: iterates over multiple FILE arguments.
func execute(cfg *config) bool {
	refSize, ok := resolveRefSize(cfg)
	if !ok {
		return false
	}
	success := true
	for _, name := range cfg.files {
		if err := processFile(name, cfg, refSize); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			success = false
		}
	}
	return success
}

// resolveRefSize returns the reference file size, or 0 if no reference.
// R2.1: stat the reference file to obtain its size.
// R2.2: exit 1 if the reference file cannot be accessed.
func resolveRefSize(cfg *config) (int64, bool) {
	if cfg.refFile == "" {
		return 0, true
	}
	info, err := os.Stat(cfg.refFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot stat '%s': %s\n",
			programName, cfg.refFile, sysErrMsg(err))
		return 0, false
	}
	return info.Size(), true
}

// processFile opens, computes the new size, and truncates a single file.
// R1.1: creates the file if it does not exist (unless -c).
// R1.4: with -c, silently skips files that do not exist.
func processFile(name string, cfg *config, refSize int64) error {
	flags := os.O_WRONLY
	if cfg.noCreate {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return nil
		}
	} else {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(name, flags, 0o666)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for writing: %s",
			name, sysErrMsg(err))
	}
	defer f.Close()
	return truncateFile(f, cfg, refSize)
}

// truncateFile computes and applies the new size to an open file.
func truncateFile(f *os.File, cfg *config, refSize int64) error {
	newSize, err := computeNewSize(f, cfg, refSize)
	if err != nil {
		return err
	}
	if err := f.Truncate(newSize); err != nil {
		return fmt.Errorf("failed to truncate '%s' at %d bytes: %s",
			f.Name(), newSize, sysErrMsg(err))
	}
	return nil
}

// computeNewSize determines the target file size based on config.
// R2.1: when -r is set, uses refSize as the base for relative ops.
func computeNewSize(f *os.File, cfg *config, refSize int64) (int64, error) {
	if cfg.size == nil {
		return refSize, nil
	}
	base := refSize
	if cfg.refFile == "" && cfg.size.op != opSet {
		info, err := f.Stat()
		if err != nil {
			return 0, fmt.Errorf("cannot fstat '%s': %s",
				f.Name(), sysErrMsg(err))
		}
		base = info.Size()
	}
	return applySize(base, cfg.size), nil
}

// applySize computes the new file size from a base and a size operation.
// R1.2: supports +, -, <, >, /, % prefix operators.
func applySize(base int64, ps *parsedSize) int64 {
	switch ps.op {
	case opSet:
		return ps.value
	case opGrow:
		return base + ps.value
	case opShrink:
		return base - ps.value
	case opAtMost:
		if base < ps.value {
			return base
		}
		return ps.value
	case opAtLeast:
		if base > ps.value {
			return base
		}
		return ps.value
	case opRoundDown:
		return roundDown(base, ps.value)
	case opRoundUp:
		return roundUp(base, ps.value)
	}
	return base
}

// roundDown returns base rounded down to the nearest multiple of mult.
func roundDown(base, mult int64) int64 {
	if mult == 0 {
		return base
	}
	return (base / mult) * mult
}

// roundUp returns base rounded up to the nearest multiple of mult.
func roundUp(base, mult int64) int64 {
	if mult == 0 {
		return base
	}
	return ((base + mult - 1) / mult) * mult
}

// parseSizeSpec parses a SIZE string with optional operator prefix.
// R1.1: numeric value with optional suffix via pkg/sizeparse.
// R1.2: operator prefix (+, -, <, >, /, %).
func parseSizeSpec(s string) (parsedSize, error) {
	if s == "" {
		return parsedSize{}, fmt.Errorf("invalid number: '%s'", s)
	}
	op, rest := extractOp(s)
	value, err := sizeparse.Parse(rest)
	if err != nil {
		return parsedSize{}, fmt.Errorf("invalid number: '%s'", s)
	}
	return parsedSize{op: op, value: value}, nil
}

// extractOp splits a size string into its operator and numeric part.
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
	}
	return opSet, s
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		next, err := handleFlag(args, i, cfg)
		if err != nil {
			return nil, err
		}
		i = next
	}
	cfg.files = args[i:]
	return validateConfig(cfg)
}

// handleFlag processes a single flag argument and returns the next index.
func handleFlag(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if v, ok := strings.CutPrefix(arg, "--size="); ok {
		return i + 1, setSize(cfg, v)
	}
	if v, ok := strings.CutPrefix(arg, "--reference="); ok {
		cfg.refFile = v
		return i + 1, nil
	}
	switch arg {
	case "--help":
		printUsage()
		os.Exit(0)
	case "--version":
		fmt.Println("truncate (go-unix-utils)")
		os.Exit(0)
	case "-c", "--no-create":
		cfg.noCreate = true
		return i + 1, nil
	case "-s", "--size":
		return consumeNextArg(args, i, 's', func(v string) error {
			return setSize(cfg, v)
		})
	case "-r", "--reference":
		return consumeNextArg(args, i, 'r', func(v string) error {
			cfg.refFile = v
			return nil
		})
	}
	return handleShortFlag(args, i, cfg)
}

// consumeNextArg takes the next arg as a value for flag flagChar.
func consumeNextArg(args []string, i int, flagChar byte, apply func(string) error) (int, error) {
	if i+1 >= len(args) {
		return 0, &usageError{fmt.Sprintf("option requires an argument -- '%c'", flagChar)}
	}
	return i + 2, apply(args[i+1])
}

// handleShortFlag handles combined short flags like -s100 or -rFILE.
func handleShortFlag(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if len(arg) > 2 && arg[0] == '-' {
		switch arg[1] {
		case 's':
			return i + 1, setSize(cfg, arg[2:])
		case 'r':
			cfg.refFile = arg[2:]
			return i + 1, nil
		}
	}
	return 0, &usageError{fmt.Sprintf("unrecognized option '%s'", arg)}
}

// setSize parses a size spec and stores it in cfg.
func setSize(cfg *config, val string) error {
	ps, err := parseSizeSpec(val)
	if err != nil {
		return err
	}
	cfg.size = &ps
	return nil
}

// validateConfig checks that required options are present.
func validateConfig(cfg *config) (*config, error) {
	if len(cfg.files) == 0 {
		return nil, &usageError{"missing file operand"}
	}
	if cfg.size == nil && cfg.refFile == "" {
		return nil, &usageError{"you must specify either '--size' or '--reference'"}
	}
	return cfg, nil
}

// sysErrMsg extracts the syscall error message from an os error.
func sysErrMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printUsage prints the help message.
func printUsage() {
	fmt.Print(`Usage: truncate OPTION... FILE...
Shrink or extend the size of each FILE to the specified size.

  -c, --no-create        do not create any files
  -r, --reference=RFILE  base size on RFILE
  -s, --size=SIZE        set or adjust the file size by SIZE bytes
      --help     display this help and exit
      --version  output version information and exit

SIZE is an integer and optional unit (example: 10K is 10*1024).
Units: K,M,G,T,P,E (powers of 1024) or KB,MB,... (powers of 1000).
SIZE may be prefixed by: '+' grow, '-' shrink, '<' at most, '>' at least,
'/' round down to multiple of, '%' round up to multiple of.
`)
}
