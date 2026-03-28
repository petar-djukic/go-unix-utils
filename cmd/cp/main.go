// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd056-cp: Copy files and directories.
// R1.1-R1.4 (basic copy, interactive, force, no-clobber),
// R2.1-R2.4 (recursive, directory refusal, dereference, no-dereference),
// R3.1-R3.4 (preserve mode/ownership/timestamps, archive, attr list, verbose),
// R4.1-R4.3 (exit codes, target-directory).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds the parsed command-line flags for cp.
type config struct {
	interactive   bool
	force         bool
	noClobber     bool
	recursive     bool
	dereference   bool
	noDereference bool
	preserve      string
	archive       bool
	verbose       bool
	targetDir     string
}

// preserveFlags controls which attributes are preserved on copy.
// R3.3: comma-separated attribute list support.
type preserveFlags struct {
	mode       bool
	ownership  bool
	timestamps bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, args))
}

// run executes the copy operation and returns the exit code.
// R4.1: exit 0 on success. R4.2: exit 1 on any error.
func run(cfg config, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "cp: missing file operand")
		return 1
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]
	if cfg.targetDir != "" {
		dest = cfg.targetDir
	}
	return dispatch(cfg, sources, dest)
}

// dispatch routes to single-file or multi-source copy.
// R3.4: multi-source to directory copy.
func dispatch(cfg config, sources []string, dest string) int {
	if len(sources) == 1 && !isDir(dest) {
		return copySingle(cfg, sources[0], dest)
	}
	return copyMultiple(cfg, sources, dest)
}

// copyMultiple copies multiple sources into a destination directory.
func copyMultiple(cfg config, sources []string, dest string) int {
	if !isDir(dest) {
		fmt.Fprintf(os.Stderr,
			"cp: target '%s' is not a directory\n", dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if copySingle(cfg, src, target) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// copySingle copies a single source to a destination path.
// R2.3/R2.4: handles symlink dereference decision.
// R4.2: detects copy-to-self by comparing source and destination inodes.
func copySingle(cfg config, src, dest string) int {
	info, err := os.Lstat(src)
	if err != nil {
		printErr("cannot stat '%s': %v", src, unwrapErr(err))
		return 1
	}
	if isSymlink(info) && !shouldDereference(cfg) {
		return copySymlink(cfg, src, dest)
	}
	if isSymlink(info) {
		info, err = os.Stat(src)
		if err != nil {
			printErr("cannot stat '%s': %v", src, unwrapErr(err))
			return 1
		}
	}
	if info.IsDir() {
		return copyDirectory(cfg, src, dest, info)
	}
	if isSameFile(src, dest) {
		printErr("'%s' and '%s' are the same file", src, dest)
		return 1
	}
	return copyFile(cfg, src, dest, info)
}

// isSameFile reports whether src and dest refer to the same inode.
// R4.2: detects copy-to-self to prevent data loss.
func isSameFile(src, dest string) bool {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false
	}
	destInfo, err := os.Stat(dest)
	if err != nil {
		return false
	}
	return os.SameFile(srcInfo, destInfo)
}

// isSymlink reports whether the file info indicates a symbolic link.
func isSymlink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

// shouldDereference returns true if symlinks should be followed.
// R2.3: -L always follows. R2.4: -P/-d never follows; default with -r.
func shouldDereference(cfg config) bool {
	if cfg.dereference {
		return true
	}
	if cfg.noDereference {
		return false
	}
	// R2.4: default is no-dereference when recursive
	return !cfg.recursive
}

// copyDirectory handles directory source arguments.
// R2.1: recursive copy. R2.2: error without -r.
func copyDirectory(cfg config, src, dest string, info fs.FileInfo) int {
	if !cfg.recursive {
		printErr("-r not specified; omitting directory '%s'", src)
		return 1
	}
	return copyDirRecursive(cfg, src, dest, info)
}

// copyDirRecursive walks src and recreates the tree under dest.
func copyDirRecursive(cfg config, src, dest string, info fs.FileInfo) int {
	if err := os.MkdirAll(dest, info.Mode()); err != nil {
		printErr("cannot create directory '%s': %v", dest, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(src, dest)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		printErr("cannot read directory '%s': %v", src, unwrapErr(err))
		return 1
	}
	exitCode := 0
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dest, entry.Name())
		if copySingle(cfg, s, d) != 0 {
			exitCode = 1
		}
	}
	preserveAttrs(cfg, src, dest)
	return exitCode
}

// copySymlink copies a symlink as a symlink without following it.
// R2.4: -P/--no-dereference preserves symlinks.
func copySymlink(cfg config, src, dest string) int {
	if shouldSkipExisting(cfg, dest) {
		return 0
	}
	target, err := os.Readlink(src)
	if err != nil {
		printErr("cannot read symlink '%s': %v", src, unwrapErr(err))
		return 1
	}
	if fileExists(dest) {
		if err := os.Remove(dest); err != nil {
			printErr("cannot remove '%s': %v", dest, unwrapErr(err))
			return 1
		}
	}
	if err := os.Symlink(target, dest); err != nil {
		printErr("cannot create symlink '%s': %v", dest, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(src, dest)
	}
	return 0
}

// copyFile copies a regular file from src to dest.
// R1.4: -n skips if dest exists. R3.1: preserves attrs if -p.
func copyFile(cfg config, src, dest string, info os.FileInfo) int {
	if shouldSkipExisting(cfg, dest) {
		return 0
	}
	if err := performCopy(cfg, src, dest, info.Mode()); err != nil {
		printErr("%v", err)
		return 1
	}
	if cfg.verbose {
		printVerbose(src, dest)
	}
	preserveAttrs(cfg, src, dest)
	return 0
}

// shouldSkipExisting checks -n and -i flags against an existing dest.
// R1.4: -n takes precedence over -i.
func shouldSkipExisting(cfg config, dest string) bool {
	if !fileExists(dest) {
		return false
	}
	if cfg.noClobber {
		return true
	}
	if cfg.interactive {
		return !confirmOverwrite(dest)
	}
	return false
}

// confirmOverwrite prompts the user on stderr and reads response.
// R1.2: prompt before overwriting.
func confirmOverwrite(dest string) bool {
	fmt.Fprintf(os.Stderr, "cp: overwrite '%s'? ", dest)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(response)
	return strings.HasPrefix(response, "y") || strings.HasPrefix(response, "Y")
}

// performCopy opens source and writes to dest, handling -f retry.
func performCopy(cfg config, src, dest string, srcMode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %v",
			src, unwrapErr(err))
	}
	defer srcFile.Close()
	return writeDestFile(cfg, srcFile, dest, srcMode)
}

