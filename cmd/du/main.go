// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du utility: recursive directory disk usage.
//
// Implements: prd009-du (R1-R5)
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// blockUnit controls the output unit for disk usage values.
type blockUnit int

const (
	unitKilo blockUnit = iota // 1024-byte blocks (default, -k)
	unitMega                  // 1048576-byte blocks (-m)
)

// options holds the parsed command-line flags for du.
type options struct {
	humanReadable bool      // -h: display sizes as human-readable strings
	allFiles      bool      // -a: print size for every file, not just directories
	maxDepth      int       // -d N / --max-depth=N: limit reported depth; -1 means unlimited
	hasMaxDepth   bool      // true when -d, -s, or --max-depth was explicitly given
	grandTotal    bool      // -c: append a grand total line after all arguments
	apparentSize  bool      // --apparent-size: use st_size instead of st_blocks
	unit          blockUnit // output block unit: unitKilo (default) or unitMega
}

// seen tracks (dev, ino) pairs for hard-link deduplication across the entire
// du invocation. Per prd009-du R3.
type seen map[uint64]map[uint64]bool

// mark records a (dev, ino) pair and reports whether it was already recorded.
// Returns true if already seen — caller should skip counting blocks for this entry.
func (s seen) mark(dev, ino uint64) bool {
	if s[dev] == nil {
		s[dev] = make(map[uint64]bool)
	}
	if s[dev][ino] {
		return true
	}
	s[dev][ino] = true
	return false
}

func main() {
	// Install SIGPIPE handler so piping du output to head exits silently.
	// Per prd009-du R5.1.
	sys.InstallSIGPIPEHandler()

	opts, paths, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %s\n", err)
		os.Exit(1)
	}

	// Per prd009-du R1.1: default to current directory when no arguments given.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	seenInodes := make(seen)
	exitCode := 0
	var grandRaw int64

	for _, path := range paths {
		raw, ok := processArg(path, opts, seenInodes)
		if !ok {
			exitCode = 1
		}
		grandRaw += raw
	}

	// Per prd009-du R2.7: print grand total line after all arguments.
	if opts.grandTotal {
		printLine(grandRaw, "total", opts)
	}

	os.Exit(exitCode)
}

// processArg processes a single command-line path argument. For directories,
// it recurses and then prints the root total. For non-directory files, it
// prints immediately. Returns the accumulated raw value and whether processing
// succeeded without errors. Per prd009-du R1.5.
func processArg(path string, opts options, seenInodes seen) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %s: %s\n", path, syserrMsg(err))
		return 0, false
	}

	if !fi.Info.IsDir() {
		// Non-directory top-level argument: count and print immediately.
		raw := entryRaw(fi, opts)
		if seenInodes.mark(fi.Dev, fi.Ino) {
			raw = 0
		}
		printLine(raw, path, opts)
		return raw, true
	}

	// Directory argument: recurse bottom-up, then print the root total.
	// Pass fi to traverseDir to avoid a redundant Lstat on the root.
	total, ok := traverseDir(path, fi, opts, seenInodes, 0)
	printLine(total, path, opts)
	return total, ok
}

// traverseDir recursively traverses a directory, accumulates raw values
// (512-byte blocks or bytes depending on opts.apparentSize), and prints size
// lines for subdirectories and files per the active flags. Returns the
// accumulated raw total and whether traversal completed without errors.
//
// fi is the pre-computed FileInfo for path; depth is the traversal depth
// relative to the top-level argument (0 for the argument itself).
// Per design decisions D1 (os.ReadDir + explicit Lstat) and D2 (bottom-up).
func traverseDir(path string, fi *sys.FileInfo, opts options, seenInodes seen, depth int) (int64, bool) {
	// Count the directory entry itself. Per design decision D2.
	dirRaw := entryRaw(fi, opts)
	if seenInodes.mark(fi.Dev, fi.Ino) {
		dirRaw = 0
	}
	total := dirRaw

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %s: %s\n", path, syserrMsg(err))
		return total, false
	}

	ok := true
	for _, entry := range entries {
		entryPath := joinPath(path, entry.Name())
		entryFI, statErr := sys.Lstat(entryPath)
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "du: %s: %s\n", entryPath, syserrMsg(statErr))
			ok = false
			continue
		}

		if entryFI.Info.IsDir() {
			// Recurse into subdirectory first (bottom-up per D2).
			// Pass entryFI to avoid a redundant Lstat inside the recursive call.
			subTotal, subOk := traverseDir(entryPath, entryFI, opts, seenInodes, depth+1)
			if !subOk {
				ok = false
			}
			total += subTotal
			// Print subdirectory line only if within the depth limit. Per D3.
			if canPrint(opts, depth+1) {
				printLine(subTotal, entryPath, opts)
			}
		} else {
			raw := entryRaw(entryFI, opts)
			if seenInodes.mark(entryFI.Dev, entryFI.Ino) {
				raw = 0
			}
			total += raw
			// Print individual file lines only when -a is active. Per prd009-du R2.3.
			if opts.allFiles && canPrint(opts, depth+1) {
				printLine(raw, entryPath, opts)
			}
		}
	}

	return total, ok
}

// canPrint reports whether an entry at the given traversal depth should be
// printed as a separate size line based on the depth limit. Per D3.
func canPrint(opts options, depth int) bool {
	if opts.hasMaxDepth {
		return depth <= opts.maxDepth
	}
	return true
}

// entryRaw returns the raw accumulated value for a single entry (excluding
// children for directories). For --apparent-size: st_size (bytes). For normal
// mode: st_blocks (512-byte units). Per prd009-du R1.2 and R2.8.
func entryRaw(fi *sys.FileInfo, opts options) int64 {
	if opts.apparentSize {
		return fi.Size
	}
	return fi.Blocks
}

