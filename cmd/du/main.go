// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/du: recursive directory disk usage reporting.
// Implements: prd009-du (R1–R5).
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is the du version string matching GNU coreutils format.
const version = "du (go-unix-utils) 0.1.0"

// sizeMode selects how sizes are computed and displayed.
type sizeMode int

const (
	modeBlocks1K sizeMode = iota // R1.2: 1024-byte blocks (default, also -k)
	modeBlocks1M                 // R2.6: 1048576-byte blocks
	modeHuman                    // R2.1: human-readable binary units
)

// duOptions holds the parsed flags for a du invocation.
type duOptions struct {
	summary      bool     // -s: print only total per argument (R2.2)
	allFiles     bool     // -a: print size for every file (R2.3)
	maxDepth     int      // -d N: limit depth of reported entries (R2.4), -1 = unlimited
	grandTotal   bool     // -c: print grand total after all arguments (R2.7)
	apparentSize bool     // --apparent-size: use st_size instead of st_blocks (R2.8)
	mode         sizeMode // size display mode
}

// inodeKey is the deduplication key for hard-link tracking. R3.1, R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	// R5.1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])
	exitCode := run(opts, args)
	os.Exit(exitCode)
}

// parseArgs parses GNU du-style flags from args. Returns options and remaining
// path arguments. Handles --, --help, --version, combined short flags, and
// --max-depth=N long form.
func parseArgs(args []string) (duOptions, []string) {
	opts := duOptions{maxDepth: -1}
	var paths []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if flagsDone {
			paths = append(paths, arg)
			continue
		}

		if arg == "--" {
			flagsDone = true
			continue
		}

		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}

		if arg == "--version" {
			fmt.Println(version)
			os.Exit(0)
		}

		if arg == "--apparent-size" {
			opts.apparentSize = true
			continue
		}

		if strings.HasPrefix(arg, "--max-depth=") {
			val := arg[len("--max-depth="):]
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", val)
				os.Exit(1)
			}
			opts.maxDepth = n
			continue
		}

		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			for j := 1; j < len(arg); j++ {
				ch := arg[j]
				switch ch {
				case 's':
					opts.summary = true
				case 'a':
					opts.allFiles = true
				case 'c':
					opts.grandTotal = true
				case 'h':
					opts.mode = modeHuman
				case 'k':
					opts.mode = modeBlocks1K
				case 'm':
					opts.mode = modeBlocks1M
				case 'd':
					// -d N: depth may be rest of arg or next arg
					rest := arg[j+1:]
					if rest != "" {
						n, err := strconv.Atoi(rest)
						if err != nil || n < 0 {
							fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", rest)
							os.Exit(1)
						}
						opts.maxDepth = n
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "du: option requires an argument -- 'd'\n")
							os.Exit(1)
						}
						n, err := strconv.Atoi(args[i])
						if err != nil || n < 0 {
							fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", args[i])
							os.Exit(1)
						}
						opts.maxDepth = n
					}
					j = len(arg) // consumed rest of arg
				default:
					fmt.Fprintf(os.Stderr, "du: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		paths = append(paths, arg)
	}

	// R2.2: -s is equivalent to --max-depth=0.
	if opts.summary {
		opts.maxDepth = 0
	}

	return opts, paths
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: du [OPTION]... [FILE]...
Summarize disk usage of the set of FILEs, recursively for directories.

  -a, --all             write counts for all files, not just directories
  -c, --total           produce a grand total
  -d, --max-depth=N     print the total for a directory only if it is N or
                          fewer levels below the command line argument
  -h, --human-readable  print sizes in human readable format (e.g., 1K 234M 2G)
  -k                    like --block-size=1K
  -m                    like --block-size=1M
  -s, --summarize       display only a total for each argument
      --apparent-size   print apparent sizes, rather than disk usage
      --help            display this help and exit
      --version         output version information and exit
