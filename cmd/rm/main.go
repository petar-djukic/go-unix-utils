// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd058-rm R1.1-R1.4, R2.1-R2.4, R3.1-R3.4
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "rm"

// errAlreadyReported signals that errors have been printed to stderr
// by the function that encountered them. The main loop should not
// print this error again.
var errAlreadyReported = errors.New("already reported")

// promptMode controls interactive prompting behavior.
type promptMode int

const (
	// promptNever disables prompting (default).
	promptNever promptMode = iota
	// promptAlways prompts before every removal (-i).
	promptAlways
	// promptOnce prompts once before bulk removal (-I).
	promptOnce
)

// stdinScanner is shared across all confirmation prompts to avoid losing
// buffered input between calls.
var stdinScanner *bufio.Scanner

// confirmPrompt writes prompt to stderr and reads a line from stdin.
// Returns true if the response starts with 'y' or 'Y'.
func confirmPrompt(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	if stdinScanner == nil {
		stdinScanner = bufio.NewScanner(os.Stdin)
	}
	if !stdinScanner.Scan() {
		return false
	}
	response := strings.TrimSpace(stdinScanner.Text())
	return strings.HasPrefix(strings.ToLower(response), "y")
}

// fileTypeDescription returns a human-readable file type string matching
// GNU rm's prompt format (e.g., "regular file", "directory", "symbolic link").
func fileTypeDescription(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode.IsDir():
		return "directory"
	case mode.IsRegular():
		if info.Size() == 0 {
			return "regular empty file"
		}
		return "regular file"
	default:
		return "file"
	}
}

