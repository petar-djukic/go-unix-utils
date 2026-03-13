// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du reports disk usage for files and directory trees.
//
// Implements: prd009-du R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R2.4, R2.5, R2.6, R2.7, R2.8, R3.1-R3.3, R4.1, R4.2, R5.1
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// inodeKey uniquely identifies a file by device and inode number.
// Used for hard-link deduplication per prd009-du R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

// config holds the parsed flag values for the du invocation.
type config struct {
	humanReadable bool // -h: human-readable output via pkg/format.HumanSize
	allFiles      bool // -a: print a size line for every file, not just directories
	megaBlocks    bool // -m: report sizes in 1048576-byte (1M) blocks
	grandTotal    bool // -c: print a grand total line after all arguments
	apparentSize  bool // --apparent-size: use st_size instead of st_blocks (R2.8)
	maxDepth      int  // -d N / --max-depth=N: max reported depth; -1 means unlimited
}

// fileBytes returns the size contribution of fi in bytes.
//
// R2.8: --apparent-size uses fi.Size (st_size) for non-directory files. Directories
// contribute 0 in apparent-size mode, matching GNU du behavior on macOS APFS where
// directory metadata is stored in filesystem B-tree structures rather than data forks.
// R1.2: default uses fi.Blocks (st_blocks, 512-byte block count) converted to bytes.
func (c *config) fileBytes(fi *sys.FileInfo) int64 {
	if c.apparentSize {
		if fi.Mode.IsDir() {
			return 0
		}
		return fi.Size
	}
	return fi.Blocks * 512
}

// formatSize converts a byte count to the configured output format.
//
// R2.1: -h uses pkg/format.HumanSize with binary (1024-base) mode.
// R2.6: -m reports sizes in 1M blocks, rounding up.
// R1.2: default output is 1K blocks.
func (c *config) formatSize(bytes int64) string {
	if c.humanReadable {
		// R2.1: format bytes as human-readable binary.
		return format.HumanSize(bytes, format.HumanSizeOpts{Binary: true})
	}
	if c.megaBlocks {
		// R2.6: convert bytes to 1048576-byte (1M) blocks, rounding up.
		return fmt.Sprintf("%d", ceilDiv(bytes, 1048576))
	}
	// R1.2: convert bytes to 1024-byte (1K) blocks, rounding up.
	return fmt.Sprintf("%d", ceilDiv(bytes, 1024))
}

func main() {
	// R5.1: install SIGPIPE handler so piping to head or similar exits cleanly.
	sys.InstallSIGPIPEHandler()

	// R2.5: -k is accepted without error; 1K blocks is already the default.
	var flagK bool
	flag.BoolVar(&flagK, "k", false, "use 1024-byte (1K) block size (default)")

	// R2.2: -s prints only a total for each argument; normalized to maxDepth=0 below.
	var flagS bool
	flag.BoolVar(&flagS, "s", false, "display only a total for each argument")

	cfg := &config{maxDepth: -1}

	// R2.1: -h displays sizes as human-readable strings.
	flag.BoolVar(&cfg.humanReadable, "h", false, "print sizes in human-readable format")
	// R2.3: -a prints a size line for every file, not just directories.
	flag.BoolVar(&cfg.allFiles, "a", false, "write count for all files, not just directories")
	// R2.4: -d N / --max-depth=N limits reported depth; -1 means unlimited.
	flag.IntVar(&cfg.maxDepth, "d", -1, "print the total for a directory only if it is N or fewer levels below the command line argument")
	flag.IntVar(&cfg.maxDepth, "max-depth", -1, "print the total for a directory only if it is N or fewer levels below the command line argument")
	// R2.6: -m reports sizes in 1M blocks.
	flag.BoolVar(&cfg.megaBlocks, "m", false, "use 1048576-byte (1M) block size")
	// R2.7: -c prints a grand total after all arguments.
	flag.BoolVar(&cfg.grandTotal, "c", false, "produce a grand total")
	// R2.8: --apparent-size reports st_size instead of st_blocks.
	flag.BoolVar(&cfg.apparentSize, "apparent-size", false, "print apparent sizes rather than disk usage")

	flag.Parse()

	// R2.2: -s is equivalent to --max-depth=0.
	if flagS {
		cfg.maxDepth = 0
	}

	args := flag.Args()
	// R1.1: default to current directory when no arguments are given.
	if len(args) == 0 {
		args = []string{"."}
	}

	// R3.3: seen map is shared across all arguments for cross-argument deduplication.
	seen := make(map[inodeKey]struct{})
	exitCode := 0
	var grandTotalBytes int64

	// R1.5: process multiple directory arguments in the order given on the command line.
	for _, arg := range args {
		// R4.2: print error and continue processing remaining arguments on failure.
		total, err := runArg(arg, seen, cfg)
		grandTotalBytes += total
		if err != nil {
			fmt.Fprintf(os.Stderr, "du: %v\n", err)
			exitCode = 1
		}
	}

	// R2.7: print grand total line after all arguments when -c is given.
	if cfg.grandTotal {
		fmt.Printf("%s\ttotal\n", cfg.formatSize(grandTotalBytes))
	}

	os.Exit(exitCode)
}

