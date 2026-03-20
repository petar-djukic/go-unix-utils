// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln R1.1–R1.4: hard link creation, R2.1–R2.4: symbolic
// link creation, R3.1: force mode, R3.3: interactive mode, R3.4: verbose mode,
// R3.5: backup mode, R4.1–R4.2: error diagnostics.
// TODO: R3.6 (-t DIRECTORY) is a non-goal per prd037-ln non_goals; not implemented.
package main

import (
	"bufio"
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

// errDeclined is a sentinel error indicating the user declined an interactive prompt.
var errDeclined = fmt.Errorf("declined")

// overwriteMode determines behavior when the destination already exists.
type overwriteMode int

const (
	overwriteNone        overwriteMode = iota // default: error on existing
	overwriteForce                             // -f: remove without prompting
	overwriteInteractive                       // -i: prompt before removing
)

// options holds parsed command-line flags.
type options struct {
	symbolic  bool          // -s, --symbolic: create symbolic links
	relative  bool          // -r, --relative: create relative symlinks
	overwrite overwriteMode // -f or -i, last flag on command line wins
	backup    bool          // -b, --backup: create backup before removing
	verbose   bool          // -v, --verbose: print each link created (R3.4)
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

	for i, arg := range args {
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
		if parseLongFlag(arg, &opts) {
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

// parseLongFlag handles known long flags. Returns true if arg was consumed.
func parseLongFlag(arg string, opts *options) bool {
	switch arg {
	case "--symbolic":
		opts.symbolic = true
	case "--relative":
		opts.relative = true
	case "--force":
		opts.overwrite = overwriteForce
	case "--interactive":
		opts.overwrite = overwriteInteractive
	case "--verbose":
		opts.verbose = true
	default:
		if arg == "--backup" || strings.HasPrefix(arg, "--backup=") {
			opts.backup = true
			return true
		}
		return false
	}
	return true
}

// parseShortFlags processes a cluster of short flags (e.g., "-sf").
// R3.1/R3.3: -f and -i overwrite each other; last flag wins.
func parseShortFlags(flags string, opts *options) {
	for _, ch := range flags {
		switch ch {
		case 's':
			opts.symbolic = true
		case 'r':
			opts.relative = true
		case 'f':
			opts.overwrite = overwriteForce
		case 'i':
			opts.overwrite = overwriteInteractive
		case 'b':
			opts.backup = true
		case 'v':
			opts.verbose = true
		}
	}
}

// linkSingle creates a link from source to dest. If dest is an existing
// directory, the link is created inside it with the source's basename.
// Implements R1.1, R1.3, R1.4, R2.1–R2.4, R3.1, R3.3, R3.4, R3.5.
func linkSingle(source, dest string, opts options) {
	if err := checkSource(source, opts.symbolic); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	// If dest is an existing directory, place link inside it.
	if isDirectory(dest) {
		dest = filepath.Join(dest, filepath.Base(source))
	}

	if _, err := os.Lstat(dest); err == nil {
		if !resolveExisting(dest, opts) {
			if opts.overwrite == overwriteNone {
				fmt.Fprintf(os.Stderr, "%s: failed to create %s '%s': File exists\n",
					progName, linkTypeName(opts.symbolic), dest)
			}
			os.Exit(1)
		}
	}

	target, err := createLink(source, dest, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create %s '%s': %s\n",
			progName, linkTypeName(opts.symbolic), dest, unwrapErr(err))
		os.Exit(1)
	}

	if opts.verbose {
		printVerbose(dest, target, opts.symbolic)
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
			if err != errDeclined {
				fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			}
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// linkIntoDir creates a single link for source inside dir.
func linkIntoDir(source, dir string, opts options) error {
	if err := checkSource(source, opts.symbolic); err != nil {
		return err
	}

	dest := filepath.Join(dir, filepath.Base(source))

	if _, err := os.Lstat(dest); err == nil {
		if !resolveExisting(dest, opts) {
			if opts.overwrite == overwriteNone {
				return fmt.Errorf("failed to create %s '%s': File exists",
					linkTypeName(opts.symbolic), dest)
			}
			return errDeclined
		}
	}

	target, err := createLink(source, dest, opts)
	if err != nil {
		return fmt.Errorf("failed to create %s '%s': %s",
			linkTypeName(opts.symbolic), dest, unwrapErr(err))
	}

	if opts.verbose {
		printVerbose(dest, target, opts.symbolic)
	}
	return nil
}

// checkSource validates the source operand before linking.
// R1.3: rejects hard links to directories. R4.1: reports access errors
// with GNU-compatible format: "failed to access 'SOURCE': error".
func checkSource(source string, symbolic bool) error {
	if symbolic {
		return nil // symlinks don't require source to exist
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to access '%s': %s", source, unwrapErr(err))
	}
	if info.IsDir() {
		return fmt.Errorf("%s: hard link not allowed for directory", source)
	}
	return nil
}

// resolveExisting handles an existing destination per overwrite mode.
// Returns true if the destination was removed and linking should proceed.
// R3.1: force removes without prompting. R3.3: interactive prompts first.
func resolveExisting(dest string, opts options) bool {
	switch opts.overwrite {
	case overwriteForce:
		removeDest(dest, opts.backup)
		return true
	case overwriteInteractive:
		if !promptReplace(dest) {
			return false
		}
		removeDest(dest, opts.backup)
		return true
	default:
		return false
	}
}

// removeDest removes the destination, optionally creating a backup first.
// R3.5: -b creates a backup with ~ suffix before removal.
func removeDest(dest string, backup bool) {
	if backup {
		os.Rename(dest, dest+"~") // best-effort backup
		return
	}
	os.Remove(dest) // best-effort removal
}

// promptReplace prompts the user on stderr before removing dest.
// R3.3: reads one line from stdin; proceeds only if response starts with y/Y.
func promptReplace(dest string) bool {
	fmt.Fprintf(os.Stderr, "%s: replace '%s'? ", progName, dest)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// createLink creates a hard or symbolic link based on opts. Returns the
// actual target value stored in the link (may differ from source when -r is used).
// R2.1: -s creates symbolic link. R2.3: target stored as-is.
// R2.4: -r computes relative path from link location to target.
func createLink(source, dest string, opts options) (string, error) {
	if opts.symbolic {
		target := source
		if opts.relative {
			// R2.4: compute relative path from link dir to target.
			var err error
			target, err = computeRelative(source, dest)
			if err != nil {
				return "", err
			}
		}
		// R2.3: target stored as-is in the symlink.
		return target, os.Symlink(target, dest)
	}
	return source, os.Link(source, dest)
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

// printVerbose prints the verbose message for a created link to stdout.
// R3.4: format matches GNU ln -v output. Hard links use =>, symlinks use ->.
func printVerbose(dest, target string, symbolic bool) {
	arrow := " => "
	if symbolic {
		arrow = " -> "
	}
	fmt.Printf("'%s'%s'%s'\n", dest, arrow, target)
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
