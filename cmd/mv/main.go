// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd057-mv R1.1–R1.4: basic file move, rename, and argument handling.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mv"

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and executes the move operation, returning the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if err := validateOperands(files, stderr); err != nil {
		return 1
	}
	return doMove(files, stderr)
}

// validateOperands checks that enough file arguments were provided. R1.4.
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

// doMove moves each source to the destination. R1.1, R1.2, R1.4.
func doMove(files []string, stderr io.Writer) int {
	dest := files[len(files)-1]
	sources := files[:len(files)-1]
	destInfo, err := os.Stat(dest)
	destIsDir := err == nil && destInfo.IsDir()
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(stderr, "%s: target '%s': Not a directory\n", progName, dest)
		return 1
	}
	return moveAll(sources, dest, destIsDir, stderr)
}

// moveAll iterates over sources and moves each one. R4.3: continues on error.
func moveAll(sources []string, dest string, destIsDir bool, stderr io.Writer) int {
	exitCode := 0
	for _, src := range sources {
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := moveSingle(src, target, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// moveSingle moves one source to the target path. R1.1, R1.3.
func moveSingle(src, dest string, stderr io.Writer) error {
	if _, err := os.Lstat(src); err != nil {
		fmt.Fprintf(stderr, "%s: cannot stat '%s': %s\n",
			progName, src, unwrapPathError(err))
		return err
	}
	if err := os.Rename(src, dest); err != nil {
		fmt.Fprintf(stderr, "%s: cannot move '%s' to '%s': %s\n",
			progName, src, dest, unwrapPathError(err))
		return err
	}
	return nil
}

// parseArgs separates flags from file arguments.
// Returns file list and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
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
			code := applyLongFlag(arg, stdout, stderr)
			if code >= 0 {
				return nil, code
			}
			continue
		}
		code := applyShortFlags(arg, stderr)
		if code >= 0 {
			return nil, code
		}
	}
	return files, -1
}

// applyShortFlags processes combined short flags.
func applyShortFlags(arg string, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		if !isValidShortFlag(arg[j]) {
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n",
				progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// isValidShortFlag returns true for recognized short flags.
// Flags that require state (-i, -f, -n, -v) are accepted but
// their behavior is implemented in later requirement groups.
func isValidShortFlag(ch byte) bool {
	switch ch {
	case 'i', 'f', 'n', 'v':
		return true
	default:
		return false
	}
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(arg string, stdout, stderr io.Writer) int {
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 0
	case arg == "--version":
		printVersion(stdout)
		return 0
	case arg == "--interactive", arg == "--force",
		arg == "--no-clobber", arg == "--verbose":
		return -1
	case arg == "--no-target-directory":
		return -1
	case arg == "--target-directory" ||
		strings.HasPrefix(arg, "--target-directory="):
		return -1
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SOURCE DEST\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... SOURCE... DIRECTORY\n", progName)
	fmt.Fprintln(w, "Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -f, --force           do not prompt before overwriting")
	fmt.Fprintln(w, "  -i, --interactive     prompt before overwrite")
	fmt.Fprintln(w, "  -n, --no-clobber      do not overwrite an existing file")
	fmt.Fprintln(w, "  -v, --verbose         explain what is being done")
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
