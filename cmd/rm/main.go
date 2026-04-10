// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/rm: remove files or directories.
// Implements srd058 R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
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

const programName = "rm"

// interactiveMode controls when rm prompts for confirmation.
// R3.1: always prompts before every removal.
// R3.2: once prompts once before removing >3 files or recursively.
// R3.4: --interactive=WHEN maps to these modes.
type interactiveMode int

const (
	interactiveNever  interactiveMode = iota // -f, --interactive=never
	interactiveOnce                          // -I, --interactive=once
	interactiveAlways                        // -i, --interactive=always
)

// options holds parsed command-line flags for rm.
type options struct {
	recursive   bool            // -r, -R, --recursive
	force       bool            // -f, --force
	dir         bool            // -d, --dir
	verbose     bool            // -v, --verbose
	interactive interactiveMode // -i, -I, --interactive=WHEN
}

// stdinReader reads yes/no responses for interactive prompts.
var stdinReader = bufio.NewReader(os.Stdin)

func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	os.Exit(run(opts, args))
}

// run validates arguments and dispatches removal.
// R2.2: with -f and no arguments, exit 0 silently.
// R3.2: -I prompts once before removing >3 files or recursively.
func run(opts options, args []string) int {
	if len(args) == 0 {
		if opts.force {
			return 0
		}
		printMissingOperand()
		return 1
	}
	if opts.interactive == interactiveOnce {
		if !confirmOnce(opts, args) {
			return 0
		}
	}
	return removeArgs(opts, args)
}

// confirmOnce prompts once if conditions for -I are met.
// R3.2: prompt when removing recursively or >3 files.
func confirmOnce(opts options, args []string) bool {
	n := len(args)
	if opts.recursive {
		return promptYesNo(fmt.Sprintf(
			"%s: remove %d %s recursively? ",
			programName, n, pluralArg(n)))
	}
	if n > 3 {
		return promptYesNo(fmt.Sprintf(
			"%s: remove %d %s? ",
			programName, n, pluralArg(n)))
	}
	return true
}

// pluralArg returns "argument" or "arguments" based on count.
func pluralArg(n int) string {
	if n == 1 {
		return "argument"
	}
	return "arguments"
}