// printLine writes a "SIZE\tPATH\n" line to stdout. For -h, formats the raw
// value as a human-readable string. For block modes, converts to the selected
// unit first. Per prd009-du R1.3 and R2.1.
func printLine(raw int64, path string, opts options) {
	if opts.humanReadable {
		var bytes int64
		if opts.apparentSize {
			bytes = raw // raw is already in bytes
		} else {
			bytes = raw * 512 // convert 512-byte blocks to bytes
		}
		fmt.Printf("%s\t%s\n", format.HumanSize(bytes, format.HumanSizeOpts{}), path)
		return
	}

	var displayVal int64
	if opts.apparentSize {
		displayVal = bytesToUnit(raw, opts)
	} else {
		displayVal = blocks512ToUnit(raw, opts)
	}
	fmt.Printf("%d\t%s\n", displayVal, path)
}

// blocks512ToUnit converts a 512-byte block count to the selected output unit.
// Per prd009-du R1.2 (1K blocks) and R2.6 (1M blocks, rounded up).
func blocks512ToUnit(blocks512 int64, opts options) int64 {
	switch opts.unit {
	case unitMega:
		// Per task R2: Blocks / 2 / 1024; minimum 1 for any non-zero count.
		if blocks512 == 0 {
			return 0
		}
		mb := blocks512 / 2 / 1024
		if mb == 0 {
			mb = 1
		}
		return mb
	default: // unitKilo
		// Per prd009-du R1.2: st_blocks / 2 gives 1K block count (floor division).
		return blocks512 / 2
	}
}

// bytesToUnit converts an apparent size in bytes to the selected block unit.
// Per prd009-du R2.8.
func bytesToUnit(size int64, opts options) int64 {
	if size == 0 {
		return 0
	}
	switch opts.unit {
	case unitMega:
		mb := size / (1024 * 1024)
		if mb == 0 {
			mb = 1
		}
		return mb
	default: // unitKilo
		// Ceiling division: any non-zero size shows at least 1K.
		return (size + 1023) / 1024
	}
}

// joinPath appends a filename to a directory path with a "/" separator,
// preserving the original prefix as-is (e.g. "./" for relative paths).
func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}

// syserrMsg extracts just the OS error string from an *os.PathError, avoiding
// the redundant "op path: error" prefix.
func syserrMsg(err error) string {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err.Error()
	}
	return err.Error()
}

// parseFlags parses du command-line arguments using manual flag parsing
// following the cmd/wc/main.go pattern. Per design decision D4.
func parseFlags(args []string) (options, []string, error) {
	opts := options{maxDepth: -1}
	var paths []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			if err := parseLongOption(arg, &opts, args, &i); err != nil {
				return options{}, nil, err
			}
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			if err := parseShortFlags(arg[1:], &opts, args, &i); err != nil {
				return options{}, nil, err
			}
			continue
		}

		paths = append(paths, arg)
	}

	return opts, paths, nil
}

// parseLongOption handles --apparent-size and --max-depth[=N] long options.
func parseLongOption(arg string, opts *options, args []string, i *int) error {
	switch {
	case arg == "--apparent-size":
		opts.apparentSize = true
	case arg == "--max-depth" || strings.HasPrefix(arg, "--max-depth="):
		if strings.HasPrefix(arg, "--max-depth=") {
			val := arg[len("--max-depth="):]
			n, err := parseNonNegInt(val)
			if err != nil {
				return fmt.Errorf("invalid argument '%s' for '--max-depth'", val)
			}
			opts.maxDepth = n
			opts.hasMaxDepth = true
		} else {
			*i++
			if *i >= len(args) {
				return fmt.Errorf("option '--max-depth' requires an argument")
			}
			n, err := parseNonNegInt(args[*i])
			if err != nil {
				return fmt.Errorf("invalid argument '%s' for '--max-depth'", args[*i])
			}
			opts.maxDepth = n
			opts.hasMaxDepth = true
		}
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// parseShortFlags handles combined short flags like -sk, -ah, -d N.
func parseShortFlags(flags string, opts *options, args []string, i *int) error {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'h':
			opts.humanReadable = true
		case 's':
			// -s is equivalent to --max-depth=0. Per prd009-du R2.2.
			opts.hasMaxDepth = true
			opts.maxDepth = 0
		case 'a':
			opts.allFiles = true
		case 'k':
			// -k selects 1024-byte block units (the default). Per prd009-du R2.5.
			opts.unit = unitKilo
		case 'm':
			opts.unit = unitMega
		case 'c':
			opts.grandTotal = true
		case 'd':
			// -d requires a value: remaining chars in the flag group, or next arg.
			if j+1 < len(flags) {
				val := flags[j+1:]
				n, err := parseNonNegInt(val)
				if err != nil {
					return fmt.Errorf("invalid argument '%s' for '-d'", val)
				}
				opts.maxDepth = n
				opts.hasMaxDepth = true
				j = len(flags) - 1 // consume the rest of the flag group
			} else {
				*i++
				if *i >= len(args) {
					return fmt.Errorf("option requires an argument -- 'd'")
				}
				n, err := parseNonNegInt(args[*i])
				if err != nil {
					return fmt.Errorf("invalid argument '%s' for '-d'", args[*i])
				}
				opts.maxDepth = n
				opts.hasMaxDepth = true
			}
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}

// parseNonNegInt parses a non-negative decimal integer from s.
func parseNonNegInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not a non-negative integer: %s", s)
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}
