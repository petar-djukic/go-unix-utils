// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd058-rm R1.1–R1.4: basic file removal and argument handling.
// Implements prd058-rm R2.1–R2.4: recursive removal, force mode, and -d flag.
// Implements prd058-rm R3.1–R3.4: interactive modes and verbose output.
// Implements prd058-rm R4.1–R4.4: exit codes and differential testing.
//
// Non-goals per PRD (E6): --preserve-root, --no-preserve-root, and
// --one-file-system are not implemented.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rm"

// interactiveMode controls when rm prompts before removal. R3.1, R3.2, R3.4.
type interactiveMode int

const (
	interactiveNone   interactiveMode = iota // no prompting (default)
	interactiveAlways                        // R3.1: -i, prompt every removal
	interactiveOnce                          // R3.2: -I, prompt once for bulk/recursive
)

// rmConfig holds parsed flag state.
type rmConfig struct {
	force       bool            // R2.2: ignore nonexistent, never prompt
	recursive   bool            // R2.1: remove directories recursively
	dir         bool            // R2.4: remove empty directories
	verbose     bool            // R3.3: print each removal
	interactive interactiveMode // R3.1, R3.2, R3.4: prompting mode
	input       *bufio.Scanner  // stdin reader for interactive prompts
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and executes the removal, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	cfg.input = bufio.NewScanner(stdin)
	if len(files) == 0 {
		return handleNoFiles(cfg, stderr)
	}
	return removeAll(files, cfg, stdout, stderr)
}

// handleNoFiles returns the appropriate exit code when no files given.
func handleNoFiles(cfg rmConfig, stderr io.Writer) int {
	if cfg.force {
		return 0
	}
	fmt.Fprintf(stderr, "%s: missing operand\n", progName)
	printTryHelp(stderr)
	return 1
}

