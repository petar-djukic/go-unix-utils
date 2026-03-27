// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chronic implements prd112-chronic: run a command quietly unless it fails.
// Requirements implemented: R1.1 (command execution and output suppression),
// R1.2 (--stderr flag), R1.3 (--verbose flag).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	// exitNotFound is returned when the command cannot be found or executed.
	exitNotFound = 100
	progName     = "chronic"
)

// run parses flags, executes the command, and returns the exit code.
func run(args []string) int {
	stderrFlag, verbose, cmdArgs := parseFlags(args)
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "%s: usage: %s [-e] [-v] COMMAND [ARGS...]\n", progName, progName)
		return 1
	}
	if verbose {
		// R1.3: print the command being run to stderr before execution.
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(cmdArgs, " "))
	}
	return executeCommand(cmdArgs, stderrFlag)
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
	exitCode := exitCodeFromError(err)
	failed := exitCode != 0
	// R1.2: with -e, also treat stderr output as a failure trigger.
	if stderrFlag && stderrBuf.Len() > 0 {
		failed = true
	}
	if failed {
		// R1.1/R1.3: on failure, write captured output to respective streams.
		os.Stdout.Write(stdoutBuf.Bytes()) //nolint:errcheck // best-effort output
		os.Stderr.Write(stderrBuf.Bytes()) //nolint:errcheck // best-effort output
	}
	return exitCode
}

// exitCodeFromError extracts the exit code from an exec error.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	// R2.2: command not found or not executable.
	return exitNotFound
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}
