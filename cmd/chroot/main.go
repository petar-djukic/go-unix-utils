// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/chroot: run a command with a different root directory.
// Implements srd100-chroot R1.1, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chroot"

const (
	exitInternal = 125
	exitNotExec  = 126
	exitNotFound = 127
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes chroot.
// Returns the exit code for the process.
func run(args []string) int {
	newRoot, command, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return exitInternal
	}
	return executeChroot(newRoot, command)
}

// parseArgs extracts NEWROOT and optional COMMAND [ARG]... from args.
func parseArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing operand")
	}
	args = handleLeadingFlags(args)
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing operand")
	}
	return args[0], args[1:], nil
}

// handleLeadingFlags processes --help, --version, --, and rejects
// unknown options before the NEWROOT positional argument.
func handleLeadingFlags(args []string) []string {
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "--":
			return args[1:]
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--version":
			fmt.Println("chroot (go-unix-utils)")
			os.Exit(0)
		case strings.HasPrefix(arg, "-") && arg != "-":
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n",
				progName, arg)
			os.Exit(exitInternal)
		default:
			return args
		}
	}
	return args
}

// printUsage writes usage information to stdout.
func printUsage() {
	fmt.Printf("Usage: %s [OPTION] NEWROOT [COMMAND [ARG]...]\n",
		progName)
	fmt.Println("Run COMMAND with root directory set to NEWROOT.")
	fmt.Println()
	fmt.Println("  --help     display this help and exit")
	fmt.Println("  --version  output version information and exit")
}

// executeChroot performs the chroot(2) syscall and executes the command.
// R1.1: chroot to NEWROOT then exec COMMAND or default shell.
func executeChroot(newRoot string, command []string) int {
	if err := syscall.Chroot(newRoot); err != nil {
		fmt.Fprintf(os.Stderr,
			"%s: cannot change root directory to '%s': %v\n",
			progName, newRoot, err)
		return exitInternal
	}
	if err := syscall.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr,
			"%s: cannot chdir to '/': %v\n", progName, err)
		return exitInternal
	}
	if len(command) == 0 {
		command = defaultCommand()
	}
	return execCommand(command)
}

// defaultCommand returns the default shell command.
// R1.1: use $SHELL -i, falling back to /bin/sh -i.
func defaultCommand() []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return []string{shell, "-i"}
}

// execCommand resolves the command via PATH lookup and replaces
// the process. R1.1, R2.1, R2.2.
func execCommand(command []string) int {
	binary, err := exec.LookPath(command[0])
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"%s: failed to run command '%s': %v\n",
			progName, command[0], err)
		return exitNotFound
	}
	err = syscall.Exec(binary, command, os.Environ())
	// syscall.Exec only returns on error
	fmt.Fprintf(os.Stderr,
		"%s: failed to run command '%s': %v\n",
		progName, command[0], err)
	return exitNotExec
}
