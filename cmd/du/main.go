// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du R1.1–R1.5, R2.1–R2.7.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "du"

// config holds parsed command-line options.
type config struct {
	summary    bool // R2.2: -s, print only total per argument
	human      bool // R2.1: -h, human-readable 1024-based output
	allFiles   bool // R2.3: -a, print entries for all files
	maxDepth   int  // R2.4: -d N / --max-depth=N, -1 = unlimited
	megaBlocks bool // R2.6: -m, report in 1M blocks
	grandTotal bool // R2.7: -c, print grand total
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run processes path arguments and returns the exit code.
// R1.1: defaults to "." when no arguments are given.
// R1.5: processes multiple arguments in order.
// R2.7: prints grand total when -c is given.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, paths := parseFlags(args, stderr)
	if len(paths) == 0 {
		paths = []string{"."}
	}
	exitCode := 0
	var cumulative int64
	for _, p := range paths {
		total, err := duPath(p, cfg, stdout, stderr)
		if err != nil {
			exitCode = 1
		}
		cumulative += total
	}
	if cfg.grandTotal {
		printEntry(stdout, cfg, cumulative, "total")
	}
	return exitCode
}

// parseFlags parses command-line flags and returns config and remaining paths.
func parseFlags(args []string, stderr io.Writer) (config, []string) {
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg config
	cfg.maxDepth = -1
	fs.BoolVar(&cfg.summary, "s", false, "display only a total for each argument")
	fs.BoolVar(&cfg.human, "h", false, "print sizes in human readable format")
	fs.BoolVar(&cfg.allFiles, "a", false, "write counts for all files")
	fs.IntVar(&cfg.maxDepth, "d", -1, "max depth of entries to display")
	fs.IntVar(&cfg.maxDepth, "max-depth", -1, "max depth of entries to display")
	// R2.5: -k is accepted for compatibility but has no visible effect.
	var kFlag bool
	fs.BoolVar(&kFlag, "k", false, "count in 1024-byte blocks (default)")
	fs.BoolVar(&cfg.megaBlocks, "m", false, "count in 1048576-byte blocks")
	fs.BoolVar(&cfg.grandTotal, "c", false, "produce a grand total")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	// R2.2: -s is equivalent to --max-depth=0.
	if cfg.summary {
		cfg.maxDepth = 0
	}
	return cfg, fs.Args()
}

// shouldPrint returns true if an entry at the given depth should be printed.
// R2.4: entries deeper than maxDepth are accumulated but not printed.
func shouldPrint(cfg config, depth int) bool {
	return cfg.maxDepth < 0 || depth <= cfg.maxDepth
}

// duPath handles a single path argument. Returns total blocks (512-byte).
// R1.4: uses Lstat to avoid following symbolic links.
func duPath(path string, cfg config, stdout, stderr io.Writer) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportErr(stderr, path, err)
		return 0, err
	}
	if !fi.Mode.IsDir() {
		printEntry(stdout, cfg, fi.Blocks, path)
		return fi.Blocks, nil
	}
	return walkDir(path, fi.Blocks, 0, cfg, stdout, stderr)
}

// walkDir recursively accumulates disk usage for a directory.
// Returns total in 512-byte blocks. R1.1: prints each subdirectory
// after its children (depth-first order) when within maxDepth.
func walkDir(dirPath string, dirBlocks int64, depth int, cfg config, stdout, stderr io.Writer) (int64, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportErr(stderr, dirPath, err)
		if shouldPrint(cfg, depth) {
			printEntry(stdout, cfg, dirBlocks, dirPath)
		}
		return dirBlocks, err
	}
	childBlocks, walkErr := processEntries(dirPath, entries, depth, cfg, stdout, stderr)
	total := dirBlocks + childBlocks
	if shouldPrint(cfg, depth) {
		printEntry(stdout, cfg, total, dirPath)
	}
	return total, walkErr
}

// processEntries iterates directory entries and accumulates block counts.
func processEntries(dirPath string, entries []os.DirEntry, depth int, cfg config, stdout, stderr io.Writer) (int64, error) {
	var total int64
	var hadErr error
	for _, entry := range entries {
		blocks, err := processOneEntry(dirPath, entry, depth, cfg, stdout, stderr)
		if err != nil {
			hadErr = err
		}
		total += blocks
	}
	return total, hadErr
}

// processOneEntry handles a single directory entry, returning its block count.
// R2.3: prints file entries when allFiles is enabled and within maxDepth.
func processOneEntry(dirPath string, entry os.DirEntry, parentDepth int, cfg config, stdout, stderr io.Writer) (int64, error) {
	entryPath := joinPath(dirPath, entry.Name())
	fi, err := sys.Lstat(entryPath) // R1.4: do not follow symlinks
	if err != nil {
		reportErr(stderr, entryPath, err)
		return 0, err
	}
	childDepth := parentDepth + 1
	if fi.Mode.IsDir() {
		return walkDir(entryPath, fi.Blocks, childDepth, cfg, stdout, stderr)
	}
	if cfg.allFiles && shouldPrint(cfg, childDepth) {
		printEntry(stdout, cfg, fi.Blocks, entryPath)
	}
	return fi.Blocks, nil
}

// humanSuffixes for binary (1024-based) mode, matching GNU du -h.
var humanSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// formatHuman formats bytes as a human-readable string matching GNU du -h.
// Uses 1024-based units with one decimal place for values with a suffix.
func formatHuman(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	value := float64(bytes)
	for i := 0; i < len(humanSuffixes)-1; i++ {
		if value < 1024.0 {
			if humanSuffixes[i] == "" {
				return fmt.Sprintf("%.0f", value)
			}
			return fmt.Sprintf("%.1f%s", value, humanSuffixes[i])
		}
		value /= 1024.0
	}
	return fmt.Sprintf("%.1f%s", value, humanSuffixes[len(humanSuffixes)-1])
}

// printEntry outputs one du line. R1.3: format is "SIZE\tPATH\n".
// R2.1: uses human-readable format when enabled.
// R2.6: uses 1M blocks when megaBlocks is enabled.
func printEntry(w io.Writer, cfg config, blocks512 int64, path string) {
	if cfg.human {
		fmt.Fprintf(w, "%s\t%s\n", formatHuman(blocks512*512), path)
		return
	}
	if cfg.megaBlocks {
		fmt.Fprintf(w, "%d\t%s\n", ceilDiv(blocks512, 2048), path)
		return
	}
	fmt.Fprintf(w, "%d\t%s\n", blocks512/2, path)
}

// ceilDiv returns the ceiling division of a by b.
// R2.6: used to convert 512-byte blocks to 1M blocks, rounding up.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

// joinPath joins a directory and entry name without cleaning the path,
// preserving prefixes like "./" that filepath.Join would remove.
func joinPath(dir, name string) string {
	return dir + string(os.PathSeparator) + name
}

// reportErr writes a diagnostic to stderr for a path error.
func reportErr(w io.Writer, path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(w, "%s: cannot access '%s': %s\n", progName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(w, "%s: %s: %s\n", progName, path, err)
}