// writeDestFile creates or overwrites the destination file.
// R1.3: if -f and dest cannot be opened, remove and retry.
func writeDestFile(
	cfg config, srcFile *os.File, dest string, srcMode os.FileMode,
) error {
	dstFile, err := createDest(dest, srcMode)
	if err != nil && cfg.force {
		dstFile, err = forceCreateDest(dest, srcMode)
	}
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %v",
			dest, unwrapErr(err))
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("error writing '%s': %v", dest, unwrapErr(err))
	}
	return nil
}

// createDest opens the destination file for writing.
func createDest(dest string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
}

// forceCreateDest removes the destination and retries creation.
// R1.3: -f removes existing destination that cannot be opened.
func forceCreateDest(dest string, mode os.FileMode) (*os.File, error) {
	if err := os.Remove(dest); err != nil {
		return nil, err
	}
	return createDest(dest, mode)
}

// preserveAttrs applies mode, ownership, and timestamps from src to dest.
// R3.1/R3.2/R3.3: preserve selected attributes after copy.
func preserveAttrs(cfg config, src, dest string) {
	if cfg.preserve == "" {
		return
	}
	attrs := parsePreserveAttrs(cfg.preserve)
	si, err := sys.Stat(src)
	if err != nil {
		return // cannot stat source for preservation
	}
	if attrs.mode {
		if err := os.Chmod(dest, si.Mode.Perm()); err != nil {
			printErr("preserving permissions for '%s': %v",
				dest, unwrapErr(err))
		}
	}
	if attrs.timestamps {
		if err := os.Chtimes(dest, si.AccessTime, si.ModTime); err != nil {
			printErr("preserving timestamps for '%s': %v",
				dest, unwrapErr(err))
		}
	}
	if attrs.ownership {
		// Ownership preservation often fails for non-root; warn only
		if err := os.Lchown(dest, int(si.Uid), int(si.Gid)); err != nil {
			printErr("preserving ownership for '%s': %v",
				dest, unwrapErr(err))
		}
	}
}

