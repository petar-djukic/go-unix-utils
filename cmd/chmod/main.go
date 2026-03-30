// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chmod implements GNU chmod: change file mode bits.
//
// Implements prd089-chmod R1.1, R1.2, R1.3, R1.4.
package main

import (
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
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "chmod (go-unix-utils) 0.1\n"

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
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes the chmod logic and returns the exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: exits 1 on any error, continues processing remaining files.
func run(args []string, stderr *os.File) int {
	recursive, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	if len(operands) == 0 {
		printError(stderr, "missing operand")
		return 1
	}
	if len(operands) == 1 {
		printError(stderr, fmt.Sprintf("missing operand after '%s'", operands[0]))
		return 1
	}
	mc, err := parseMode(operands[0])
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	return applyToFiles(operands[1:], mc, recursive, stderr)
}

// applyToFiles applies the mode change to each file and returns the exit code.
func applyToFiles(files []string, mc *modeChange, recursive bool, stderr *os.File) int {
	exitCode := 0
	for _, file := range files {
		if err := chmodPath(file, mc, recursive); err != nil {
			printError(stderr, err.Error())
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (bool, []string, error) {
	recursive := false
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
		recursive, err = handleFlag(arg, recursive)
		if err != nil {
			return false, nil, err
		}
	}
	return recursive, operands, nil
}

// handleFlag processes a single flag argument.
func handleFlag(arg string, recursive bool) (bool, error) {
	switch arg {
	case "--recursive":
		return true, nil
	case "--help":
		fmt.Fprint(os.Stdout, helpText) //nolint:errcheck
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText) //nolint:errcheck
		os.Exit(0)
	default:
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			return parseShortFlags(arg[1:], recursive)
		}
		return recursive, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return recursive, nil
}

// parseShortFlags processes combined short flags like -R.
func parseShortFlags(flags string, recursive bool) (bool, error) {
	for _, c := range flags {
		switch c {
		case 'R':
			recursive = true
		default:
			return recursive, fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return recursive, nil
}

// chmodPath applies the mode change to path, optionally recursing.
// R2.1: -R changes modes recursively for directories and their contents.
func chmodPath(path string, mc *modeChange, recursive bool) error {
	if !recursive {
		return chmodFile(path, mc)
	}
	return filepath.WalkDir(path, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("cannot access '%s': %w", p, err)
		}
		return chmodFile(p, mc)
	})
}

// chmodFile applies the mode change to a single file.
func chmodFile(path string, mc *modeChange) error {
	newMode, err := resolveMode(path, mc)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("changing permissions of '%s': %w", path, err)
	}
	return nil
}

// resolveMode computes the new mode for a file.
// R1.1: octal modes apply directly.
// R1.2: symbolic modes apply relative to the current file mode.
func resolveMode(path string, mc *modeChange) (os.FileMode, error) {
	if mc.octal {
		return mc.mode, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("cannot access '%s': %w", path, err)
	}
	return applySymbolicClauses(mc.clauses, info.Mode()), nil
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

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}
