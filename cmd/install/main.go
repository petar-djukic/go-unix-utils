// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/install: copy files and set attributes.
// Implements srd101 R1.1-R1.4: core file copy with mode and ownership.
// Implements srd101 R2.1-R2.4: directory creation, backup, verbose, target-directory.
package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "install"
const defaultMode = os.FileMode(0o755)
const defaultBackupSuffix = "~"

// TODO: Task R4/AC6 requested -s/--strip, but srd101 non_goals explicitly
// states "cmd/install does not implement --strip (binary stripping requires
// external strip tool)". Skipped per E6 (non-goals enforcement).

// options holds parsed command-line flags for install.
type options struct {
	mode            string // R1.2: -m/--mode
	owner           string // R1.3: -o/--owner
	group           string // R1.4: -g/--group
	dirMode         bool   // R2.1: -d/--directory
	createDirs      bool   // R2.2: -D
	backup          bool   // R2.3: -b/--backup
	suffix          string // R2.3: --suffix
	verbose         bool   // R2.4: -v/--verbose
	targetDir       string // R2.4: -t/--target-directory
	noTargetDir     bool   // R2.4: -T/--no-target-directory
}

// modeAction represents the operator in a symbolic mode clause.
type modeAction int

const (
	modeSet    modeAction = iota // '='
	modeAdd                      // '+'
	modeRemove                   // '-'
)

// modeClause represents a single parsed symbolic mode clause (e.g., u+rwx).
type modeClause struct {
	who    string     // subset of "ugoa"; empty means umask-dependent
	action modeAction // +, -, =
	perms  string     // subset of "rwxXst"
}

// R1.1, R3.3: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	os.Exit(run(opts, args))
}

// run validates arguments, resolves mode, and dispatches installation.
func run(opts options, args []string) int {
	if opts.dirMode {
		return runDirMode(opts, args)
	}
	return runCopyMode(opts, args)
}

// runDirMode handles -d/--directory: create directories like mkdir -p.
// R2.1: create each given directory and any missing parent directories.
func runDirMode(opts options, args []string) int {
	if len(args) == 0 {
		printMissingOperand()
		return 1
	}
	fileMode, err := resolveMode(opts.mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid mode %q\n", programName, opts.mode)
		return 1
	}
	exitCode := 0
	for _, dir := range args {
		if err := createDirWithParents(opts, dir, fileMode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// createDirWithParents creates a directory and all parents, applying mode/ownership.
func createDirWithParents(opts options, dir string, fileMode os.FileMode) error {
	if err := os.MkdirAll(dir, fileMode); err != nil {
		return fmt.Errorf("cannot create directory '%s': %s", dir, unwrapErr(err))
	}
	// R2.1: set mode explicitly after creation (MkdirAll applies umask)
	if err := os.Chmod(dir, fileMode); err != nil {
		return fmt.Errorf("cannot change permissions of '%s': %s",
			dir, unwrapErr(err))
	}
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "%s: creating directory '%s'\n", programName, dir)
	}
	return applyOwnership(opts, dir)
}

// runCopyMode handles normal file copy mode.
func runCopyMode(opts options, args []string) int {
	dest, sources, err := resolveDestAndSources(opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	fileMode, err := resolveMode(opts.mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid mode %q\n", programName, opts.mode)
		return 1
	}
	return installFiles(opts, sources, dest, fileMode)
}

// resolveDestAndSources determines dest and sources based on -t/-T flags.
func resolveDestAndSources(opts options, args []string) (string, []string, error) {
	if opts.targetDir != "" {
		if len(args) == 0 {
			return "", nil, fmt.Errorf("missing file operand")
		}
		return opts.targetDir, args, nil
	}
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing file operand")
	}
	if len(args) == 1 {
		return "", nil, fmt.Errorf(
			"missing destination file operand after '%s'", args[0])
	}
	return args[len(args)-1], args[:len(args)-1], nil
}

// installFiles copies each source to dest with mode and ownership.
// R1.1: last argument is destination; multiple sources require directory dest.
func installFiles(opts options, sources []string, dest string, fileMode os.FileMode) int {
	if opts.createDirs {
		return installWithCreateDirs(opts, sources, dest, fileMode)
	}
	destIsDir := isDir(dest)
	if !opts.noTargetDir && len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr,
			"%s: target '%s': Not a directory\n", programName, dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := resolveTarget(dest, src, destIsDir && !opts.noTargetDir)
		if err := installOneCopy(opts, src, target, fileMode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// installWithCreateDirs handles -D: create leading directories then copy.
// R2.2: create all leading destination directory components.
func installWithCreateDirs(opts options, sources []string, dest string, fileMode os.FileMode) int {
	destIsDir := isDir(dest)
	exitCode := 0
	for _, src := range sources {
		target := resolveTarget(dest, src, destIsDir && !opts.noTargetDir)
		dir := filepath.Dir(target)
		if err := os.MkdirAll(dir, defaultMode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %s\n",
				programName, dir, unwrapErr(err))
			exitCode = 1
			continue
		}
		if err := installOneCopy(opts, src, target, fileMode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// installOneCopy copies one source file, handling backup and verbose.
func installOneCopy(opts options, src, dest string, fileMode os.FileMode) error {
	if opts.backup {
		if err := makeBackup(dest, opts.suffix); err != nil {
			return err
		}
	}
	if err := installSingle(opts, src, dest, fileMode); err != nil {
		return err
	}
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "'%s' -> '%s'\n", src, dest)
	}
	return nil
}

// makeBackup creates a backup of dest if it exists.
// R2.3: append suffix to original filename.
func makeBackup(dest, suffix string) error {
	if suffix == "" {
		suffix = defaultBackupSuffix
	}
	if _, err := os.Lstat(dest); os.IsNotExist(err) {
		return nil
	}
	backupPath := dest + suffix
	if err := os.Rename(dest, backupPath); err != nil {
		return fmt.Errorf("cannot backup '%s': %s", dest, unwrapErr(err))
	}
	return nil
}

// installSingle copies one source file to dest and applies attributes.
// R1.1: copy file content. R1.2: set mode. R1.3/R1.4: set ownership.
func installSingle(opts options, src, dest string, fileMode os.FileMode) error {
	if err := copyFileContent(src, dest); err != nil {
		return err
	}
	if err := os.Chmod(dest, fileMode); err != nil {
		return fmt.Errorf("cannot change permissions of '%s': %s",
			dest, unwrapErr(err))
	}
	return applyOwnership(opts, dest)
}

// copyFileContent copies file data from src to dest.
func copyFileContent(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, unwrapErr(err))
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %s",
			dest, unwrapErr(err))
	}
	return finishCopy(in, out, dest)
}

