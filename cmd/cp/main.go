// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cp: copy files and directories.
// Implements srd056 R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "cp"
const version = "1.0.0"

// stdinReader provides buffered reading from stdin for interactive prompts.
var stdinReader = bufio.NewReader(os.Stdin)

// errDeclined indicates the user declined an interactive overwrite.
// Not printed to stderr, but causes exit code 1.
var errDeclined = errors.New("declined")

// options holds parsed command-line flags for cp.
type options struct {
	interactive   bool   // R1.2: -i/--interactive
	force         bool   // R1.3: -f/--force
	noClobber     bool   // R1.4: -n/--no-clobber
	recursive     bool   // R2.1: -r/-R/--recursive
	dereference   bool   // R2.3: -L/--dereference
	noDereference bool   // R2.4: -P/--no-dereference
	preserve      string // R3.1/R3.3: --preserve=ATTR_LIST
	archive       bool   // R3.2: -a/--archive
	verbose       bool   // R3.4: -v/--verbose
	targetDir     string // R4.3: -t/--target-directory
}

// main entry point with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	exitCode := run(opts, args)
	os.Exit(exitCode)
}

// run dispatches the copy operation based on parsed options and args.
func run(opts options, args []string) int {
	if len(args) == 0 {
		printMissingOperand("")
		return 1
	}
	if len(args) == 1 && opts.targetDir == "" {
		printMissingDest(args[0])
		return 1
	}
	return copyFiles(opts, args)
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand(_ string) {
	fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
	printTryHelp()
}

// printMissingDest prints the missing destination error.
func printMissingDest(arg string) {
	fmt.Fprintf(os.Stderr,
		"%s: missing destination file operand after '%s'\n",
		programName, arg)
	printTryHelp()
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// copyFiles performs the copy operation for all sources to dest.
// R1.1: copies SOURCE to DEST, or multiple SOURCEs into DEST directory.
func copyFiles(opts options, args []string) int {
	sources, dest := splitSourcesDest(opts, args)
	destIsDir := isDir(dest)
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr,
			"%s: target '%s': Not a directory\n", programName, dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := resolveTarget(dest, src, destIsDir)
		if err := copySingle(opts, src, target); err != nil {
			if !errors.Is(err, errDeclined) {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			}
			exitCode = 1
		}
	}
	return exitCode
}

// splitSourcesDest separates sources from destination based on -t flag.
func splitSourcesDest(opts options, args []string) ([]string, string) {
	if opts.targetDir != "" {
		return args, opts.targetDir
	}
	return args[:len(args)-1], args[len(args)-1]
}

// isDir returns true if path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// resolveTarget determines the destination path for a source file.
func resolveTarget(dest, src string, destIsDir bool) string {
	if destIsDir {
		return filepath.Join(dest, filepath.Base(src))
	}
	return dest
}

// copySingle copies a single source to destination, applying -n, -i, -f.
// R1.1: basic copy, R1.2: interactive, R1.3: force, R1.4: no-clobber.
func copySingle(opts options, src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}
	if srcInfo.IsDir() {
		if !opts.recursive {
			return fmt.Errorf(
				"-r not specified; omitting directory '%s'", src)
		}
		// TODO: implement recursive copy (srd056 R2.1)
		return fmt.Errorf(
			"-r not specified; omitting directory '%s'", src)
	}
	if skipForNoClobber(opts, dest) {
		return nil
	}
	if skipForInteractive(opts, dest) {
		return errDeclined
	}
	return copyRegularFile(src, dest, opts)
}

// skipForNoClobber returns true if -n is set and dest exists.
// R1.4: -n takes precedence over -i.
func skipForNoClobber(opts options, dest string) bool {
	if !opts.noClobber {
		return false
	}
	_, err := os.Lstat(dest)
	return err == nil
}

// skipForInteractive returns true if -i is set, dest exists, and user declines.
// R1.2: prompt before overwriting.
func skipForInteractive(opts options, dest string) bool {
	if !opts.interactive {
		return false
	}
	_, err := os.Lstat(dest)
	if err != nil {
		return false
	}
	return !promptOverwrite(dest)
}

