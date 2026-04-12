// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/chmod: change file mode bits.
// Implements srd089 R1.1-R1.4 (mode specification),
// R2.1-R2.4 (recursive and output control),
// R3.1-R3.2 (special mode bits and reference),
// R4.1-R4.3 (exit codes and SIGPIPE).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "chmod"

// errAlreadyReported signals that errors were already printed to stderr
// by the recursive walker, so the caller should not print again.
var errAlreadyReported = fmt.Errorf("errors already reported")

// modeAction represents the operator in a symbolic mode clause.
type modeAction int

const (
	modeSet    modeAction = iota // '='
	modeAdd                      // '+'
	modeRemove                   // '-'
)

// symlinkPolicy controls how symlinks are handled during recursive traversal.
type symlinkPolicy int

const (
	symlinkNone    symlinkPolicy = iota // -P: don't follow symlinks (default)
	symlinkCmdLine                      // -H: follow command-line symlinks only
	symlinkAll                          // -L: follow all symlinks
)

// modeClause represents a single parsed symbolic mode clause (e.g., u+rwx).
// When who is empty, no ugoa was specified and umask filtering applies.
type modeClause struct {
	who    string     // subset of "ugoa"; empty means umask-dependent
	action modeAction // +, -, =
	perms  string     // subset of "rwxXst"
}

// mode represents a parsed mode specification, either octal or symbolic.
type mode struct {
	octal    uint32       // R1.1: Go os.FileMode bits (valid when isOctal is true)
	symbolic []modeClause // R1.2: parsed symbolic clauses
	isOctal  bool         // true when the mode was specified as an octal number
}

// options holds parsed command-line flags for chmod.
type options struct {
	recursive bool          // R2.1: -R/--recursive
	verbose   bool          // R2.2: -v/--verbose
	changes   bool          // R2.3: -c/--changes
	silent    bool          // R2.4: -f/--silent/--quiet
	reference string        // R3.2: --reference=RFILE
	symlinks  symlinkPolicy // R3.1-R3.2: -H/-L/-P symlink traversal
}

// TODO: Task R1 requested --preserve-root/--no-preserve-root (srd089 R4.2),
// but srd089 non_goals explicitly states "cmd/chmod does not implement
// --preserve-root (not applicable on macOS)". Skipped per E6.

// R4.3, R1.1: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()

	opts, modeSpec, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		os.Exit(1)
	}

	exitCode := run(opts, modeSpec, files)
	os.Exit(exitCode)
}

// parseArgs separates flags, mode specification, and file operands.
// Supports short flags (-R, -v, -c, -f, -H, -L, -P), combined short flags,
// and long forms (--recursive, --verbose, --changes, --silent,
// --quiet, --reference=RFILE).
func parseArgs(rawArgs []string) (options, string, []string) {
	var opts options
	var positional []string
	endOfFlags := false

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if endOfFlags {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(&opts, rawArgs, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if isShortFlags(arg) {
				parseShortFlags(&opts, arg[1:])
				continue
			}
		}
		positional = append(positional, arg)
	}

	// First positional is mode spec (unless --reference is used),
	// rest are files.
	if opts.reference != "" {
		return opts, "", positional
	}
	if len(positional) == 0 {
		return opts, "", nil
	}
	return opts, positional[0], positional[1:]
}

// isShortFlags checks if arg (without leading -) contains only
// valid short flag characters.
func isShortFlags(arg string) bool {
	for _, c := range arg[1:] {
		switch c {
		case 'R', 'v', 'c', 'f', 'H', 'L', 'P':
			// valid short flag
		default:
			return false
		}
	}
	return true
}

// parseLongFlag handles long-form flags for chmod.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	flag := rawArgs[idx]
	switch {
	case flag == "--recursive":
		opts.recursive = true
	case flag == "--verbose":
		opts.verbose = true
	case flag == "--changes":
		opts.changes = true
	case flag == "--silent", flag == "--quiet":
		opts.silent = true
	case strings.HasPrefix(flag, "--reference="):
		// R3.2: --reference=RFILE
		opts.reference = strings.TrimPrefix(flag, "--reference=")
	}
	return idx
}