// rmOpts holds all flag state for an rm invocation.
type rmOpts struct {
	force         bool       // -f: ignore nonexistent files, never prompt
	recursive     bool       // -r/-R: remove directories recursively
	dir           bool       // -d: remove empty directories
	verbose       bool       // -v: print each removal
	oneFileSystem bool       // --one-file-system: skip directories on different devices
	prompt        promptMode // -i/-I: interactive prompting mode
	preserveRoot  bool       // --preserve-root (default true)
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	var opts rmOpts
	opts.preserveRoot = true // R3.4: --preserve-root is the default

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			opts.force = true
			opts.prompt = promptNever
		case arg == "--recursive":
			opts.recursive = true
		case arg == "--dir":
			opts.dir = true
		case arg == "--verbose":
			opts.verbose = true
		case arg == "--one-file-system":
			opts.oneFileSystem = true
		case arg == "--preserve-root":
			opts.preserveRoot = true
		case arg == "--no-preserve-root":
			opts.preserveRoot = false
		case arg == "--interactive":
			opts.force = false
			opts.prompt = promptAlways
		case strings.HasPrefix(arg, "--interactive="):
			val := arg[len("--interactive="):]
			switch val {
			case "never":
				opts.prompt = promptNever
			case "once":
				opts.prompt = promptOnce
				opts.force = false
			case "always":
				opts.prompt = promptAlways
				opts.force = false
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--interactive'\n", programName, val)
				os.Exit(1)
			}
		case arg == "--version":
			fmt.Println("rm (go-unix-utils) 0.1")
			os.Exit(0)
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'f':
					opts.force = true
					opts.prompt = promptNever
				case 'r', 'R':
					opts.recursive = true
				case 'd':
					opts.dir = true
				case 'v':
					opts.verbose = true
				case 'i':
					opts.force = false
					opts.prompt = promptAlways
				case 'I':
					opts.force = false
					opts.prompt = promptOnce
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R1.1: With no operands, print usage to stderr and exit 1.
	// R2.2: With -f and no operands, GNU rm exits 0 silently.
	if len(operands) == 0 {
		if opts.force {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R3.4: --preserve-root refuses recursive removal of '/'.
	if opts.preserveRoot && opts.recursive {
		for _, path := range operands {
			if filepath.Clean(path) == "/" {
				fmt.Fprintf(os.Stderr, "%s: it is dangerous to operate recursively on '/'\n", programName)
				fmt.Fprintf(os.Stderr, "%s: use --no-preserve-root to override this failsafe\n", programName)
				os.Exit(1)
			}
		}
	}

	// R3.2: -I prompts once before removing more than three files or when
	// removing recursively.
	if opts.prompt == promptOnce {
		needPrompt := len(operands) > 3 || opts.recursive
		if needPrompt {
			noun := "arguments"
			if len(operands) == 1 {
				noun = "argument"
			}
			var prompt string
			if opts.recursive {
				prompt = fmt.Sprintf("%s: remove %d %s recursively? ", programName, len(operands), noun)
			} else {
				prompt = fmt.Sprintf("%s: remove %d %s? ", programName, len(operands), noun)
			}
			if !confirmPrompt(prompt) {
				os.Exit(0)
			}
		}
	}

	exitCode := 0
	for _, path := range operands {
		if err := removeFile(path, opts); err != nil {
			if !errors.Is(err, errAlreadyReported) {
				fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			}
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// removeFile removes a single file or directory at path, respecting the options.
//
// R1.1: Remove files using os.Remove (unlink).
// R1.2: Without -r or -d, refuse to remove directories.
// R1.3: Refuse to remove '.' or '..'.
// R1.4: Print error and continue on failure.
// R2.1: -r removes directories recursively.
// R2.2: -f suppresses errors for nonexistent files.
// R2.4: -d removes empty directories.
// R3.1: -i prompts before every removal.
// R3.3: -v prints removal messages to stdout.
func removeFile(path string, opts rmOpts) error {
	// Check if the path exists.
	info, statErr := os.Lstat(path)
	if statErr != nil {
		// R2.2: -f suppresses errors for nonexistent files.
		if opts.force && errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(statErr))
	}

	if info.IsDir() {
		// R1.3/D3: Reject '.' and '..' when -r or -d is active.
		base := filepath.Base(path)
		if (opts.recursive || opts.dir) && (base == "." || base == "..") {
			return fmt.Errorf("refusing to remove '.' or '..' directory: skipping '%s'", path)
		}

		// R2.1: -r removes directories recursively.
		if opts.recursive {
			return removeRecursive(path, opts)
		}

		// R2.4: -d removes empty directories.
		if opts.dir {
			// R3.1: -i prompts before removal.
			if opts.prompt == promptAlways {
				if !confirmPrompt(fmt.Sprintf("%s: remove directory '%s'? ", programName, path)) {
					return nil
				}
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
			}
			if opts.verbose {
				fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
			}
			return nil
		}

		// R1.2: Without -r or -d, refuse to remove directories.
		return fmt.Errorf("cannot remove '%s': Is a directory", path)
	}

	// R3.1: -i prompts before every removal.
	if opts.prompt == promptAlways {
		desc := fileTypeDescription(info)
		if !confirmPrompt(fmt.Sprintf("%s: remove %s '%s'? ", programName, desc, path)) {
			return nil
		}
	}

	// R1.1: remove the file.
	if err := os.Remove(path); err != nil {
		// R2.2: -f suppresses errors for nonexistent files (race condition).
		if opts.force && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}

	// R3.3: -v prints each removal to stdout.
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
	}

	return nil
}

// entry records a path and whether it is a directory, used for post-order removal.
type entry struct {
	path  string
	isDir bool
}

// removeRecursive removes a directory tree rooted at path.
//
// R2.1: Recursive directory removal.
// R2.3: --one-file-system skips directories on different devices.
// R3.1: -i prompts before each removal (uses interactive traversal).
// D2: Symlinks are removed but never followed.
// D4: Post-order removal via reversed WalkDir collection (non-interactive).
func removeRecursive(path string, opts rmOpts) error {
	// R3.1: interactive mode uses depth-first traversal with per-entry prompts.
	if opts.prompt == promptAlways {
		return removeRecursiveInteractive(path, opts)
	}

	var rootDev uint64
	if opts.oneFileSystem {
		fi, err := sys.Lstat(path)
		if err != nil {
			return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
		}
		rootDev = fi.Dev
	}

	var entries []entry
	hadError := false

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission denied or other error accessing this entry.
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, p, sysErrMsg(err))
			hadError = true
			return nil
		}

		// R2.3: --one-file-system skips directories on different devices.
		if opts.oneFileSystem && d.IsDir() && p != path {
			fi, statErr := sys.Lstat(p)
			if statErr != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, p, sysErrMsg(statErr))
				hadError = true
				return filepath.SkipDir
			}
			if fi.Dev != rootDev {
				fmt.Fprintf(os.Stderr, "%s: skipping '%s', since it's on a different device\n", programName, p)
				return filepath.SkipDir
			}
		}

		entries = append(entries, entry{path: p, isDir: d.IsDir()})
		return nil
	})

	// D4: Remove in reverse order (children before parents).
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if err := os.Remove(e.path); err != nil {
			if opts.force && errors.Is(err, os.ErrNotExist) {
				continue
			}
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, e.path, sysErrMsg(err))
			hadError = true
			continue
		}
		// R3.3: -v prints each removal to stdout.
		if opts.verbose {
			if e.isDir {
				fmt.Fprintf(os.Stdout, "removed directory '%s'\n", e.path)
			} else {
				fmt.Fprintf(os.Stdout, "removed '%s'\n", e.path)
			}
		}
	}

	if hadError {
		return errAlreadyReported
	}
	return nil
}

