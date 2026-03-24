// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd058-rm: Remove files or directories.
// R1.1 (basic file removal), R1.2 (refuse directories without -r),
// R1.3 (refuse . and ..), R1.4 (continue on error),
// R2.1-R2.3 (recursive removal with force), R2.4 (empty dir removal),
// R3.1 (-i always prompt), R3.2 (-I once prompt),
// R3.3 (-v verbose), R3.4 (--interactive=WHEN).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// promptMode controls interactive confirmation behavior.
// R2.2: -f overrides -i/-I; last specified flag wins.
type promptMode int

const (
	pmDefault promptMode = iota
	pmAlways                     // -i / --interactive=always
	pmOnce                       // -I / --interactive=once
	pmNever                      // -f / --force / --interactive=never
)

// config holds the parsed command-line flags for rm.
type config struct {
	prompt    promptMode
	recursive bool
	dir       bool
	verbose   bool
}

// isForce reports whether force mode is active (pmNever).
func (c config) isForce() bool {
	return c.prompt == pmNever
}

// stdinReader buffers interactive responses from stdin.
var stdinReader = bufio.NewReader(os.Stdin)

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, args))
}

// run executes the rm operation and returns the exit code.
// R4.1: exit 0 on success. R4.2: exit 1 on any error.
func run(cfg config, args []string) int {
	if len(args) == 0 {
		if cfg.isForce() {
			return 0
		}
		printErr("missing operand")
		return 1
	}
	// R3.2: -I prompts once when >3 args or recursive.
	if cfg.prompt == pmOnce && shouldPromptOnce(cfg, len(args)) {
		if !promptOnce(len(args), cfg.recursive) {
			return 0
		}
	}
	return removeAll(cfg, args)
}

// shouldPromptOnce reports whether -I conditions are met.
// D3: triggers when count > 3 or recursive is active.
func shouldPromptOnce(cfg config, count int) bool {
	return cfg.recursive || count > 3
}

// promptOnce asks the user to confirm bulk removal.
// R3.2: format matches GNU rm.
func promptOnce(count int, recursive bool) bool {
	noun := "arguments"
	if count == 1 {
		noun = "argument"
	}
	if recursive {
		fmt.Fprintf(os.Stderr,
			"rm: remove %d %s recursively? ", count, noun)
	} else {
		fmt.Fprintf(os.Stderr,
			"rm: remove %d %s? ", count, noun)
	}
	return readYesNo()
}

// removeAll removes all arguments and returns the exit code.
// R1.4: continues on error.
func removeAll(cfg config, args []string) int {
	exitCode := 0
	for _, arg := range args {
		if removeOne(cfg, arg) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeOne removes a single path and returns 0 on success, 1 on error.
// R1.3: refuse . and .. entries.
func removeOne(cfg config, path string) int {
	if isDotOrDotDot(path) {
		printErr(
			"refusing to remove '.' or '..' directory: skipping '%s'",
			path)
		return 1
	}
	// Default behavior: refuse to remove root directory.
	// TODO: --preserve-root/--no-preserve-root flags are PRD
	// non-goals (prd058-rm). Only default refusal is implemented.
	if cfg.recursive && isRoot(path) {
		printErr(
			"it is dangerous to operate recursively on '/'")
		return 1
	}
	info, err := os.Lstat(path)
	if err != nil {
		return handleStatError(cfg, path, err)
	}
	if info.IsDir() {
		return handleDir(cfg, path, info)
	}
	return removeFile(cfg, path, info)
}

// handleStatError handles os.Lstat errors.
// R2.2: -f ignores nonexistent files and exits 0.
func handleStatError(cfg config, path string, err error) int {
	if os.IsNotExist(err) && cfg.isForce() {
		return 0
	}
	printErr("cannot remove '%s': %v", path, unwrapErr(err))
	return 1
}

// handleDir routes directory removal based on flags.
// R1.2: without -r, refuse to remove directories.
// R2.1: -r removes directories recursively.
// R2.4: -d removes empty directories.
func handleDir(cfg config, path string, info os.FileInfo) int {
	if cfg.recursive {
		return removeRecursive(cfg, path)
	}
	if cfg.dir {
		return removeSingleDir(cfg, path, info)
	}
	printErr("cannot remove '%s': Is a directory", path)
	return 1
}

// removeFile removes a single non-directory file.
// R1.1: remove file using os.Remove (calls unlink(2)).
// R3.1: -i prompts before removal.
func removeFile(cfg config, path string, info os.FileInfo) int {
	if cfg.prompt == pmAlways && !promptRemove(path, info) {
		return 0
	}
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(path)
	}
	return 0
}

// removeSingleDir removes a single empty directory.
// R2.4: -d removes empty directories.
// R3.1: -i prompts before removal.
func removeSingleDir(cfg config, path string, info os.FileInfo) int {
	if cfg.prompt == pmAlways && !promptRemove(path, info) {
		return 0
	}
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerboseDir(path)
	}
	return 0
}

