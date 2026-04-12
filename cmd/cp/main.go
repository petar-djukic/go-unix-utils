// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cp: copy files and directories.
// Implements srd056 R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
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
var errDeclined = errors.New("declined")

// errPartialCopy indicates some entries in a recursive copy failed.
var errPartialCopy = errors.New("partial copy")

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

// copyState holds shared mutable state for a copy operation tree.
type copyState struct {
	opts      options
	prs       preserveSet
	hardLinks map[devIno]string
}

// main entry point with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	os.Exit(run(opts, args))
}

// run dispatches the copy operation based on parsed options and args.
func run(opts options, args []string) int {
	if len(args) == 0 {
		printMissingOperand()
		return 1
	}
	if len(args) == 1 && opts.targetDir == "" {
		printMissingDest(args[0])
		return 1
	}
	return copyFiles(opts, args)
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand() {
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

// copyFiles creates a copyState and dispatches all source copies.
// R1.1: copies SOURCE to DEST, or multiple SOURCEs into DEST directory.
func copyFiles(opts options, args []string) int {
	cs := &copyState{
		opts:      opts,
		prs:       parsePreserve(opts.preserve),
		hardLinks: make(map[devIno]string),
	}
	sources, dest := splitSourcesDest(cs.opts, args)
	return copyAllSources(cs, sources, dest)
}

// copyAllSources copies each source to dest, accumulating exit codes.
func copyAllSources(cs *copyState, sources []string, dest string) int {
	destIsDir := isDir(dest)
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr,
			"%s: target '%s': Not a directory\n", programName, dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := resolveTarget(dest, src, destIsDir)
		if err := copySingle(cs, src, target); err != nil {
			reportErr(err)
			exitCode = 1
		}
	}
	return exitCode
}

// reportErr prints err to stderr unless it is a sentinel that was already
// handled (errDeclined, errPartialCopy).
func reportErr(err error) {
	if errors.Is(err, errDeclined) || errors.Is(err, errPartialCopy) {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
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

// shouldDeref returns true when symlinks should be followed.
// R2.3: -L forces dereference. R2.4: -P forces no-dereference.
// Default with -r is -P (no dereference); without -r, follow symlinks.
func shouldDeref(opts options) bool {
	if opts.dereference {
		return true
	}
	if opts.noDereference {
		return false
	}
	return !opts.recursive
}

// sysStatSource stats path using pkg/sys for extended file metadata.
// D4: uses pkg/sys for platform-specific file metadata.
func sysStatSource(path string, deref bool) (*sys.FileInfo, error) {
	if deref {
		return sys.Stat(path)
	}
	return sys.Lstat(path)
}

// copySingle copies a single source to destination, dispatching by type.
// R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
func copySingle(cs *copyState, src, dest string) error {
	srcInfo, err := sysStatSource(src, shouldDeref(cs.opts))
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}
	if srcInfo.Mode&os.ModeSymlink != 0 {
		return copySymlink(cs, src, dest)
	}
	if srcInfo.Mode.IsDir() {
		return copyDirOrRefuse(cs, src, dest)
	}
	return copyRegEntry(cs, src, dest, srcInfo)
}

// copyRegEntry handles no-clobber, interactive, hard links, and file copy.
// R3.3: checks for hard link opportunity before copying.
// R3.4: prints verbose message before copy.
func copyRegEntry(cs *copyState, src, dest string, info *sys.FileInfo) error {
	if skipForNoClobber(cs.opts, dest) {
		return nil
	}
	if skipForInteractive(cs.opts, dest) {
		return errDeclined
	}
	if prev, ok := checkHardLink(cs, info, dest); ok {
		verbosePrint(cs.opts, src, dest)
		return makeHardLink(prev, dest)
	}
	verbosePrint(cs.opts, src, dest)
	if err := copyFileContent(src, dest, cs.opts); err != nil {
		return err
	}
	return applyFilePreserve(cs.prs, dest, info)
}

// makeHardLink creates a hard link, removing dest if it exists.
func makeHardLink(from, to string) error {
	os.Remove(to) // best-effort removal of existing
	if err := os.Link(from, to); err != nil {
		return fmt.Errorf("cannot create hard link '%s': %s",
			to, sysErrMsg(err))
	}
	return nil
}

// copyDirOrRefuse copies a directory recursively or returns an error.
// R2.1: recursive copy when -r is set. R2.2: error without -r.
func copyDirOrRefuse(cs *copyState, src, dest string) error {
	if !cs.opts.recursive {
		return fmt.Errorf(
			"-r not specified; omitting directory '%s'", src)
	}
	return copyDir(cs, src, dest)
}

// copyDir creates the destination directory, copies entries, then preserves.
// R2.1: preserves directory structure. R3.1: preserves dir attributes.
func copyDir(cs *copyState, src, dest string) error {
	srcInfo, err := sys.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}
	if err := mkDestDir(dest, srcInfo); err != nil {
		return err
	}
	verbosePrint(cs.opts, src, dest)
	cpErr := copyDirEntries(cs, src, dest)
	// R3.1: apply preserve after entries so timestamps reflect final state.
	pErr := applyFilePreserve(cs.prs, dest, srcInfo)
	if cpErr != nil {
		return cpErr
	}
	return pErr
}

// mkDestDir creates the destination directory with source permissions.
func mkDestDir(dest string, srcInfo *sys.FileInfo) error {
	err := os.Mkdir(dest, srcInfo.Mode.Perm())
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("cannot create directory '%s': %s",
			dest, sysErrMsg(err))
	}
	return nil
}

