// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chmod implements GNU chmod: change file mode bits.
//
// Implements prd089-chmod R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chmod"

const helpText = `Usage: chmod [OPTION]... MODE[,MODE]... FILE...
  or:  chmod [OPTION]... --reference=RFILE FILE...
Change the mode of each FILE to MODE.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
  -R, --recursive        change files and directories recursively
      --reference=RFILE  use RFILE's mode instead of MODE values
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "chmod (go-unix-utils) 0.1\n"

// options holds the parsed command-line options.
type options struct {
	recursive bool
	verbose   bool
	changes   bool
	silent    bool
	reference string // R3.2: --reference=RFILE
}

// modeChange represents a parsed mode specification (octal or symbolic).
type modeChange struct {
	octal   bool
	mode    os.FileMode
	clauses []clause
}

// clause represents one symbolic mode clause like "u+rwx".
type clause struct {
	who   string // combination of u, g, o; empty means all
	op    byte   // '+', '-', '='
	perms string // combination of r, w, x, X, s, t
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the chmod logic and returns the exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: exits 1 on any error, continues processing remaining files.
// R4.1: exits 0 when all files processed successfully.
// R4.2: exits 1 when any file cannot be accessed or mode is invalid.
func run(args []string, stdout, stderr *os.File) int {
	opts, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, opts.silent, err.Error())
		return 1
	}
	if opts.reference != "" {
		return runWithReference(operands, opts, stdout, stderr)
	}
	return runWithMode(operands, opts, stdout, stderr)
}

// runWithMode handles the standard MODE FILE... invocation.
func runWithMode(operands []string, opts options, stdout, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, false, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	if len(operands) == 1 {
		msg := fmt.Sprintf("missing operand after '%s'", operands[0])
		printError(stderr, false, msg)
		printTryHelp(stderr)
		return 1
	}
	mc, err := parseMode(operands[0])
	if err != nil {
		printError(stderr, opts.silent, err.Error())
		return 1
	}
	return applyToFiles(operands[1:], mc, opts, stdout, stderr)
}

// runWithReference handles --reference=RFILE FILE... invocation.
// R3.2: sets each FILE's mode to match RFILE's mode.
func runWithReference(operands []string, opts options, stdout, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, false, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	refMode, err := getFileMode(opts.reference)
	if err != nil {
		msg := fmt.Sprintf("failed to get attributes of '%s': %s",
			opts.reference, sysErrorMsg(err))
		printError(stderr, opts.silent, msg)
		return 1
	}
	mc := &modeChange{octal: true, mode: refMode}
	return applyToFiles(operands, mc, opts, stdout, stderr)
}

// applyToFiles applies the mode change to each file and returns the exit code.
func applyToFiles(files []string, mc *modeChange, opts options, stdout, stderr *os.File) int {
	exitCode := 0
	for _, file := range files {
		if err := chmodPath(file, mc, opts, stdout, stderr); err != nil {
			printError(stderr, opts.silent, err.Error())
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var operands []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		var err error
		opts, err = handleFlag(arg, opts)
		if err != nil {
			return opts, nil, err
		}
	}
	return opts, operands, nil
}

// handleFlag processes a single flag argument.
func handleFlag(arg string, opts options) (options, error) {
	// R3.2: --reference=RFILE
	if strings.HasPrefix(arg, "--reference=") {
		opts.reference = arg[len("--reference="):]
		return opts, nil
	}
	switch arg {
	case "--recursive":
		opts.recursive = true
		return opts, nil
	case "--verbose":
		opts.verbose = true
		opts.changes = false
		return opts, nil
	case "--changes":
		opts.changes = true
		opts.verbose = false
		return opts, nil
	case "--silent", "--quiet":
		opts.silent = true
		return opts, nil
	case "--help":
		fmt.Fprint(os.Stdout, helpText) //nolint:errcheck
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText) //nolint:errcheck
		os.Exit(0)
	default:
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			return parseShortFlags(arg[1:], opts)
		}
		return opts, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return opts, nil
}

// parseShortFlags processes combined short flags like -Rv.
func parseShortFlags(flags string, opts options) (options, error) {
	for _, c := range flags {
		switch c {
		case 'R':
			opts.recursive = true
		case 'v':
			// R2.2: -v enables verbose, overrides -c
			opts.verbose = true
			opts.changes = false
		case 'c':
			// R2.3: -c enables changes-only, overrides -v
			opts.changes = true
			opts.verbose = false
		case 'f':
			// R2.4: -f enables silent mode
			opts.silent = true
		default:
			return opts, fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return opts, nil
}

// chmodPath applies the mode change to path, optionally recursing.
// R2.1: -R changes modes recursively for directories and their contents.
func chmodPath(path string, mc *modeChange, opts options, stdout, stderr *os.File) error {
	if !opts.recursive {
		return chmodFile(path, mc, opts, stdout)
	}
	var walkErr error
	err := filepath.WalkDir(path, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			msg := fmt.Sprintf("cannot access '%s': %s", p, sysErrorMsg(err))
			printError(stderr, opts.silent, msg)
			walkErr = fmt.Errorf("walk error")
			return nil // continue walking
		}
		if chErr := chmodFile(p, mc, opts, stdout); chErr != nil {
			printError(stderr, opts.silent, chErr.Error())
			walkErr = fmt.Errorf("walk error")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if walkErr != nil {
		return walkErr
	}
	return nil
}

// chmodFile applies the mode change to a single file.
// R2.2: verbose prints a diagnostic for every file.
// R2.3: changes prints a diagnostic only when mode changed.
func chmodFile(path string, mc *modeChange, opts options, stdout *os.File) error {
	oldMode, err := getFileMode(path)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %s", path, sysErrorMsg(err))
	}
	newMode := resolveMode(mc, oldMode)
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("changing permissions of '%s': %s", path, sysErrorMsg(err))
	}
	printDiagnostic(stdout, opts, path, oldMode, newMode)
	return nil
}

// getFileMode returns the current permission and special bits of a file.
func getFileMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	m := info.Mode()
	return m.Perm() | m&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky), nil
}

// sysErrorMsg extracts the underlying system error message from a Go error.
func sysErrorMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printDiagnostic prints verbose or changes-only output.
func printDiagnostic(stdout *os.File, opts options, path string, oldMode, newMode os.FileMode) {
	if !opts.verbose && !opts.changes {
		return
	}
	if opts.changes && oldMode == newMode {
		return
	}
	msg := formatDiagnostic(path, oldMode, newMode)
	fmt.Fprintln(stdout, msg) //nolint:errcheck
}

// formatDiagnostic formats a diagnostic message for a mode change.
func formatDiagnostic(path string, oldMode, newMode os.FileMode) string {
	if oldMode == newMode {
		return fmt.Sprintf("mode of '%s' retained as %04o (%s)",
			path, fileModeToUnix(oldMode), symbolicPerms(oldMode))
	}
	return fmt.Sprintf("mode of '%s' changed from %04o (%s) to %04o (%s)",
		path, fileModeToUnix(oldMode), symbolicPerms(oldMode),
		fileModeToUnix(newMode), symbolicPerms(newMode))
}

// fileModeToUnix converts os.FileMode to Unix mode_t for display.
func fileModeToUnix(m os.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		mode |= 0o1000
	}
	return mode
}

// symbolicPerms returns the symbolic permission string (e.g., "rwxr-xr-x").
func symbolicPerms(m os.FileMode) string {
	var buf [9]byte
	const rwx = "rwx"
	perm := m.Perm()
	for i := range 9 {
		if perm&(1<<uint(8-i)) != 0 {
			buf[i] = rwx[i%3]
		} else {
			buf[i] = '-'
		}
	}
	applySpecialBits(m, &buf)
	return string(buf[:])
}

// applySpecialBits modifies the permission string for setuid/setgid/sticky.
func applySpecialBits(m os.FileMode, buf *[9]byte) {
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

// resolveMode computes the new mode for a file.
// R1.1: octal modes apply directly.
// R1.2: symbolic modes apply relative to the current file mode.
func resolveMode(mc *modeChange, currentMode os.FileMode) os.FileMode {
	if mc.octal {
		return mc.mode
	}
	return applySymbolicClauses(mc.clauses, currentMode)
}

// applySymbolicClauses applies all symbolic clauses to the current mode.
func applySymbolicClauses(clauses []clause, mode os.FileMode) os.FileMode {
	for _, c := range clauses {
		mode = c.apply(mode)
	}
	return mode
}

// apply applies a single symbolic clause to the current mode.
func (c clause) apply(mode os.FileMode) os.FileMode {
	bits := c.computeBits(mode)
	switch c.op {
	case '+':
		return mode | bits
	case '-':
		return mode &^ bits
	case '=':
		return (mode &^ c.whoMask()) | bits
	}
	return mode
}

// computeBits returns the permission bits for this clause.
func (c clause) computeBits(currentMode os.FileMode) os.FileMode {
	who := normalizeWho(c.who)
	isDir := currentMode.IsDir()
	hasExec := currentMode&0o111 != 0
	var bits os.FileMode
	for _, p := range c.perms {
		bits |= permBitForChar(p, who, isDir, hasExec)
	}
	return bits
}

// normalizeWho expands empty or "a"-containing who to "ugo".
func normalizeWho(who string) string {
	if who == "" || strings.Contains(who, "a") {
		return "ugo"
	}
	return who
}

// permBitForChar returns the mode bits for a single permission character.
// R3.1: supports setuid (u+s), setgid (g+s), and sticky bit (o+t).
func permBitForChar(p rune, who string, isDir, hasExec bool) os.FileMode {
	switch p {
	case 'r':
		return classBits(who, 4)
	case 'w':
		return classBits(who, 2)
	case 'x':
		return classBits(who, 1)
	case 'X':
		if isDir || hasExec {
			return classBits(who, 1)
		}
		return 0
	case 's':
		return suidBits(who)
	case 't':
		return os.ModeSticky
	}
	return 0
}

// classBits maps a base permission bit to the correct positions for who classes.
func classBits(who string, baseBit os.FileMode) os.FileMode {
	var bits os.FileMode
	if strings.ContainsRune(who, 'u') {
		bits |= baseBit << 6
	}
	if strings.ContainsRune(who, 'g') {
		bits |= baseBit << 3
	}
	if strings.ContainsRune(who, 'o') {
		bits |= baseBit
	}
	return bits
}

// suidBits returns setuid/setgid bits based on who.
func suidBits(who string) os.FileMode {
	var bits os.FileMode
	if strings.ContainsRune(who, 'u') {
		bits |= os.ModeSetuid
	}
	if strings.ContainsRune(who, 'g') {
		bits |= os.ModeSetgid
	}
	return bits
}

// whoMask returns the mask covering all bits affected by this clause's who.
func (c clause) whoMask() os.FileMode {
	who := normalizeWho(c.who)
	mask := classBits(who, 7)
	mask |= suidBits(who)
	if strings.ContainsRune(who, 'o') {
		mask |= os.ModeSticky
	}
	return mask
}

// parseMode parses a mode string (octal or symbolic).
// R1.1: octal mode parsing.
// R1.2: symbolic mode parsing.
func parseMode(modeStr string) (*modeChange, error) {
	if isOctalMode(modeStr) {
		return parseOctalMode(modeStr)
	}
	clauses, err := parseSymbolicClauses(modeStr)
	if err != nil {
		return nil, err
	}
	return &modeChange{clauses: clauses}, nil
}

// isOctalMode returns true if the mode string consists only of octal digits.
func isOctalMode(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

// parseOctalMode parses an octal mode string into a modeChange.
// R1.1: parse octal mode strings (e.g., 755, 0644).
func parseOctalMode(s string) (*modeChange, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %q", s)
	}
	return &modeChange{octal: true, mode: unixToFileMode(val)}, nil
}

// unixToFileMode converts a Unix mode_t value to os.FileMode.
func unixToFileMode(m uint64) os.FileMode {
	mode := os.FileMode(m & 0o777)
	if m&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

// parseSymbolicClauses parses a comma-separated symbolic mode string.
// R1.2: symbolic mode with comma-separated clauses.
func parseSymbolicClauses(modeStr string) ([]clause, error) {
	parts := strings.Split(modeStr, ",")
	clauses := make([]clause, 0, len(parts))
	for _, part := range parts {
		c, err := parseOneClause(part)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, c)
	}
	return clauses, nil
}

// parseOneClause parses a single symbolic clause like "u+rwx".
func parseOneClause(s string) (clause, error) {
	i := 0
	for i < len(s) && strings.ContainsRune("ugoa", rune(s[i])) {
		i++
	}
	who := s[:i]
	if i >= len(s) || !strings.ContainsRune("+-=", rune(s[i])) {
		return clause{}, fmt.Errorf("invalid mode: %q", s)
	}
	op := s[i]
	i++
	perms := s[i:]
	for _, c := range perms {
		if !strings.ContainsRune("rwxXst", c) {
			return clause{}, fmt.Errorf("invalid mode: %q", s)
		}
	}
	return clause{who: who, op: op, perms: perms}, nil
}

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// printError prints a formatted error to stderr, unless silent mode suppresses it.
// R2.4: -f/--silent/--quiet suppresses most error messages.
func printError(stderr *os.File, silent bool, msg string) {
	if silent {
		return
	}
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "Try ... --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}
