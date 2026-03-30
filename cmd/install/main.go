// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/install implements GNU install: copy files and set attributes.
//
// Implements prd101-install R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "install"
const defaultBackupSuffix = "~"
const defaultMode = os.FileMode(0o755)

const helpText = `Usage: install [OPTION]... [-T] SOURCE DEST
  or:  install [OPTION]... SOURCE... DIRECTORY
  or:  install [OPTION]... -t DIRECTORY SOURCE...
  or:  install [OPTION]... -d DIRECTORY...

Copy files and set attributes.

      --backup[=CONTROL]  make a backup of each existing destination file
  -b                  like --backup but does not accept an argument
  -C, --compare       compare source and destination, do not install if identical
  -d, --directory     treat all arguments as directory names; create all
                        components of the specified directories
  -D                  create all leading components of DEST except the last,
                        or all components of --target-directory,
                        then copy SOURCE to DEST
  -g, --group=GROUP   set group ownership, instead of process' current group
  -m, --mode=MODE     set permission mode (as in chmod), instead of rwxr-xr-x
  -o, --owner=OWNER   set ownership (super-user only)
  -v, --verbose       print the name of each directory as it is created
  -S, --suffix=SUFFIX  override the usual backup suffix
  -t, --target-directory=DIRECTORY  copy all SOURCE arguments into DIRECTORY
  -T, --no-target-directory  treat DEST as a normal file
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "install (go-unix-utils) 0.1\n"

type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp
	parseVer
)

type backupControl int

const (
	backupNone     backupControl = iota
	backupSimple
	backupNumbered
	backupExisting
)

// installOptions holds parsed command-line flags.
// R1.1: mode, R1.2: modeSet, R1.3: owner, R1.4: group.
// R3.1: compare.
type installOptions struct {
	mode          os.FileMode
	modeSet       bool
	owner         string
	group         string
	directory     bool
	targetDir     string
	noTargetDir   bool
	backupCtrl    backupControl
	suffix        string
	createLeading bool
	verbose       bool
	compare       bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the install logic and returns the exit code.
// R3.2: returns 0 on success, 1 on any error.
func run(args []string, stdout, stderr *os.File) int {
	opts, operands, result, err := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0
	}
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1
	}
	if opts.directory {
		return installDirectories(operands, opts, stdout, stderr)
	}
	return installFiles(operands, opts, stdout, stderr)
}

// installDirectories creates directories with -d mode (R2.1).
func installDirectories(dirs []string, opts installOptions, stdout, stderr *os.File) int {
	if len(dirs) == 0 {
		printError(stderr, "missing file operand")
		printTryHelp(stderr)
		return 1
	}
	mode := defaultMode
	if opts.modeSet {
		mode = opts.mode
	}
	exitCode := 0
	for _, dir := range dirs {
		if createDir(dir, mode, opts, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// createDir creates a single directory and its missing parents, applying
// mode and ownership to each newly created component (R2.1).
func createDir(dir string, mode os.FileMode, opts installOptions, stdout, stderr *os.File) int {
	for _, comp := range pathComponents(dir) {
		if isDir(comp) {
			continue
		}
		if mkdirComponent(comp, mode, opts, stdout, stderr) != 0 {
			return 1
		}
	}
	// R1.2: Always apply mode to the target dir, even if it existed.
	if err := os.Chmod(dir, mode); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot set permissions on '%s': %s", dir, stripPathError(err)))
		return 1
	}
	return applyOwnership(dir, opts, stderr)
}

// mkdirComponent creates a single directory component with mode and ownership.
func mkdirComponent(comp string, mode os.FileMode, opts installOptions, stdout, stderr *os.File) int {
	if err := os.Mkdir(comp, mode); err != nil && !os.IsExist(err) {
		printError(stderr, fmt.Sprintf(
			"cannot create directory '%s': %s", comp, stripPathError(err)))
		return 1
	}
	// R1.2: Chmod explicitly because Mkdir is affected by umask.
	if err := os.Chmod(comp, mode); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot set permissions on '%s': %s", comp, stripPathError(err)))
		return 1
	}
	if applyOwnership(comp, opts, stderr) != 0 {
		return 1
	}
	// R2.4: Print each created directory when verbose.
	if opts.verbose {
		fmt.Fprintf(stdout, "install: creating directory '%s'\n", comp) //nolint:errcheck
	}
	return 0
}

// createLeadingDirs creates leading directory components for -D mode (R2.2).
// Uses default 0755 mode for leading directories, not the -m mode.
func createLeadingDirs(dir string, opts installOptions, stdout, stderr *os.File) int {
	for _, comp := range pathComponents(dir) {
		if isDir(comp) {
			continue
		}
		if err := os.Mkdir(comp, 0o755); err != nil && !os.IsExist(err) {
			printError(stderr, fmt.Sprintf(
				"cannot create directory '%s': %s", comp, stripPathError(err)))
			return 1
		}
		// R2.4: Print each created directory when verbose.
		if opts.verbose {
			fmt.Fprintf(stdout, "install: creating directory '%s'\n", comp) //nolint:errcheck
		}
	}
	return 0
}

// pathComponents returns cumulative path components from root to leaf.
// Example: "a/b/c" → ["a", "a/b", "a/b/c"].
func pathComponents(dir string) []string {
	dir = filepath.Clean(dir)
	var components []string
	current := dir
	for current != "." && current != "/" {
		components = append([]string{current}, components...)
		current = filepath.Dir(current)
	}
	return components
}

// installFiles dispatches file copy based on flags and operand count.
func installFiles(operands []string, opts installOptions, stdout, stderr *os.File) int {
	if opts.targetDir != "" && opts.noTargetDir {
		printError(stderr, "cannot combine --target-directory and --no-target-directory")
		printTryHelp(stderr)
		return 1
	}
	if opts.targetDir != "" {
		return installIntoDir(operands, opts.targetDir, opts, stdout, stderr)
	}
	if len(operands) < 2 {
		return missingOperandError(operands, stderr)
	}
	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]
	if !opts.noTargetDir && (len(sources) > 1 || isDir(dest)) {
		return installIntoDir(sources, dest, opts, stdout, stderr)
	}
	if len(sources) > 1 {
		printError(stderr, fmt.Sprintf("extra operand '%s'", sources[1]))
		printTryHelp(stderr)
		return 1
	}
	return installSingleFile(sources[0], dest, opts, stdout, stderr)
}

// missingOperandError reports a missing operand error.
func missingOperandError(operands []string, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, "missing file operand")
	} else {
		printError(stderr, fmt.Sprintf(
			"missing destination file operand after '%s'", operands[0]))
	}
	printTryHelp(stderr)
	return 1
}

// installIntoDir copies all sources into the target directory.
func installIntoDir(sources []string, dir string, opts installOptions, stdout, stderr *os.File) int {
	if opts.createLeading {
		if createLeadingDirs(dir, opts, stdout, stderr) != 0 {
			return 1
		}
	}
	if !isDir(dir) {
		printError(stderr, fmt.Sprintf("target '%s' is not a directory", dir))
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		dest := filepath.Join(dir, filepath.Base(src))
		if installSingleFile(src, dest, opts, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// installSingleFile copies one source file to dest with attributes.
// R1.1: copy with mode. R2.2: create leading dirs. R2.3: backup before overwrite.
// R3.1: skip if -C and source/dest are identical.
func installSingleFile(src, dest string, opts installOptions, stdout, stderr *os.File) int {
	if opts.createLeading {
		dir := filepath.Dir(dest)
		if createLeadingDirs(dir, opts, stdout, stderr) != 0 {
			return 1
		}
	}
	mode := defaultMode
	if opts.modeSet {
		mode = opts.mode
	}
	// R3.1: skip install if -C and files are identical.
	if opts.compare && filesEqual(src, dest, mode) {
		return 0
	}
	if opts.backupCtrl != backupNone {
		if err := makeBackup(dest, opts); err != nil {
			printError(stderr, fmt.Sprintf(
				"cannot backup '%s': %s", dest, err))
			return 1
		}
	}
	if err := copyFile(src, dest, mode); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot install '%s' to '%s': %s", src, dest, stripPathError(err)))
		return 1
	}
	if applyOwnership(dest, opts, stderr) != 0 {
		return 1
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "'%s' -> '%s'\n", src, dest) //nolint:errcheck
	}
	return 0
}

// filesEqual reports whether src and dest have identical content and
// permissions. Returns false if either file cannot be read (R3.1).
func filesEqual(src, dest string, mode os.FileMode) bool {
	destInfo, err := os.Stat(dest)
	if err != nil {
		return false
	}
	if destInfo.Mode().Perm() != mode.Perm() {
		return false
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false
	}
	if srcInfo.Size() != destInfo.Size() {
		return false
	}
	return contentEqual(src, dest)
}

// contentEqual compares file contents byte-for-byte.
func contentEqual(pathA, pathB string) bool {
	a, err := os.ReadFile(pathA)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(pathB)
	if err != nil {
		return false
	}
	return bytes.Equal(a, b)
}

// copyFile reads src and writes to dest with the given mode.
// D3: always copy (not rename) to match GNU install behavior.
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	destFile, err := os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close() //nolint:errcheck // close after copy error
		return err
	}
	if err := destFile.Close(); err != nil {
		return err
	}
	// R1.2: set mode explicitly to override umask.
	return os.Chmod(dest, mode)
}

// makeBackup creates a backup of dest if it exists (R2.3).
func makeBackup(dest string, opts installOptions) error {
	if _, err := os.Lstat(dest); err != nil {
		return nil // no file to back up
	}
	backupPath := resolveBackupPath(dest, opts)
	return os.Rename(dest, backupPath)
}

// resolveBackupPath determines the backup file path based on control.
func resolveBackupPath(dest string, opts installOptions) string {
	suffix := opts.suffix
	if suffix == "" {
		suffix = defaultBackupSuffix
	}
	switch opts.backupCtrl {
	case backupNumbered:
		return numberedBackupPath(dest)
	case backupExisting:
		if hasNumberedBackup(dest) {
			return numberedBackupPath(dest)
		}
		return dest + suffix
	default:
		return dest + suffix
	}
}

// numberedBackupPath finds the next available numbered backup path.
func numberedBackupPath(dest string) string {
	for i := 1; ; i++ {
		path := fmt.Sprintf("%s.~%d~", dest, i)
		if _, err := os.Lstat(path); err != nil {
			return path
		}
	}
}

// hasNumberedBackup checks if any numbered backup exists for dest.
func hasNumberedBackup(dest string) bool {
	_, err := os.Lstat(dest + ".~1~")
	return err == nil
}

// applyOwnership sets owner and group on path (R1.3, R1.4).
func applyOwnership(path string, opts installOptions, stderr *os.File) int {
	uid := -1
	gid := -1
	if opts.owner != "" {
		u, err := lookupUser(opts.owner)
		if err != nil {
			printError(stderr, fmt.Sprintf("invalid user '%s'", opts.owner))
			return 1
		}
		uid = u
	}
	if opts.group != "" {
		g, err := lookupGroup(opts.group)
		if err != nil {
			printError(stderr, fmt.Sprintf("invalid group '%s'", opts.group))
			return 1
		}
		gid = g
	}
	if uid == -1 && gid == -1 {
		return 0
	}
	if err := os.Chown(path, uid, gid); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot change ownership of '%s': %s", path, stripPathError(err)))
		return 1
	}
	return 0
}

// lookupUser resolves a username or numeric UID to a UID integer.
func lookupUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(u.Uid)
}

// lookupGroup resolves a group name or numeric GID to a GID integer.
func lookupGroup(name string) (int, error) {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(g.Gid)
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (installOptions, []string, parseResult, error) {
	opts := installOptions{suffix: defaultBackupSuffix}
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
			consumed, result, err := parseLongFlag(arg, args[i+1:], &opts)
			if result != parseOK {
				return opts, nil, result, nil
			}
			if err != nil {
				return opts, nil, parseOK, err
			}
			i += consumed
			continue
		}
		consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
		if err != nil {
			return opts, nil, parseOK, err
		}
		i += consumed
	}
	return opts, operands, parseOK, nil
}

// parseLongFlag handles long-form flags.
func parseLongFlag(flag string, remaining []string, opts *installOptions) (int, parseResult, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--help":
		return 0, parseHelp, nil
	case "--version":
		return 0, parseVer, nil
	case "--directory":
		opts.directory = true
	case "--verbose":
		opts.verbose = true
	case "--compare":
		opts.compare = true
	case "--no-target-directory":
		opts.noTargetDir = true
	case "--backup":
		return parseLongBackup(value, hasValue, opts)
	case "--mode":
		return parseLongMode(value, hasValue, remaining, opts)
	case "--owner":
		return parseLongStringOpt(value, hasValue, remaining, "owner", &opts.owner)
	case "--group":
		return parseLongStringOpt(value, hasValue, remaining, "group", &opts.group)
	case "--target-directory":
		return parseLongStringOpt(value, hasValue, remaining, "target-directory", &opts.targetDir)
	case "--suffix":
		return parseLongStringOpt(value, hasValue, remaining, "suffix", &opts.suffix)
	default:
		return 0, parseOK, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, parseOK, nil
}

// parseLongBackup handles --backup and --backup=CONTROL.
func parseLongBackup(value string, hasValue bool, opts *installOptions) (int, parseResult, error) {
	if hasValue {
		ctrl, err := parseBackupControl(value)
		if err != nil {
			return 0, parseOK, err
		}
		opts.backupCtrl = ctrl
	} else {
		opts.backupCtrl = backupExisting
	}
	return 0, parseOK, nil
}

// parseLongMode handles --mode and --mode=MODE.
func parseLongMode(value string, hasValue bool, remaining []string, opts *installOptions) (int, parseResult, error) {
	var modeStr string
	consumed := 0
	if hasValue {
		modeStr = value
	} else if len(remaining) > 0 {
		modeStr = remaining[0]
		consumed = 1
	} else {
		return 0, parseOK, fmt.Errorf("option '--mode' requires an argument")
	}
	m, err := parseMode(modeStr)
	if err != nil {
		return consumed, parseOK, err
	}
	opts.mode = m
	opts.modeSet = true
	return consumed, parseOK, nil
}

// parseLongStringOpt handles a long flag that takes a string argument.
func parseLongStringOpt(value string, hasValue bool, remaining []string, name string, target *string) (int, parseResult, error) {
	if hasValue {
		*target = value
		return 0, parseOK, nil
	}
	if len(remaining) > 0 {
		*target = remaining[0]
		return 1, parseOK, nil
	}
	return 0, parseOK, fmt.Errorf("option '--%s' requires an argument", name)
}

// parseShortFlags handles short flags and combined forms.
func parseShortFlags(flags string, remaining []string, opts *installOptions) (int, error) {
	consumed := 0
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'd':
			opts.directory = true
		case 'v':
			opts.verbose = true
		case 'b':
			if opts.backupCtrl == backupNone {
				opts.backupCtrl = backupSimple
			}
		case 'C':
			opts.compare = true
		case 'D':
			opts.createLeading = true
		case 'T':
			opts.noTargetDir = true
		case 'm':
			return parseShortMode(flags[i+1:], remaining, consumed, opts)
		case 'o':
			return parseShortStringOpt(flags[i+1:], remaining, consumed, 'o', &opts.owner)
		case 'g':
			return parseShortStringOpt(flags[i+1:], remaining, consumed, 'g', &opts.group)
		case 't':
			return parseShortStringOpt(flags[i+1:], remaining, consumed, 't', &opts.targetDir)
		case 'S':
			return parseShortStringOpt(flags[i+1:], remaining, consumed, 'S', &opts.suffix)
		default:
			return consumed, fmt.Errorf("invalid option -- '%c'", flags[i])
		}
	}
	return consumed, nil
}

// parseShortMode handles -m flag with its value.
func parseShortMode(rest string, remaining []string, consumed int, opts *installOptions) (int, error) {
	var modeStr string
	if rest != "" {
		modeStr = rest
	} else if len(remaining) > consumed {
		modeStr = remaining[consumed]
		consumed++
	} else {
		return consumed, fmt.Errorf("option requires an argument -- 'm'")
	}
	m, err := parseMode(modeStr)
	if err != nil {
		return consumed, err
	}
	opts.mode = m
	opts.modeSet = true
	return consumed, nil
}

// parseShortStringOpt handles a short flag that takes a string argument.
func parseShortStringOpt(rest string, remaining []string, consumed int, flag byte, target *string) (int, error) {
	if rest != "" {
		*target = rest
		return consumed, nil
	}
	if len(remaining) > consumed {
		*target = remaining[consumed]
		return consumed + 1, nil
	}
	return consumed, fmt.Errorf("option requires an argument -- '%c'", flag)
}

// parseMode parses an octal permission mode string.
func parseMode(s string) (os.FileMode, error) {
	mode, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode '%s'", s)
	}
	return os.FileMode(mode), nil
}

// parseBackupControl parses a --backup=CONTROL value.
func parseBackupControl(s string) (backupControl, error) {
	switch s {
	case "none", "off":
		return backupNone, nil
	case "numbered", "t":
		return backupNumbered, nil
	case "existing", "nil":
		return backupExisting, nil
	case "simple", "never":
		return backupSimple, nil
	default:
		return backupNone, fmt.Errorf("invalid backup type '%s'", s)
	}
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

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}