`)
}

// run processes all path arguments with the given options. Returns the exit code.
func run(opts duOptions, paths []string) int {
	// R1.1: default to current directory when no arguments given.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	seen := make(map[inodeKey]bool) // R3.1: hard-link deduplication across invocation
	exitCode := 0
	var grandTotal int64

	for _, p := range paths {
		total, err := walkPath(opts, p, 0, seen)
		if err != 0 {
			exitCode = 1
		}
		grandTotal += total
	}

	// R2.7: grand total line.
	if opts.grandTotal {
		printEntry(opts, grandTotal, "total")
	}

	return exitCode
}

// walkPath recursively traverses a path, printing entries and returning the
// accumulated size in raw bytes (512-byte block equivalents or apparent bytes).
// depth is the current recursion depth relative to the argument (0 = the argument itself).
func walkPath(opts duOptions, path string, depth int, seen map[inodeKey]bool) (int64, int) {
	// R1.4: use Lstat to avoid following symlinks.
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access '%s': %s\n", path, extractSyscallError(err))
		return 0, 1
	}

	// Symlinks: report their own size but do not follow.
	if fi.Mode&os.ModeSymlink != 0 {
		size := entrySize(fi, opts.apparentSize)
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		if seen[key] {
			size = 0
		} else {
			seen[key] = true
		}
		// Print if -a or if this is a top-level argument (depth 0).
		if (opts.allFiles || depth == 0) && (opts.maxDepth < 0 || depth <= opts.maxDepth) {
			printEntry(opts, size, path)
		}
		return size, 0
	}

	if !fi.Mode.IsDir() {
		// Regular file (or special file).
		size := entrySize(fi, opts.apparentSize)
		// R3.1: hard-link deduplication.
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		if seen[key] {
			size = 0
		} else {
			seen[key] = true
		}
		// R2.3: -a prints individual files; top-level arguments always print.
		if (opts.allFiles || depth == 0) && (opts.maxDepth < 0 || depth <= opts.maxDepth) {
			printEntry(opts, size, path)
		}
		return size, 0
	}

	// Directory: recurse into children using filesystem order (not sorted).
	// GNU du uses readdir() order; Go's os.ReadDir sorts alphabetically.
	dir, err2 := os.Open(path)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %s\n", path, extractSyscallError(err2))
		dirSize := entrySize(fi, opts.apparentSize)
		if opts.maxDepth < 0 || depth <= opts.maxDepth {
			printEntry(opts, dirSize, path)
		}
		return dirSize, 1
	}
	names, readErr := dir.Readdirnames(-1)
	dir.Close()
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %s\n", path, extractSyscallError(readErr))
		dirSize := entrySize(fi, opts.apparentSize)
		if opts.maxDepth < 0 || depth <= opts.maxDepth {
			printEntry(opts, dirSize, path)
		}
		return dirSize, 1
	}

	exitCode := 0
	var total int64

	for _, name := range names {
		childPath := filepath.Join(path, name)
		childSize, childErr := walkPath(opts, childPath, depth+1, seen)
		total += childSize
		if childErr != 0 {
			exitCode = 1
		}
	}

	// Add the directory's own block allocation.
	dirSize := entrySize(fi, opts.apparentSize)
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if !seen[key] {
		seen[key] = true
		total += dirSize
	}

	// Print directory entry if within depth limit.
	if opts.maxDepth < 0 || depth <= opts.maxDepth {
		printEntry(opts, total, path)
	}

	return total, exitCode
}

// entrySize returns the size in 512-byte block units for a file entry.
// R2.8: --apparent-size converts st_size to 512-byte blocks (ceiling);
// default uses st_blocks directly. For directories, uses st_blocks in both
// modes to match GNU du behavior (directories' apparent sizes are not counted
// when st_blocks is 0, as on APFS).
func entrySize(fi *sys.FileInfo, apparentSize bool) int64 {
	if apparentSize && !fi.Mode.IsDir() {
		// Convert apparent size (bytes) to 512-byte blocks, ceiling.
		// Matches GNU du: ST_NBLOCKSIZE = 512, (size + 511) / 512.
		return (fi.Size + 511) / 512
	}
	return fi.Blocks
}

// printEntry formats and prints a single "SIZE\tPATH\n" line. R1.3.
// size512 is the size in 512-byte block units.
func printEntry(opts duOptions, size512 int64, path string) {
	var sizeStr string
	switch opts.mode {
	case modeHuman:
		// R2.1: human-readable binary mode (1024-based units).
		sizeStr = humanReadable(size512 * 512)
	case modeBlocks1M:
		// R2.6: 1M blocks. 1M = 2048 512-byte blocks. Integer division, round up.
		blocks := (size512 + 2047) / 2048
		sizeStr = strconv.FormatInt(blocks, 10)
	default:
		// R1.2, R2.5: 1K blocks. 1K = 2 512-byte blocks. Integer division, round up.
		blocks := (size512 + 1) / 2
		sizeStr = strconv.FormatInt(blocks, 10)
	}
	fmt.Printf("%s\t%s\n", sizeStr, path)
}

// humanSuffixes are 1024-based unit labels matching GNU coreutils du -h.
var humanSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// humanReadable formats a byte count as a human-readable string matching GNU du -h.
// R2.1: uses 1024-based units. GNU formatting rules:
//   - values < 10: one decimal place (e.g., "4.0K")
//   - values >= 10: no decimal place (e.g., "12K")
func humanReadable(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	val := float64(bytes)
	for i := 0; i < len(humanSuffixes)-1; i++ {
		if val < 1024 {
			return formatHuman(val, humanSuffixes[i])
		}
		val /= 1024
	}
	return formatHuman(val, humanSuffixes[len(humanSuffixes)-1])
}

// formatHuman renders a value with a suffix using GNU formatting rules.
func formatHuman(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	if val < 10 {
		return fmt.Sprintf("%.1f%s", val, suffix)
	}
	return fmt.Sprintf("%.0f%s", val, suffix)
}

// extractSyscallError extracts the innermost syscall error string from an error
// chain, matching the format GNU du uses (e.g., "No such file or directory").
// Capitalizes the first letter to match GNU strerror() output.
func extractSyscallError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst returns s with the first byte uppercased. GNU coreutils uses
// strerror() which capitalizes error messages; Go's syscall errors are lowercase.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