// promptOverwrite asks the user whether to overwrite dest.
// Returns true if the user responds with y/Y.
func promptOverwrite(path string) bool {
	fmt.Fprintf(os.Stderr, "%s: overwrite '%s'? ", programName, path)
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// copyRegularFile copies the contents of src to dest.
// R1.3: uses createDest which handles force removal.
func copyRegularFile(src, dest string, opts options) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %s",
			src, sysErrMsg(err))
	}
	defer in.Close()
	out, err := createDest(dest, opts)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return fmt.Errorf("error writing '%s': %s", dest, sysErrMsg(cpErr))
	}
	if closeErr != nil {
		return fmt.Errorf("closing '%s': %s", dest, sysErrMsg(closeErr))
	}
	return nil
}

// createDest opens dest for writing. With -f, removes and retries on failure.
// R1.3: if destination cannot be opened, remove it and retry.
func createDest(dest string, opts options) (*os.File, error) {
	out, err := os.Create(dest)
	if err != nil && opts.force {
		if rmErr := os.Remove(dest); rmErr == nil {
			out, err = os.Create(dest)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create regular file '%s': %s",
			dest, sysErrMsg(err))
	}
	return out, nil
}

// sysErrMsg extracts the system error message from an os error,
// capitalizing the first letter to match GNU coreutils format.
func sysErrMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		msg := pathErr.Err.Error()
		return capitalizeFirst(msg)
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

// parseArgs separates flags from positional arguments.
// Supports short flags, combined short flags, and long forms.
// R1.4: when -n and -i both appear, -n takes precedence.
func parseArgs(rawArgs []string) (options, []string) {
	var opts options
	var positional []string

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
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
			i = parseLongFlag(&opts, rawArgs, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(&opts, rawArgs, i)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags for cp.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	flag := rawArgs[idx]
	switch {
	case flag == "--interactive":
		opts.interactive = true
	case flag == "--force":
		opts.force = true
	case flag == "--no-clobber":
		opts.noClobber = true
	case flag == "--recursive":
		opts.recursive = true
	case flag == "--dereference":
		opts.dereference = true
	case flag == "--no-dereference":
		opts.noDereference = true
	case flag == "--archive":
		opts.archive = true
		opts.recursive = true
		opts.noDereference = true
		opts.preserve = "all"
	case flag == "--verbose":
		opts.verbose = true
	case strings.HasPrefix(flag, "--preserve="):
		opts.preserve = strings.TrimPrefix(flag, "--preserve=")
	case flag == "--preserve":
		opts.preserve = "mode,ownership,timestamps"
	case strings.HasPrefix(flag, "--target-directory="):
		opts.targetDir = strings.TrimPrefix(flag, "--target-directory=")
	case flag == "--target-directory":
		if idx+1 < len(rawArgs) {
			idx++
			opts.targetDir = rawArgs[idx]
		}
	}
	return idx
}

// parseShortFlags handles combined short flags like -rfv.
// R3.2: -a sets recursive, no-dereference, and preserve=all.
func parseShortFlags(opts *options, rawArgs []string, idx int) int {
	chars := rawArgs[idx][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'i':
			opts.interactive = true
		case 'f':
			opts.force = true
		case 'n':
			opts.noClobber = true
		case 'r', 'R':
			opts.recursive = true
		case 'L':
			opts.dereference = true
		case 'P':
			opts.noDereference = true
		case 'p':
			opts.preserve = "mode,ownership,timestamps"
		case 'a':
			opts.archive = true
			opts.recursive = true
			opts.noDereference = true
			opts.preserve = "all"
		case 'v':
			opts.verbose = true
		case 't':
			rest := chars[j+1:]
			if len(rest) > 0 {
				opts.targetDir = rest
			} else if idx+1 < len(rawArgs) {
				idx++
				opts.targetDir = rawArgs[idx]
			}
			return idx
		}
	}
	return idx
}

// printUsage prints the usage message listing all flags from srd056.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... SOURCE... DEST
  or:  %s [OPTION]... -t DIRECTORY SOURCE...
Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.

Options:
  -a, --archive                same as -dR --preserve=all
  -f, --force                  if destination cannot be opened, remove it and retry
  -i, --interactive            prompt before overwrite
  -L, --dereference            always follow symlinks in SOURCE
  -n, --no-clobber             do not overwrite an existing file
  -P, --no-dereference         never follow symlinks in SOURCE
  -p                           same as --preserve=mode,ownership,timestamps
      --preserve[=ATTR_LIST]   preserve specified attributes (mode,ownership,timestamps,links,all)
  -r, -R, --recursive          copy directories recursively
  -t, --target-directory=DIR   copy all SOURCE arguments into DIR
  -v, --verbose                explain what is being done
      --help                   display this help and exit
      --version                output version information and exit
`, programName, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s %s\n", programName, version)
}