// removeRecursive removes a directory tree depth-first.
// R2.1: -r removes directories and contents recursively.
// R2.3: -r with -f silently removes without prompting.
// R3.1: -i prompts "descend into directory?" before recursing.
func removeRecursive(cfg config, path string) int {
	if cfg.prompt == pmAlways && !promptDescend(path) {
		return 0
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	exitCode := removeEntries(cfg, path, entries)
	if removeEmptyDir(cfg, path) != 0 {
		exitCode = 1
	}
	return exitCode
}

// removeEntries removes all entries within a directory.
// R1.4: continues on error.
func removeEntries(
	cfg config, parent string, entries []os.DirEntry,
) int {
	exitCode := 0
	for _, entry := range entries {
		child := filepath.Join(parent, entry.Name())
		if removeOne(cfg, child) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeEmptyDir removes a now-empty directory after recursive removal.
// R3.1: -i prompts "remove directory?" after emptying.
func removeEmptyDir(cfg config, path string) int {
	if cfg.prompt == pmAlways && !promptRemoveDir(path) {
		return 0
	}
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerboseDir(path)
	}
	return 0
}

// isDotOrDotDot reports whether the path ends with "." or "..".
// R1.3: prevent accidental directory tree destruction.
func isDotOrDotDot(path string) bool {
	base := filepath.Base(path)
	return base == "." || base == ".."
}

// isRoot reports whether path resolves to the filesystem root.
func isRoot(path string) bool {
	return filepath.Clean(path) == "/"
}

// fileTypeDesc returns the GNU rm-style type descriptor for a file.
func fileTypeDesc(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode.IsDir():
		return "directory"
	case mode.IsRegular() && info.Size() == 0:
		return "regular empty file"
	default:
		return "regular file"
	}
}

// promptRemove asks the user to confirm removal of a specific file.
// R3.1: writes prompt to stderr.
func promptRemove(path string, info os.FileInfo) bool {
	fmt.Fprintf(os.Stderr, "rm: remove %s '%s'? ",
		fileTypeDesc(info), path)
	return readYesNo()
}

// promptDescend asks the user to confirm descending into a directory.
// R3.1: writes prompt to stderr before recursive traversal.
func promptDescend(path string) bool {
	fmt.Fprintf(os.Stderr,
		"rm: descend into directory '%s'? ", path)
	return readYesNo()
}

// promptRemoveDir asks the user to confirm removal of an emptied dir.
func promptRemoveDir(path string) bool {
	fmt.Fprintf(os.Stderr,
		"rm: remove directory '%s'? ", path)
	return readYesNo()
}

// readYesNo reads a line from stdin and returns true for y/Y.
func readYesNo() bool {
	response, err := stdinReader.ReadString('\n')
	if err != nil {
		return false
	}
	return isYes(response)
}

// isYes reports whether a response starts with y or Y.
func isYes(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) > 0 && (s[0] == 'y' || s[0] == 'Y')
}

// printVerbose prints the removal message for a file.
// R3.3: format matches GNU rm: "removed 'FILE'".
func printVerbose(path string) {
	fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
}

// printVerboseDir prints the removal message for a directory.
func printVerboseDir(path string) {
	fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
}

// printErr prints a formatted error to stderr in GNU rm format.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "rm: "+format+"\n", args...)
}

// unwrapErr extracts the inner error from os.PathError.
func unwrapErr(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(
	args []string,
) (cfg config, operands []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			operands = append(operands, args[i+1:]...)
			return
		}
		exit = parseOneArg(args[i], &cfg, &operands)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}
	return
}

// parseOneArg handles a single argument token.
func parseOneArg(
	arg string, cfg *config, operands *[]string,
) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case isLongFlag(arg):
		return parseLongFlag(arg, cfg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], cfg)
	default:
		*operands = append(*operands, arg)
	}
	return -1
}

// isLongFlag returns true for --prefixed flags.
func isLongFlag(arg string) bool {
	return strings.HasPrefix(arg, "--") && len(arg) > 2
}

// parseLongFlag handles --option flags.
// TODO: --preserve-root/--no-preserve-root are PRD non-goals (prd058-rm).
// TODO: --one-file-system is a PRD non-goal (prd058-rm).
func parseLongFlag(arg string, cfg *config) int {
	switch {
	case arg == "--force":
		cfg.prompt = pmNever
	case arg == "--recursive":
		cfg.recursive = true
	case arg == "--dir":
		cfg.dir = true
	case arg == "--verbose":
		cfg.verbose = true
	case arg == "--interactive" ||
		strings.HasPrefix(arg, "--interactive="):
		return parseInteractiveFlag(arg, cfg)
	default:
		fmt.Fprintf(os.Stderr,
			"rm: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// parseInteractiveFlag handles --interactive and --interactive=WHEN.
// R3.4: WHEN is 'never', 'once', or 'always'.
func parseInteractiveFlag(arg string, cfg *config) int {
	if arg == "--interactive" {
		cfg.prompt = pmAlways
		return -1
	}
	when := arg[len("--interactive="):]
	switch when {
	case "never":
		cfg.prompt = pmNever
	case "once":
		cfg.prompt = pmOnce
	case "always":
		cfg.prompt = pmAlways
	default:
		fmt.Fprintf(os.Stderr,
			"rm: invalid argument '%s' for '--interactive'\n",
			when)
		return 1
	}
	return -1
}

// parseShortFlags processes clustered short flags like -rf.
func parseShortFlags(flags string, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		if exit := applyShortFlag(flags[j], cfg); exit >= 0 {
			return exit
		}
	}
	return -1
}

// applyShortFlag applies a single short flag character.
func applyShortFlag(ch byte, cfg *config) int {
	switch ch {
	case 'f':
		cfg.prompt = pmNever
	case 'r', 'R':
		cfg.recursive = true
	case 'd':
		cfg.dir = true
	case 'v':
		cfg.verbose = true
	case 'i':
		cfg.prompt = pmAlways
	case 'I':
		cfg.prompt = pmOnce
	default:
		fmt.Fprintf(os.Stderr,
			"rm: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: rm [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

  -f, --force           ignore nonexistent files and arguments, never prompt
  -i                    prompt before every removal
  -I                    prompt once before removing more than three files, or
                          when removing recursively
      --interactive[=WHEN]  prompt according to WHEN: never, once (-I), or
                          always (-i); without WHEN, prompt always
  -r, -R, --recursive   remove directories and their contents recursively
  -d, --dir             remove empty directories
  -v, --verbose         explain what is being done
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"rm (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
