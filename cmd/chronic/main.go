// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chronic implements prd112-chronic: run a command quietly unless it fails.
// Requirements implemented: R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	// exitCmdNotFound is returned when COMMAND cannot be found (R2.1).
	exitCmdNotFound = 127
	// exitCmdNoExec is returned when COMMAND cannot be executed (R2.2).
	exitCmdNoExec = 126
	progName      = "chronic"
)

// run parses flags, executes the command, and returns the exit code.
func run(args []string) int {
	if code, handled := handleInfoFlags(args); handled {
		return code
	}
	stderrFlag, verbose, cmdArgs := parseFlags(args)
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "%s: usage: %s [-ev] COMMAND...\n", progName, progName)
		return 1
	}
	if verbose {
		// R1.3: print the command being run to stderr before execution.
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(cmdArgs, " "))
	}
	return executeCommand(cmdArgs, stderrFlag)
}

// handleInfoFlags checks for --help and --version flags before other parsing.
// R2.3: supports --help and --version per GNU convention.
func handleInfoFlags(args []string) (int, bool) {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "--help":
			printHelp()
			return 0, true
		case "--version":
			printVersion()
			return 0, true
		}
	}
	return 0, false
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print("Usage: chronic [-e] [-v] COMMAND [ARGS...]\n" +
		"Run a command quietly unless it fails.\n\n" +
		"  -e, --stderr     trigger output on stderr\n" +
		"  -v, --verbose    print command to stderr before running\n" +
		"      --help       display this help and exit\n" +
		"      --version    output version information and exit\n")
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Println("chronic (go-unix-utils)")
}

// parseFlags extracts -e/--stderr and -v/--verbose flags from the argument list.
// Returns (stderrFlag, verbose, remaining args).
func parseFlags(args []string) (bool, bool, []string) {
	stderrFlag := false
	verbose := false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-e", "--stderr":
			stderrFlag = true
		case "-v", "--verbose":
			verbose = true
		case "--":
			return stderrFlag, verbose, args[i+1:]
		default:
			return stderrFlag, verbose, args[i:]
		}
		i++
	}
	return stderrFlag, verbose, nil
}

// executeCommand runs the command and handles output suppression.
func executeCommand(cmdArgs []string, stderrFlag bool) int {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil && !isExitError(err) {
		// R2.1/R2.2: command could not be found or executed.
		return handleExecError(err, cmdArgs[0])
	}
	return reportOutput(err, stderrFlag, &stdoutBuf, &stderrBuf)
}

// isExitError reports whether err is an *exec.ExitError.
func isExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// handleExecError prints an error message and returns the appropriate exit code.
// R2.1: exits 127 when command is not found.
// R2.2: exits 126 when command cannot be executed.
func handleExecError(err error, cmdName string) int {
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "%s: %s: command not found\n", progName, cmdName)
		return exitCmdNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		fmt.Fprintf(os.Stderr, "%s: %s: Permission denied\n", progName, cmdName)
		return exitCmdNoExec
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, cmdName, err)
	return exitCmdNotFound
}

// reportOutput handles the output suppression logic and returns the exit code.
// R1.1: suppress output on success, display on failure.
// R1.2: with -e, also trigger on stderr output.
func reportOutput(err error, stderrFlag bool, stdoutBuf, stderrBuf *bytes.Buffer) int {
	exitCode := commandExitCode(err)
	failed := exitCode != 0
	if stderrFlag && stderrBuf.Len() > 0 {
		failed = true
	}
	if failed {
		os.Stdout.Write(stdoutBuf.Bytes()) //nolint:errcheck // best-effort output
		os.Stderr.Write(stderrBuf.Bytes()) //nolint:errcheck // best-effort output
	}
	return exitCode
}

// commandExitCode extracts the exit code from an exec error.
func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 0
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}
