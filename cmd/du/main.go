// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du: Recursive Directory Disk Usage.
// Covers R1.1-R1.5 (default traversal, block counting, output format,
// symlink handling, multiple arguments),
// R2.1 (-h/--human-readable), R2.2 (-s/--summarize), R2.3 (-a/--all),
// R2.4 (-d/--max-depth), R2.5 (-k), R2.6 (-m), R2.7 (-c/--total),
// R3.1-R3.3 (hard-link deduplication via device+inode tracking),
// R4.1-R4.2 (exit codes with error continuation),
// R5.1 (SIGPIPE handler installation).
//
// TODO: prd009 R2.2 (--si) is a non_goal per PRD non_goals: "--si (1000-based units) is out of scope".
// TODO: prd009 R2.4 (-B/--block-size=SIZE) is a non_goal per PRD non_goals: "--block-size=SIZE with arbitrary unit suffixes is out of scope; only -k and -m are supported".
// TODO: prd009 R2.8 (--apparent-size) not in this task scope.
// TODO: prd009 -L/--dereference is a non_goal per PRD non_goals: "-L / --dereference (dereference all symlinks during traversal) is out of scope".
// TODO: prd009 --exclude is a non_goal per PRD non_goals: "--exclude and --exclude-from pattern-based exclusion filters are out of scope".
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// devIno is the hard-link deduplication key.
// R3.2: keyed by (Dev, Ino) pair from sys.FileInfo.
type devIno struct {
	Dev uint64
	Ino uint64
}

// duConfig holds parsed command-line options.
type duConfig struct {
	all           bool  // R2.3: -a/--all reports all files
	humanReadable bool  // R2.1: -h/--human-readable
	summarize     bool  // R2.2: -s/--summarize
	total         bool  // R2.7: -c/--total
	maxDepth      int   // R2.4: -d/--max-depth; -1 means unlimited
	oneFileSystem bool  // -x/--one-file-system
	blockSize     int64 // output block size in bytes; default 1024
	args          []string
}

// humanSuffixes are the unit suffixes for 1024-based human-readable output.
var humanSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

func main() {
	// R5.1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %s\n", err)
		os.Exit(1)
	}

	os.Exit(run(cfg))
}

// parseArgs extracts flags and positional arguments.
func parseArgs(args []string) (duConfig, error) {
	cfg := duConfig{blockSize: 1024, maxDepth: -1}
	if err := parseFlagsAndArgs(args, &cfg); err != nil {
		return duConfig{}, err
	}
	// D3: -s sets maxDepth to 0 when no explicit -d was given.
	if cfg.summarize && cfg.maxDepth < 0 {
		cfg.maxDepth = 0
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg, nil
}

// parseFlagsAndArgs iterates over args, populating cfg fields.
// Returns an error for unrecognized options, nil on success.
func parseFlagsAndArgs(args []string, cfg *duConfig) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return nil
		}
		if val, ok := strings.CutPrefix(arg, "--max-depth="); ok {
			if err := parseMaxDepth(val, cfg); err != nil {
				return err
			}
			continue
		}
		if handleFlag(arg, cfg) {
			continue
		}
		consumed, err := handleDepthFlag(arg, args, i, cfg)
		if err != nil {
			return err
		}
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		cfg.args = append(cfg.args, arg)
	}
	return nil
}

// handleDepthFlag handles -d N, --max-depth N, and -dN flag forms.
// Returns the number of extra args consumed (0 if not a depth flag).
func handleDepthFlag(arg string, args []string, i int, cfg *duConfig) (int, error) {
	if arg == "-d" || arg == "--max-depth" {
		if i+1 >= len(args) {
			return 0, fmt.Errorf("option '%s' requires an argument", arg)
		}
		if err := parseMaxDepth(args[i+1], cfg); err != nil {
			return 0, err
		}
		return 2, nil
	}
	if strings.HasPrefix(arg, "-d") && len(arg) > 2 {
		if err := parseMaxDepth(arg[2:], cfg); err != nil {
			return 0, err
		}
		return 1, nil
	}
	return 0, nil
}

// parseMaxDepth parses and sets the max-depth value.
func parseMaxDepth(val string, cfg *duConfig) error {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid maximum depth '%s'", val)
	}
	cfg.maxDepth = n
	return nil
}

// handleFlag processes a single flag argument, returning true if recognized.
func handleFlag(arg string, cfg *duConfig) bool {
	switch arg {
	case "-a", "--all":
		cfg.all = true
	case "-h", "--human-readable":
		// R2.1: human-readable binary (1024-based) output.
		cfg.humanReadable = true
	case "-s", "--summarize":
		// R2.2: display only total per argument.
		cfg.summarize = true
	case "-c", "--total":
		// R2.7: print grand total after all arguments.
		cfg.total = true
	case "-x", "--one-file-system":
		// Skip directories on different filesystems.
		cfg.oneFileSystem = true
	case "-k":
		// R2.5: equivalent to --block-size=1K (default, no visible effect).
		cfg.blockSize = 1024
		cfg.humanReadable = false
	case "-m":
		// R2.6: equivalent to --block-size=1M.
		cfg.blockSize = 1048576
		cfg.humanReadable = false
	default:
		return false
	}
	return true
}

