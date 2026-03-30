// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mknod implements GNU mknod: create block or character special files.
//
// Implements prd093-mknod R1.1, R1.2, R1.3.
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

const progName = "mknod"

// nodeType identifies the kind of special file to create.
type nodeType int

const (
	nodeBlock nodeType = iota
	nodeChar
	nodeFIFO
)

type options struct {
	mode  string   // octal mode string; empty means use default
	name  string   // file name to create
	ntype nodeType // type of special file
	major uint32   // major device number (block/char only)
	minor uint32   // minor device number (block/char only)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses arguments and creates the special file.
func run(args []string, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	mode, err := resolveMode(opts.mode, opts.ntype)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	dev := int(unix.Mkdev(opts.major, opts.minor))
	if err := syscall.Mknod(opts.name, mode, dev); err != nil {
		fmt.Fprintf(stderr, "%s: %s: %s\n", progName, opts.name, errMessage(err)) //nolint:errcheck
		return 1
	}
	return 0
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	positional, err := parseFlags(args, &opts)
	if err != nil {
		return opts, err
	}
	return opts, parsePositional(positional, &opts)
}

// parseFlags processes flag arguments and returns remaining positional args.
func parseFlags(args []string, opts *options) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return positional, nil
		}
		needsNext, isFlag, err := parseFlagArg(arg, opts)
		if err != nil {
			return nil, err
		}
		if !isFlag {
			positional = append(positional, arg)
			continue
		}
		if needsNext {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("option requires an argument -- 'm'")
			}
			opts.mode = args[i]
		}
	}
	return positional, nil
}

// parseFlagArg processes a single argument.
// Returns (needsNext, isFlag, error).
func parseFlagArg(arg string, opts *options) (bool, bool, error) {
	switch {
	case arg == "--mode":
		return true, true, nil
	case strings.HasPrefix(arg, "--mode="):
		opts.mode = arg[len("--mode="):]
		return false, true, nil
	case strings.HasPrefix(arg, "--"):
		return false, false, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		needsNext, err := parseShortFlags(arg[1:], opts)
		return needsNext, true, err
	default:
		return false, false, nil
	}
}

// parseShortFlags processes bundled short flags like -m0600.
// Returns true if the next argument should be consumed as a mode value.
func parseShortFlags(flags string, opts *options) (bool, error) {
	for i, ch := range flags {
		switch ch {
		case 'm':
			rest := flags[i+1:]
			if rest != "" {
				opts.mode = rest
				return false, nil
			}
			return true, nil
		default:
			return false, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return false, nil
}

// parsePositional validates NAME TYPE [MAJOR MINOR] positional arguments.
// R1.1: accepts NAME TYPE [MAJOR MINOR].
func parsePositional(args []string, opts *options) error {
	if len(args) == 0 {
		return fmt.Errorf("missing operand")
	}
	if len(args) == 1 {
		return fmt.Errorf("missing operand after '%s'", args[0])
	}
	opts.name = args[0]
	ntype, err := parseDeviceType(args[1])
	if err != nil {
		return err
	}
	opts.ntype = ntype
	return validateDeviceArgs(args, opts)
}

// validateDeviceArgs checks MAJOR MINOR presence based on device type.
// R1.3: FIFO must not have device numbers; block/char must have them.
func validateDeviceArgs(args []string, opts *options) error {
	if opts.ntype == nodeFIFO {
		if len(args) > 2 {
			return fmt.Errorf("%s: FIFO special files cannot have major and minor device numbers", opts.name)
		}
		return nil
	}
	if len(args) < 4 {
		return fmt.Errorf("missing operand after '%s'", args[len(args)-1])
	}
	if len(args) > 4 {
		return fmt.Errorf("extra operand '%s'", args[4])
	}
	major, err := parseDeviceNumber(args[2], "major")
	if err != nil {
		return err
	}
	minor, err := parseDeviceNumber(args[3], "minor")
	if err != nil {
		return err
	}
	opts.major = major
	opts.minor = minor
	return nil
}

// parseDeviceType converts a type string to nodeType.
// R1.1: b=block, c/u=character, p=FIFO.
func parseDeviceType(s string) (nodeType, error) {
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

// parseDeviceNumber parses a decimal device number string.
// R1.3: non-numeric device numbers are rejected.
func parseDeviceNumber(s, which string) (uint32, error) {
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s device number '%s'", which, s)
	}
	return uint32(val), nil
}

// resolveMode returns the full mode (type bits | permission bits).
// R1.2: default 0666 for FIFOs, 0660 for device files.
func resolveMode(modeStr string, ntype nodeType) (uint32, error) {
	perm := defaultPerm(ntype)
	if modeStr != "" {
		var err error
		perm, err = parseMode(modeStr)
		if err != nil {
			return 0, err
		}
	}
	return typeModeFlag(ntype) | perm, nil
}

// defaultPerm returns the default permission bits for the node type.
func defaultPerm(ntype nodeType) uint32 {
	if ntype == nodeFIFO {
		return 0o666
	}
	return 0o660
}

// parseMode converts an octal mode string to permission bits.
func parseMode(s string) (uint32, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode: %q", s)
	}
	return uint32(val), nil
}

// typeModeFlag returns the file type bits for syscall.Mknod.
func typeModeFlag(ntype nodeType) uint32 {
	switch ntype {
	case nodeBlock:
		return syscall.S_IFBLK
	case nodeChar:
		return syscall.S_IFCHR
	default:
		return syscall.S_IFIFO
	}
}

// errMessage extracts the inner error message from an error.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