// parsePreserveAttrs converts a preserve attribute string to flags.
// R3.3: supports comma-separated list and "all" keyword.
func parsePreserveAttrs(preserve string) preserveFlags {
	if preserve == "all" {
		return preserveFlags{mode: true, ownership: true, timestamps: true}
	}
	var pf preserveFlags
	for _, attr := range strings.Split(preserve, ",") {
		switch strings.TrimSpace(attr) {
		case "mode":
			pf.mode = true
		case "ownership":
			pf.ownership = true
		case "timestamps":
			pf.timestamps = true
		}
	}
	return pf
}

// printVerbose prints the copy operation to stdout in GNU cp format.
// R3.4: verbose output for each file copied.
func printVerbose(src, dest string) {
	fmt.Printf("'%s' -> '%s'\n", src, dest)
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

// printErr prints a formatted error to stderr in GNU cp format.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cp: "+format+"\n", args...)
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
		cfg.interactive = true
	case arg == "--force":
		cfg.force = true
	case arg == "--no-clobber":
		cfg.noClobber = true
	case arg == "--recursive":
		cfg.recursive = true
	case arg == "--dereference":
		cfg.dereference = true
	case arg == "--no-dereference":
		cfg.noDereference = true
	case arg == "--archive":
		applyArchive(cfg)
	case arg == "--verbose":
		cfg.verbose = true
	case strings.HasPrefix(arg, "--preserve"):
		return parsePreserveFlag(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--target-directory"):
		return parseTargetDirFlag(arg, args, i, cfg)
	default:
		fmt.Fprintf(os.Stderr, "cp: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// parseShortFlags processes clustered short flags like -rfi.
func parseShortFlags(
	flags string, args []string, i *int, cfg *config,
) int {
	for j := 0; j < len(flags); j++ {
		if exit := applyShortFlag(
			flags[j], flags[j+1:], args, i, cfg,
		); exit >= 0 {
			return exit
		}
		if consumesRemainder(flags[j]) {
			break
		}
	}
	return -1
}

// consumesRemainder returns true for short flags that consume the rest.
func consumesRemainder(ch byte) bool {
	return ch == 't'
}

// applyShortFlag applies a single short flag character.
func applyShortFlag(
	ch byte, remainder string, args []string, i *int, cfg *config,
) int {
	switch ch {
	case 'i':
		cfg.interactive = true
	case 'f':
		cfg.force = true
	case 'n':
		cfg.noClobber = true
	case 'r', 'R':
		cfg.recursive = true
	case 'L':
		cfg.dereference = true
	case 'P':
		cfg.noDereference = true
	case 'd':
		cfg.noDereference = true
	case 'p':
		cfg.preserve = "mode,ownership,timestamps"
	case 'a':
		applyArchive(cfg)
	case 'v':
		cfg.verbose = true
	case 't':
		return consumeTargetDir(remainder, args, i, cfg)
	default:
		fmt.Fprintf(os.Stderr, "cp: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// applyArchive sets flags equivalent to -dR --preserve=all.
// R3.2: -a is -dR --preserve=all.
func applyArchive(cfg *config) {
	cfg.recursive = true
	cfg.noDereference = true
	cfg.preserve = "all"
}

// parsePreserveFlag handles --preserve and --preserve=ATTR_LIST.
func parsePreserveFlag(
	arg string, args []string, i *int, cfg *config,
) int {
	if strings.HasPrefix(arg, "--preserve=") {
		cfg.preserve = arg[len("--preserve="):]
		return -1
	}
	if *i+1 >= len(args) {
		cfg.preserve = "mode,ownership,timestamps"
		return -1
	}
	*i++
	cfg.preserve = args[*i]
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
			"cp: option requires an argument -- 'target-directory'")
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
			"cp: option requires an argument -- 't'")
		return 1
	}
	*i++
	cfg.targetDir = args[*i]
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: cp [OPTION]... SOURCE... DEST
Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.

  -a, --archive                same as -dR --preserve=all
  -d                           same as --no-dereference
  -f, --force                  if an existing destination file cannot be
                                 opened, remove it and try again
  -i, --interactive            prompt before overwrite
  -L, --dereference            always follow symbolic links in SOURCE
  -n, --no-clobber             do not overwrite an existing file
  -P, --no-dereference         never follow symbolic links in SOURCE
  -p                           same as --preserve=mode,ownership,timestamps
      --preserve[=ATTR_LIST]   preserve the specified attributes
  -r, -R, --recursive          copy directories recursively
  -t, --target-directory=DIR   copy all SOURCE arguments into DIR
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
	_, err := fmt.Fprintf(os.Stdout, "cp (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