// runArg processes one command-line argument (file or directory).
// Returns the total size in bytes for the argument and any traversal error.
func runArg(path string, seen map[inodeKey]struct{}, cfg *config) (int64, error) {
	// R1.4: use Lstat so symbolic links are not followed.
	fi, err := sys.Lstat(path)
	if err != nil {
		return 0, err
	}

	if !fi.Mode.IsDir() {
		// Single file argument: count and print its size.
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		if _, dup := seen[key]; dup {
			// R3.1, R3.3: already counted; skip entirely matching GNU du behavior.
			return 0, nil
		}
		seen[key] = struct{}{}
		sizeBytes := cfg.fileBytes(fi)
		// R1.3: "SIZE\tPATH\n" format.
		fmt.Printf("%s\t%s\n", cfg.formatSize(sizeBytes), path)
		return sizeBytes, nil
	}

	// R3.3: if this directory root was already counted by a previous argument, skip it
	// entirely (no output line, no contribution to grand total), matching GNU du behavior.
	rootKey := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if _, dup := seen[rootKey]; dup {
		return 0, nil
	}

	// Directory: recurse with depth 0 (the argument itself).
	// R2.4: walkDir uses depth to decide which children to print.
	total, ok := walkDir(path, seen, cfg, 0)
	fmt.Printf("%s\t%s\n", cfg.formatSize(total), path)
	if !ok {
		return total, fmt.Errorf("errors encountered while traversing %q", path)
	}
	return total, nil
}

// walkDir recursively traverses dir at the given depth, printing entries according
// to cfg, and returns the total size in bytes for dir and all its contents.
//
// R1.1: recursive directory traversal with accumulated usage.
// R1.4: sys.Lstat does not follow symbolic links.
// R2.4: entries deeper than cfg.maxDepth are accumulated into their parent but not printed.
// R2.3: -a prints a size line for every file encountered (subject to depth limit).
// R3.1-R3.3: hard-link deduplication via shared seen map.
func walkDir(dir string, seen map[inodeKey]struct{}, cfg *config, depth int) (total int64, ok bool) {
	// R1.4: Lstat — does not follow symlinks.
	fi, err := sys.Lstat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access %q: %v\n", dir, err)
		return 0, false
	}

	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if _, dup := seen[key]; dup {
		// R3.1: directory already counted; skip to avoid double-counting hard links.
		return 0, true
	}
	seen[key] = struct{}{}

	// Include the directory inode's own size contribution.
	total = cfg.fileBytes(fi)
	ok = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory %q: %v\n", dir, err)
		return total, false
	}

	for _, entry := range entries {
		childPath := filepath.Join(dir, entry.Name())
		childDepth := depth + 1
		if entry.IsDir() {
			// Recurse into subdirectory; walkDir handles its own dedup.
			subtotal, childOK := walkDir(childPath, seen, cfg, childDepth)
			if !childOK {
				ok = false
			}
			total += subtotal
			// R2.4: print subdirectory line only if within maxDepth (or unlimited).
			// R1.1, R1.3: printed in post-order (after all its children).
			if cfg.maxDepth < 0 || childDepth <= cfg.maxDepth {
				fmt.Printf("%s\t%s\n", cfg.formatSize(subtotal), childPath)
			}
		} else {
			// File or symlink: count blocks with deduplication.
			// R1.4: Lstat does not follow the symlink.
			childFi, lstatErr := sys.Lstat(childPath)
			if lstatErr != nil {
				fmt.Fprintf(os.Stderr, "du: cannot access %q: %v\n", childPath, lstatErr)
				ok = false
				continue
			}
			childKey := inodeKey{Dev: childFi.Dev, Ino: childFi.Ino}
			if _, dup := seen[childKey]; dup {
				// R3.1: already counted; skip entirely (no output, no contribution).
				continue
			}
			seen[childKey] = struct{}{}
			childBytes := cfg.fileBytes(childFi)
			total += childBytes
			// R2.3: -a prints a size line for every file encountered.
			// R2.4: only print within maxDepth (or unlimited).
			if cfg.allFiles && (cfg.maxDepth < 0 || childDepth <= cfg.maxDepth) {
				fmt.Printf("%s\t%s\n", cfg.formatSize(childBytes), childPath)
			}
		}
	}

	return total, ok
}

// ceilDiv returns the ceiling of a / b for non-negative a and positive b.
// Used to convert byte counts to block units with rounding up.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}
