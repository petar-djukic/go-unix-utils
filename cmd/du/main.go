// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du reports disk usage for directories and files, matching GNU du output.
//
// Implements: prd009-du R1, R2, R3, R4, R5
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component)
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName   = "du"
	totalLabel = "total"
)

// blockUnit selects the output unit for sizes. (prd009-du R2)
type blockUnit int

const (
	unitKilo  blockUnit = iota // 1024-byte blocks (default, -k) (R2.5)
	unitMega                   // 1048576-byte blocks (-m) (R2.6)
	unitHuman                  // human-readable (-h) (R2.1)
)

// options holds parsed command-line flags. (prd009-du R2)
type options struct {
	allFiles     bool      // -a: report all files (R2.3)
	grandTotal   bool      // -c: print grand total line (R2.7)
	apparentSize bool      // --apparent-size: use st_size (R2.8)
	maxDepth     int       // -d N / --max-depth=N; -1 = unlimited (R2.4)
	unit         blockUnit // output unit
	operands     []string  // path arguments
}

// parseArgs parses command-line arguments into options. (prd009-du R2)
func parseArgs(args []string) (options, error) {
	opts := options{maxDepth: -1}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			opts.operands = append(opts.operands, args[i+1:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--apparent-size":
				opts.apparentSize = true
			case arg == "--summarize":
				opts.maxDepth = 0
			case arg == "--all":
				opts.allFiles = true
			case arg == "--total":
				opts.grandTotal = true
			case arg == "--human-readable":
				opts.unit = unitHuman
			case strings.HasPrefix(arg, "--max-depth="):
				val := arg[len("--max-depth="):]
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					return opts, fmt.Errorf("%s: invalid maximum depth '%s'", progName, val)
				}
				opts.maxDepth = n
			default:
				return opts, fmt.Errorf("%s: unrecognized option '%s'", progName, arg)
			}
			i++
			continue
		}

		// Short flags (single or combined like -shc).
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'a':
					opts.allFiles = true
				case 's':
					opts.maxDepth = 0
				case 'c':
					opts.grandTotal = true
				case 'h':
					opts.unit = unitHuman
				case 'k':
					opts.unit = unitKilo
				case 'm':
					opts.unit = unitMega
				case 'd':
					rest := arg[j+1:]
					if rest != "" {
						n, err := strconv.Atoi(rest)
						if err != nil || n < 0 {
							return opts, fmt.Errorf("%s: invalid maximum depth '%s'", progName, rest)
						}
						opts.maxDepth = n
						j = len(arg)
						continue
					}
					i++
					if i >= len(args) {
						return opts, fmt.Errorf("%s: option requires an argument -- 'd'", progName)
					}
					n, err := strconv.Atoi(args[i])
					if err != nil || n < 0 {
						return opts, fmt.Errorf("%s: invalid maximum depth '%s'", progName, args[i])
					}
					opts.maxDepth = n
				default:
					return opts, fmt.Errorf("%s: invalid option -- '%c'", progName, arg[j])
				}
				j++
			}
			i++
			continue
		}

		// Not a flag: operand.
		opts.operands = append(opts.operands, arg)
		i++
	}

	// Default to current directory when no operands given. (R1.1)
	if len(opts.operands) == 0 {
		opts.operands = []string{"."}
	}
	return opts, nil
}

// formatSize converts a byte count to a display string based on the output
// unit. Uses ceiling division for block-unit conversions to match GNU du.
// (prd009-du R1.2, R2.1, R2.5, R2.6)
func formatSize(bytes int64, unit blockUnit) string {
	switch unit {
	case unitHuman:
		return format.HumanSize(bytes, format.HumanSizeOpts{Binary: true})
	case unitMega:
		if bytes <= 0 {
			return "0"
		}
		return strconv.FormatInt((bytes+1048575)/1048576, 10)
	default: // unitKilo
		if bytes <= 0 {
			return "0"
		}
		return strconv.FormatInt((bytes+1023)/1024, 10)
	}
}