// run processes all arguments and returns the exit code.
// R1.5: processes arguments in command-line order.
// R3.3: shared seen map across all arguments.
// R2.7: accumulates grand total for -c flag.
func run(cfg duConfig) int {
	exitCode := 0
	seen := make(map[devIno]bool)
	var grandTotal int64
	for _, arg := range cfg.args {
		argTotal := walkArg(arg, cfg, seen, &exitCode)
		grandTotal += argTotal
	}
	// R2.7: print grand total line when -c is given.
	if cfg.total {
		printEntry(grandTotal, "total", cfg)
	}
	return exitCode
}

// walkArg processes a single top-level command-line argument.
// R1.4: uses Lstat to avoid following symlinks.
// Returns the total block count for grand total accumulation.
func walkArg(path string, cfg duConfig, seen map[devIno]bool, code *int) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportError("cannot access", path, err)
		*code = 1
		return 0
	}
	if fi.Mode.IsDir() {
		return walkDir(path, fi, cfg, seen, code, 0, fi.Dev)
	}
	// Top-level file arguments are always printed.
	blocks, _ := entryBlocks(fi, seen)
	printEntry(blocks, path, cfg)
	return blocks
}

// walkDir recursively traverses a directory and prints accumulated sizes.
// R1.1: prints one line per subdirectory with accumulated block count.
// depth tracks recursion depth for --max-depth limiting.
// rootDev is the device ID of the argument for --one-file-system.
func walkDir(path string, fi *sys.FileInfo, cfg duConfig, seen map[devIno]bool, code *int, depth int, rootDev uint64) int64 {
	total, _ := entryBlocks(fi, seen)
	entries, err := os.ReadDir(path)
	if err != nil {
		reportError("cannot read directory", path, err)
		*code = 1
	}
	for _, e := range entries {
		child := joinPath(path, e.Name())
		total += walkChild(child, cfg, seen, code, depth+1, rootDev)
	}
	if shouldPrint(depth, cfg) {
		printEntry(total, path, cfg)
	}
	return total
}

// shouldPrint returns true if an entry at the given depth should be printed.
// R2.4: entries deeper than maxDepth are accumulated but not printed.
func shouldPrint(depth int, cfg duConfig) bool {
	if cfg.maxDepth >= 0 && depth > cfg.maxDepth {
		return false
	}
	return true
}

// walkChild processes a single entry during directory traversal.
// R2.3: prints file entries only when all (-a) is true.
// R3.1: skips already-seen inodes entirely (no output, no size).
// D2: skips entries on different filesystems when -x is active.
func walkChild(path string, cfg duConfig, seen map[devIno]bool, code *int, depth int, rootDev uint64) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportError("cannot access", path, err)
		*code = 1
		return 0
	}
	// D2: skip directories on different filesystems.
	if cfg.oneFileSystem && fi.Dev != rootDev {
		return 0
	}
	if fi.Mode.IsDir() {
		return walkDir(path, fi, cfg, seen, code, depth, rootDev)
	}
	blocks, isNew := entryBlocks(fi, seen)
	if isNew && cfg.all && shouldPrint(depth, cfg) {
		printEntry(blocks, path, cfg)
	}
	return blocks
}

// entryBlocks returns the 512-byte block count for an entry, with dedup.
// R3.1: returns (0, false) if the inode was already counted.
// R3.2: deduplication key is (Dev, Ino) pair.
func entryBlocks(fi *sys.FileInfo, seen map[devIno]bool) (int64, bool) {
	key := devIno{fi.Dev, fi.Ino}
	if seen[key] {
		return 0, false
	}
	seen[key] = true
	return fi.Blocks, true
}

// joinPath concatenates parent and child preserving the parent prefix.
// Unlike filepath.Join, this does not clean "." prefixes away.
func joinPath(parent, child string) string {
	return parent + "/" + child
}

// printEntry prints a du output line with the configured size format.
// R1.3: format is "SIZE\tPATH\n".
func printEntry(blocks512 int64, path string, cfg duConfig) {
	fmt.Printf("%s\t%s\n", formatSize(blocks512, cfg), path)
}

// formatSize converts 512-byte block counts to the display format.
// R2.1: -h uses GNU-du-compatible human-readable binary (1024-based) units.
// R2.5: -k uses 1024-byte blocks (default).
// R2.6: -m uses 1048576-byte blocks with ceiling division.
func formatSize(blocks512 int64, cfg duConfig) string {
	if cfg.humanReadable {
		return formatHumanBinary(blocks512 * 512)
	}
	bytes := blocks512 * 512
	return fmt.Sprintf("%d", ceilDiv(bytes, cfg.blockSize))
}

// formatHumanBinary formats bytes as GNU du -h compatible output (1024-based).
// GNU du shows one decimal place for values < 10 (e.g., "4.0K") and no
// decimal for values >= 10 (e.g., "12K"). This differs from
// format.HumanSize which omits ".0" for exact values.
func formatHumanBinary(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	value := float64(bytes)
	idx := 0
	for value >= 1024 && idx < len(humanSuffixes)-1 {
		value /= 1024
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%d", bytes)
	}
	return formatHumanValue(value, humanSuffixes[idx])
}

// formatHumanValue formats a scaled value with its suffix.
// Values < 10 get one decimal place; values >= 10 get none.
func formatHumanValue(value float64, suffix string) string {
	if value >= 10 {
		return fmt.Sprintf("%.0f%s", value, suffix)
	}
	return fmt.Sprintf("%.1f%s", value, suffix)
}

// ceilDiv returns the ceiling of a / b for positive values.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

// reportError prints a diagnostic to stderr.
// R4.2: continues processing after errors.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "du: %s '%s': %s\n", action, path, msg)
}
