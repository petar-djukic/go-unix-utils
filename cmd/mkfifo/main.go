// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd092-mkfifo: Make FIFOs (Named Pipes).
// Covers R1.1-R1.4 (FIFO creation, mode setting, error handling),
// R2.1-R2.3 (exit codes, SIGPIPE handling).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, names, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "mkfifo: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'mkfifo --help' for more information.")
		os.Exit(1)
	}

	os.Exit(run(cfg, names))
}

// config holds parsed flag state.
type config struct {
	mode    os.FileMode
	modeSet bool
}

// run creates FIFOs and returns the exit code.
// R2.1: exits 0 on success, R2.2: exits 1 if any creation fails.
func run(cfg config, names []string) int {
	mode := os.FileMode(0o666)
	if cfg.modeSet {
		mode = cfg.mode
	}
	exitCode := 0
	for _, name := range names {
		if err := syscall.Mkfifo(name, uint32(mode)); err != nil {
			printFifoError(name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// printFifoError formats an error in GNU style:
// mkfifo: cannot create fifo 'NAME': Reason
func printFifoError(name string, err error) {
	reason := capitalizeFirst(err.Error())
	fmt.Fprintf(os.Stderr, "mkfifo: cannot create fifo '%s': %s\n",
		name, reason)
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (cfg config, names []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			names = append(names, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case arg == "-m":
			// R1.3: -m MODE sets permission bits.
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr,
					"mkfifo: option requires an argument -- 'm'")
				return config{}, nil, 1
			}
			mode, err := parseMode(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"mkfifo: invalid mode '%s'\n", args[i])
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "--mode="):
			val := strings.TrimPrefix(arg, "--mode=")
			mode, err := parseMode(val)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"mkfifo: invalid mode '%s'\n", val)
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "-m"):
			// Combined -mMODE form.
			val := arg[2:]
			mode, err := parseMode(val)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"mkfifo: invalid mode '%s'\n", val)
				return config{}, nil, 1
			}
			cfg.mode = mode
			cfg.modeSet = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr,
				"mkfifo: unrecognized option '%s'\n", arg)
			return config{}, nil, 1
		default:
			names = append(names, args[i:]...)
			return
		}
	}
	return
}

// parseMode parses an octal permission string.
// R1.3: MODE is an octal value like "0666" or "600".
func parseMode(s string) (os.FileMode, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(val), nil
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mkfifo [OPTION]... NAME...
Create named pipes (FIFOs) with the given NAMEs.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file permission bits to MODE, not a=rw - umask

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"mkfifo (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