// entryBytes returns the byte count for a file info entry based on whether
// apparent-size mode is active. (prd009-du R1.2, R2.8)
func entryBytes(fi *sys.FileInfo, apparentSize bool) int64 {
	if apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// markVisited records an inode as seen in the nested visited map and returns
// true if it was already visited. The map is keyed by Dev then Ino.
// (prd009-du R3.1, R3.2, R3.3, D3)
func markVisited(visited map[uint64]map[uint64]bool, dev, ino uint64) bool {
	devMap := visited[dev]
	if devMap == nil {
		devMap = make(map[uint64]bool)
		visited[dev] = devMap
	}
	if devMap[ino] {
		return true
	}
	devMap[ino] = true
	return false
}

// walkDir traverses dirPath depth-first, accumulating byte counts bottom-up.
// Returns total bytes for the subtree rooted at dirPath. depth is relative to
// the operand (0 = the operand itself). (prd009-du R1.1, R1.4, R1.5)
func walkDir(
	dirPath string,
	depth int,
	opts *options,
	visited map[uint64]map[uint64]bool,
	stdout, stderr *strings.Builder,
) int64 {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot read directory '%s': %s\n",
			progName, dirPath, sysErrMsg(err))
	}

	var total int64
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		fi, err := sys.Lstat(entryPath)
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot access '%s': %s\n",
				progName, entryPath, sysErrMsg(err))
			continue
		}

		if fi.Mode.IsDir() {
			subtotal := walkDir(entryPath, depth+1, opts, visited, stdout, stderr)
			total += subtotal
		} else {
			// Hard-link deduplication for non-directory entries. (R3.1, R3.2, R3.3)
			var bytes int64
			if !markVisited(visited, fi.Dev, fi.Ino) {
				bytes = entryBytes(fi, opts.apparentSize)
			}
			total += bytes
			// Print file line if -a is set and within depth limit. (R2.3, R2.4)
			if opts.allFiles && (opts.maxDepth < 0 || depth+1 <= opts.maxDepth) {
				fmt.Fprintf(stdout, "%s\t%s\n", formatSize(bytes, opts.unit), entryPath)
			}
		}
	}

	// Add the directory's own disk allocation. (R1.2)
	fi, err := sys.Lstat(dirPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot access '%s': %s\n",
			progName, dirPath, sysErrMsg(err))
	} else if !markVisited(visited, fi.Dev, fi.Ino) {
		total += entryBytes(fi, opts.apparentSize)
	}

	// Print directory line if within depth limit. (R1.3, R2.4)
	if opts.maxDepth < 0 || depth <= opts.maxDepth {
		fmt.Fprintf(stdout, "%s\t%s\n", formatSize(total, opts.unit), dirPath)
	}

	return total
}

// sysErrMsg extracts the underlying OS error message and capitalizes the first
// letter to match GNU coreutils strerror() format. (prd009-du R4.2)
func sysErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		msg := pe.Err.Error()
		if len(msg) > 0 {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
		return msg
	}
	return err.Error()
}

// run processes du arguments and returns stdout output, stderr output, and exit
// code. Separating I/O from os.Exit allows unit tests to call run directly
// without subprocess spawning. (D1)
func run(args []string) (stdout, stderr string, code int) {
	opts, err := parseArgs(args)
	if err != nil {
		return "", err.Error() + "\n", 1
	}

	var outBuf, errBuf strings.Builder
	visited := make(map[uint64]map[uint64]bool)
	var grandTotal int64

	for _, operand := range opts.operands {
		fi, err := sys.Lstat(operand)
		if err != nil {
			fmt.Fprintf(&errBuf, "%s: cannot access '%s': %s\n",
				progName, operand, sysErrMsg(err))
			continue
		}

		if fi.Mode.IsDir() {
			total := walkDir(operand, 0, &opts, visited, &outBuf, &errBuf)
			grandTotal += total
		} else {
			// File operand. (R1.1)
			var bytes int64
			if !markVisited(visited, fi.Dev, fi.Ino) {
				bytes = entryBytes(fi, opts.apparentSize)
			}
			grandTotal += bytes
			fmt.Fprintf(&outBuf, "%s\t%s\n", formatSize(bytes, opts.unit), operand)
		}
	}

	// Grand total line. (R2.7)
	if opts.grandTotal {
		fmt.Fprintf(&outBuf, "%s\t%s\n", formatSize(grandTotal, opts.unit), totalLabel)
	}

	code = 0
	if errBuf.Len() > 0 {
		code = 1
	}
	return outBuf.String(), errBuf.String(), code
}

func main() {
	sys.InstallSIGPIPEHandler() // (prd009-du R5.1)
	stdout, stderr, code := run(os.Args[1:])
	if stdout != "" {
		fmt.Print(stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	os.Exit(code)
}