// parseShortFlags handles combined short flags like -Rvc.
func parseShortFlags(opts *options, chars string) {
	for _, c := range chars {
		switch c {
		case 'R':
			opts.recursive = true
		case 'v':
			opts.verbose = true
		case 'c':
			opts.changes = true
		case 'f':
			opts.silent = true
		case 'H':
			opts.symlinks = symlinkCmdLine
		case 'L':
			opts.symlinks = symlinkAll
		case 'P':
			opts.symlinks = symlinkNone
		}
	}
}

// run applies the mode change to all files and returns the exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: continues processing remaining files on error, exits 1.
// R4.1: exits 0 when all files processed successfully.
// R4.2: exits 1 when any file fails.
func run(opts options, modeSpec string, files []string) int {
	m, err := resolveMode(opts, modeSpec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}

	exitCode := 0
	for _, file := range files {
		if err := applyMode(opts, m, file); err != nil {
			if !opts.silent && err != errAlreadyReported {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			}
			exitCode = 1
		}
	}
	return exitCode
}

// resolveMode determines the target mode from --reference or mode spec.
// R3.2: when --reference is set, reads the mode from the reference file.
func resolveMode(opts options, modeSpec string) (mode, error) {
	if opts.reference != "" {
		return modeFromReference(opts.reference)
	}
	return parseMode(modeSpec)
}

// modeFromReference reads the mode bits from a reference file.
// R3.2: --reference=RFILE sets each FILE's mode to match RFILE's mode.
func modeFromReference(rfile string) (mode, error) {
	fi, err := sys.Stat(rfile)
	if err != nil {
		return mode{}, fmt.Errorf("cannot stat %q: %w", rfile, err)
	}
	return mode{
		octal: uint32(fi.Mode.Perm()) |
			uint32(fi.Mode&os.ModeSetuid) |
			uint32(fi.Mode&os.ModeSetgid) |
			uint32(fi.Mode&os.ModeSticky),
		isOctal: true,
	}, nil
}

// parseMode parses a mode specification string into a mode struct.
// R1.1: accepts octal modes (e.g., 755, 0644).
// R1.2: accepts symbolic modes (e.g., u+x, go-w, a=rw, u=rwx,go=rx).
func parseMode(spec string) (mode, error) {
	if len(spec) == 0 {
		return mode{}, fmt.Errorf("missing operand")
	}
	if isOctalMode(spec) {
		return parseOctalMode(spec)
	}
	return parseSymbolicMode(spec)
}

// isOctalMode checks if spec is a valid octal mode string.
func isOctalMode(spec string) bool {
	for _, c := range spec {
		if c < '0' || c > '7' {
			return false
		}
	}
	return len(spec) > 0
}

// parseOctalMode parses an octal mode string into a mode struct.
// R1.1: converts Unix octal to Go os.FileMode representation,
// handling basic permissions (0-0777) and special bits (4000, 2000, 1000).
func parseOctalMode(spec string) (mode, error) {
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
	return mode{octal: uint32(goMode), isOctal: true}, nil
}

// parseSymbolicMode parses a symbolic mode string into a mode struct.
// R1.2: supports [ugoa][+-=][rwxXst] with comma-separated clauses.
// R3.1: supports setuid (u+s), setgid (g+s), and sticky bit (o+t).
func parseSymbolicMode(spec string) (mode, error) {
	clauses := strings.Split(spec, ",")
	var parsed []modeClause
	for _, clause := range clauses {
		mc, err := parseSingleClause(clause)
		if err != nil {
			return mode{}, err
		}
		parsed = append(parsed, mc)
	}
	return mode{symbolic: parsed}, nil
}

// parseSingleClause parses a single symbolic mode clause (e.g., u+rwx).
// R1.2: format is [ugoa...][+-=][rwxXst...].
// When no who characters are present, who is left empty to indicate
// umask-dependent behavior per GNU chmod semantics.
func parseSingleClause(clause string) (modeClause, error) {
	if len(clause) == 0 {
		return modeClause{}, fmt.Errorf("invalid mode: empty clause")
	}

	var mc modeClause
	i := 0

	// Parse who characters (leave empty if none specified)
	for i < len(clause) {
		c := clause[i]
		if c == 'u' || c == 'g' || c == 'o' || c == 'a' {
			mc.who += string(c)
			i++
		} else {
			break
		}
	}

	// Parse action
	if i >= len(clause) {
		return modeClause{}, fmt.Errorf("invalid mode: %q", clause)
	}
	switch clause[i] {
	case '+':
		mc.action = modeAdd
	case '-':
		mc.action = modeRemove
	case '=':
		mc.action = modeSet
	default:
		return modeClause{}, fmt.Errorf("invalid mode: %q", clause)
	}
	i++

	// Parse permission characters
	for i < len(clause) {
		c := clause[i]
		if c == 'r' || c == 'w' || c == 'x' || c == 'X' || c == 's' || c == 't' {
			mc.perms += string(c)
			i++
		} else {
			return modeClause{}, fmt.Errorf("invalid mode: %q", clause)
		}
	}

	return mc, nil
}

