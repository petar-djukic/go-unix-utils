// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cp implements GNU cp: copy files and directories.
//
// Implements prd056-cp R1.1-R1.4, R2.1-R2.4.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "cp"

const helpText = `Usage: cp [OPTION]... [-T] SOURCE... DEST
  or:  cp [OPTION]... SOURCE... DIRECTORY
  or:  cp [OPTION]... -t DIRECTORY SOURCE...
Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.

Mandatory arguments to long options are mandatory for short options too.
  -f, --force                  if an existing destination file cannot be
                                 opened, remove it and try again
  -i, --interactive            prompt before overwrite
  -L, --dereference            always follow symbolic links in SOURCE
  -n, --no-clobber             do not overwrite an existing file
  -P, --no-dereference         never follow symbolic links in SOURCE
  -r, -R, --recursive          copy directories recursively
  -t, --target-directory=DIRECTORY  copy all SOURCE arguments into DIRECTORY
  -v, --verbose                explain what is being done
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "cp (go-unix-utils) 0.1\n"

type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp
	parseVer
)

// cpOptions holds parsed command-line flags.
// R1.1-R1.4: force, interactive, noClobber, verbose, targetDir.
// R2.1-R2.4: recursive, dereference, noDereference.
type cpOptions struct {
	force         bool
	interactive   bool
	noClobber     bool
	verbose       bool
	targetDir     string
	recursive     bool
	dereference   bool
	noDereference bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the cp logic and returns the exit code.
func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts, operands, result, err := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0
	}
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1
	}
	return dispatch(operands, opts, stdin, stdout, stderr)
}

// dispatch routes based on operand count and target-directory mode.
func dispatch(operands []string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	if opts.targetDir != "" {
		return copyIntoDir(operands, opts.targetDir, opts, stdin, stdout, stderr)
	}
	if len(operands) < 2 {
		if len(operands) == 0 {
			printError(stderr, "missing file operand")
		} else {
			printError(stderr, fmt.Sprintf(
				"missing destination file operand after '%s'", operands[0]))
		}
		printTryHelp(stderr)
		return 1
	}
	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]
	if len(sources) > 1 || isDir(dest) {
		return copyIntoDir(sources, dest, opts, stdin, stdout, stderr)
	}
	return copySingle(sources[0], dest, opts, stdin, stdout, stderr)
}

// copyIntoDir copies each source into the destination directory.
// R1.1: multi-source copy into directory.
func copyIntoDir(sources []string, dir string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	if !isDir(dir) {
		printError(stderr, fmt.Sprintf("target '%s' is not a directory", dir))
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		dest := filepath.Join(dir, filepath.Base(src))
		if copySingle(src, dest, opts, stdin, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// copySingle copies one source entry (file, directory, or symlink) to dest.
// R2.1: directories are copied recursively when -r is set.
// R2.2: directories without -r produce an error.
// R2.3: -L follows symlinks.
// R2.4: -P preserves symlinks (default with -r).
func copySingle(src, dest string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	info, err := os.Lstat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0

	// R2.4: preserve symlinks when not dereferencing.
	if isSymlink && !shouldDereference(opts) {
		return copySymlinkEntry(src, dest, opts, stdin, stdout, stderr)
	}

	// R2.3: follow symlink to get real file info.
	if isSymlink {
		info, err = os.Stat(src)
		if err != nil {
			printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
				src, stripPathError(err)))
			return 1
		}
	}

	if info.IsDir() {
		if !opts.recursive {
			// R2.2: refuse to copy directory without -r.
			printError(stderr, fmt.Sprintf(
				"-r not specified; omitting directory '%s'", src))
			return 1
		}
		return copyDir(src, dest, opts, stdin, stdout, stderr)
	}

	return copyRegularFile(src, dest, info.Mode(), opts, stdin, stdout, stderr)
}

// copySymlinkEntry copies a symbolic link, preserving it as a symlink.
// R2.4: -P no-dereference.
func copySymlinkEntry(src, dest string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	if !handleDestConflict(dest, opts, stdin, stderr) {
		return 0
	}
	target, err := os.Readlink(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot read symlink '%s': %s", src, err))
		return 1
	}
	// Remove existing dest; os.Symlink cannot overwrite.
	os.Remove(dest) //nolint:errcheck // best-effort removal
	if err := os.Symlink(target, dest); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot create symlink '%s': %s", dest, err))
		return 1
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "'%s' -> '%s'\n", src, dest) //nolint:errcheck
	}
	return 0
}

// copyDir recursively copies a directory tree from src to dest.
// R2.1: -r recursive directory copy.
func copyDir(src, dest string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	srcInfo, err := os.Stat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	if err := os.MkdirAll(dest, srcInfo.Mode().Perm()); err != nil {
		printError(stderr, fmt.Sprintf("cannot create directory '%s': %s",
			dest, err))
		return 1
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "'%s' -> '%s'\n", src, dest) //nolint:errcheck
	}
	return copyDirEntries(src, dest, opts, stdin, stdout, stderr)
}

// copyDirEntries reads and copies all entries from src directory into dest.
func copyDirEntries(src, dest string, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	entries, err := os.ReadDir(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot read directory '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	exitCode := 0
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if copySingle(srcPath, destPath, opts, stdin, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// copyRegularFile copies a regular file from src to dest.
func copyRegularFile(src, dest string, mode os.FileMode, opts cpOptions, stdin *os.File, stdout, stderr *os.File) int {
	if !handleDestConflict(dest, opts, stdin, stderr) {
		return 0
	}
	if err := copyFile(src, dest, mode, opts); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot copy '%s' to '%s': %s", src, dest, err))
		return 1
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "'%s' -> '%s'\n", src, dest) //nolint:errcheck
	}
	return 0
}

// shouldDereference returns true when symlinks should be followed.
// R2.3: -L forces dereferencing.
// R2.4: -P forces no dereferencing; default with -r.
func shouldDereference(opts cpOptions) bool {
	if opts.dereference {
		return true
	}
	if opts.noDereference {
		return false
	}
	// Default: -P when recursive, dereference otherwise.
	return !opts.recursive
}

// handleDestConflict manages -n, -i, and -f for an existing destination.
// Returns true if the copy should proceed, false to skip.
func handleDestConflict(dest string, opts cpOptions, stdin *os.File, stderr *os.File) bool {
	if _, err := os.Lstat(dest); err != nil {
		return true // dest doesn't exist
	}
	// R1.4: -n takes precedence over -i.
	if opts.noClobber {
		return false
	}
	// R1.2: -i prompts the user.
	if opts.interactive {
		return promptOverwrite(dest, stdin, stderr)
	}
	return true
}

// promptOverwrite asks the user whether to overwrite dest.
// R1.2: -i interactive prompt.
func promptOverwrite(dest string, stdin *os.File, stderr *os.File) bool {
	fmt.Fprintf(stderr, "%s: overwrite '%s'? ", progName, dest) //nolint:errcheck
	var response string
	if _, err := fmt.Fscanln(stdin, &response); err != nil {
		return false
	}
	return strings.HasPrefix(response, "y") || strings.HasPrefix(response, "Y")
}

// copyFile performs the actual file copy with force retry logic.
// R1.3: -f removes dest and retries if open fails.
func copyFile(src, dest string, mode os.FileMode, opts cpOptions) error {
	err := doCopy(src, dest, mode)
	if err == nil {
		return nil
	}
	// R1.3: if -f and dest exists, remove it and retry.
	if opts.force {
		if removeErr := os.Remove(dest); removeErr == nil {
			return doCopy(src, dest, mode)
		}
	}
	return err
}

// doCopy opens src and writes its contents to dest with the given mode.
func doCopy(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close()
		return err
	}
	return destFile.Close()
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (cpOptions, []string, parseResult, error) {
	var opts cpOptions
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
			consumed, result, err := parseLongFlag(arg, args[i+1:], &opts)
			if result != parseOK {
				return opts, nil, result, nil
			}
			if err != nil {
				return opts, nil, parseOK, err
			}
			i += consumed
			continue
		}
		consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
		if err != nil {
			return opts, nil, parseOK, err
		}
		i += consumed
	}

	return opts, operands, parseOK, nil
}

// parseLongFlag handles long-form flags.
func parseLongFlag(flag string, remaining []string, opts *cpOptions) (int, parseResult, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--help":
		return 0, parseHelp, nil
	case "--version":
		return 0, parseVer, nil
	case "--force":
		opts.force = true
	case "--interactive":
		opts.interactive = true
	case "--no-clobber":
		opts.noClobber = true
	case "--verbose":
		opts.verbose = true
	case "--recursive":
		opts.recursive = true
	case "--dereference":
		opts.dereference = true
		opts.noDereference = false
	case "--no-dereference":
		opts.noDereference = true
		opts.dereference = false
	case "--target-directory":
		if hasValue {
			opts.targetDir = value
		} else if len(remaining) > 0 {
			opts.targetDir = remaining[0]
			return 1, parseOK, nil
		} else {
			return 0, parseOK, fmt.Errorf(
				"option '--target-directory' requires an argument")
		}
	default:
		return 0, parseOK, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, parseOK, nil
}

// parseShortFlags handles short flags and combined forms.
func parseShortFlags(flags string, remaining []string, opts *cpOptions) (int, error) {
	consumed := 0
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'f':
			opts.force = true
		case 'i':
			opts.interactive = true
		case 'n':
			opts.noClobber = true
		case 'v':
			opts.verbose = true
		case 'r', 'R':
			opts.recursive = true
		case 'L':
			opts.dereference = true
			opts.noDereference = false
		case 'P':
			opts.noDereference = true
			opts.dereference = false
		case 't':
			rest := flags[i+1:]
			if rest != "" {
				opts.targetDir = rest
			} else if len(remaining) > consumed {
				opts.targetDir = remaining[consumed]
				consumed++
			} else {
				return consumed, fmt.Errorf(
					"option requires an argument -- 't'")
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

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if arg starts with '--'.
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// stripPathError extracts the inner error message from *os.PathError.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "try help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}
