// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd057-mv: Move or rename files.
// R1.1-R1.4 (basic move/rename, multi-source, directory move, dest-is-dir),
// R2.1-R2.2 (interactive prompt, force overwrite with last-flag-wins).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// overwriteMode tracks the last-specified overwrite flag.
// R2.2: when -i and -f are combined, the last flag given wins.
type overwriteMode int

const (
	owDefault     overwriteMode = iota
	owInteractive               // -i / --interactive
	owForce                     // -f / --force
	owNoClobber                 // -n / --no-clobber
)

// config holds the parsed command-line flags for mv.
type config struct {
	overwrite   overwriteMode
	verbose     bool
	targetDir   string
	noTargetDir bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, args))
}

// run executes the move operation and returns the exit code.
// R4.1: exit 0 on success. R4.2: exit 1 on any error.
func run(cfg config, args []string) int {
	if len(args) < 2 {
		printErr("missing file operand")
		return 1
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]
	if cfg.targetDir != "" {
		dest = cfg.targetDir
	}
	return dispatch(cfg, sources, dest)
}

// dispatch routes to single-file or multi-source move.
// R1.2: multiple sources to directory.
func dispatch(cfg config, sources []string, dest string) int {
	if len(sources) == 1 && !isDir(dest) {
		return moveSingle(cfg, sources[0], dest)
	}
	return moveMultiple(cfg, sources, dest)
}

// moveMultiple moves multiple sources into a destination directory.
// R1.2: each SOURCE is moved into DEST directory.
// R4.3: continue on error, exit 1 if any failed.
func moveMultiple(cfg config, sources []string, dest string) int {
	if !isDir(dest) {
		printErr("target '%s' is not a directory", dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if moveSingle(cfg, src, target) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// moveSingle moves a single source to a destination path.
// R1.4: if dest is an existing directory, move src into it.
func moveSingle(cfg config, src, dest string) int {
	if isDir(dest) && !cfg.noTargetDir {
		dest = filepath.Join(dest, filepath.Base(src))
	}
	if _, err := os.Lstat(src); err != nil {
		printErr("cannot stat '%s': %v", src, unwrapErr(err))
		return 1
	}
	if shouldSkipExisting(cfg, dest) {
		return 0
	}
	return performMove(cfg, src, dest)
}

// performMove attempts os.Rename, falling back to copy+remove on EXDEV.
// R1.1: same-filesystem rename or cross-device copy-then-delete.
func performMove(cfg config, src, dest string) int {
	err := os.Rename(src, dest)
	if err == nil {
		if cfg.verbose {
			printVerbose(src, dest)
		}
		return 0
	}
	if isCrossDevice(err) {
		return crossDeviceMove(cfg, src, dest)
	}
	printErr("cannot move '%s' to '%s': %v", src, dest, unwrapErr(err))
	return 1
}

// isCrossDevice reports whether the error indicates an EXDEV failure.
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// crossDeviceMove copies src to dest then removes src.
// R1.1: fallback for cross-filesystem moves.
func crossDeviceMove(cfg config, src, dest string) int {
	info, err := os.Lstat(src)
	if err != nil {
		printErr("cannot stat '%s': %v", src, unwrapErr(err))
		return 1
	}
	if info.IsDir() {
		return crossDeviceMoveDir(cfg, src, dest, info)
	}
	return crossDeviceMoveFile(cfg, src, dest, info)
}

// crossDeviceMoveFile copies a regular file across devices then removes it.
func crossDeviceMoveFile(
	cfg config, src, dest string, info os.FileInfo,
) int {
	if err := copyFileContents(src, dest, info.Mode()); err != nil {
		printErr("%v", err)
		return 1
	}
	if err := os.Remove(src); err != nil {
		printErr("cannot remove '%s': %v", src, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(src, dest)
	}
	return 0
}

// crossDeviceMoveDir recursively copies a directory across devices.
// R1.3: directories move without requiring a recursive flag.
func crossDeviceMoveDir(
	cfg config, src, dest string, info os.FileInfo,
) int {
	if err := os.MkdirAll(dest, info.Mode()); err != nil {
		printErr("cannot create directory '%s': %v",
			dest, unwrapErr(err))
		return 1
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		printErr("cannot read directory '%s': %v",
			src, unwrapErr(err))
		return 1
	}
	if moveDirEntries(cfg, src, dest, entries) != 0 {
		return 1
	}
	return removeSrcDir(cfg, src, dest)
}

// moveDirEntries moves each entry from src to dest directory.
func moveDirEntries(
	cfg config, src, dest string, entries []os.DirEntry,
) int {
	exitCode := 0
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dest, entry.Name())
		if moveSingle(cfg, s, d) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeSrcDir removes the source directory after a successful move.
func removeSrcDir(cfg config, src, dest string) int {
	if err := os.Remove(src); err != nil {
		printErr("cannot remove '%s': %v", src, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(src, dest)
	}
	return 0
}

// copyFileContents copies file data from src to dest.
func copyFileContents(
	src, dest string, mode os.FileMode,
) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %v",
			src, unwrapErr(err))
	}
	defer srcFile.Close()
	return writeDestFile(srcFile, dest, mode)
}

// writeDestFile writes data from r to a new file at dest.
func writeDestFile(
	r io.Reader, dest string, mode os.FileMode,
) error {
	dstFile, err := os.OpenFile(
		dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("cannot create '%s': %v",
			dest, unwrapErr(err))
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, r); err != nil {
		return fmt.Errorf("error writing '%s': %v", dest, unwrapErr(err))
	}
	return nil
}

// shouldSkipExisting checks overwrite mode against an existing dest.
// R2.1: -i prompts. R2.2: -f does not prompt.
func shouldSkipExisting(cfg config, dest string) bool {
	if !fileExists(dest) {
		return false
	}
	if cfg.overwrite == owNoClobber {
		return true
	}
	if cfg.overwrite == owInteractive {
		return !confirmOverwrite(dest)
	}
	return false
}

// confirmOverwrite prompts the user on stderr and reads response.
// R2.1: prompt before overwriting an existing destination.
func confirmOverwrite(dest string) bool {
	fmt.Fprintf(os.Stderr, "mv: overwrite '%s'? ", dest)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(response)
	return strings.HasPrefix(response, "y") ||
		strings.HasPrefix(response, "Y")
}

// printVerbose prints the move operation to stderr in GNU mv format.
// R3.1: verbose output for each file moved.
func printVerbose(src, dest string) {
	fmt.Fprintf(os.Stderr, "renamed '%s' -> '%s'\n", src, dest)
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// unwrapErr extracts the inner error from os.PathError.
func unwrapErr(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// printErr prints a formatted error to stderr in GNU mv format.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "mv: "+format+"\n", args...)
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (cfg config, operands []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			operands = append(operands, args[i+1:]...)
			return
		}
		exit = parseOneArg(args[i], args, &i, &cfg, &operands)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}
	return
}

// parseOneArg handles a single argument token.
func parseOneArg(
	arg string, args []string, i *int, cfg *config, operands *[]string,
) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case isLongFlag(arg):
		return parseLongFlag(arg, args, i, cfg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], args, i, cfg)
	default:
		*operands = append(*operands, arg)
	}
	return -1
}