// finishCopy performs the data copy and closes the output file.
func finishCopy(in, out *os.File, dest string) error {
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return fmt.Errorf("error writing '%s': %s", dest, unwrapErr(cpErr))
	}
	if closeErr != nil {
		return fmt.Errorf("closing '%s': %s", dest, unwrapErr(closeErr))
	}
	return nil
}

// applyOwnership sets owner and group on dest based on -o and -g flags.
// R1.3: -o/--owner sets the file owner.
// R1.4: -g/--group sets the file group.
func applyOwnership(opts options, dest string) error {
	uid := -1
	gid := -1
	if opts.owner != "" {
		resolved, err := lookupUID(opts.owner)
		if err != nil {
			return fmt.Errorf("invalid user %q", opts.owner)
		}
		uid = resolved
	}
	if opts.group != "" {
		resolved, err := lookupGID(opts.group)
		if err != nil {
			return fmt.Errorf("invalid group %q", opts.group)
		}
		gid = resolved
	}
	if uid == -1 && gid == -1 {
		return nil
	}
	if err := os.Chown(dest, uid, gid); err != nil {
		return fmt.Errorf("cannot change ownership of '%s': %s",
			dest, unwrapErr(err))
	}
	return nil
}

// lookupUID resolves a username or numeric string to a UID.
func lookupUID(name string) (int, error) {
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(u.Uid)
}

// lookupGID resolves a group name or numeric string to a GID.
func lookupGID(name string) (int, error) {
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(g.Gid)
}

// resolveMode returns the file mode from the -m spec or the default 0755.
// R1.2: supports octal and symbolic mode specifications.
func resolveMode(spec string) (os.FileMode, error) {
	if spec == "" {
		return defaultMode, nil
	}
	if isOctalMode(spec) {
		return parseOctalMode(spec)
	}
	return applySymbolicMode(spec, defaultMode)
}

