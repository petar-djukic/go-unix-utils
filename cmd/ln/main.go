// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ln: create links between files.
// Implements srd037 R1.1-R1.4, R2.1-R2.3,
// R3.1 (-f/--force), R3.2 (-n/--no-dereference), R3.3 (-i/--interactive),
// R3.4 (-v/--verbose), R3.5 (-b/--backup), R3.6 (-S/--suffix).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// errDeclined indicates the user declined an interactive prompt.
// This is not printed but causes a non-zero exit code.
var errDeclined = errors.New("declined")

const programName = "ln"
const defaultBackupSuffix = "~"

// options holds parsed command-line flags for ln.
type options struct {
	symbolic      bool   // R2.1: -s/--symbolic
	force         bool   // R3.1: -f/--force
	interactive   bool   // R3.3: -i/--interactive
	noDereference bool   // R3.2: -n/--no-dereference
	verbose       bool   // R3.4: -v/--verbose
	backup        bool   // R3.5: -b/--backup
	backupMethod  string // R3.5: numbered, existing, simple, none
	suffix        string // R3.6: -S/--suffix
}

// R1.1: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	exitCode := run(opts, args)
	os.Exit(exitCode)
}

// parseArgs separates flags from positional arguments.
// Supports short flags (-s, -f, -i, -n, -v, -b, -S), combined short flags,
// and long forms (--symbolic, --force, --interactive, --no-dereference,
// --verbose, --backup, --backup=METHOD, --suffix=SUFFIX).
// R3.3: when -f and -i both appear, the last one on the command line wins.
func parseArgs(rawArgs []string) (options, []string) {
	opts := options{
		suffix:       defaultBackupSuffix,
		backupMethod: "existing",
	}
	var positional []string

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			positional = append(positional, rawArgs[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			parseLongFlag(&opts, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(&opts, rawArgs, i)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags.
// R3.3: --force and --interactive are mutually exclusive; last wins.
// R3.5: --backup without =METHOD uses the default method (existing).
func parseLongFlag(opts *options, flag string) {
	switch {
	case flag == "--symbolic":
		opts.symbolic = true
	case flag == "--force":
		opts.force = true
		opts.interactive = false
	case flag == "--interactive":
		opts.interactive = true
		opts.force = false
	case flag == "--no-dereference":
		opts.noDereference = true
	case flag == "--verbose":
		opts.verbose = true
	case flag == "--backup":
		opts.backup = true
	case strings.HasPrefix(flag, "--backup="):
		opts.backup = true
		opts.backupMethod = strings.TrimPrefix(flag, "--backup=")
	case strings.HasPrefix(flag, "--suffix="):
		opts.suffix = strings.TrimPrefix(flag, "--suffix=")
	}
}

// parseShortFlags handles combined short flags like -sf.
// R3.3: -f and -i are mutually exclusive; the rightmost character wins.
// R3.6: -S consumes the remainder of the flag or the next argument as suffix.
func parseShortFlags(opts *options, rawArgs []string, idx int) int {
	chars := rawArgs[idx][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 's':
			opts.symbolic = true
		case 'f':
			opts.force = true
			opts.interactive = false
		case 'i':
			opts.interactive = true
			opts.force = false
		case 'n':
			opts.noDereference = true
		case 'v':
			opts.verbose = true
		case 'b':
			opts.backup = true
		case 'S':
			rest := chars[j+1:]
			if len(rest) > 0 {
				opts.suffix = rest
			} else if idx+1 < len(rawArgs) {
				idx++
				opts.suffix = rawArgs[idx]
			}
			return idx
		}
	}
	return idx
}

// run dispatches to the correct link form based on argument count.
func run(opts options, args []string) int {
	switch len(args) {
	case 1:
		return linkSingleArg(opts, args[0])
	case 2:
		return linkTwoArgs(opts, args[0], args[1])
	default:
		return linkMultiArgs(opts, args)
	}
}

// linkSingleArg implements the single-argument form: ln TARGET.
// R1.4: creates a link in the current directory with the same basename.
func linkSingleArg(opts options, target string) int {
	linkName := filepath.Base(target)
	return handleLinkResult(createLink(opts, target, linkName))
}

// linkTwoArgs implements the two-argument form: ln TARGET LINK_NAME.
// R1.1: creates a link named linkName pointing to target.
// If linkName is an existing directory, creates the link inside it.
func linkTwoArgs(opts options, target, linkName string) int {
	if isDirDest(opts, linkName) {
		linkName = filepath.Join(linkName, filepath.Base(target))
	}
	return handleLinkResult(createLink(opts, target, linkName))
}

// linkMultiArgs implements the multi-argument form: ln TARGET... DIRECTORY.
// R1.2: creates links in directory for each target.
func linkMultiArgs(opts options, args []string) int {
	dir := args[len(args)-1]
	targets := args[:len(args)-1]

	if !isDirDest(opts, dir) {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n",
			programName, dir)
		return 1
	}

	exitCode := 0
	for _, target := range targets {
		linkName := filepath.Join(dir, filepath.Base(target))
		if rc := handleLinkResult(createLink(opts, target, linkName)); rc != 0 {
			exitCode = rc
		}
	}
	return exitCode
}

// handleLinkResult converts a createLink error to an exit code.
// R3.3: errDeclined causes exit code 1 without printing an error message.
func handleLinkResult(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, errDeclined) {
		return 1
	}
	printError(err)
	return 1
}

// createLink creates a hard or symbolic link from target to linkName.
// R3.4: prints verbose output after successful link creation.
func createLink(opts options, target, linkName string) error {
	if err := handleExisting(opts, linkName); err != nil {
		return err
	}
	if opts.symbolic {
		if err := createSymLink(target, linkName); err != nil {
			return err
		}
	} else {
		if err := createHardLink(target, linkName); err != nil {
			return err
		}
	}
	if opts.verbose {
		printVerbose(opts, target, linkName)
	}
	return nil
}

// handleExisting handles pre-existing destination files.
// R3.1: force removes unconditionally (with optional backup).
// R3.3: interactive prompts before removing (with optional backup).
func handleExisting(opts options, linkName string) error {
	if !pathExists(linkName) {
		return nil
	}
	if opts.force {
		return backupAndRemove(opts, linkName)
	}
	if opts.interactive {
		if !promptReplace(linkName) {
			return errDeclined
		}
		return backupAndRemove(opts, linkName)
	}
	return nil
}

// backupAndRemove creates a backup (if enabled) then removes the file.
// R3.5: backup is created before removal when -b or --backup is active.
func backupAndRemove(opts options, path string) error {
	if needsBackup(opts) {
		if err := createBackup(opts, path); err != nil {
			return err
		}
	}
	removeExisting(path)
	return nil
}

// needsBackup returns true when backup is enabled and method is not "none".
func needsBackup(opts options) bool {
	m := opts.backupMethod
	return opts.backup && m != "none" && m != "off"
}

// createBackup renames path to its computed backup name.
// R3.5: supports numbered, existing, simple, and none methods.
func createBackup(opts options, path string) error {
	bp := computeBackupPath(opts, path)
	if err := os.Rename(path, bp); err != nil {
		return fmt.Errorf("cannot backup '%s': %s",
			path, unwrapOSError(err))
	}
	return nil
}

// computeBackupPath determines the backup filename for path.
// R3.5: numbered → path.~N~, existing → numbered if any exist else simple,
// simple → path + suffix.
func computeBackupPath(opts options, path string) string {
	switch opts.backupMethod {
	case "numbered", "t":
		return nextNumberedBackup(path)
	case "existing", "nil":
		if hasNumberedBackup(path) {
			return nextNumberedBackup(path)
		}
		return path + opts.suffix
	default: // "simple", "never", or unrecognized
		return path + opts.suffix
	}
}

// nextNumberedBackup returns the next available path.~N~ name.
func nextNumberedBackup(path string) string {
	for i := 1; ; i++ {
		bp := fmt.Sprintf("%s.~%d~", path, i)
		if !pathExists(bp) {
			return bp
		}
	}
}

// hasNumberedBackup checks if any path.~N~ backup file exists.
func hasNumberedBackup(path string) bool {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	prefix := base + ".~"
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, "~") {
			return true
		}
	}
	return false
}

