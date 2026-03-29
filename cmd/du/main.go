// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements prd009 R1.1-R1.5, R2.1-R2.8, R3.1-R3.3, R4.1, R4.2, R5.1.
//
// TODO: prd009 R3.2 -H (--dereference-args) and -L (--dereference) are listed
// in non_goals. Default no-follow-symlink behavior (Lstat) satisfies R1.4.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "du"

// duOptions holds command-line flag values for du.
type duOptions struct {
	blockSize     int64
	humanReadable bool
	apparentSize  bool
	maxDepth      int
	maxDepthSet   bool
	total         bool
	threshold     int64
	thresholdSet  bool
	oneFileSystem bool
}

func main() {
	// R5.1: SIGPIPE handler for piped output to head/grep -q
	sys.InstallSIGPIPEHandler()

	opts := parseFlags()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	seen := make(map[uint64]map[uint64]bool)
	exitCode := 0
	var grandTotal int64

	// R1.5: process multiple arguments in order
	for _, arg := range args {
		argBytes, hasErr := duWalk(arg, 0, seen, opts, 0)
		if hasErr {
			exitCode = 1
		}
		grandTotal += argBytes
	}

	// R2.7: grand total line
	if opts.total {
		fmt.Printf("%s\ttotal\n", formatSize(grandTotal, opts))
	}

	// R4.1, R4.2: exit 0 on success, exit 1 on any traversal error
	os.Exit(exitCode)
}

// parseFlags parses du command-line flags and returns options.
func parseFlags() duOptions {
	opts := duOptions{blockSize: 1024, maxDepth: -1}
	var kFlag, mFlag, summarize, showVersion bool
	var thresholdStr string

	registerSizeFlags(&opts, &kFlag, &mFlag)
	registerDisplayFlags(&opts, &summarize)
	registerFilterFlags(&opts, &thresholdStr)
	flag.BoolVar(&showVersion, "version", false, "")

	// R4.2: --help prints usage to stdout and exits 0
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = func() {
		printUsage()
	}

	flag.Parse()

	// R4.2: --version prints version info to stdout and exits 0
	if showVersion {
		printVersion()
		os.Exit(0)
	}

	applyFlagDefaults(&opts, mFlag, summarize, thresholdStr)
	return opts
}

// printUsage writes the usage synopsis and flag descriptions to stdout.
func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(os.Stdout, "Summarize device usage of the set of FILEs, recursively for directories.")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "  -a, --all             write counts for all files, not just directories")
	fmt.Fprintln(os.Stdout, "      --apparent-size    print apparent sizes rather than device usage")
	fmt.Fprintln(os.Stdout, "  -c, --total            produce a grand total")
	fmt.Fprintln(os.Stdout, "  -d, --max-depth=N      print the total for a directory only if it is N or")
	fmt.Fprintln(os.Stdout, "                           fewer levels below the command line argument")
	fmt.Fprintln(os.Stdout, "  -h, --human-readable   print sizes in human readable format (e.g., 1K 234M 2G)")
	fmt.Fprintln(os.Stdout, "  -k                     like --block-size=1K")
	fmt.Fprintln(os.Stdout, "  -m                     like --block-size=1M")
	fmt.Fprintln(os.Stdout, "  -s, --summarize        display only a total for each argument")
	fmt.Fprintln(os.Stdout, "  -t, --threshold=SIZE   exclude entries smaller than SIZE if positive,")
	fmt.Fprintln(os.Stdout, "                           or entries greater than SIZE if negative")
	fmt.Fprintln(os.Stdout, "  -x, --one-file-system  skip directories on different file systems")
	fmt.Fprintln(os.Stdout, "      --version          output version information and exit")
	fmt.Fprintln(os.Stdout, "      --help             display this help and exit")
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s (go-unix-utils) 0.1\n", progName)
}

// registerSizeFlags registers size-related flags.
func registerSizeFlags(opts *duOptions, kFlag, mFlag *bool) {
	flag.BoolVar(kFlag, "k", false, "display sizes in 1024-byte blocks")
	flag.BoolVar(mFlag, "m", false, "display sizes in 1048576-byte blocks")
	flag.BoolVar(&opts.humanReadable, "h", false, "human-readable output")
	flag.BoolVar(&opts.humanReadable, "human-readable", false, "human-readable output")
	flag.BoolVar(&opts.apparentSize, "apparent-size", false, "print apparent sizes")
}

// registerDisplayFlags registers display-related flags.
func registerDisplayFlags(opts *duOptions, summarize *bool) {
	flag.BoolVar(summarize, "s", false, "display only a total for each argument")
	flag.BoolVar(summarize, "summarize", false, "display only a total for each argument")
	flag.BoolVar(&opts.total, "c", false, "produce a grand total")
	flag.BoolVar(&opts.total, "total", false, "produce a grand total")
	flag.IntVar(&opts.maxDepth, "d", -1, "max display depth")
	flag.IntVar(&opts.maxDepth, "max-depth", -1, "max display depth")
}

// registerFilterFlags registers filtering-related flags.
func registerFilterFlags(opts *duOptions, thresholdStr *string) {
	flag.StringVar(thresholdStr, "t", "", "size threshold")
	flag.StringVar(thresholdStr, "threshold", "", "size threshold")
	flag.BoolVar(&opts.oneFileSystem, "x", false, "skip directories on different file systems")
	flag.BoolVar(&opts.oneFileSystem, "one-file-system", false, "skip directories on different file systems")
}

