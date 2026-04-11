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
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "chmod"

// modeAction represents the operator in a symbolic mode clause.
type modeAction int

const (
	modeSet    modeAction = iota // '='
	modeAdd                      // '+'
	modeRemove                   // '-'
)

// modeClause represents a single parsed symbolic mode clause (e.g., u+rwx).
type modeClause struct {
	who    string     // subset of "ugoa"
	action modeAction // +, -, =
	perms  string     // subset of "rwxXst"
}

// mode represents a parsed mode specification, either octal or symbolic.
type mode struct {
	octal    uint32       // R1.1: octal mode value (valid when isOctal is true)
	symbolic []modeClause // R1.2: parsed symbolic clauses
	isOctal  bool         // true when the mode was specified as an octal number
}

// options holds parsed command-line flags for chmod.
type options struct {
	recursive bool // R2.1: -R/--recursive
	verbose   bool // R2.2: -v/--verbose
	changes   bool // R2.3: -c/--changes
	silent    bool // R2.4: -f/--silent/--quiet
	reference string // R3.2: --reference=RFILE
}

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
// Supports short flags (-R, -v, -c, -f), combined short flags,
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
		case 'R', 'v', 'c', 'f':
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
			if !opts.silent {
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
	// TODO: implement reference file mode reading using sys.Stat
	fi, err := sys.Stat(rfile)
	if err != nil {
		return mode{}, fmt.Errorf("cannot stat %q: %w", rfile, err)
	}
	return mode{
		octal:   uint32(fi.Mode.Perm()) | uint32(fi.Mode&os.ModeSetuid) | uint32(fi.Mode&os.ModeSetgid) | uint32(fi.Mode&os.ModeSticky),
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
// R1.1: sets the file's permission bits to the octal value.
func parseOctalMode(spec string) (mode, error) {
	var val uint32
	for _, c := range spec {
		val = val*8 + uint32(c-'0')
	}
	return mode{octal: val, isOctal: true}, nil
}

// parseSymbolicMode parses a symbolic mode string into a mode struct.
// R1.2: supports [ugoa][+-=][rwxXst] with comma-separated clauses.
// R3.1: supports setuid (u+s), setgid (g+s), and sticky bit (o+t).
func parseSymbolicMode(spec string) (mode, error) {
	// TODO: implement full symbolic mode parsing
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
func parseSingleClause(clause string) (modeClause, error) {
	// TODO: implement single clause parsing
	if len(clause) == 0 {
		return modeClause{}, fmt.Errorf("invalid mode: empty clause")
	}

	var mc modeClause
	i := 0

	// Parse who characters
	for i < len(clause) {
		c := clause[i]
		if c == 'u' || c == 'g' || c == 'o' || c == 'a' {
			mc.who += string(c)
			i++
		} else {
			break
		}
	}

	// Default to 'a' if no who specified
	if mc.who == "" {
		mc.who = "a"
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
// R2.2: prints diagnostic when verbose.
// R2.3: prints diagnostic only when mode changed.
func applyMode(opts options, m mode, path string) error {
	// TODO: implement mode application
	if opts.recursive {
		return applyModeRecursive(opts, m, path)
	}
	return applyModeToFile(opts, m, path)
}

// applyModeRecursive recursively applies mode changes to a directory tree.
// R2.1: -R/--recursive changes modes for directories and their contents.
func applyModeRecursive(opts options, m mode, path string) error {
	// TODO: implement recursive traversal
	return fmt.Errorf("not implemented")
}

// applyModeToFile applies the mode change to a single file.
func applyModeToFile(opts options, m mode, path string) error {
	// TODO: implement single file mode change
	return fmt.Errorf("not implemented")
}

// computeNewMode calculates the new permission bits for a file.
// R1.1: for octal modes, returns the octal value directly.
// R1.2: for symbolic modes, applies each clause to the current mode.
// R3.1: handles setuid, setgid, and sticky bit.
func computeNewMode(m mode, current os.FileMode) os.FileMode {
	// TODO: implement mode computation
	if m.isOctal {
		return os.FileMode(m.octal)
	}
	return current
}

// printDiagnostic prints a verbose or changes-only diagnostic message.
// R2.2: format matches GNU chmod verbose output.
// R2.3: only prints when mode actually changed (changes mode).
func printDiagnostic(opts options, path string, oldMode, newMode os.FileMode) {
	// TODO: implement diagnostic output
	if opts.changes && oldMode == newMode {
		return
	}
	if opts.verbose || opts.changes {
		fmt.Fprintf(os.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, uint32(oldMode.Perm()), oldMode.String(),
			uint32(newMode.Perm()), newMode.String())
	}
}
