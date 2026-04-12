// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mknod: make block or character special files.
// Implements srd093 R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "mknod"

// usageText is the --help output printed to stdout.
const usageText = `Usage: mknod [OPTION]... NAME TYPE [MAJOR MINOR]
Create the special file NAME of the given TYPE.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file permission bits to MODE, not a=rw - umask
      --help        display this help and exit
      --version     output version information and exit

Both MAJOR and MINOR must be specified when TYPE is b, c, or u.
When TYPE is p, a FIFO is created, and MAJOR and MINOR must not be specified.

TYPE may be:
  b      create a block (buffered) special file
  c, u   create a character (unbuffered) special file
  p      create a FIFO
`

// versionText is the --version output printed to stdout.
const versionText = "mknod (go-unix-utils) 0.1.0\n"

// R1.2: Default modes before umask.
const (
	defaultFIFOMode   = os.FileMode(0o666)
	defaultDeviceMode = os.FileMode(0o660)
)

// nodeType represents the type of special file to create.
type nodeType int

const (
	nodeBlock nodeType = iota
	nodeChar
	nodeFIFO
)

// config holds parsed command-line options for mknod.
type config struct {
	mode    string // -m, --mode=MODE
	help    bool   // --help
	version bool   // --version
	args    []string
}

// parsedArgs holds validated positional arguments.
type parsedArgs struct {
	name     string
	ntype    nodeType
	major    uint64
	minor    uint64
	hasMajor bool
}

// R2.3: main entry with SIGPIPE handler and flag parsing.
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

// run executes the mknod logic and returns the exit code.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	parsed, err := validatePositional(cfg.args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}

	if cfg.mode != "" {
		if _, err := parseMode(cfg.mode, parsed.ntype); err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid mode '%s'\n", programName, cfg.mode)
			return 1
		}
	}

	if err := createNode(cfg, parsed); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	return 0
}

// validatePositional validates NAME TYPE [MAJOR MINOR] arguments.
// R1.1: TYPE is b (block), c/u (character), or p (FIFO).
// R1.3: validates argument counts per type.
func validatePositional(args []string) (parsedArgs, error) {
	if len(args) == 0 {
		return parsedArgs{}, fmt.Errorf(
			"missing operand\nTry '%s --help' for more information.", programName)
	}
	if len(args) == 1 {
		return parsedArgs{}, fmt.Errorf(
			"missing operand after '%s'\nTry '%s --help' for more information.",
			args[0], programName)
	}

	name := args[0]
	ntype, err := parseNodeType(args[1])
	if err != nil {
		return parsedArgs{}, err
	}

	if ntype == nodeFIFO {
		return validateFIFOArgs(name, args)
	}
	return validateDeviceArgs(name, ntype, args)
}

// parseNodeType converts a type string to a nodeType constant.
func parseNodeType(s string) (nodeType, error) {
	switch s {
	case "b":
		return nodeBlock, nil
	case "c", "u":
		return nodeChar, nil
	case "p":
		return nodeFIFO, nil
	default:
		return 0, fmt.Errorf("invalid device type '%s'", s)
	}
}

// validateFIFOArgs checks that FIFO type has no device numbers.
func validateFIFOArgs(name string, args []string) (parsedArgs, error) {
	if len(args) > 2 {
		return parsedArgs{}, fmt.Errorf(
			"Fifos do not have major and minor device numbers.")
	}
	return parsedArgs{name: name, ntype: nodeFIFO}, nil
}

// validateDeviceArgs checks that block/char types have MAJOR MINOR.
func validateDeviceArgs(name string, ntype nodeType, args []string) (parsedArgs, error) {
	if len(args) < 4 {
		return parsedArgs{}, fmt.Errorf(
			"missing operand after '%s'\nTry '%s --help' for more information.",
			args[len(args)-1], programName)
	}
	if len(args) > 4 {
		return parsedArgs{}, fmt.Errorf(
			"extra operand '%s'\nTry '%s --help' for more information.",
			args[4], programName)
	}

	major, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return parsedArgs{}, fmt.Errorf("invalid major device number '%s'", args[2])
	}
	minor, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return parsedArgs{}, fmt.Errorf("invalid minor device number '%s'", args[3])
	}

	return parsedArgs{
		name:     name,
		ntype:    ntype,
		major:    major,
		minor:    minor,
		hasMajor: true,
	}, nil
}

// createNode creates the special file described by parsed arguments.
// R1.1: uses syscall.Mkfifo for FIFOs, unix.Mknod for device files.
func createNode(cfg config, p parsedArgs) error {
	mode := defaultModeForType(p.ntype)
	if cfg.mode != "" {
		parsed, err := parseMode(cfg.mode, p.ntype)
		if err != nil {
			return fmt.Errorf("invalid mode '%s'", cfg.mode)
		}
		mode = parsed
	}

	if p.ntype == nodeFIFO {
		return createFIFO(p.name, mode)
	}
	return createDevice(p, mode)
}

// createFIFO creates a FIFO (named pipe) at the given path.
func createFIFO(path string, mode os.FileMode) error {
	if err := syscall.Mkfifo(path, uint32(mode)); err != nil {
		return fmt.Errorf("cannot create fifo '%s': %v", path, err)
	}
	return nil
}

// createDevice creates a block or character special file.
func createDevice(p parsedArgs, mode os.FileMode) error {
	var sysMode uint32
	switch p.ntype {
	case nodeBlock:
		sysMode = uint32(mode) | syscall.S_IFBLK
	case nodeChar:
		sysMode = uint32(mode) | syscall.S_IFCHR
	}
	dev := unix.Mkdev(uint32(p.major), uint32(p.minor))
	if err := unix.Mknod(p.name, sysMode, int(dev)); err != nil {
		return fmt.Errorf("%s: %v", p.name, err)
	}
	return nil
}

// defaultModeForType returns the default permission mode for a node type.
// R1.2: 0666 for FIFOs, 0660 for device files.
func defaultModeForType(nt nodeType) os.FileMode {
	if nt == nodeFIFO {
		return defaultFIFOMode
	}
	return defaultDeviceMode
}

// parseMode parses a mode string as either octal or symbolic.
// R1.2: supports both octal (0644) and symbolic (a=rw) forms.
func parseMode(mode string, nt nodeType) (os.FileMode, error) {
	if len(mode) > 0 && mode[0] >= '0' && mode[0] <= '9' {
		return parseOctalMode(mode)
	}
	return parseSymbolicMode(mode, defaultModeForType(nt))
}

// parseOctalMode parses an octal permission string like "0644" or "644".
func parseOctalMode(mode string) (os.FileMode, error) {
	val, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode '%s'", mode)
	}
	return os.FileMode(val), nil
}

// parseSymbolicMode parses a symbolic mode string like "a=rw"
// and applies it to the base mode.
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

// applySymbolicClause applies one clause like "a=rw" or "go-w".
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
				who = 0o777
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

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.args = append(cfg.args, arg)
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

	if strings.HasPrefix(arg, "--mode=") {
		cfg.mode = arg[len("--mode="):]
		return 0, nil
	}

	switch arg {
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		cfg.mode = args[idx+1]
		return 1, nil
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseShortFlags processes bundled short flags like -m0644.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i, ch := range flags {
		switch ch {
		case 'm':
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
