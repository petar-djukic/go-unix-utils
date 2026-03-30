// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rm implements GNU rm: remove files or directories.
//
// Implements prd058-rm R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rm"

const helpText = `Usage: rm [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

  -f, --force           ignore nonexistent files and arguments, never prompt
  -i                    prompt before every removal
  -I                    prompt once before removing more than three files, or
                          when removing recursively
      --interactive[=WHEN]  prompt according to WHEN: never, once (-I), or
                          always (-i); without WHEN, prompt always
  -d, --dir             remove empty directories
  -r, -R, --recursive   remove directories and their contents recursively
  -v, --verbose         explain what is being done
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "rm (go-unix-utils) 0.1\n"

type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp
	parseVer
)

// interactMode controls when the user is prompted.
// R3.1: always prompts before every removal.
// R3.2: once prompts before >3 files or recursive directories.
// R3.4: --interactive=WHEN selects the mode.
type interactMode int

const (
	interactNone   interactMode = iota // no interactive flag specified
	interactNever                      // -f / --interactive=never
	interactOnce                       // -I / --interactive=once
	interactAlways                     // -i / --interactive=always
)

// rmOptions holds parsed command-line flags.
type rmOptions struct {
	force       bool
	interactive interactMode
	recursive   bool
	dir         bool
	verbose     bool
}

// rmContext bundles runtime state for removal operations.
type rmContext struct {
	opts   rmOptions
	stdin  *bufio.Reader
	stdout *os.File
	stderr *os.File
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the rm logic and returns the exit code.
func run(args []string, stdin io.Reader, stdout, stderr *os.File) int {
	opts, operands, result, err := parseArgs(args)
	if code, done := handleParseResult(result, err, stdout, stderr); done {
		return code
	}
	if len(operands) == 0 {
		return handleNoOperands(opts, stderr)
	}
	ctx := &rmContext{
		opts:   opts,
		stdin:  bufio.NewReader(stdin),
		stdout: stdout,
		stderr: stderr,
	}
	return removeAll(operands, ctx)
}

// handleParseResult handles help, version, and parse errors.
func handleParseResult(
	result parseResult, err error, stdout, stderr *os.File,
) (int, bool) {
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0, true
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0, true
	}
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1, true
	}
	return 0, false
}

// handleNoOperands handles the case when no file arguments are provided.
// R4.3: with -f, exits 0 even with no operands.
func handleNoOperands(opts rmOptions, stderr *os.File) int {
	if opts.force {
		return 0
	}
	printError(stderr, "missing operand")
	printTryHelp(stderr)
	return 1
}

// removeAll processes each operand and returns the overall exit code.
// R1.4: continues with remaining files on error.
// R3.2: -I prompts once before removing >3 files or recursively.
// R4.1: returns 0 when all removals succeed.
// R4.2: returns 1 when any removal fails; continues with remaining files.
func removeAll(operands []string, ctx *rmContext) int {
	if shouldPromptOnce(operands, ctx.opts) {
		if !promptYes(ctx, promptOnceMsg(operands)) {
			return 0
		}
	}
	exitCode := 0
	for _, path := range operands {
		if removeOne(path, ctx) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// shouldPromptOnce returns true if -I requires a one-time prompt.
// R3.2: prompt once when >3 files or when recursive with directories.
func shouldPromptOnce(operands []string, opts rmOptions) bool {
	if opts.interactive != interactOnce {
		return false
	}
	if len(operands) > 3 {
		return true
	}
	if !opts.recursive {
		return false
	}
	for _, p := range operands {
		info, err := os.Lstat(p)
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// promptOnceMsg returns the -I bulk prompt message.
func promptOnceMsg(operands []string) string {
	if len(operands) > 3 {
		return fmt.Sprintf("remove %d arguments? ", len(operands))
	}
	return "remove all arguments recursively? "
}

// promptYes writes a prompt to stderr and reads a y/n response from stdin.
func promptYes(ctx *rmContext, msg string) bool {
	fmt.Fprintf(ctx.stderr, //nolint:errcheck
		"%s: %s", progName, msg)
	line, _ := ctx.stdin.ReadString('\n')
	trimmed := strings.TrimSpace(line)
	return len(trimmed) > 0 && (trimmed[0] == 'y' || trimmed[0] == 'Y')
}

// removeOne removes a single file or directory entry.
// R1.1: removes files via os.Remove (unlink).
// R1.2: refuses directories without -r.
// R1.3: refuses '.' and '..'.
// R4.3: with -f, returns 0 for non-existent files.
func removeOne(path string, ctx *rmContext) int {
	if isDotOrDotDot(path) {
		printRefuseDot(ctx.stderr, path)
		return 1
	}
	info, err := os.Lstat(path)
	if err != nil {
		if ctx.opts.force && os.IsNotExist(err) {
			return 0
		}
		printRemoveError(ctx.stderr, path, err)
		return 1
	}
	if info.IsDir() {
		return removeDirEntry(path, ctx)
	}
	return removeFileEntry(path, info, ctx)
}

// removeDirEntry handles directory removal logic.
// R1.2: without -r or -d, refuse.
func removeDirEntry(path string, ctx *rmContext) int {
	if ctx.opts.recursive {
		return removeRecursive(path, ctx)
	}
	if ctx.opts.dir {
		return removeEmptyDir(path, ctx)
	}
	printError(ctx.stderr, fmt.Sprintf(
		"cannot remove '%s': Is a directory", path))
	return 1
}

// removeEmptyDir attempts to remove an empty directory.
// R2.4: -d removes empty directories.
// R3.1: -i prompts before removal.
func removeEmptyDir(path string, ctx *rmContext) int {
	if ctx.opts.interactive == interactAlways {
		if !promptYes(ctx, fmt.Sprintf(
			"remove directory '%s'? ", path)) {
			return 0
		}
	}
	if err := os.Remove(path); err != nil {
		printRemoveError(ctx.stderr, path, err)
		return 1
	}
	if ctx.opts.verbose {
		printRemovedDir(ctx.stdout, path)
	}
	return 0
}

// removeRecursive removes a directory and all contents.
// R3.1: -i prompts before descending into directories.
func removeRecursive(path string, ctx *rmContext) int {
	if ctx.opts.interactive == interactAlways {
		if !promptYes(ctx, fmt.Sprintf(
			"descend into directory '%s'? ", path)) {
			return 0
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		printRemoveError(ctx.stderr, path, err)
		return 1
	}
	exitCode := removeChildren(path, entries, ctx)
	return removeDirSelf(path, exitCode, ctx)
}

// removeChildren removes all entries in a directory.
func removeChildren(
	parent string, entries []os.DirEntry, ctx *rmContext,
) int {
	exitCode := 0
	for _, entry := range entries {
		child := filepath.Join(parent, entry.Name())
		if removeOne(child, ctx) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeDirSelf removes the directory after its children are removed.
// R3.1: -i prompts before removing the directory itself.
func removeDirSelf(path string, childExit int, ctx *rmContext) int {
	if ctx.opts.interactive == interactAlways {
		if !promptYes(ctx, fmt.Sprintf(
			"remove directory '%s'? ", path)) {
			return childExit
		}
	}
	if err := os.Remove(path); err != nil {
		printRemoveError(ctx.stderr, path, err)
		return 1
	}
	if ctx.opts.verbose {
		printRemovedDir(ctx.stdout, path)
	}
	return childExit
}

// fileTypeDesc returns a description of the file type for -i prompts.
func fileTypeDesc(info os.FileInfo) string {
	if info.Mode()&os.ModeSymlink != 0 {
		return "symbolic link"
	}
	if info.Size() == 0 {
		return "regular empty file"
	}
	return "regular file"
}

// removeFileEntry removes a regular file or symlink.
// R1.1: unlink via os.Remove.
// R3.1: -i prompts before removal.
func removeFileEntry(
	path string, info os.FileInfo, ctx *rmContext,
) int {
	if ctx.opts.interactive == interactAlways {
		desc := fileTypeDesc(info)
		if !promptYes(ctx, fmt.Sprintf(
			"remove %s '%s'? ", desc, path)) {
			return 0
		}
	}
	if err := os.Remove(path); err != nil {
		printRemoveError(ctx.stderr, path, err)
		return 1
	}
	if ctx.opts.verbose {
		printRemoved(ctx.stdout, path)
	}
	return 0
}

// isDotOrDotDot returns true if the base name of path is "." or "..".
// R1.3: refuse removal of dot entries.
func isDotOrDotDot(path string) bool {
	cleaned := strings.TrimRight(path, "/")
	if cleaned == "" {
		return false
	}
	base := filepath.Base(cleaned)
	return base == "." || base == ".."
}

// printRefuseDot prints the GNU-compatible refusal message for '.' or '..'.
func printRefuseDot(stderr *os.File, path string) {
	fmt.Fprintf(stderr, //nolint:errcheck
		"%s: refusing to remove '.' or '..' directory: skipping '%s'\n",
		progName, path)
}

// printRemoveError prints a removal error with context.
func printRemoveError(stderr *os.File, path string, err error) {
	fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n", //nolint:errcheck
		progName, path, stripPathError(err))
}

// printRemoved prints verbose removal output for files.
func printRemoved(stdout *os.File, path string) {
	fmt.Fprintf(stdout, "removed '%s'\n", path) //nolint:errcheck
}

// printRemovedDir prints verbose removal output for directories.
func printRemovedDir(stdout *os.File, path string) {
	fmt.Fprintf(stdout, "removed directory '%s'\n", path) //nolint:errcheck
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (rmOptions, []string, parseResult, error) {
	var opts rmOptions
	var operands []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			result, err := parseLongFlag(arg, &opts)
			if result != parseOK {
				return opts, nil, result, nil
			}
			if err != nil {
				return opts, nil, parseOK, err
			}
			continue
		}
		if err := parseShortFlags(arg[1:], &opts); err != nil {
			return opts, nil, parseOK, err
		}
	}

	return opts, operands, parseOK, nil
}

// parseLongFlag handles long-form flags.
func parseLongFlag(flag string, opts *rmOptions) (parseResult, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--help":
		return parseHelp, nil
	case "--version":
		return parseVer, nil
	case "--force":
		opts.force = true
		opts.interactive = interactNever
	case "--recursive":
		opts.recursive = true
	case "--dir":
		opts.dir = true
	case "--verbose":
		opts.verbose = true
	case "--interactive":
		return parseOK, parseLongInteractive(value, hasValue, opts)
	default:
		return parseOK, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return parseOK, nil
}

// parseLongInteractive handles --interactive[=WHEN].
// R3.4: WHEN is 'never' (like -f), 'once' (like -I), or 'always' (like -i).
func parseLongInteractive(
	value string, hasValue bool, opts *rmOptions,
) error {
	if !hasValue {
		// --interactive without =WHEN is "always" (like -i).
		opts.force = false
		opts.interactive = interactAlways
		return nil
	}
	switch value {
	case "never":
		opts.interactive = interactNever
	case "once":
		opts.force = false
		opts.interactive = interactOnce
	case "always":
		opts.force = false
		opts.interactive = interactAlways
	default:
		return fmt.Errorf(
			"invalid argument '%s' for '--interactive'", value)
	}
	return nil
}

// parseShortFlags handles short flags and combined forms.
// R3.1: -i sets interactive=always.
// R3.2: -I sets interactive=once.
// R2.2: -f overrides -i and -I (last flag wins).
func parseShortFlags(flags string, opts *rmOptions) error {
	for i := range len(flags) {
		switch flags[i] {
		case 'f':
			opts.force = true
			opts.interactive = interactNever
		case 'r', 'R':
			opts.recursive = true
		case 'd':
			opts.dir = true
		case 'v':
			opts.verbose = true
		case 'i':
			opts.force = false
			opts.interactive = interactAlways
		case 'I':
			opts.force = false
			opts.interactive = interactOnce
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[i])
		}
	}
	return nil
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
	fmt.Fprintf(stderr, //nolint:errcheck
		"Try '%s --help' for more information.\n", progName)
}
