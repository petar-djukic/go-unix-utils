// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du reports recursive directory disk usage, matching GNU du output
// format under LC_ALL=C.
//
// Implements prd009-du R1, R2, R3, R4, R5.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "du"

// sizeMode determines how output sizes are formatted.
type sizeMode int

const (
	sizeKiloBlocks  sizeMode = iota // default and -k: 1024-byte blocks
	sizeMegaBlocks                  // -m: 1048576-byte blocks
	sizeHumanBinary                 // -h: human-readable 1024-based
	sizeHumanSI                     // --si: human-readable 1000-based
	sizeCustomBlock                 // --block-size=N
)

type config struct {
	allFiles     bool     // -a / --all
	summarize    bool     // -s / --summarize
	grandTotal   bool     // -c / --total
	apparentSize bool     // -b / --apparent-size
	derefLinks   bool     // -L / --dereference
	nullTerm     bool     // -0 / --null
	maxDepth     int      // -d N / --max-depth=N; -1 means unlimited
	mode         sizeMode // output size mode
	blockSize    int64    // custom block size for sizeCustomBlock
}

// inodeKey uniquely identifies a file for hard-link deduplication (prd009-du R3.2).
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	sys.InstallSIGPIPEHandler() // prd009-du R5.1

	cfg, args := parseArgs(os.Args[1:])

	// -s is equivalent to --max-depth=0 (prd009-du R2.2).
	if cfg.summarize && cfg.maxDepth < 0 {
		cfg.maxDepth = 0
	}

	// Default to current directory (prd009-du R1.1).
	if len(args) == 0 {
		args = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	seen := make(map[inodeKey]bool)
	exitCode := 0
	var grandTotal int64

	// Process each argument in order (prd009-du R1.5).
	for _, arg := range args {
		total, hasErr := walkPath(out, cfg, arg, 0, seen)
		if hasErr {
			exitCode = 1
		}
		grandTotal += total
	}

	// Grand total line when -c is given (prd009-du R2.7).
	if cfg.grandTotal {
		printLine(out, cfg, grandTotal, "total")
	}

	if err := out.Flush(); err != nil {
		os.Exit(1) // prd009-du R4
	}
	os.Exit(exitCode)
}

// parseArgs parses GNU-style flags including combined short flags (-sh)
// and long flags with = or space-separated values (--max-depth=3).
// Follows the manual parsing pattern from cmd/wc and cmd/cat.
func parseArgs(args []string) (config, []string) {
	cfg := config{maxDepth: -1}
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}

		if strings.HasPrefix(arg, "--") {
			parseLongFlag(&cfg, arg[2:], args, &i)
			continue
		}

		// Short flags may be combined: -sh means -s -h.
		parseShortFlags(&cfg, arg[1:], args, &i)
	}

	return cfg, files
}

// parseLongFlag handles a single long flag (after stripping the -- prefix).
func parseLongFlag(cfg *config, raw string, args []string, i *int) {
	name, value, hasValue := strings.Cut(raw, "=")
	switch name {
	case "all":
		cfg.allFiles = true
	case "summarize":
		cfg.summarize = true
	case "total":
		cfg.grandTotal = true
	case "human-readable":
		cfg.mode = sizeHumanBinary
	case "si":
		cfg.mode = sizeHumanSI
	case "apparent-size":
		cfg.apparentSize = true
	case "max-depth":
		if hasValue {
			cfg.maxDepth = parseDepth(value)
		} else if *i+1 < len(args) {
			*i++
			cfg.maxDepth = parseDepth(args[*i])
		}
	case "block-size":
		if hasValue {
			cfg.blockSize = parseBlockSize(value)
		} else if *i+1 < len(args) {
			*i++
			cfg.blockSize = parseBlockSize(args[*i])
		}
		cfg.mode = sizeCustomBlock
	case "dereference":
		cfg.derefLinks = true
	case "null":
		cfg.nullTerm = true
	default:
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '--%s'\n", progName, name)
		os.Exit(1)
	}
}

// parseShortFlags processes combined short flags. When a flag requires a
// value (-d), the rest of the flag string is consumed as the value; if
// empty, the next argument is consumed.
func parseShortFlags(cfg *config, flags string, args []string, i *int) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			cfg.allFiles = true
		case 's':
			cfg.summarize = true
		case 'c':
			cfg.grandTotal = true
		case 'h':
			cfg.mode = sizeHumanBinary
		case 'k':
			cfg.mode = sizeKiloBlocks
		case 'm':
			cfg.mode = sizeMegaBlocks
		case 'b':
			cfg.apparentSize = true
		case 'L':
			cfg.derefLinks = true
		case 'P':
			cfg.derefLinks = false
		case '0':
			cfg.nullTerm = true
		case 'd':
			rest := flags[j+1:]
			if rest != "" {
				cfg.maxDepth = parseDepth(rest)
			} else if *i+1 < len(args) {
				*i++
				cfg.maxDepth = parseDepth(args[*i])
			}
			return // -d consumes the rest of the flag string
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
			os.Exit(1)
		}
	}
}