// copyDirEntries reads and copies each entry from src to dest.
// Prints errors for individual entries and returns errPartialCopy on failure.
func copyDirEntries(cs *copyState, src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot open directory '%s' for reading: %s",
			src, sysErrMsg(err))
	}
	hadErr := false
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if err := copySingle(cs, srcPath, destPath); err != nil {
			reportErr(err)
			hadErr = true
		}
	}
	if hadErr {
		return errPartialCopy
	}
	return nil
}

// copySymlink copies a symbolic link, preserving it as a symlink.
// R2.4: reads the link target and creates a new symlink at dest.
// R3.4: prints verbose message after creating symlink.
func copySymlink(cs *copyState, src, dest string) error {
	if skipForNoClobber(cs.opts, dest) {
		return nil
	}
	if skipForInteractive(cs.opts, dest) {
		return errDeclined
	}
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("cannot read symlink '%s': %s",
			src, sysErrMsg(err))
	}
	os.Remove(dest) // best-effort removal to avoid EEXIST
	if err := os.Symlink(target, dest); err != nil {
		return fmt.Errorf("cannot create symlink '%s': %s",
			dest, sysErrMsg(err))
	}
	verbosePrint(cs.opts, src, dest)
	return nil
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

// copyFileContent copies file data from src to dest.
// R1.3: uses createDest which handles -f removal.
func copyFileContent(src, dest string, opts options) error {
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
	return finishCopy(in, out, dest)
}

// finishCopy performs the data copy and closes the output file.
func finishCopy(in, out *os.File, dest string) error {
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

// verbosePrint prints the copy operation when -v is set.
// R3.4: prints "'src' -> 'dest'" to stdout.
func verbosePrint(opts options, src, dest string) {
	if opts.verbose {
		fmt.Printf("'%s' -> '%s'\n", src, dest)
	}
}

// sysErrMsg extracts the system error message from an os error,
// capitalizing the first letter to match GNU coreutils format.
func sysErrMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
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

// parseArgs separates flags from positional arguments.
// Supports short flags, combined short flags, and long forms.
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
		setArchive(opts)
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

// setArchive applies -a flag settings.
// R3.2: -a is equivalent to -dR --preserve=all.
func setArchive(opts *options) {
	opts.archive = true
	opts.recursive = true
	opts.noDereference = true
	opts.preserve = "all"
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
			setArchive(opts)
		case 'v':
			opts.verbose = true
		case 'd':
			opts.noDereference = true
			opts.preserve = mergePreserveLinks(opts.preserve)
		case 't':
			idx = parseTargetDirShort(opts, chars[j+1:], rawArgs, idx)
			return idx
		}
	}
	return idx
}

// mergePreserveLinks adds "links" to the preserve string if not already present.
func mergePreserveLinks(current string) string {
	if current == "" {
		return "links"
	}
	if current == "all" || strings.Contains(current, "links") {
		return current
	}
	return current + ",links"
}

// parseTargetDirShort handles -t with remaining chars or next arg.
func parseTargetDirShort(opts *options, rest string, rawArgs []string, idx int) int {
	if len(rest) > 0 {
		opts.targetDir = rest
	} else if idx+1 < len(rawArgs) {
		idx++
		opts.targetDir = rawArgs[idx]
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
