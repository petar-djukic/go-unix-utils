// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ln implements GNU ln: create links between files.
//
// Implements prd037-ln R1.1-R1.4, R2.1-R2.4, R3.1, R3.4, R3.5, R3.6, R4.1-R4.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ln"

// lnOptions holds parsed command-line flags.
type lnOptions struct {
	symbolic     bool
	relative     bool
	force        bool
	verbose      bool
	backup       bool
	backupMethod string // "simple", "numbered", "existing", "none"
	suffix       string
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
	opts := lnOptions{suffix: "~"}
	var operands []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "-" || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			consumed, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += consumed
			continue
		}
		consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
		if err != nil {
			return opts, nil, err
		}
		i += consumed
	}

	return opts, operands, nil
}

// parseLongFlag handles long-form flags including --backup[=METHOD] and --suffix=SUFFIX.
func parseLongFlag(flag string, remaining []string, opts *lnOptions) (int, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--symbolic":
		opts.symbolic = true
	case "--relative":
		opts.relative = true
	case "--force":
		opts.force = true
	case "--verbose":
		opts.verbose = true
	case "--backup":
		opts.backup = true
		if hasValue {
			if err := validateBackupMethod(value); err != nil {
				return 0, err
			}
			opts.backupMethod = normalizeBackupMethod(value)
		} else {
			opts.backupMethod = "existing"
		}
	case "--suffix":
		if hasValue {
			opts.suffix = value
		} else if len(remaining) > 0 {
			opts.suffix = remaining[0]
			return 1, nil
		} else {
			return 0, fmt.Errorf("option '--suffix' requires an argument")
		}
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, nil
}

// parseShortFlags handles -s, -r, -f, -v, -b, -S and combined forms like -sfv.
func parseShortFlags(flags string, remaining []string, opts *lnOptions) (int, error) {
	consumed := 0
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 's':
			opts.symbolic = true
		case 'r':
			opts.relative = true
		case 'f':
			opts.force = true
		case 'v':
			opts.verbose = true
		case 'b':
			opts.backup = true
			opts.backupMethod = "existing"
		case 'S':
			rest := flags[i+1:]
			if rest != "" {
				opts.suffix = rest
			} else if len(remaining) > consumed {
				opts.suffix = remaining[consumed]
				consumed++
			} else {
				return consumed, fmt.Errorf("option requires an argument -- 'S'")
			}
			return consumed, nil
		default:
			return consumed, fmt.Errorf("invalid option -- '%c'", flags[i])
		}
	}
	return consumed, nil
}

// splitLongFlag splits --name=value into components.
func splitLongFlag(flag string) (string, string, bool) {
	name, value, ok := strings.Cut(flag, "=")
	if ok {
		return name, value, true
	}
	return flag, "", false
}

// validateBackupMethod checks if method is a valid backup control value.
func validateBackupMethod(method string) error {
	switch method {
	case "none", "off", "numbered", "t",
		"existing", "nil", "simple", "never":
		return nil
	default:
		return fmt.Errorf("invalid backup type '%s'", method)
	}
}

