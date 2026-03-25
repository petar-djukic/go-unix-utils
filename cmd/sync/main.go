// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd085-sync: Synchronize Cached Writes to Persistent Storage.
// Covers R1.1-R1.4 (sync modes), R2.1-R2.3 (exit codes, SIGPIPE).
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const progName = "sync"

func main() {
	// R2.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and dispatches to the appropriate sync mode.
func run(args []string) int {
	opts, files, code := parseArgs(args)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		// R1.1: no arguments, sync all filesystems.
		syscall.Sync()
		return 0
	}
	return syncFiles(files, opts)
}

// syncOpts holds the parsed command-line flags.
type syncOpts struct {
	data       bool // -d / --data: use fdatasync
	fileSystem bool // -f / --file-system: sync filesystem
}

// parseArgs parses command-line arguments into options and file operands.
// Returns exit code >= 0 when the caller should exit immediately, -1 to proceed.
func parseArgs(args []string) (syncOpts, []string, int) {
	var opts syncOpts
	var files []string
	endFlags := false
	for _, arg := range args {
		code := classifyArg(arg, endFlags, &opts, &files, &endFlags)
		if code >= 0 {
			return opts, nil, code
		}
	}
	return opts, files, checkOperands(opts, files)
}

// classifyArg routes a single argument to the appropriate handler.
func classifyArg(
	arg string, endFlags bool,
	opts *syncOpts, files *[]string, flagsDone *bool,
) int {
	if endFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
		*files = append(*files, arg)
		return -1
	}
	if arg == "--" {
		*flagsDone = true
		return -1
	}
	if strings.HasPrefix(arg, "--") {
		return handleLongFlag(arg, opts)
	}
	return handleShortFlags(arg[1:], opts)
}

// handleLongFlag processes --data, --file-system, --help, --version.
func handleLongFlag(arg string, opts *syncOpts) int {
	switch arg {
	case "--data":
		opts.data = true
		return -1
	case "--file-system":
		opts.fileSystem = true
		return -1
	case "--help":
		printHelp()
		return 0
	case "--version":
		printVersion()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp()
		return 1
	}
}

// handleShortFlags processes combined short flags (e.g., -d, -f, -df).
func handleShortFlags(flags string, opts *syncOpts) int {
	for _, ch := range flags {
		switch ch {
		case 'd':
			opts.data = true
		case 'f':
			opts.fileSystem = true
		default:
			fmt.Fprintf(os.Stderr,
				"%s: invalid option -- '%c'\n", progName, ch)
			printTryHelp()
			return 1
		}
	}
	return -1
}

// checkOperands validates that file operands are present when -d or -f is set.
// R1.3, R1.4: exit 1 if no FILE is given with -d or -f.
func checkOperands(opts syncOpts, files []string) int {
	if (opts.data || opts.fileSystem) && len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		printTryHelp()
		return 1
	}
	return -1
}

// syncFiles syncs each named file. Returns 0 on success, 1 on any error.
// R2.1: exit 0 when all succeed. R2.2: exit 1 when any fails.
func syncFiles(files []string, opts syncOpts) int {
	exitCode := 0
	for _, path := range files {
		if err := syncOneFile(path, opts); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// syncOneFile opens a file and applies the requested sync operation.
func syncOneFile(path string, opts syncOpts) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		printFileError("error opening", path, unwrapError(err))
		return err
	}
	defer f.Close() // best-effort close
	fd := int(f.Fd())
	if err := applySyncOp(fd, opts); err != nil {
		printFileError("error writing", path, err.Error())
		return err
	}
	return nil
}

// applySyncOp calls the appropriate syscall based on the flags.
func applySyncOp(fd int, opts syncOpts) error {
	switch {
	case opts.fileSystem:
		// R1.4: sync the filesystem containing the file.
		// macOS lacks syncfs(2); sync(2) is the portable fallback
		// matching GNU coreutils behavior on Darwin.
		syscall.Sync()
		return nil
	case opts.data:
		// R1.3: fdatasync to sync data without metadata.
		// Darwin lacks unix.Fdatasync; fsync is the portable fallback.
		return syscall.Fsync(fd)
	default:
		// R1.2: fsync to sync data and metadata.
		return syscall.Fsync(fd)
	}
}

// printFileError prints a file operation error to stderr in GNU format.
func printFileError(action, path, reason string) {
	fmt.Fprintf(os.Stderr, "%s: %s '%s': %s\n",
		progName, action, path, capitalizeFirst(reason))
}

// unwrapError extracts the underlying error message from an os.PathError.
func unwrapError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Fprint(os.Stdout, `Usage: sync [OPTION] [FILE]...
Force changed blocks to disk, update the super block.

  -d, --data             sync only file data, no unneeded metadata
  -f, --file-system      sync the file systems that contain the files

      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Fprintf(os.Stdout, "sync (go-unix-utils) %s\n", version)
}
