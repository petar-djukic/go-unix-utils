// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mkdir: create directories.
// Implements srd034 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "mkdir"

// usageText is the --help output printed to stdout.
const usageText = `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed
  -v, --verbose     print a message for each created directory
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "mkdir (go-unix-utils) 0.1.0\n"

// defaultDirMode is the base mode for new directories before umask.
// R3.2: 0777 modified by umask when -m is not given.
const defaultDirMode = os.FileMode(0o777)

// TODO: Task R3 requests -Z/--context flag (SELinux context), but srd034
// non_goals explicitly excludes it: "cmd/mkdir does not implement SELinux
// context options (-Z, --context)." Per constitution E6, skipping.

// config holds parsed command-line options for mkdir.
type config struct {
	parents bool   // -p, --parents
	mode    string // -m, --mode=MODE
	verbose bool   // -v, --verbose
	help    bool   // --help
	version bool   // --version
	dirs    []string
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

// run executes the mkdir logic and returns the exit code.
// R1.2: processes each directory argument independently.
// R1.3, R1.4: prints error and continues on failure.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	if len(cfg.dirs) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}

	// R3.1: validate mode upfront before creating any directories.
	if cfg.mode != "" {
		if _, err := parseMode(cfg.mode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid mode '%s'\n", programName, cfg.mode)
			return 1
		}
	}

	exitCode := 0
	for _, dir := range cfg.dirs {
		if err := createDir(cfg, dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// createDir creates a single directory per the current config.
// R1.1: creates directory with os.Mkdir using default permissions.
// R1.3: returns error when directory already exists.
// R1.4: returns error when parent does not exist.
func createDir(cfg config, dir string) error {
	if cfg.parents {
		return createWithParents(cfg, dir)
	}
	if err := os.Mkdir(dir, defaultDirMode); err != nil {
		return formatMkdirError(dir, err)
	}
	if err := applyModeIfSet(cfg, dir); err != nil {
		return err
	}
	printVerbose(cfg, dir)
	return nil
}

// createWithParents creates a directory and its parents using -p semantics.
// R2.1: creates intermediate parent directories as needed.
// R2.2: no error when the target directory already exists.
// R2.3: no error when intermediate directories already exist.
// R3.3: applies -m mode only to the final target directory.
// R3.4: prints verbose message for each directory actually created.
func createWithParents(cfg config, dir string) error {
	toCreate := collectMissingDirs(dir)
	for _, d := range toCreate {
		err := os.Mkdir(d, defaultDirMode)
		if err != nil && !os.IsExist(err) {
			return formatMkdirError(dir, err)
		}
		if err == nil {
			printVerbose(cfg, d)
		}
	}
	// R3.3: apply mode only to the leaf, only if newly created.
	if len(toCreate) > 0 {
		return applyModeIfSet(cfg, dir)
	}
	return nil
}

// collectMissingDirs returns the chain of directories that need to be
// created, ordered from shallowest to deepest.
func collectMissingDirs(path string) []string {
	var missing []string
	cur := filepath.Clean(path)
	for cur != "." && cur != "/" {
		if isDir(cur) {
			break
		}
		missing = append(missing, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	reverseStrings(missing)
	return missing
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// applyModeIfSet applies the -m mode to dir when cfg.mode is non-empty.
// R3.1: parses octal and symbolic modes, then applies via os.Chmod.
// R3.3: called only on the leaf directory, not intermediates.
func applyModeIfSet(cfg config, dir string) error {
	if cfg.mode == "" {
		return nil
	}
	perm, err := parseMode(cfg.mode)
	if err != nil {
		return fmt.Errorf("invalid mode '%s'", cfg.mode)
	}
	return os.Chmod(dir, perm)
}

// printVerbose prints a diagnostic message when verbose mode is enabled.
// R3.4: format matches GNU mkdir: "mkdir: created directory 'NAME'".
func printVerbose(cfg config, dir string) {
	if cfg.verbose {
		fmt.Fprintf(os.Stdout, "%s: created directory '%s'\n", programName, dir)
	}
}

// parseMode parses a mode string as either octal or symbolic.
// R3.1: supports both octal (0755, 755) and symbolic (u=rwx,go=rx) forms.
func parseMode(mode string) (os.FileMode, error) {
	if len(mode) > 0 && mode[0] >= '0' && mode[0] <= '9' {
		return parseOctalMode(mode)
	}
	return parseSymbolicMode(mode, defaultDirMode)
}

// parseOctalMode parses an octal permission string like "0755" or "755".
func parseOctalMode(mode string) (os.FileMode, error) {
	val, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode '%s'", mode)
	}
	return os.FileMode(val), nil
}

// parseSymbolicMode parses a symbolic mode string like "u=rwx,go=rx"
// and applies it to the base mode.
// R3.1: format is [ugoa...][+-=][rwxXst...] clauses separated by commas.
func parseSymbolicMode(mode string, base os.FileMode) (os.FileMode, error) {
	result := base
	for _, clause := range strings.Split(mode, ",") {
		var err error
		result, err = applySymbolicClause(clause, result)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

// applySymbolicClause applies one clause like "u=rwx" or "go-w".
func applySymbolicClause(clause string, perm os.FileMode) (os.FileMode, error) {
	if clause == "" {
		return 0, fmt.Errorf("invalid mode clause")
	}
	who, rest := parseWho(clause)
	if len(rest) == 0 {
		return 0, fmt.Errorf("invalid mode clause '%s'", clause)
	}
	return applyPermOps(who, rest, perm)
}

// parseWho extracts the ugoa prefix from a symbolic clause, returning
// a bitmask and the remaining string after the who characters.
func parseWho(clause string) (uint, string) {
	var who uint
	i := 0
	for i < len(clause) {
		switch clause[i] {
		case 'u':
			who |= 0o700
		case 'g':
			who |= 0o070
		case 'o':
			who |= 0o007
		case 'a':
			who |= 0o777
		default:
			if who == 0 {
				who = 0o777 // default to all when no who specified
			}
			return who, clause[i:]
		}
		i++
	}
	if who == 0 {
		who = 0o777
	}
	return who, clause[i:]
}

// applyPermOps processes operator+permissions pairs in a clause.
func applyPermOps(who uint, rest string, perm os.FileMode) (os.FileMode, error) {
	for len(rest) > 0 {
		if rest[0] != '+' && rest[0] != '-' && rest[0] != '=' {
			return 0, fmt.Errorf("invalid operator '%c'", rest[0])
		}
		op := rest[0]
		rest = rest[1:]
		var bits os.FileMode
		rest, bits = parsePermBits(rest)
		perm = applyOp(perm, op, who, bits)
	}
	return perm, nil
}

// parsePermBits reads rwxXst characters and returns remaining string and bits.
func parsePermBits(s string) (string, os.FileMode) {
	var bits os.FileMode
	i := 0
	for i < len(s) {
		switch s[i] {
		case 'r':
			bits |= 0o444
		case 'w':
			bits |= 0o222
		case 'x', 'X':
			// R3.1: X acts as x for directories (mkdir always creates dirs).
			bits |= 0o111
		case 's':
			bits |= os.ModeSetuid | os.ModeSetgid
		case 't':
			bits |= os.ModeSticky
		default:
			return s[i:], bits
		}
		i++
	}
	return s[i:], bits
}

// applyOp applies a single +, -, or = operation with who mask.
func applyOp(perm os.FileMode, op byte, who uint, bits os.FileMode) os.FileMode {
	// Mask standard rwx bits by who; special bits pass through.
	masked := (bits & os.FileMode(who)) |
		(bits & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
	switch op {
	case '+':
		perm |= masked
	case '-':
		perm &^= masked
	case '=':
		perm = (perm &^ os.FileMode(who)) | masked
	}
	return perm
}

// formatMkdirError wraps a mkdir error to match GNU mkdir output format.
func formatMkdirError(dir string, err error) error {
	return fmt.Errorf("cannot create directory '%s': %s", dir, unwrapOSError(err))
}

// unwrapOSError extracts the underlying message from an *os.PathError.
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.dirs = append(cfg.dirs, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		skip, err := parseFlag(&cfg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	return cfg, nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, args, idx)
	}
	return parseShortFlags(cfg, args, idx)
}

// parseLongFlag handles --name and --name=value flags.
func parseLongFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]

	// Handle --mode=VALUE form.
	if strings.HasPrefix(arg, "--mode=") {
		cfg.mode = arg[len("--mode="):]
		return 0, nil
	}

	switch arg {
	case "--parents":
		cfg.parents = true
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		cfg.mode = args[idx+1]
		return 1, nil
	case "--verbose":
		cfg.verbose = true
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseShortFlags processes bundled short flags like -pv.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i, ch := range flags {
		switch ch {
		case 'p':
			cfg.parents = true
		case 'v':
			cfg.verbose = true
		case 'm':
			// -m requires a value: rest of this arg or next arg.
			rest := flags[i+1:]
			if len(rest) > 0 {
				cfg.mode = rest
				return 0, nil
			}
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("option requires an argument -- 'm'")
			}
			cfg.mode = args[idx+1]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