// normalizeBackupMethod maps aliases to canonical names.
func normalizeBackupMethod(method string) string {
	switch method {
	case "t":
		return "numbered"
	case "nil":
		return "existing"
	case "never":
		return "simple"
	case "off":
		return "none"
	default:
		return method
	}
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
// R3.4: prints verbose output to stdout on success.
func createLink(target, linkName string, opts lnOptions, stderr *os.File) int {
	if !handleExisting(linkName, opts, stderr) {
		return 1
	}
	var effectiveTarget string
	var exitCode int
	if opts.symbolic {
		effectiveTarget, exitCode = createSymlink(target, linkName, opts, stderr)
	} else {
		effectiveTarget = target
		exitCode = createHardLink(target, linkName, stderr)
	}
	if exitCode == 0 && opts.verbose {
		printVerbose(linkName, effectiveTarget, opts.symbolic)
	}
	return exitCode
}

// handleExisting manages an existing destination: backup and/or removal.
// R3.1: -f removes existing destination.
// R3.5: -b creates backup before removal.
func handleExisting(dest string, opts lnOptions, stderr *os.File) bool {
	if _, err := os.Lstat(dest); err != nil {
		return true // doesn't exist; let link creation proceed
	}

	if !opts.force {
		return true // let link creation fail with EEXIST
	}

	if opts.backup && opts.backupMethod != "none" {
		if err := makeBackup(dest, opts); err != nil {
			printError(stderr, fmt.Sprintf("cannot backup '%s': %s", dest, err))
			return false
		}
		return true // backup renames dest, so it's already gone
	}

	if err := os.Remove(dest); err != nil {
		printError(stderr, fmt.Sprintf("cannot remove '%s': %s", dest, err))
		return false
	}
	return true
}

// makeBackup creates a backup of path according to the backup method.
// R3.5: backup creation with method selection.
func makeBackup(path string, opts lnOptions) error {
	switch opts.backupMethod {
	case "numbered":
		return createNumberedBackup(path)
	case "existing":
		if hasNumberedBackup(path) {
			return createNumberedBackup(path)
		}
		return os.Rename(path, path+opts.suffix)
	default: // "simple" or fallback
		return os.Rename(path, path+opts.suffix)
	}
}

// createNumberedBackup renames path to path.~N~ with the next available N.
func createNumberedBackup(path string) error {
	for n := 1; ; n++ {
		backup := fmt.Sprintf("%s.~%d~", path, n)
		if _, err := os.Lstat(backup); os.IsNotExist(err) {
			return os.Rename(path, backup)
		}
	}
}

// hasNumberedBackup checks if any numbered backup (path.~N~) exists.
func hasNumberedBackup(path string) bool {
	matches, _ := filepath.Glob(path + ".~[0-9]*~")
	return len(matches) > 0
}

// createHardLink creates a hard link.
// R1.1: hard link creation.
// R1.3: rejects directories.
// R1.4: rejects existing destinations.
// R4.1, R4.2: error messages for non-existent targets, permission denied, cross-device.
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

// createSymlink creates a symbolic link, returning the effective target and exit code.
// R2.1: -s creates symbolic links.
// R2.2: allows symlinks to directories.
// R2.3: stores target string as-is (unless -r).
// R2.4: -r computes relative path from link location to target.
func createSymlink(target, linkName string, opts lnOptions, stderr *os.File) (string, int) {
	linkTarget := target
	if opts.relative {
		rel, err := computeRelativePath(target, linkName)
		if err != nil {
			printError(stderr, err.Error())
			return "", 1
		}
		linkTarget = rel
	}
	if err := os.Symlink(linkTarget, linkName); err != nil {
		printSymlinkError(stderr, linkName, err)
		return "", 1
	}
	return linkTarget, 0
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

// printVerbose prints the verbose link creation message to stdout.
// R3.4: -v prints each link created.
func printVerbose(linkName, target string, symbolic bool) {
	arrow := "=>"
	if symbolic {
		arrow = "->"
	}
	fmt.Printf("'%s' %s '%s'\n", linkName, arrow, target)
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
// R1.4, R4.1, R4.2: reports existing destination, non-existent target, and cross-device errors.
func printLinkError(stderr *os.File, linkName string, err error) {
	fmt.Fprintf(stderr, "%s: failed to create hard link '%s': %s\n", progName, linkName, err) //nolint:errcheck
}

// printSymlinkError prints a symlink failure error to stderr.
func printSymlinkError(stderr *os.File, linkName string, err error) {
	fmt.Fprintf(stderr, "%s: failed to create symbolic link '%s': %s\n", progName, linkName, err) //nolint:errcheck
}

// TODO: prd037-ln non_goals: -L (--logical) and -P (--physical) dereference modes
// are explicitly excluded. Do not implement.

// TODO: prd037-ln non_goals: -t DIRECTORY and -T (--no-target-directory) flags
// are explicitly excluded. Do not implement.
