// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln R1.1–R1.4: hard link creation with single target,
// multi-target directory mode, directory rejection, and existing destination
// error handling. R2.1–R2.4: symbolic link creation with -s, directory
// support, as-is target storage, and -r relative path computation.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "ln"

// helpText is the usage message printed for --help.
const helpText = `Usage: ln [OPTION]... TARGET LINK_NAME
  or:  ln [OPTION]... TARGET... DIRECTORY
Create hard links (by default) or symbolic links to files.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version message printed for --version.
const versionText = `ln (go-unix-utils) 1.0
`

// options holds parsed command-line flags.
type options struct {
	symbolic bool // -s, --symbolic: create symbolic links
	relative bool // -r, --relative: create relative symlinks
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, operands := parseArgs(os.Args[1:])

	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\nTry '%s --help' for more information.\n", progName, progName)
		os.Exit(1)
	}

	if len(operands) == 1 {
		fmt.Fprintf(os.Stderr, "%s: missing destination file operand after '%s'\nTry '%s --help' for more information.\n",
			progName, operands[0], progName)
		os.Exit(1)
	}

	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]

	if len(sources) > 1 {
		linkMultipleIntoDir(sources, dest, opts)
	} else {
		linkSingle(sources[0], dest, opts)
	}
}

// parseArgs extracts options and operands from the argument list.
// Handles --help and --version by printing and exiting.
func parseArgs(args []string) (options, []string) {
	var opts options
	var operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "--help" {
			printAndExit(helpText)
		}
		if arg == "--version" {
			printAndExit(versionText)
		}
		if arg == "--symbolic" {
			opts.symbolic = true
			continue
		}
		if arg == "--relative" {
			opts.relative = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			operands = append(operands, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			parseShortFlags(arg[1:], &opts)
			continue
		}
		operands = append(operands, arg)
	}
	return opts, operands
}

// parseShortFlags processes a cluster of short flags (e.g., "-sr").
func parseShortFlags(flags string, opts *options) {
	for _, ch := range flags {
		switch ch {
		case 's':
			opts.symbolic = true
		case 'r':
			opts.relative = true
		}
	}
}

// linkSingle creates a link from source to dest. If dest is an existing
// directory, the link is created inside it with the source's basename.
// Implements R1.1, R1.3, R1.4, R2.1–R2.4.
func linkSingle(source, dest string, opts options) {
	if !opts.symbolic && isDirectory(source) {
		// R1.3: reject hard links to directories.
		fmt.Fprintf(os.Stderr, "%s: %s: hard link not allowed for directory\n", progName, source)
		os.Exit(1)
	}

	// If dest is an existing directory, place link inside it.
	if isDirectory(dest) {
		dest = filepath.Join(dest, filepath.Base(source))
	}

	// R1.4: error when destination already exists.
	if _, err := os.Lstat(dest); err == nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create %s '%s': File exists\n",
			progName, linkTypeName(opts.symbolic), dest)
		os.Exit(1)
	}

	if err := createLink(source, dest, opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create %s '%s' => '%s': %s\n",
			progName, linkTypeName(opts.symbolic), dest, source, unwrapErr(err))
		os.Exit(1)
	}
}

// linkMultipleIntoDir creates links for each source in the target directory.
// Implements R1.2.
func linkMultipleIntoDir(sources []string, dir string, opts options) {
	if !isDirectory(dir) {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", progName, dir)
		os.Exit(1)
	}

	exitCode := 0
	for _, source := range sources {
		if err := linkIntoDir(source, dir, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// linkIntoDir creates a single link for source inside dir.
func linkIntoDir(source, dir string, opts options) error {
	if !opts.symbolic && isDirectory(source) {
		// R1.3: reject hard links to directories.
		return fmt.Errorf("%s: hard link not allowed for directory", source)
	}

	dest := filepath.Join(dir, filepath.Base(source))

	// R1.4: error when destination already exists.
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("failed to create %s '%s': File exists",
			linkTypeName(opts.symbolic), dest)
	}

	if err := createLink(source, dest, opts); err != nil {
		return fmt.Errorf("failed to create %s '%s' => '%s': %s",
			linkTypeName(opts.symbolic), dest, source, unwrapErr(err))
	}
	return nil
}

// createLink creates a hard or symbolic link based on opts.
// R2.1: -s creates symbolic link. R2.3: target stored as-is.
// R2.4: -r computes relative path from link location to target.
func createLink(source, dest string, opts options) error {
	if opts.symbolic {
		target := source
		if opts.relative {
			// R2.4: compute relative path from link dir to target.
			var err error
			target, err = computeRelative(source, dest)
			if err != nil {
				return err
			}
		}
		// R2.3: target stored as-is in the symlink.
		return os.Symlink(target, dest)
	}
	return os.Link(source, dest)
}

// computeRelative computes a relative path from the directory containing
// dest to the source, resolving both to absolute paths first.
// Implements R2.4.
func computeRelative(source, dest string) (string, error) {
	absSrc, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("computing absolute source: %w", err)
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", fmt.Errorf("computing absolute dest: %w", err)
	}
	destDir := filepath.Dir(absDest)
	rel, err := filepath.Rel(destDir, absSrc)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}
	return rel, nil
}

// linkTypeName returns "hard link" or "symbolic link" for error messages.
func linkTypeName(symbolic bool) string {
	if symbolic {
		return "symbolic link"
	}
	return "hard link"
}

// isDirectory reports whether path is an existing directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// unwrapErr extracts the innermost error message, stripping os.PathError
// wrappers for cleaner output.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	return err.Error()
}

// printAndExit writes text to stdout and exits 0 on success or 1 on write error.
func printAndExit(text string) {
	_, err := fmt.Fprint(os.Stdout, text)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