// promptYesNo writes prompt to stderr and reads a yes/no response.
func promptYesNo(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// removeArgs removes each argument, continuing on errors.
// R1.4: errors are printed to stderr; processing continues.
func removeArgs(opts options, args []string) int {
	exitCode := 0
	for _, arg := range args {
		if err := removeOne(opts, arg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// removeOne removes a single path.
// R1.3: refuses . and .. before any filesystem access.
func removeOne(opts options, path string) error {
	if isDotOrDotDot(path) {
		return fmt.Errorf(
			"refusing to remove '.' or '..' directory: skipping '%s'",
			path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		if opts.force && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if info.IsDir() {
		return handleDir(opts, path)
	}
	return removeFilePrompt(opts, path, info)
}

// isDotOrDotDot returns true if path's base is "." or "..".
// R1.3: prevents accidental directory tree destruction.
func isDotOrDotDot(path string) bool {
	base := filepath.Base(path)
	return base == "." || base == ".."
}

// removeFilePrompt removes a file, prompting if interactive=always.
// R3.1: -i prompts before every removal.
func removeFilePrompt(opts options, path string, info os.FileInfo) error {
	if opts.interactive == interactiveAlways {
		desc := fileTypeDesc(info)
		prompt := fmt.Sprintf(
			"%s: remove %s '%s'? ", programName, desc, path)
		if !promptYesNo(prompt) {
			return nil
		}
	}
	return removeFile(opts, path)
}

// handleDir handles directory removal based on flags.
// R1.2: without -r or -d, refuses to remove directories.
// R2.1: -r removes directories and their contents recursively.
// R2.4: -d removes empty directories only.
func handleDir(opts options, path string) error {
	if opts.recursive {
		return removeRecursivePrompt(opts, path)
	}
	if opts.dir {
		return removeDirPrompt(opts, path)
	}
	return fmt.Errorf("cannot remove '%s': Is a directory", path)
}

// removeRecursivePrompt prompts to descend if interactive=always.
// R3.1: -i prompts "descend into directory?" before recursing.
func removeRecursivePrompt(opts options, path string) error {
	if opts.interactive == interactiveAlways {
		prompt := fmt.Sprintf(
			"%s: descend into directory '%s'? ", programName, path)
		if !promptYesNo(prompt) {
			return nil
		}
	}
	return removeRecursive(opts, path)
}

// removeRecursive recursively removes a directory tree.
// R2.1: -r removes directories and their contents.
// R2.3: combined with -f, silently removes without prompting.
func removeRecursive(opts options, path string) error {
	if opts.verbose || opts.interactive == interactiveAlways {
		return removeTreeWalk(opts, path)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	return nil
}

// removeTreeWalk removes a directory tree entry by entry.
func removeTreeWalk(opts options, path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := removeTreeChild(opts, child, entry); err != nil {
			return err
		}
	}
	return removeDirFinal(opts, path)
}

// removeTreeChild removes a single child entry during tree walk.
func removeTreeChild(
	opts options, child string, entry os.DirEntry,
) error {
	if entry.IsDir() {
		return removeRecursivePrompt(opts, child)
	}
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			child, sysErrMsg(err))
	}
	return removeFilePrompt(opts, child, info)
}

// removeDirFinal removes the directory itself after children.
// R3.1: prompts "remove directory?" if interactive=always.
// R3.3: prints verbose output after removal.
func removeDirFinal(opts options, path string) error {
	if opts.interactive == interactiveAlways {
		prompt := fmt.Sprintf(
			"%s: remove directory '%s'? ", programName, path)
		if !promptYesNo(prompt) {
			return nil
		}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if opts.verbose {
		printRemovedDir(path)
	}
	return nil
}

// removeDirPrompt removes an empty directory with optional prompt.
// R3.1: -i prompts before removal. R2.4: -d removes empty dirs.
func removeDirPrompt(opts options, path string) error {
	if opts.interactive == interactiveAlways {
		prompt := fmt.Sprintf(
			"%s: remove directory '%s'? ", programName, path)
		if !promptYesNo(prompt) {
			return nil
		}
	}
	return removeDirEntry(opts, path)
}

// removeDirEntry removes an empty directory.
// R2.4: -d removes empty directories only.
func removeDirEntry(opts options, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if opts.verbose {
		printRemovedDir(path)
	}
	return nil
}

// removeFile removes a regular file or symlink.
// R1.1: uses os.Remove (unlink(2)) for file removal.
func removeFile(opts options, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if opts.verbose {
		printRemoved(path)
	}
	return nil
}

// fileTypeDesc returns the file type description for prompts.
// Matches GNU rm format used in interactive prompts.
func fileTypeDesc(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return "character special file"
	case mode&os.ModeDevice != 0:
		return "block special file"
	case mode.IsRegular() && info.Size() == 0:
		return "regular empty file"
	case mode.IsRegular():
		return "regular file"
	default:
		return "file"
	}
}

// sysErrMsg extracts the system error message from an os error.
func sysErrMsg(err error) string {
	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// printRemoved prints verbose removal output for files.
// R3.3: format matches GNU rm "removed 'PATH'".
func printRemoved(path string) {
	fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
}

// printRemovedDir prints verbose removal output for directories.
// R3.3: format matches GNU rm "removed directory 'PATH'".
func printRemovedDir(path string) {
	fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
}

// parseArgs separates flags from positional arguments.
// R2.2: -f overrides -i and -I (last-wins semantics).
func parseArgs(rawArgs []string) (options, []string) {
	var opts options
	var positional []string
	for i, arg := range rawArgs {
		if arg == "--" {
			positional = append(positional, rawArgs[i+1:]...)
			break
		}
		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--") {
			parseLongFlag(&opts, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			parseShortFlags(&opts, arg)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags for rm.
// R3.4: --interactive=WHEN controls prompting mode.
func parseLongFlag(opts *options, arg string) {
	if strings.HasPrefix(arg, "--interactive") {
		parseInteractiveFlag(opts, arg)
		return
	}
	switch arg {
	case "--recursive":
		opts.recursive = true
	case "--force":
		opts.force = true
		opts.interactive = interactiveNever
	case "--dir":
		opts.dir = true
	case "--verbose":
		opts.verbose = true
	}
}

// parseShortFlags handles combined short flags for rm.
// R2.2: -f overrides -i/-I. R3.1: -i sets always. R3.2: -I sets once.
func parseShortFlags(opts *options, arg string) {
	for _, ch := range arg[1:] {
		switch byte(ch) {
		case 'r', 'R':
			opts.recursive = true
		case 'f':
			opts.force = true
			opts.interactive = interactiveNever
		case 'd':
			opts.dir = true
		case 'v':
			opts.verbose = true
		case 'i':
			opts.force = false
			opts.interactive = interactiveAlways
		case 'I':
			opts.force = false
			opts.interactive = interactiveOnce
		}
	}
}

// parseInteractiveFlag handles --interactive and --interactive=WHEN.
// R3.4: WHEN is never, once, or always.
func parseInteractiveFlag(opts *options, arg string) {
	if arg == "--interactive" {
		opts.interactive = interactiveAlways
		return
	}
	if !strings.HasPrefix(arg, "--interactive=") {
		return
	}
	when := arg[len("--interactive="):]
	switch when {
	case "never", "no", "none":
		opts.interactive = interactiveNever
	case "once":
		opts.interactive = interactiveOnce
	case "always", "yes":
		opts.interactive = interactiveAlways
	}
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand() {
	fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
	printTryHelp()
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// printUsage prints the usage message.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

Options:
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
`, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s 1.0.0\n", programName)
}
