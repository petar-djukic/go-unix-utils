// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd056-cp R1.1: basic file copying (single and multi-source).
// Implements prd056-cp R1.2: interactive mode (-i, --interactive).
// Implements prd056-cp R1.3: force mode (-f, --force).
// Implements prd056-cp R1.4: no-clobber mode (-n, --no-clobber).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "cp"

// options holds parsed GNU cp flags for R1.1–R1.4.
type options struct {
	interactive bool // -i, --interactive (R1.2)
	force       bool // -f, --force (R1.3)
	noClobber   bool // -n, --no-clobber (R1.4)
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and executes the copy operation, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if err := validateOperands(files, stderr); err != nil {
		return 1
	}
	return doCopy(opts, files, stdin, stderr)
}

// validateOperands checks that enough file arguments were provided.
func validateOperands(files []string, stderr io.Writer) error {
	if len(files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		printTryHelp(stderr)
		return fmt.Errorf("missing operand")
	}
	if len(files) == 1 {
		fmt.Fprintf(stderr, "%s: missing destination file operand after '%s'\n",
			progName, files[0])
		printTryHelp(stderr)
		return fmt.Errorf("missing destination")
	}
	return nil
}

// doCopy copies each source to the destination. R1.1: single file copy and
// multi-source copy into a directory.
func doCopy(opts options, files []string, stdin io.Reader, stderr io.Writer) int {
	dest := files[len(files)-1]
	sources := files[:len(files)-1]
	destInfo, err := os.Stat(dest)
	destIsDir := err == nil && destInfo.IsDir()
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(stderr, "%s: target '%s': Not a directory\n", progName, dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := copySingle(opts, src, target, stdin, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// copySingle copies one source file to the target path, applying R1.2–R1.4.
func copySingle(opts options, src, dest string, stdin io.Reader, stderr io.Writer) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot stat '%s': %s\n",
			progName, src, unwrapPathError(err))
		return err
	}
	if srcInfo.IsDir() {
		fmt.Fprintf(stderr, "%s: -r not specified; omitting directory '%s'\n",
			progName, src)
		return fmt.Errorf("omitting directory")
	}
	if isSameFile(src, dest) {
		fmt.Fprintf(stderr, "%s: '%s' and '%s' are the same file\n",
			progName, src, dest)
		return fmt.Errorf("same file")
	}
	skip, skipErr := checkSkip(opts, dest, stdin, stderr)
	if skip {
		return skipErr
	}
	return performCopy(opts, src, dest, stderr)
}

// isSameFile returns true when src and dest refer to the same file.
func isSameFile(src, dest string) bool {
	si, err := os.Stat(src)
	if err != nil {
		return false
	}
	di, err := os.Stat(dest)
	if err != nil {
		return false
	}
	return os.SameFile(si, di)
}

// checkSkip checks -n (R1.4) and -i (R1.2) flags. Returns (true, nil) to
// skip silently, (true, error) to skip with failure, (false, nil) to proceed.
func checkSkip(opts options, dest string, stdin io.Reader, stderr io.Writer) (bool, error) {
	if _, err := os.Lstat(dest); err != nil {
		return false, nil // dest doesn't exist, no conflict
	}
	// R1.4: -n no-clobber takes precedence over -i.
	if opts.noClobber {
		return true, nil
	}
	// R1.2: -i interactive prompts before overwriting.
	if opts.interactive {
		fmt.Fprintf(stderr, "%s: overwrite '%s'? ", progName, dest)
		if !readYes(stdin) {
			return true, fmt.Errorf("not overwritten")
		}
	}
	return false, nil
}

// readYes reads one line from r and returns true if it starts with 'y' or 'Y'.
func readYes(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	line := scanner.Text()
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// performCopy opens the source and writes to the destination.
func performCopy(opts options, src, dest string, stderr io.Writer) error {
	srcFile, err := os.Open(src)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot open '%s' for reading: %s\n",
			progName, src, unwrapPathError(err))
		return err
	}
	defer srcFile.Close() // best-effort close on read-only file
	return writeDestFile(opts, srcFile, dest, stderr)
}

// writeDestFile creates the destination and copies data from srcFile.
func writeDestFile(opts options, srcFile *os.File, dest string, stderr io.Writer) error {
	destFile, err := createDest(opts, dest)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot create regular file '%s': %s\n",
			progName, dest, unwrapPathError(err))
		return err
	}
	_, copyErr := io.Copy(destFile, srcFile)
	closeErr := destFile.Close()
	if copyErr != nil {
		fmt.Fprintf(stderr, "%s: error writing '%s': %s\n",
			progName, dest, copyErr)
		return copyErr
	}
	return closeErr
}

// createDest opens the destination for writing. R1.3: with -f, removes and
// retries if the initial open fails.
func createDest(opts options, dest string) (*os.File, error) {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil && opts.force {
		if rmErr := os.Remove(dest); rmErr == nil {
			return os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		}
	}
	return f, err
}

// parseArgs separates flags from file arguments.
// Returns parsed options, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	var opts options
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(&opts, arg, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			continue
		}
		code := applyShortFlags(&opts, arg, stderr)
		if code >= 0 {
			return opts, nil, code
		}
	}
	return opts, files, -1
}

// applyShortFlags processes combined short flags (e.g., -ifn).
func applyShortFlags(o *options, arg string, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		if !applyShortFlag(o, arg[j]) {
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// applyShortFlag applies a single-character flag. Returns false if unrecognized.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'i':
		o.interactive = true
	case 'f':
		o.force = true
	case 'n':
		o.noClobber = true
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(o *options, arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--interactive":
		o.interactive = true
	case "--force":
		o.force = true
	case "--no-clobber":
		o.noClobber = true
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SOURCE DEST\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... SOURCE... DIRECTORY\n", progName)
	fmt.Fprintln(w, "Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -f, --force           if an existing destination file cannot be")
	fmt.Fprintln(w, "                          opened, remove it and try again")
	fmt.Fprintln(w, "  -i, --interactive     prompt before overwrite")
	fmt.Fprintln(w, "  -n, --no-clobber      do not overwrite an existing file")
	fmt.Fprintln(w, "      --help            display this help and exit")
	fmt.Fprintln(w, "      --version         output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