// isOctalMode checks if spec is a valid octal mode string.
func isOctalMode(spec string) bool {
	if len(spec) == 0 {
		return false
	}
	for _, c := range spec {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

// parseOctalMode parses an octal mode string into os.FileMode.
// Handles basic permissions (0-0777) and special bits (4000, 2000, 1000).
func parseOctalMode(spec string) (os.FileMode, error) {
	var val uint32
	for _, c := range spec {
		val = val*8 + uint32(c-'0')
	}
	goMode := os.FileMode(val & 0o777)
	if val&0o4000 != 0 {
		goMode |= os.ModeSetuid
	}
	if val&0o2000 != 0 {
		goMode |= os.ModeSetgid
	}
	if val&0o1000 != 0 {
		goMode |= os.ModeSticky
	}
	return goMode, nil
}

// applySymbolicMode parses and applies symbolic mode clauses to base mode.
func applySymbolicMode(spec string, base os.FileMode) (os.FileMode, error) {
	clauses := strings.Split(spec, ",")
	result := base
	for _, clause := range clauses {
		mc, err := parseSingleClause(clause)
		if err != nil {
			return 0, err
		}
		result = applyClause(mc, result)
	}
	return result, nil
}

// parseSingleClause parses a single symbolic mode clause (e.g., u+rwx).
// Format is [ugoa...][+-=][rwxXst...].
func parseSingleClause(clause string) (modeClause, error) {
	if len(clause) == 0 {
		return modeClause{}, fmt.Errorf("invalid mode: empty clause")
	}
	var mc modeClause
	i := parseWhoChars(clause, &mc)
	if i >= len(clause) {
		return modeClause{}, fmt.Errorf("invalid mode: %q", clause)
	}
	i, err := parseAction(clause, i, &mc)
	if err != nil {
		return modeClause{}, err
	}
	if err := parsePermChars(clause[i:], clause, &mc); err != nil {
		return modeClause{}, err
	}
	return mc, nil
}

// parseWhoChars extracts who characters from the clause start.
func parseWhoChars(clause string, mc *modeClause) int {
	i := 0
	for i < len(clause) {
		c := clause[i]
		if c == 'u' || c == 'g' || c == 'o' || c == 'a' {
			mc.who += string(c)
			i++
		} else {
			break
		}
	}
	return i
}

// parseAction extracts the mode action (+, -, =) from the clause.
func parseAction(clause string, i int, mc *modeClause) (int, error) {
	switch clause[i] {
	case '+':
		mc.action = modeAdd
	case '-':
		mc.action = modeRemove
	case '=':
		mc.action = modeSet
	default:
		return i, fmt.Errorf("invalid mode: %q", clause)
	}
	return i + 1, nil
}

// parsePermChars extracts permission characters from the clause.
func parsePermChars(rest, clause string, mc *modeClause) error {
	for _, c := range rest {
		switch c {
		case 'r', 'w', 'x', 'X', 's', 't':
			mc.perms += string(c)
		default:
			return fmt.Errorf("invalid mode: %q", clause)
		}
	}
	return nil
}

// applyClause applies one symbolic mode clause to the current mode.
func applyClause(mc modeClause, current os.FileMode) os.FileMode {
	bits := resolvePermBits(mc, current)
	basicMask := effectiveBasicMask(mc.who)
	maskedBasic := bits & basicMask
	specialBits := bits & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)

	switch mc.action {
	case modeAdd:
		return current | maskedBasic | specialBits
	case modeRemove:
		return current &^ (maskedBasic | specialBits)
	case modeSet:
		specMask := effectiveSpecialMask(mc.who)
		cleared := current &^ basicMask &^ specMask
		return cleared | maskedBasic | specialBits
	}
	return current
}

// effectiveBasicMask returns the basic permission mask for who.
// When who is empty, uses ~umask per GNU semantics.
func effectiveBasicMask(who string) os.FileMode {
	if who == "" {
		umask := syscall.Umask(0)
		syscall.Umask(umask)
		return 0o777 &^ os.FileMode(umask)
	}
	return whoToBasicMask(who)
}

// effectiveSpecialMask returns the special bits mask for who.
func effectiveSpecialMask(who string) os.FileMode {
	if who == "" {
		return os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	}
	return whoToSpecialMask(who)
}

// resolvePermBits converts permission characters to os.FileMode bits.
func resolvePermBits(mc modeClause, current os.FileMode) os.FileMode {
	var bits os.FileMode
	isDir := current&os.ModeDir != 0
	hasExec := current.Perm()&0o111 != 0
	for _, c := range mc.perms {
		switch c {
		case 'r':
			bits |= 0o444
		case 'w':
			bits |= 0o222
		case 'x':
			bits |= 0o111
		case 'X':
			if isDir || hasExec {
				bits |= 0o111
			}
		case 's':
			bits |= suidBitsForWho(mc.who)
		case 't':
			bits |= os.ModeSticky
		}
	}
	return bits
}

// suidBitsForWho returns setuid/setgid bits based on who specifier.
func suidBitsForWho(who string) os.FileMode {
	var bits os.FileMode
	if who == "" || strings.ContainsAny(who, "ua") {
		bits |= os.ModeSetuid
	}
	if who == "" || strings.ContainsAny(who, "ga") {
		bits |= os.ModeSetgid
	}
	return bits
}

// whoToBasicMask converts who characters to a basic permission bitmask.
func whoToBasicMask(who string) os.FileMode {
	var mask os.FileMode
	for _, c := range who {
		switch c {
		case 'u':
			mask |= 0o700
		case 'g':
			mask |= 0o070
		case 'o':
			mask |= 0o007
		case 'a':
			mask |= 0o777
		}
	}
	return mask
}

// whoToSpecialMask converts who characters to special mode bits mask.
func whoToSpecialMask(who string) os.FileMode {
	var mask os.FileMode
	for _, c := range who {
		switch c {
		case 'u':
			mask |= os.ModeSetuid
		case 'g':
			mask |= os.ModeSetgid
		case 'o':
			mask |= os.ModeSticky
		case 'a':
			mask |= os.ModeSetuid | os.ModeSetgid | os.ModeSticky
		}
	}
	return mask
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

// unwrapErr extracts the underlying error message from *os.PathError.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// parseArgs separates flags from positional arguments.
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

// parseLongFlag handles long-form flags for install.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	flag := rawArgs[idx]
	switch {
	case strings.HasPrefix(flag, "--mode="):
		opts.mode = strings.TrimPrefix(flag, "--mode=")
	case flag == "--mode":
		idx = consumeLongArg(&opts.mode, rawArgs, idx)
	case strings.HasPrefix(flag, "--owner="):
		opts.owner = strings.TrimPrefix(flag, "--owner=")
	case flag == "--owner":
		idx = consumeLongArg(&opts.owner, rawArgs, idx)
	case strings.HasPrefix(flag, "--group="):
		opts.group = strings.TrimPrefix(flag, "--group=")
	case flag == "--group":
		idx = consumeLongArg(&opts.group, rawArgs, idx)
	case flag == "--directory":
		opts.dirMode = true
	case flag == "--backup":
		opts.backup = true
	case strings.HasPrefix(flag, "--suffix="):
		opts.suffix = strings.TrimPrefix(flag, "--suffix=")
	case flag == "--suffix":
		idx = consumeLongArg(&opts.suffix, rawArgs, idx)
	case flag == "--verbose":
		opts.verbose = true
	case strings.HasPrefix(flag, "--target-directory="):
		opts.targetDir = strings.TrimPrefix(flag, "--target-directory=")
	case flag == "--target-directory":
		idx = consumeLongArg(&opts.targetDir, rawArgs, idx)
	case flag == "--no-target-directory":
		opts.noTargetDir = true
	}
	return idx
}

// consumeLongArg reads the next argument as a long flag value.
func consumeLongArg(target *string, rawArgs []string, idx int) int {
	if idx+1 < len(rawArgs) {
		idx++
		*target = rawArgs[idx]
	}
	return idx
}

// parseShortFlags handles combined short flags like -m 755.
func parseShortFlags(opts *options, rawArgs []string, idx int) int {
	chars := rawArgs[idx][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'm':
			return consumeShortArg(&opts.mode, chars[j+1:], rawArgs, idx)
		case 'o':
			return consumeShortArg(&opts.owner, chars[j+1:], rawArgs, idx)
		case 'g':
			return consumeShortArg(&opts.group, chars[j+1:], rawArgs, idx)
		case 't':
			return consumeShortArg(&opts.targetDir, chars[j+1:], rawArgs, idx)
		case 'd':
			opts.dirMode = true
		case 'D':
			opts.createDirs = true
		case 'b':
			opts.backup = true
		case 'v':
			opts.verbose = true
		case 'T':
			opts.noTargetDir = true
		}
	}
	return idx
}

// consumeShortArg reads the value for a short flag with an argument.
// If remaining chars exist after the flag, they are the value.
// Otherwise the next argument is consumed.
func consumeShortArg(target *string, rest string, rawArgs []string, idx int) int {
	if len(rest) > 0 {
		*target = rest
	} else if idx+1 < len(rawArgs) {
		idx++
		*target = rawArgs[idx]
	}
	return idx
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand() {
	fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
	printTryHelp()
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// printUsage prints the usage message.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... SOURCE... DEST
Copy SOURCE to DEST, setting permission and ownership attributes.

Options:
  -b, --backup            make a backup of each existing destination file
  -d, --directory         create all arguments as directories
  -D                      create all leading destination components, then copy
  -g, --group=GROUP       set group ownership (R1.4)
  -m, --mode=MODE         set permission mode (default: 0755) (R1.2)
  -o, --owner=OWNER       set ownership (R1.3)
      --suffix=SUFFIX     override the backup suffix (default ~)
  -t, --target-directory=DIR  install into DIR
  -T, --no-target-directory   treat DEST as a normal file
  -v, --verbose           print the name of each file as it is installed
      --help              display this help and exit
      --version           output version information and exit
`, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s 1.0.0\n", programName)
}