// applyMode applies the parsed mode to a single file.
// R2.1: when recursive, traverses directories.
func applyMode(opts options, m mode, path string) error {
	if opts.recursive {
		return applyModeRecursive(opts, m, path)
	}
	return applyModeToFile(opts, m, path)
}

// applyModeRecursive recursively applies mode changes to a directory tree.
// R2.1: -R/--recursive changes modes for directories and their contents.
// R3.1-R3.2: respects -H/-L/-P symlink traversal policy.
func applyModeRecursive(opts options, m mode, root string) error {
	hadError := false
	walkChmod(opts, m, root, true, &hadError)
	if hadError {
		return errAlreadyReported
	}
	return nil
}

// walkChmod recursively applies mode changes, respecting symlink policy.
// R3.1: -P (default) skips symlinks.
// R3.2: -H follows command-line symlinks, -L follows all symlinks.
func walkChmod(opts options, m mode, path string, isRoot bool, hadErr *bool) {
	fi, skip := resolveEntry(opts, path, isRoot, hadErr)
	if skip || fi == nil {
		return
	}
	if err := applyModeToFile(opts, m, path); err != nil {
		reportFileError(opts, err, hadErr)
	}
	if fi.IsDir() {
		walkChildren(opts, m, path, hadErr)
	}
}

// resolveEntry checks the path and decides whether to process it.
// Returns the file info and whether to skip the entry.
// A nil fi with skip=true means the entry is a symlink not being followed.
// A nil fi with skip=false means an error occurred (already reported).
func resolveEntry(opts options, path string, isRoot bool, hadErr *bool) (os.FileInfo, bool) {
	lfi, err := os.Lstat(path)
	if err != nil {
		reportWalkError(opts, path, err, hadErr)
		return nil, false
	}
	if lfi.Mode()&os.ModeSymlink == 0 {
		return lfi, false
	}
	if !shouldFollowSymlink(opts.symlinks, isRoot) {
		return nil, true
	}
	fi, err := os.Stat(path)
	if err != nil {
		reportWalkError(opts, path, err, hadErr)
		return nil, false
	}
	return fi, false
}

// shouldFollowSymlink returns true if the symlink should be followed
// based on the policy and whether the path is a command-line argument.
func shouldFollowSymlink(policy symlinkPolicy, isRoot bool) bool {
	return policy == symlinkAll || (policy == symlinkCmdLine && isRoot)
}

// walkChildren reads directory entries and recurses into each child.
func walkChildren(opts options, m mode, dir string, hadErr *bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		reportWalkError(opts, dir, err, hadErr)
		return
	}
	for _, e := range entries {
		walkChmod(opts, m, filepath.Join(dir, e.Name()), false, hadErr)
	}
}

// reportWalkError prints a directory traversal error to stderr.
func reportWalkError(opts options, path string, err error, hadErr *bool) {
	if !opts.silent {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %s\n",
			programName, path, unwrapPathError(err))
	}
	*hadErr = true
}

// reportFileError prints a file mode change error to stderr.
func reportFileError(opts options, err error, hadErr *bool) {
	if !opts.silent {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
	}
	*hadErr = true
}

// applyModeToFile applies the mode change to a single file.
// R1.1, R1.2: reads current mode, computes new mode, applies via os.Chmod.
// R1.4: returns error when file cannot be accessed.
// Uses os.Lstat so symlinks themselves are not followed; chmod(2) follows
// symlinks when called by os.Chmod.
func applyModeToFile(opts options, m mode, path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %s",
			path, unwrapPathError(err))
	}
	oldMode := fi.Mode()
	newMode := computeNewMode(m, oldMode)
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("changing permissions of '%s': %s",
			path, unwrapPathError(err))
	}
	printDiagnostic(opts, path, oldMode, newMode)
	return nil
}

