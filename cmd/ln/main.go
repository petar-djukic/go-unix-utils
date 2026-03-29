// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ln implements GNU ln: create links between files.
//
// Implements prd037-ln R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ln"

// lnOptions holds parsed command-line flags.
type lnOptions struct {
	symbolic bool
	relative bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes the ln logic and returns the exit code.
func run(args []string, stderr *os.File) int {
	opts, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1
	}

	if len(operands) == 0 {
		printError(stderr, "missing file operand")
		printTryHelp(stderr)
		return 1
	}

	if len(operands) == 1 {
		return createLink(operands[0], filepath.Base(operands[0]), opts, stderr)
	}

	return dispatchLink(operands, opts, stderr)
}

// dispatchLink routes to directory or single-link creation.
func dispatchLink(operands []string, opts lnOptions, stderr *os.File) int {
	dest := operands[len(operands)-1]
	targets := operands[:len(operands)-1]

	if isDir(dest) {
		return linkIntoDir(targets, dest, opts, stderr)
	}

	if len(targets) > 1 {
		printError(stderr, fmt.Sprintf("target '%s' is not a directory", dest))
		return 1
	}

	return createLink(targets[0], dest, opts, stderr)
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (lnOptions, []string, error) {
	var opts lnOptions
	var operands []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || arg == "-" || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			if err := parseLongFlag(arg, &opts); err != nil {
				return opts, nil, err
			}
			continue
		}
		if err := parseShortFlags(arg[1:], &opts); err != nil {
			return opts, nil, err
		}
	}

	return opts, operands, nil
}

// parseLongFlag handles --symbolic and --relative.
func parseLongFlag(flag string, opts *lnOptions) error {
	switch flag {
	case "--symbolic":
		opts.symbolic = true
	case "--relative":
		opts.relative = true
	default:
		return fmt.Errorf("unrecognized option '%s'", flag)
	}
	return nil
}

// parseShortFlags handles -s, -r, and combined forms like -sr.
func parseShortFlags(flags string, opts *lnOptions) error {
	for _, ch := range flags {
		switch ch {
		case 's':
			opts.symbolic = true
		case 'r':
			opts.relative = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if arg starts with '--'.
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}

// createLink creates a single link (hard or symbolic).
func createLink(target, linkName string, opts lnOptions, stderr *os.File) int {
	if opts.symbolic {
		return createSymlink(target, linkName, opts, stderr)
	}
	return createHardLink(target, linkName, stderr)
}

// createHardLink creates a hard link.
// R1.1: hard link creation.
// R1.3: rejects directories.
// R1.4: rejects existing destinations.
func createHardLink(target, linkName string, stderr *os.File) int {
	if err := validateHardLinkTarget(target); err != nil {
		printError(stderr, fmt.Sprintf("hard link not allowed for directory '%s'", target))
		return 1
	}
	if err := os.Link(target, linkName); err != nil {
		printLinkError(stderr, linkName, err)
		return 1
	}
	return 0
}

// createSymlink creates a symbolic link.
// R2.1: -s creates symbolic links.
// R2.2: allows symlinks to directories.
// R2.3: stores target string as-is (unless -r).
// R2.4: -r computes relative path from link location to target.
func createSymlink(target, linkName string, opts lnOptions, stderr *os.File) int {
	linkTarget := target
	if opts.relative {
		rel, err := computeRelativePath(target, linkName)
		if err != nil {
			printError(stderr, err.Error())
			return 1
		}
		linkTarget = rel
	}
	if err := os.Symlink(linkTarget, linkName); err != nil {
		printSymlinkError(stderr, linkName, err)
		return 1
	}
	return 0
}

// computeRelativePath computes the relative path from linkName's directory to target.
// R2.4: relative symlink path computation.
func computeRelativePath(target, linkName string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to resolve '%s': %w", target, err)
	}
	linkDir := filepath.Dir(linkName)
	absLinkDir, err := filepath.Abs(linkDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve '%s': %w", linkDir, err)
	}
	rel, err := filepath.Rel(absLinkDir, absTarget)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	return rel, nil
}

// linkIntoDir creates links for each target inside dir.
// R1.2: multiple targets into a directory.
func linkIntoDir(targets []string, dir string, opts lnOptions, stderr *os.File) int {
	exitCode := 0
	for _, target := range targets {
		linkName := filepath.Join(dir, filepath.Base(target))
		if createLink(target, linkName, opts, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// validateHardLinkTarget returns an error if target is a directory.
// R1.3: hard links to directories are not allowed.
func validateHardLinkTarget(target string) error {
	info, err := os.Stat(target)
	if err != nil {
		return nil // Let os.Link report the actual error.
	}
	if info.IsDir() {
		return fmt.Errorf("directory")
	}
	return nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "try help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printLinkError prints a hard link failure error to stderr.
// R1.4: reports existing destination errors.
func printLinkError(stderr *os.File, linkName string, err error) {
	fmt.Fprintf(stderr, "%s: failed to create hard link '%s': %s\n", progName, linkName, err) //nolint:errcheck
}

// printSymlinkError prints a symlink failure error to stderr.
func printSymlinkError(stderr *os.File, linkName string, err error) {
	fmt.Fprintf(stderr, "%s: failed to create symbolic link '%s': %s\n", progName, linkName, err) //nolint:errcheck
}