// removeRecursiveInteractive removes a directory tree with per-entry prompting.
// Used when -i is active to prompt before each file and directory removal.
//
// R3.1: prompt before descending into directories and before each removal.
func removeRecursiveInteractive(path string, opts rmOpts) error {
	// Prompt to descend into the directory.
	if !confirmPrompt(fmt.Sprintf("%s: descend into directory '%s'? ", programName, path)) {
		return nil
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}

	hadError := false
	for _, de := range dirEntries {
		childPath := filepath.Join(path, de.Name())

		if de.IsDir() {
			if err := removeRecursiveInteractive(childPath, opts); err != nil {
				if !errors.Is(err, errAlreadyReported) {
					fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
				}
				hadError = true
			}
			continue
		}

		// Get file info for type description.
		info, statErr := os.Lstat(childPath)
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, childPath, sysErrMsg(statErr))
			hadError = true
			continue
		}

		desc := fileTypeDescription(info)
		if !confirmPrompt(fmt.Sprintf("%s: remove %s '%s'? ", programName, desc, childPath)) {
			continue
		}

		if err := os.Remove(childPath); err != nil {
			if opts.force && errors.Is(err, os.ErrNotExist) {
				continue
			}
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, childPath, sysErrMsg(err))
			hadError = true
			continue
		}

		if opts.verbose {
			fmt.Fprintf(os.Stdout, "removed '%s'\n", childPath)
		}
	}

	// Prompt to remove the directory itself.
	if !confirmPrompt(fmt.Sprintf("%s: remove directory '%s'? ", programName, path)) {
		if hadError {
			return errAlreadyReported
		}
		return nil
	}

	if err := os.Remove(path); err != nil {
		if opts.force && errors.Is(err, os.ErrNotExist) {
			if hadError {
				return errAlreadyReported
			}
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}

	if opts.verbose {
		fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
	}

	if hadError {
		return errAlreadyReported
	}
	return nil
}

// printUsage prints a brief usage message to stdout.
func printUsage() {
	fmt.Println("Usage: rm [OPTION]... [FILE]...")
	fmt.Println("Remove (unlink) the FILE(s).")
	fmt.Println()
	fmt.Println("  -f, --force           ignore nonexistent files and arguments, never prompt")
	fmt.Println("  -i                    prompt before every removal")
	fmt.Println("  -I                    prompt once before removing more than three files, or")
	fmt.Println("                          when removing recursively")
	fmt.Println("  -r, -R, --recursive   remove directories and their contents recursively")
	fmt.Println("  -d, --dir             remove empty directories")
	fmt.Println("  -v, --verbose         explain what is being done")
	fmt.Println("      --one-file-system when removing a hierarchy recursively, skip any")
	fmt.Println("                          directory on a different file system")
	fmt.Println("      --preserve-root   do not remove '/' (default)")
	fmt.Println("      --no-preserve-root  do not treat '/' specially")
	fmt.Println("      --help            display this help and exit")
	fmt.Println("      --version         output version information and exit")
}

// sysErrMsg extracts the underlying syscall error message string from a
// (possibly wrapped) error, producing GNU-compatible messages like
// "No such file or directory" rather than Go's "stat /path: no such file...".
func sysErrMsg(err error) string {
	var msg string
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg = errno.Error()
	} else {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			msg = pathErr.Err.Error()
		} else {
			msg = err.Error()
		}
	}
	return capitalizeFirst(msg)
}

// capitalizeFirst returns s with its first rune uppercased, matching GNU
// coreutils error message capitalization (e.g., "No such file or directory").
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