// removeAll removes each file, returning 0 on full success, 1 on any error.
// R1.4: continues with remaining files on error.
// R3.2: prompts once before bulk operations with -I.
func removeAll(files []string, cfg rmConfig, stdout, stderr io.Writer) int {
	if !promptBulk(files, cfg, stderr) {
		return 0
	}
	exitCode := 0
	for _, f := range files {
		if err := removeSingle(f, cfg, stdout, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// promptBulk handles -I prompting. Returns true to proceed, false to abort.
// R3.2: prompt once before removing more than three files or recursively.
func promptBulk(files []string, cfg rmConfig, stderr io.Writer) bool {
	if cfg.interactive != interactiveOnce {
		return true
	}
	n := len(files)
	suffix := pluralSuffix(n)
	if n > 3 {
		fmt.Fprintf(stderr, "%s: remove %d argument%s? ",
			progName, n, suffix)
		return readResponse(cfg.input)
	}
	if cfg.recursive {
		fmt.Fprintf(stderr, "%s: remove %d argument%s recursively? ",
			progName, n, suffix)
		return readResponse(cfg.input)
	}
	return true
}

// pluralSuffix returns "s" for counts != 1, "" for 1.
func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// readResponse reads one line and returns true if it starts with y or Y.
func readResponse(scanner *bufio.Scanner) bool {
	if !scanner.Scan() {
		return false
	}
	line := scanner.Text()
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// removeSingle removes one file or directory. R1.1, R1.2, R1.3, R2.1, R2.4.
func removeSingle(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	// R1.3: refuse to remove "." or ".."
	base := filepath.Base(path)
	if base == "." || base == ".." {
		fmt.Fprintf(stderr,
			"%s: refusing to remove '.' or '..' directory: skipping '%s'\n",
			progName, path)
		return fmt.Errorf("refused")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return handleLstatError(err, path, cfg, stderr)
	}
	if info.IsDir() {
		return removeDirectory(path, cfg, stdout, stderr)
	}
	return removeFile(path, info, cfg, stdout, stderr)
}

// handleLstatError handles errors from os.Lstat in removeSingle.
func handleLstatError(err error, path string, cfg rmConfig, stderr io.Writer) error {
	if cfg.force && os.IsNotExist(err) {
		return nil // R2.2: silently ignore nonexistent with -f
	}
	fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
		progName, path, unwrapPathError(err))
	return err
}

// removeDirectory handles directory removal based on flags.
// R2.1: -r removes recursively. R2.4: -d removes empty directories.
// R1.2: without -r or -d, directory removal fails.
func removeDirectory(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if cfg.recursive {
		return removeRecursive(path, cfg, stdout, stderr)
	}
	if cfg.dir {
		return removeEmptyDir(path, cfg, stdout, stderr)
	}
	fmt.Fprintf(stderr, "%s: cannot remove '%s': Is a directory\n",
		progName, path)
	return fmt.Errorf("is a directory")
}

// removeRecursive removes a directory tree depth-first. R2.1, R2.3.
// R3.1: with -i, prompts before descending into the directory.
func removeRecursive(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if !promptDescent(path, cfg, stderr) {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	firstErr := removeChildren(entries, path, cfg, stdout, stderr)
	if firstErr != nil {
		return firstErr
	}
	return removeEmptyDir(path, cfg, stdout, stderr)
}

// removeChildren removes all entries within a directory.
func removeChildren(entries []os.DirEntry, parent string, cfg rmConfig, stdout, stderr io.Writer) error {
	var firstErr error
	for _, entry := range entries {
		child := filepath.Join(parent, entry.Name())
		if err := removeSingle(child, cfg, stdout, stderr); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// promptDescent prompts before descending into a directory with -i. R3.1.
func promptDescent(path string, cfg rmConfig, stderr io.Writer) bool {
	if cfg.interactive != interactiveAlways {
		return true
	}
	wp := writeProtectedPrefix(path)
	fmt.Fprintf(stderr, "%s: descend into %sdirectory '%s'? ",
		progName, wp, path)
	return readResponse(cfg.input)
}

// removeEmptyDir removes a single empty directory. R2.4.
// R3.1: with -i, prompts before removing the directory.
func removeEmptyDir(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if cfg.interactive == interactiveAlways {
		wp := writeProtectedPrefix(path)
		fmt.Fprintf(stderr, "%s: remove %sdirectory '%s'? ",
			progName, wp, path)
		if !readResponse(cfg.input) {
			return nil
		}
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(stdout, "removed directory '%s'\n", path)
	}
	return nil
}

// removeFile removes a regular file (or symlink). R1.1.
// R3.1: with -i, prompts before removing the file.
func removeFile(path string, info os.FileInfo, cfg rmConfig, stdout, stderr io.Writer) error {
	if cfg.interactive == interactiveAlways {
		if !promptRemoveFile(path, info, cfg, stderr) {
			return nil
		}
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	if cfg.verbose {
		// R3.3: GNU rm prints verbose to stdout
		fmt.Fprintf(stdout, "removed '%s'\n", path)
	}
	return nil
}

// promptRemoveFile prompts before removing a file with -i. R3.1.
func promptRemoveFile(path string, info os.FileInfo, cfg rmConfig, stderr io.Writer) bool {
	desc := fileTypeDesc(info)
	wp := writeProtectedPrefix(path)
	fmt.Fprintf(stderr, "%s: remove %s%s '%s'? ",
		progName, wp, desc, path)
	return readResponse(cfg.input)
}

// fileTypeDesc returns the GNU-style file type description for prompts.
func fileTypeDesc(info os.FileInfo) string {
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return "symbolic link"
	}
	if mode.IsRegular() {
		if info.Size() == 0 {
			return "regular empty file"
		}
		return "regular file"
	}
	if mode&os.ModeNamedPipe != 0 {
		return "fifo"
	}
	if mode&os.ModeSocket != 0 {
		return "socket"
	}
	if mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0 {
		return "character special file"
	}
	if mode&os.ModeDevice != 0 {
		return "block special file"
	}
	return "weird file"
}

// writeProtectedPrefix returns "write-protected " if the file is not
// writable, or "" otherwise. Matches GNU rm prompt format.
func writeProtectedPrefix(path string) string {
	if unix.Access(path, unix.W_OK) != nil {
		return "write-protected "
	}
	return ""
}

// parseArgs separates flags from file arguments and builds config.
// Returns config, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (rmConfig, []string, int) {
	var cfg rmConfig
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(arg, &cfg, stdout, stderr)
			if code >= 0 {
				return cfg, nil, code
			}
			continue
		}
		code := applyShortFlags(arg, &cfg, stderr)
		if code >= 0 {
			return cfg, nil, code
		}
	}
	return cfg, files, -1
}

// applyShortFlags processes combined short flags.
func applyShortFlags(arg string, cfg *rmConfig, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'f':
			cfg.force = true
			cfg.interactive = interactiveNone // R2.2: overrides -i/-I
		case 'i':
			cfg.force = false
			cfg.interactive = interactiveAlways // R3.1
		case 'I':
			cfg.force = false
			cfg.interactive = interactiveOnce // R3.2
		case 'r', 'R':
			cfg.recursive = true // R2.1
		case 'd':
			cfg.dir = true // R2.4
		case 'v':
			cfg.verbose = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n",
				progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// applyLongFlag handles --long-name flags.
func applyLongFlag(arg string, cfg *rmConfig, stdout, stderr io.Writer) int {
	if arg == "--interactive" || strings.HasPrefix(arg, "--interactive=") {
		return applyInteractiveFlag(arg, cfg, stderr)
	}
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	case "--force":
		cfg.force = true
		cfg.interactive = interactiveNone
		return -1
	case "--recursive":
		cfg.recursive = true // R2.1
		return -1
	case "--dir":
		cfg.dir = true // R2.4
		return -1
	case "--verbose":
		cfg.verbose = true
		return -1
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// applyInteractiveFlag handles --interactive and --interactive=WHEN. R3.4.
func applyInteractiveFlag(arg string, cfg *rmConfig, stderr io.Writer) int {
	when := "always" // bare --interactive defaults to always
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		when = arg[idx+1:]
	}
	switch when {
	case "never":
		cfg.interactive = interactiveNone
	case "once":
		cfg.interactive = interactiveOnce
	case "always":
		cfg.interactive = interactiveAlways
	default:
		fmt.Fprintf(stderr,
			"%s: invalid argument '%s' for '--interactive'\n", progName, when)
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
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Remove (unlink) the FILE(s).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -d, --dir             remove empty directories")
	fmt.Fprintln(w, "  -f, --force           ignore nonexistent files and arguments, never prompt")
	fmt.Fprintln(w, "  -i                    prompt before every removal")
	fmt.Fprintln(w, "  -I                    prompt once before removing more than three files,")
	fmt.Fprintln(w, "                          or when removing recursively")
	fmt.Fprintln(w, "      --interactive[=WHEN]  prompt according to WHEN: never, once (-I),")
	fmt.Fprintln(w, "                              or always (-i); without WHEN, prompt always")
	fmt.Fprintln(w, "  -r, -R, --recursive   remove directories and their contents recursively")
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