// parseDepth parses a non-negative integer for --max-depth.
func parseDepth(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid maximum depth '%s'\n", progName, s)
		os.Exit(1)
	}
	return n
}

// parseBlockSize parses a positive integer for --block-size.
func parseBlockSize(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid --block-size argument '%s'\n", progName, s)
		os.Exit(1)
	}
	return n
}

// walkPath traverses a path and accumulates disk usage. For directories, it
// recurses into children and sums their sizes. Returns the total raw byte
// count and whether any error occurred during traversal.
func walkPath(out *bufio.Writer, cfg config, path string, depth int, seen map[inodeKey]bool) (int64, bool) {
	fi, err := statPath(cfg, path)
	if err != nil {
		printStatError(path, err)
		return 0, true
	}

	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}

	// Non-directory entry.
	if !fi.Mode.IsDir() {
		size := rawSize(cfg, fi)
		// Hard-link deduplication: count each inode once (prd009-du R3.1).
		if seen[key] {
			size = 0
		} else {
			seen[key] = true
		}
		if shouldPrint(cfg, depth, false) {
			printLine(out, cfg, size, path)
		}
		return size, false
	}

	// Directory: detect cycles via seen set (relevant with -L).
	if seen[key] {
		return 0, false
	}
	seen[key] = true

	size := rawSize(cfg, fi)

	entries, err := os.ReadDir(path)
	if err != nil {
		printReadError(path, err)
		if shouldPrint(cfg, depth, true) {
			printLine(out, cfg, size, path)
		}
		return size, true
	}

	hasErr := false
	for _, entry := range entries {
		childPath := joinPath(path, entry.Name())
		childSize, childErr := walkPath(out, cfg, childPath, depth+1, seen)
		size += childSize
		if childErr {
			hasErr = true
		}
	}

	// Directories are printed after their children (post-order, prd009-du R1.1).
	if shouldPrint(cfg, depth, true) {
		printLine(out, cfg, size, path)
	}

	return size, hasErr
}

// statPath returns file metadata using pkg/sys.Lstat (default) or
// pkg/sys.Stat when -L is active (prd009-du R1.4, design decision D2).
func statPath(cfg config, path string) (*sys.FileInfo, error) {
	if cfg.derefLinks {
		return sys.Stat(path)
	}
	return sys.Lstat(path)
}

// rawSize returns the raw byte count for a file entry. Uses fi.Blocks * 512
// for disk usage or fi.Size for apparent size (prd009-du R1.2, R2.8).
func rawSize(cfg config, fi *sys.FileInfo) int64 {
	if cfg.apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// shouldPrint reports whether an entry at the given depth should produce
// output. Directories are always printed within the depth limit. Files
// require -a or must be a top-level argument (depth 0).
func shouldPrint(cfg config, depth int, isDir bool) bool {
	if cfg.maxDepth >= 0 && depth > cfg.maxDepth {
		return false
	}
	return isDir || cfg.allFiles || depth == 0
}

// formatSize converts raw bytes to the output string per the configured
// size mode (prd009-du R1.2, R2.1, R2.5, R2.6).
func formatSize(cfg config, bytes int64) string {
	switch cfg.mode {
	case sizeHumanBinary:
		return format.HumanSize(bytes, format.HumanSizeOpts{Binary: true})
	case sizeHumanSI:
		return format.HumanSize(bytes, format.HumanSizeOpts{Binary: false})
	case sizeMegaBlocks:
		return strconv.FormatInt(ceilDiv(bytes, 1048576), 10)
	case sizeCustomBlock:
		return strconv.FormatInt(ceilDiv(bytes, cfg.blockSize), 10)
	default: // sizeKiloBlocks
		return strconv.FormatInt(ceilDiv(bytes, 1024), 10)
	}
}

// ceilDiv returns the ceiling of a / b for positive values.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// printLine writes one output line: SIZE<tab>PATH<terminator>.
func printLine(out *bufio.Writer, cfg config, bytes int64, path string) {
	size := formatSize(cfg, bytes)
	fmt.Fprintf(out, "%s\t%s", size, path)
	if cfg.nullTerm {
		out.WriteByte(0)
	} else {
		out.WriteByte('\n')
	}
}

// joinPath concatenates directory and entry name, preserving the original
// path format (e.g., "./sub" for "." arguments).
func joinPath(dir, name string) string {
	if len(dir) > 0 && dir[len(dir)-1] == '/' {
		return dir + name
	}
	return dir + "/" + name
}

// printStatError reports a stat error to stderr (prd009-du R4.2).
func printStatError(path string, err error) {
	fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, extractSysError(err))
}

// printReadError reports a directory read error to stderr (prd009-du R4.2).
func printReadError(path string, err error) {
	fmt.Fprintf(os.Stderr, "%s: cannot read directory '%s': %v\n", progName, path, extractSysError(err))
}

// extractSysError extracts the underlying system error from an os.PathError,
// removing the operation and path prefix that Go adds.
func extractSysError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