// printVerbose prints the link creation to stdout.
// R3.4: format is 'LINK' -> 'TARGET' for symlinks,
// 'LINK' => 'TARGET' for hard links.
func printVerbose(opts options, target, linkName string) {
	arrow := "=>"
	if opts.symbolic {
		arrow = "->"
	}
	fmt.Printf("'%s' %s '%s'\n", linkName, arrow, target)
}

// promptReplace prompts the user on stderr before removing a destination.
// R3.3: format is "ln: replace 'DEST'? ". Reads one line from stdin.
// Proceeds if response starts with 'y' or 'Y'; declines otherwise.
func promptReplace(dest string) bool {
	fmt.Fprintf(os.Stderr, "%s: replace '%s'? ", programName, dest)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// pathExists checks whether a path exists using Lstat (does not follow symlinks).
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// removeExisting removes the destination file if it exists.
// R3.1: best-effort removal; errors are ignored because the
// subsequent link call will report a meaningful error if needed.
func removeExisting(path string) {
	// best-effort removal; link creation reports errors if path remains
	os.Remove(path)
}

// createSymLink creates a symbolic link.
// R2.1, R2.2, R2.3: symbolic link stores target as-is.
func createSymLink(target, linkName string) error {
	if err := os.Symlink(target, linkName); err != nil {
		return fmt.Errorf("failed to create symbolic link '%s': %s",
			linkName, unwrapOSError(err))
	}
	return nil
}

// createHardLink creates a hard link from target to linkName.
// R1.3: returns error when target is a directory.
// R1.4: returns error when linkName already exists.
func createHardLink(target, linkName string) error {
	if err := os.Link(target, linkName); err != nil {
		return fmt.Errorf("failed to create hard link '%s': %s",
			linkName, unwrapOSError(err))
	}
	return nil
}

// unwrapOSError extracts the underlying message from an *os.PathError or
// *os.LinkError.
func unwrapOSError(err error) string {
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printError prints a formatted error to stderr in GNU ln style.
func printError(err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
}

// isDirDest checks if path is a directory for destination resolution.
// R3.2: when noDereference is true, uses Lstat so a symlink to a directory
// is treated as a regular file rather than followed.
func isDirDest(opts options, path string) bool {
	if opts.noDereference {
		fi, err := os.Lstat(path)
		return err == nil && fi.IsDir()
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