// applyFlagDefaults resolves flag interactions after parsing.
func applyFlagDefaults(opts *duOptions, mFlag, summarize bool, thresholdStr string) {
	if mFlag {
		opts.blockSize = 1048576
	}
	if opts.maxDepth >= 0 {
		opts.maxDepthSet = true
	}
	// R2.2: -s is equivalent to --max-depth=0
	if summarize {
		opts.maxDepth = 0
		opts.maxDepthSet = true
	}
	if thresholdStr != "" {
		parseThreshold(opts, thresholdStr)
	}
}

// parseThreshold parses the -t/--threshold value and sets options.
func parseThreshold(opts *duOptions, s string) {
	val, err := sizeparse.ParseWithOptions(s, sizeparse.ParseOptions{AllowSign: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid threshold '%s': %v\n", progName, s, err)
		os.Exit(1)
	}
	opts.threshold = val
	opts.thresholdSet = true
}

// fileRawBytes returns the size of a file in bytes.
// R2.8: --apparent-size uses fi.Size; default uses fi.Blocks * 512.
func fileRawBytes(fi *sys.FileInfo, apparentSize bool) int64 {
	if apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// formatSize formats a raw byte count for display.
// R2.1: -h uses format.HumanSize with Binary=true.
// R2.5, R2.6: otherwise divides by blockSize with ceiling.
func formatSize(rawBytes int64, opts duOptions) string {
	if opts.humanReadable {
		return format.HumanSize(rawBytes, format.HumanSizeOpts{Binary: true})
	}
	return fmt.Sprintf("%d", ceilDiv(rawBytes, opts.blockSize))
}

// ceilDiv returns the ceiling of a/b for positive a.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// shouldPrint checks whether an entry should be printed based on depth and threshold.
// R2.4: entries deeper than maxDepth are accumulated but not printed.
func shouldPrint(rawBytes int64, depth int, opts duOptions) bool {
	if opts.maxDepthSet && depth > opts.maxDepth {
		return false
	}
	if opts.thresholdSet {
		return passesThreshold(rawBytes, opts.threshold)
	}
	return true
}

// passesThreshold checks if a size passes the threshold filter.
// Positive threshold excludes entries smaller; negative excludes entries larger.
func passesThreshold(rawBytes, threshold int64) bool {
	if threshold >= 0 {
		return rawBytes >= threshold
	}
	return rawBytes <= -threshold
}

// duWalk recursively computes disk usage for path at the given depth.
// Returns total raw bytes and whether any error occurred.
// rootDev is the device ID of the starting argument for --one-file-system;
// it is set automatically on the first call (depth 0).
// R1.4: uses sys.Lstat to avoid following symbolic links.
func duWalk(path string, depth int, seen map[uint64]map[uint64]bool, opts duOptions, rootDev uint64) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		// R4.2: diagnostic to stderr, skip entry, continue processing
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, err)
		return 0, true
	}

	// D1: capture root device from starting argument for -x
	if depth == 0 {
		rootDev = fi.Dev
	}

	// -x/--one-file-system: skip entries on different filesystems
	if opts.oneFileSystem && fi.Dev != rootDev {
		return 0, false
	}

	if isDuplicate(fi, seen) {
		return 0, false
	}

	rawBytes := fileRawBytes(fi, opts.apparentSize)
	if !fi.Mode.IsDir() {
		return rawBytes, false
	}

	childBytes, hasErr := walkChildren(path, depth, seen, opts, rootDev)
	rawBytes += childBytes

	// R1.3: SIZE\tPATH\n, filtered by depth and threshold
	if shouldPrint(rawBytes, depth, opts) {
		fmt.Printf("%s\t%s\n", formatSize(rawBytes, opts), path)
	}
	return rawBytes, hasErr
}

// isDuplicate checks whether a file has already been counted.
// R3.1: tracks by dev+ino across the entire invocation.
// R3.3: deduplication is per invocation, not per argument.
func isDuplicate(fi *sys.FileInfo, seen map[uint64]map[uint64]bool) bool {
	if fi.Mode.IsDir() || fi.Nlink <= 1 {
		return false
	}
	inodes, ok := seen[fi.Dev]
	if !ok {
		inodes = make(map[uint64]bool)
		seen[fi.Dev] = inodes
	}
	if inodes[fi.Ino] {
		return true
	}
	inodes[fi.Ino] = true
	return false
}

// walkChildren reads directory entries and accumulates their disk usage.
// R4.2: errors reading directory contents produce stderr diagnostics.
func walkChildren(dir string, depth int, seen map[uint64]map[uint64]bool, opts duOptions, rootDev uint64) (int64, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot read directory '%s': %v\n", progName, dir, err)
		return 0, true
	}

	var total int64
	hasErr := false
	for _, e := range entries {
		childPath := dir + "/" + e.Name()
		childBytes, childErr := duWalk(childPath, depth+1, seen, opts, rootDev)
		total += childBytes
		if childErr {
			hasErr = true
		}
	}
	return total, hasErr
}
