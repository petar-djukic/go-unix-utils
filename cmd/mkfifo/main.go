// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mkfifo implements GNU mkfifo: create named pipes (FIFOs).
//
// Implements prd092-mkfifo R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mkfifo"

type options struct {
	mode  string // octal mode string; empty means default 0666
	names []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.names = append(opts.names, args[i+1:]...)
			return opts, nil
		}
		needsNext, err := parseArg(arg, &opts)
		if err != nil {
			return opts, err
		}
		if needsNext {
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("option requires an argument -- 'm'")
			}
			opts.mode = args[i]
		}
	}
	return opts, nil
}

// parseArg processes a single argument. Returns true if the next
// argument should be consumed as a mode value.
func parseArg(arg string, opts *options) (bool, error) {
	switch {
	case arg == "--mode":
		return true, nil
	case strings.HasPrefix(arg, "--mode="):
		opts.mode = arg[len("--mode="):]
	case strings.HasPrefix(arg, "--"):
		return false, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], opts)
	default:
		opts.names = append(opts.names, arg)
	}
	return false, nil
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

// parseMode converts an octal mode string to a syscall mode value.
// R1.3: default is 0666.
func parseMode(s string) (uint32, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode: %q", s)
	}
	return uint32(val), nil
}

// run creates FIFOs specified as positional arguments.
// R1.2: processes each name independently.
// R1.4: reports errors per name without aborting remaining arguments.
func run(args []string, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)                          //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}
	if len(opts.names) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)                   //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}
	mode, err := resolveMode(opts.mode)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	exitCode := 0
	for _, name := range opts.names {
		if err := createFIFO(name, mode); err != nil {
			reportError(stderr, name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveMode returns the mode to use for FIFO creation.
// R1.3: default 0666; explicit -m overrides.
func resolveMode(modeStr string) (uint32, error) {
	if modeStr == "" {
		return 0o666, nil
	}
	return parseMode(modeStr)
}

// createFIFO creates a single FIFO at the given path.
// R1.1: uses mkfifo(2) system call.
func createFIFO(path string, mode uint32) error {
	return syscall.Mkfifo(path, mode)
}

// reportError writes a mkfifo error to stderr in GNU format.
func reportError(stderr *os.File, name string, err error) {
	fmt.Fprintf(stderr, "%s: cannot create fifo '%s': %s\n", progName, name, errMessage(err)) //nolint:errcheck
}

// errMessage extracts the inner error message from a syscall error.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