// unwrapPathError extracts the underlying error message from *os.PathError.
func unwrapPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// computeNewMode calculates the new permission bits for a file.
// R1.1: for octal modes, returns the stored Go os.FileMode value directly.
// R1.2: for symbolic modes, applies each clause to the current mode.
func computeNewMode(m mode, current os.FileMode) os.FileMode {
	if m.isOctal {
		return os.FileMode(m.octal)
	}
	result := current
	for _, clause := range m.symbolic {
		result = applyClauseToMode(clause, result)
	}
	return result
}

// applyClauseToMode applies one symbolic mode clause to the current mode.
// R1.2: handles +, -, = operators with who mask.
// When who is empty, basic permission bits are filtered by ~umask.
func applyClauseToMode(mc modeClause, current os.FileMode) os.FileMode {
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

// effectiveBasicMask returns the basic permission mask for the who specifier.
// When who is empty (no ugoa specified), uses ~umask per GNU chmod semantics.
func effectiveBasicMask(who string) os.FileMode {
	if who == "" {
		umask := syscall.Umask(0)
		syscall.Umask(umask)
		return 0o777 &^ os.FileMode(umask)
	}
	return whoToBasicMask(who)
}

// effectiveSpecialMask returns the special bits mask for the who specifier.
// When who is empty, all special bits are included (not affected by umask).
func effectiveSpecialMask(who string) os.FileMode {
	if who == "" {
		return os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	}
	return whoToSpecialMask(who)
}

// resolvePermBits converts permission characters to os.FileMode bits.
// R1.2: handles rwxXst characters.
// R3.1: maps s to setuid/setgid based on who, t to sticky.
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
// R3.1: u+s → setuid, g+s → setgid, a+s → both.
// Empty who (no ugoa specified) acts like 'a' for special bits.
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

// printDiagnostic prints a verbose or changes-only diagnostic message.
// R2.2: -v prints a diagnostic for every file processed.
// R2.3: -c prints a diagnostic only when mode actually changed.
// Format matches GNU chmod: "mode of 'X' changed from OOOO (SSS) to OOOO (SSS)"
// or "mode of 'X' retained as OOOO (SSS)".
func printDiagnostic(opts options, path string, old, new os.FileMode) {
	if !opts.verbose && !opts.changes {
		return
	}
	changed := modeToOctal(old) != modeToOctal(new)
	if opts.changes && !changed {
		return
	}
	if changed {
		fmt.Fprintf(os.Stdout,
			"mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, modeToOctal(old), formatPermString(old),
			modeToOctal(new), formatPermString(new))
	} else {
		fmt.Fprintf(os.Stdout,
			"mode of '%s' retained as %04o (%s)\n",
			path, modeToOctal(new), formatPermString(new))
	}
}

// modeToOctal converts Go os.FileMode to Unix-style octal representation
// including special bits (setuid=4000, setgid=2000, sticky=1000).
func modeToOctal(m os.FileMode) uint32 {
	val := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		val |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		val |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		val |= 0o1000
	}
	return val
}

// formatPermString returns the 9-character symbolic permission string
// (e.g., "rwxr-xr-x") with special bit markers (s/S, t/T).
func formatPermString(m os.FileMode) string {
	var buf [9]byte
	perm := m.Perm()
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			buf[i] = "rwxrwxrwx"[i]
		} else {
			buf[i] = '-'
		}
	}
	applySpecialBitMarkers(m, &buf)
	return string(buf[:])
}

// applySpecialBitMarkers overlays setuid, setgid, and sticky bit markers
// onto the 9-character permission string.
func applySpecialBitMarkers(m os.FileMode, buf *[9]byte) {
	if m&os.ModeSetuid != 0 {
		if buf[2] == 'x' {
			buf[2] = 's'
		} else {
			buf[2] = 'S'
		}
	}
	if m&os.ModeSetgid != 0 {
		if buf[5] == 'x' {
			buf[5] = 's'
		} else {
			buf[5] = 'S'
		}
	}
	if m&os.ModeSticky != 0 {
		if buf[8] == 'x' {
			buf[8] = 't'
		} else {
			buf[8] = 'T'
		}
	}
}