// isLongFlag returns true for --prefixed flags.
func isLongFlag(arg string) bool {
	return strings.HasPrefix(arg, "--") && len(arg) > 2
}

// parseLongFlag handles --option and --option=value flags.
func parseLongFlag(
	arg string, args []string, i *int, cfg *config,
) int {
	switch {
	case arg == "--interactive":
		cfg.overwrite = owInteractive
	case arg == "--force":
		cfg.overwrite = owForce
	case arg == "--no-clobber":
		cfg.overwrite = owNoClobber
	case arg == "--verbose":
		cfg.verbose = true
	case arg == "--no-target-directory":
		cfg.noTargetDir = true
	case strings.HasPrefix(arg, "--target-directory"):
		return parseTargetDirFlag(arg, args, i, cfg)
	default:
		fmt.Fprintf(os.Stderr, "mv: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// parseShortFlags processes clustered short flags like -fi.
func parseShortFlags(
	flags string, args []string, i *int, cfg *config,
) int {
	for j := 0; j < len(flags); j++ {
		if exit := applyShortFlag(
			flags[j], flags[j+1:], args, i, cfg,
		); exit >= 0 {
			return exit
		}
		if flags[j] == 't' {
			break
		}
	}
	return -1
}

// applyShortFlag applies a single short flag character.
// R2.2: -i and -f overwrite each other; last one wins.
func applyShortFlag(
	ch byte, remainder string, args []string, i *int, cfg *config,
) int {
	switch ch {
	case 'i':
		cfg.overwrite = owInteractive
	case 'f':
		cfg.overwrite = owForce
	case 'n':
		cfg.overwrite = owNoClobber
	case 'v':
		cfg.verbose = true
	case 'T':
		cfg.noTargetDir = true
	case 't':
		return consumeTargetDir(remainder, args, i, cfg)
	default:
		fmt.Fprintf(os.Stderr, "mv: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// parseTargetDirFlag handles --target-directory and --target-directory=DIR.
func parseTargetDirFlag(
	arg string, args []string, i *int, cfg *config,
) int {
	if strings.HasPrefix(arg, "--target-directory=") {
		cfg.targetDir = arg[len("--target-directory="):]
		return -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintln(os.Stderr,
			"mv: option requires an argument -- 'target-directory'")
		return 1
	}
	*i++
	cfg.targetDir = args[*i]
	return -1
}

// consumeTargetDir reads the target directory from remainder or next arg.
func consumeTargetDir(
	remainder string, args []string, i *int, cfg *config,
) int {
	if len(remainder) > 0 {
		cfg.targetDir = remainder
		return -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintln(os.Stderr,
			"mv: option requires an argument -- 't'")
		return 1
	}
	*i++
	cfg.targetDir = args[*i]
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mv [OPTION]... SOURCE... DEST
Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.

  -f, --force                  do not prompt before overwriting
  -i, --interactive            prompt before overwrite
  -n, --no-clobber             do not overwrite an existing file
  -t, --target-directory=DIR   move all SOURCE arguments into DIR
  -T, --no-target-directory    treat DEST as a normal file
  -v, --verbose                explain what is being done
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
	_, err := fmt.Fprintf(os.Stdout, "mv (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
