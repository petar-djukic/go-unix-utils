// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd101-install: Copy files and set attributes.
// R1.1 (basic file copy with 755 default), R1.2 (-m mode),
// R1.3 (-o owner), R1.4 (-g group), R2.1 (-d directory creation),
// R3.2 (exit codes), R3.3 (SIGPIPE handling).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// defaultMode is the default permission mode for installed files.
// R1.1: 755 by default, matching GNU install.
const defaultMode os.FileMode = 0o755

// config holds the parsed command-line flags for install.
type config struct {
	dirMode    bool   // -d: create directories
	mode       string // -m MODE: permission mode (octal)
	owner      string // -o OWNER
	group      string // -g GROUP
	verbose    bool   // -v
	targetDir  string // -t DIR
	createDirs bool   // -D: create leading dirs
	compare    bool   // -C: compare before install
	backup     bool   // -b: backup existing files
	suffix     string // --suffix: backup suffix
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, args))
}

// run dispatches to directory mode or copy mode.
// R3.2: exit 0 on success, exit 1 on error.
func run(cfg config, args []string) int {
	if cfg.dirMode {
		return runDirMode(cfg, args)
	}
	return runCopyMode(cfg, args)
}

// runDirMode creates directories with -d flag.
// R2.1: create each given directory and any missing parents.
func runDirMode(cfg config, args []string) int {
	if len(args) == 0 {
		printErr("missing file operand")
		return 1
	}
	exitCode := 0
	perm := resolveMode(cfg)
	for _, dir := range args {
		if mkdirWithVerbose(cfg, dir, perm) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// mkdirWithVerbose creates a directory and prints verbose output.
func mkdirWithVerbose(cfg config, dir string, perm os.FileMode) int {
	if err := os.MkdirAll(dir, perm); err != nil {
		printErr("cannot create directory '%s': %v",
			dir, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		fmt.Fprintf(os.Stdout,
			"install: creating directory '%s'\n", dir)
	}
	return 0
}

// runCopyMode handles the file installation mode.
func runCopyMode(cfg config, args []string) int {
	if len(args) < 2 && cfg.targetDir == "" {
		printErr("missing file operand")
		return 1
	}
	if cfg.targetDir != "" {
		return installToTarget(cfg, args)
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]
	return dispatch(cfg, sources, dest)
}

// installToTarget copies sources into the -t target directory.
func installToTarget(cfg config, sources []string) int {
	if len(sources) == 0 {
		printErr("missing file operand")
		return 1
	}
	return installMultiple(cfg, sources, cfg.targetDir)
}

// dispatch routes to single-file or multi-source install.
// R1.3: when destination is an existing directory, copy into it.
func dispatch(cfg config, sources []string, dest string) int {
	if len(sources) == 1 && !isDir(dest) {
		return installFile(cfg, sources[0], dest)
	}
	return installMultiple(cfg, sources, dest)
}

// installMultiple copies multiple sources into a directory.
// R1.3: preserves source basenames when copying to a directory.
func installMultiple(cfg config, sources []string, dest string) int {
	if !isDir(dest) {
		printErr("target '%s' is not a directory", dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if installFile(cfg, src, target) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// installFile copies a single source file to destination.
func installFile(cfg config, src, dest string) int {
	if cfg.createDirs {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			printErr("cannot create directory '%s': %v",
				filepath.Dir(dest), unwrapErr(err))
			return 1
		}
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		printErr("cannot stat '%s': %v", src, unwrapErr(err))
		return 1
	}
	if srcInfo.IsDir() {
		printErr("omitting directory '%s'", src)
		return 1
	}
	return performInstall(cfg, src, dest)
}

// performInstall executes the file copy with permissions.
func performInstall(cfg config, src, dest string) int {
	if cfg.compare && filesMatch(src, dest, resolveMode(cfg)) {
		return 0
	}
	perm := resolveMode(cfg)
	if cfg.backup && fileExists(dest) {
		makeBackup(cfg, dest)
	}
	if err := copyWithPerm(src, dest, perm); err != nil {
		printErr("%v", err)
		return 1
	}
	applyOwnership(cfg, dest)
	if cfg.verbose {
		fmt.Printf("'%s' -> '%s'\n", src, dest)
	}
	return 0
}

// filesMatch checks if src and dest have identical content and mode.
// R3.1: -C skips install when files are identical.
func filesMatch(src, dest string, mode os.FileMode) bool {
	destInfo, err := os.Stat(dest)
	if err != nil {
		return false
	}
	if destInfo.Mode().Perm() != mode.Perm() {
		return false
	}
	srcData, err := os.ReadFile(src)
	if err != nil {
		return false
	}
	destData, err := os.ReadFile(dest)
	if err != nil {
		return false
	}
	return string(srcData) == string(destData)
}

// makeBackup creates a backup of the destination file.
// R2.3: -b makes a backup with configurable suffix.
func makeBackup(cfg config, dest string) {
	suffix := "~"
	if cfg.suffix != "" {
		suffix = cfg.suffix
	}
	backupPath := dest + suffix
	// best-effort: ignore backup errors
	_ = os.Rename(dest, backupPath)
}

// copyWithPerm copies src to dest, removing dest first.
func copyWithPerm(src, dest string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %v",
			src, unwrapErr(err))
	}
	defer srcFile.Close()
	return writeDest(srcFile, dest, perm)
}

// writeDest removes existing dest and creates a new file.
func writeDest(srcFile *os.File, dest string, perm os.FileMode) error {
	if fileExists(dest) {
		if err := os.Remove(dest); err != nil {
			return fmt.Errorf("cannot remove '%s': %v",
				dest, unwrapErr(err))
		}
	}
	dstFile, err := os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("cannot create '%s': %v",
			dest, unwrapErr(err))
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("error writing '%s': %v",
			dest, unwrapErr(err))
	}
	return nil
}

// resolveMode parses the -m flag value or returns the default.
// R1.2: -m MODE sets the permission mode (octal).
func resolveMode(cfg config) os.FileMode {
	if cfg.mode == "" {
		return defaultMode
	}
	mode, err := strconv.ParseUint(cfg.mode, 8, 32)
	if err != nil {
		printErr("invalid mode '%s'", cfg.mode)
		return defaultMode
	}
	return os.FileMode(mode)
}

// applyOwnership sets owner and group on the installed file.
// R1.3: -o OWNER. R1.4: -g GROUP.
func applyOwnership(cfg config, path string) {
	uid := resolveOwnerUID(cfg.owner)
	gid := resolveGroupGID(cfg.group)
	if uid >= 0 || gid >= 0 {
		if err := os.Chown(path, uid, gid); err != nil {
			printErr("cannot change ownership of '%s': %v",
				path, unwrapErr(err))
		}
	}
}

// resolveOwnerUID looks up the user and returns their UID.
// Returns -1 if owner is empty or lookup fails.
func resolveOwnerUID(owner string) int {
	if owner == "" {
		return -1
	}
	u, err := user.Lookup(owner)
	if err != nil {
		printErr("invalid user '%s'", owner)
		return -1
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1
	}
	return uid
}

// resolveGroupGID looks up the group and returns its GID.
// Returns -1 if group is empty or lookup fails.
func resolveGroupGID(group string) int {
	if group == "" {
		return -1
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		printErr("invalid group '%s'", group)
		return -1
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return -1
	}
	return gid
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

// printErr prints a formatted error to stderr in GNU install format.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "install: "+format+"\n", args...)
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
	case arg == "--directory":
		cfg.dirMode = true
	case arg == "--verbose":
		cfg.verbose = true
	case arg == "--compare":
		cfg.compare = true
	case arg == "--backup":
		cfg.backup = true
	case strings.HasPrefix(arg, "--mode"):
		return parseFlagValue(arg, "--mode", args, i, &cfg.mode)
	case strings.HasPrefix(arg, "--owner"):
		return parseFlagValue(arg, "--owner", args, i, &cfg.owner)
	case strings.HasPrefix(arg, "--group"):
		return parseFlagValue(arg, "--group", args, i, &cfg.group)
	case strings.HasPrefix(arg, "--target-directory"):
		return parseFlagValue(
			arg, "--target-directory", args, i, &cfg.targetDir)
	case strings.HasPrefix(arg, "--suffix"):
		return parseFlagValue(arg, "--suffix", args, i, &cfg.suffix)
	default:
		fmt.Fprintf(os.Stderr,
			"install: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// parseFlagValue extracts a value from --flag=value or --flag value.
func parseFlagValue(
	arg, prefix string, args []string, i *int, dest *string,
) int {
	eqForm := prefix + "="
	if strings.HasPrefix(arg, eqForm) {
		*dest = arg[len(eqForm):]
		return -1
	}
	if arg != prefix {
		fmt.Fprintf(os.Stderr,
			"install: unrecognized option '%s'\n", arg)
		return 1
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"install: option '%s' requires an argument\n", prefix)
		return 1
	}
	*i++
	*dest = args[*i]
	return -1
}

// parseShortFlags processes clustered short flags.
func parseShortFlags(
	flags string, args []string, i *int, cfg *config,
) int {
	for j := 0; j < len(flags); j++ {
		exit := applyShortFlag(
			flags[j], flags[j+1:], args, i, cfg)
		if exit >= 0 {
			return exit
		}
		if shortFlagConsumesRest(flags[j]) {
			break
		}
	}
	return -1
}

// shortFlagConsumesRest returns true for flags that consume remainder.
func shortFlagConsumesRest(ch byte) bool {
	return ch == 'm' || ch == 'o' || ch == 'g' || ch == 't'
}

// applyShortFlag applies a single short flag character.
func applyShortFlag(
	ch byte, remainder string, args []string, i *int, cfg *config,
) int {
	switch ch {
	case 'd':
		cfg.dirMode = true
	case 'v':
		cfg.verbose = true
	case 'C':
		cfg.compare = true
	case 'D':
		cfg.createDirs = true
	case 'b':
		cfg.backup = true
	case 'm':
		return consumeShortValue(remainder, args, i, &cfg.mode, 'm')
	case 'o':
		return consumeShortValue(remainder, args, i, &cfg.owner, 'o')
	case 'g':
		return consumeShortValue(remainder, args, i, &cfg.group, 'g')
	case 't':
		return consumeShortValue(
			remainder, args, i, &cfg.targetDir, 't')
	default:
		fmt.Fprintf(os.Stderr,
			"install: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// consumeShortValue reads a value from the remainder or next argument.
func consumeShortValue(
	remainder string, args []string, i *int, dest *string, flag byte,
) int {
	if len(remainder) > 0 {
		*dest = remainder
		return -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"install: option requires an argument -- '%c'\n", flag)
		return 1
	}
	*i++
	*dest = args[*i]
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: install [OPTION]... SOURCE DEST
  or:  install [OPTION]... SOURCE... DIRECTORY
  or:  install [OPTION]... -t DIRECTORY SOURCE...
  or:  install [OPTION]... -d DIRECTORY...

Copy files and set attributes.

  -b                         make a backup of each existing destination file
  -C, --compare              compare and do not install if identical
  -d, --directory            create all leading components of DEST
  -D                         create all leading destination components
  -g, --group=GROUP          set group ownership
  -m, --mode=MODE            set permission mode (default 0755)
  -o, --owner=OWNER          set ownership
  -t, --target-directory=DIR copy all SOURCE arguments into DIR
  -v, --verbose              print the name of each file as installed
      --suffix=SUFFIX        override the usual backup suffix
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
		"install (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
